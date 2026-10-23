package proxyfailover

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/clash"
)

var ErrTestBusy = errors.New("该账号正在检测，请等待完成后重试")
var ErrNoTestProxy = errors.New("暂无可用代理供检测，请稍后重试；本次未向 Adobe 提交请求")

// The lease is routing ownership, never permission to clear health or operator state.
type TestLease struct {
	AccountID int64
	Owner     string
	SourceID  string
}

func credentialHashSQL(alias string) string {
	return "encode(sha256(convert_to(jsonb_build_array(COALESCE(" + alias + "credentials->>'cookie',''),COALESCE(" + alias + "credentials->>'api_key',''))::text,'UTF8')),'hex')"
}

// Refresh408Metadata publishes derived state for account lists/details. The
// tables remain authoritative; existing extra fields and operator flags survive.
// id=0 refreshes all affected accounts during the regular maintenance round.
func Refresh408Metadata(ctx context.Context, tx *sql.Tx, id int64) error {
	_, err := tx.ExecContext(ctx, `WITH state AS (
 SELECT a.id,CASE WHEN EXISTS(SELECT 1 FROM adobe_proxy_408_blocks b WHERE b.account_id=a.id)
 OR EXISTS(SELECT 1 FROM adobe_proxy_408_holds h WHERE h.account_id=a.id)
 THEN jsonb_build_object('managed',EXISTS(SELECT 1 FROM proxy_auto_allocation WHERE enabled),'state',CASE WHEN EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=a.id AND l.expires_at>NOW())
 THEN 'testing' ELSE 'quarantined' END,'models',COALESCE((SELECT jsonb_agg(scope ORDER BY scope) FROM adobe_proxy_408_blocks b WHERE b.account_id=a.id),'[]'::jsonb))
 ELSE NULL::jsonb END AS value FROM accounts a WHERE a.deleted_at IS NULL AND a.platform='adobe' AND ($1::bigint=0 OR a.id=$1)
 AND (a.extra ? 'adobe_proxy_408' OR EXISTS(SELECT 1 FROM adobe_proxy_408_blocks b WHERE b.account_id=a.id)
 OR EXISTS(SELECT 1 FROM adobe_proxy_408_holds h WHERE h.account_id=a.id))
), changed AS (UPDATE accounts a SET extra=CASE WHEN s.value IS NULL THEN a.extra-'adobe_proxy_408'
 ELSE COALESCE(a.extra,'{}'::jsonb)||jsonb_build_object('adobe_proxy_408',s.value) END,updated_at=NOW()
 FROM state s WHERE a.id=s.id AND a.extra->'adobe_proxy_408' IS DISTINCT FROM s.value RETURNING a.id)
 INSERT INTO scheduler_outbox(event_type,payload) SELECT 'account_bulk_changed',jsonb_build_object('account_ids',jsonb_agg(id)) FROM changed HAVING count(*)>0`, id)
	return err
}

func lockTestRouting(ctx context.Context, tx *sql.Tx) (AllocationPolicy, error) {
	var policy AllocationPolicy
	var document []byte
	if err := tx.QueryRowContext(ctx, `SELECT document FROM clash_subscription WHERE id=1 FOR SHARE`).Scan(&document); err != nil {
		return policy, err
	}
	if _, err := tx.ExecContext(ctx, `LOCK TABLE accounts IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return policy, err
	}
	err := tx.QueryRowContext(ctx, `SELECT enabled,revision FROM proxy_auto_allocation WHERE id=1 FOR SHARE`).Scan(&policy.Enabled, &policy.Revision)
	return policy, err
}

// Remove expired or invalid ownership before freeing routing. A crash never
// automatically replays a generation or promotes an unacknowledged test.
func quarantine408(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM adobe_proxy_test_leases l WHERE expires_at<=NOW() OR NOT EXISTS(
 SELECT 1 FROM accounts a JOIN proxies p ON p.id=l.proxy_id WHERE a.id=l.account_id AND a.deleted_at IS NULL
 AND a.proxy_id=l.proxy_id AND `+credentialHashSQL("a.")+`=l.credential_hash
 AND p.updated_at=l.proxy_updated_at AND p.deleted_at IS NULL AND p.status='active'
 AND (p.expires_at IS NULL OR p.expires_at>NOW()))`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO adobe_proxy_408_holds(account_id,resume_schedulable)
	SELECT a.id,COALESCE(a.schedulable,false) OR COALESCE(a.proxy_auto_paused AND a.error_message=$1,false) FROM accounts a WHERE a.deleted_at IS NULL AND a.platform='adobe'
 AND (EXISTS(SELECT 1 FROM adobe_proxy_408_blocks b WHERE b.account_id=a.id)
 OR EXISTS(SELECT 1 FROM adobe_proxy_408_holds h WHERE h.account_id=a.id))
 AND NOT EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=a.id)
 ON CONFLICT(account_id) DO UPDATE SET resume_schedulable=adobe_proxy_408_holds.resume_schedulable OR EXCLUDED.resume_schedulable`, allocationWaitingMessage); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `WITH previous AS MATERIALIZED (
 SELECT a.id,a.proxy_id FROM accounts a WHERE a.deleted_at IS NULL AND a.platform='adobe'
 AND EXISTS(SELECT 1 FROM adobe_proxy_408_holds h WHERE h.account_id=a.id)
 AND NOT EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=a.id)
 AND (a.proxy_id IS NOT NULL OR a.schedulable)
), updated AS (UPDATE accounts a SET proxy_id=NULL,proxy_fallback_origin_id=NULL,schedulable=false,updated_at=NOW()
 FROM previous p WHERE a.id=p.id RETURNING a.id), events AS (
 INSERT INTO proxy_failover_events(account_id,from_proxy_id,to_proxy_id,created_at)
 SELECT p.id,p.proxy_id,0,clock_timestamp() FROM previous p JOIN updated u ON u.id=p.id WHERE p.proxy_id IS NOT NULL)
 SELECT id FROM updated`)
	if err != nil {
		return err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if err = allocationOutbox(ctx, tx, ids); err != nil {
		return err
	}
	return Refresh408Metadata(ctx, tx, 0)
}

func (m *Manager) AcquireTestProxy(ctx context.Context, id int64, scope string) (*TestLease, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	policy, err := lockTestRouting(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !policy.Enabled {
		return nil, nil
	}
	var blocked bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM adobe_proxy_408_blocks WHERE account_id=$1)
 OR EXISTS(SELECT 1 FROM adobe_proxy_408_holds WHERE account_id=$1)`, id).Scan(&blocked)
	if err != nil {
		return nil, err
	}
	opts, _ := ctx.Value(testRouteOptionsKey{}).(TestRouteOptions)
	// Existing healthy bindings keep their affinity. A review or unusable route
	// borrows under the same ownership/capacity and operator-state protections.
	psInitial, err := loadProxies(ctx, tx)
	if err != nil {
		return nil, err
	}
	var currentID int64
	var originalResume bool
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(proxy_id,0),COALESCE(schedulable,false) OR COALESCE(proxy_auto_paused AND error_message=$2,false) FROM accounts WHERE id=$1`, id, allocationWaitingMessage).Scan(&currentID, &originalResume); err != nil {
		return nil, err
	}
	if !blocked && len(opts.ExcludeSources) == 0 && (currentID == 0 || psInitial[currentID].available(time.Now())) {
		return nil, nil
	}
	var manual bool
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(extra->'manual_proxy_enabled'='true'::jsonb,false) FROM accounts WHERE id=$1`, id).Scan(&manual); err != nil {
		return nil, err
	}
	if manual {
		return nil, ErrNoTestProxy
	}

	// Reconcile regular business demand first; this also expires abandoned leases.
	if err = m.allocate(ctx, tx, policy, false); err != nil {
		return nil, err
	}
	var busy bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM adobe_proxy_test_leases WHERE account_id=$1)`, id).Scan(&busy); err != nil {
		return nil, err
	}
	if busy {
		return nil, ErrTestBusy
	}
	var eligible bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE id=$1 AND platform='adobe' AND deleted_at IS NULL
 AND (expires_at IS NULL OR expires_at>NOW()) AND status IN ('active','error'))`, id).Scan(&eligible); err != nil {
		return nil, err
	}
	if !eligible {
		return nil, ErrNoTestProxy
	}
	ps, err := loadProxies(ctx, tx)
	if err != nil {
		return nil, err
	}
	hs, err := loadHealth(ctx, tx)
	if err != nil {
		return nil, err
	}
	accounts, err := loadAllocationAccounts(ctx, tx)
	if err != nil {
		return nil, err
	}
	used := map[int64]int{}
	victims := map[int64]int64{}
	for _, a := range accounts {
		if a.Current != 0 {
			used[a.Current]++
			if a.BindingOnly && !a.VerificationPriority && !a.Quarantined408 && !a.TestLease {
				victims[a.Current] = a.ID
			}
		}
	}
	var best int64
	for pid, p := range ps {
		h := hs[pid]
		if !testRouteAvailable(p, opts, time.Now()) || h.Status != "healthy" || h.fingerprint != p.fingerprint() || time.Since(h.CheckedAt) > clash.ProbeFreshness(len(ps)) {
			continue
		}
		if p.generationBlocked {
			var busyRoute bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM adobe_proxy_test_leases WHERE proxy_id=$1 AND expires_at>NOW())`, pid).Scan(&busyRoute); err != nil {
				return nil, err
			}
			if busyRoute {
				continue
			}
		}
		if used[pid] >= accountsPerProxy && victims[pid] == 0 {
			continue
		}
		if best == 0 || generationRouteScore(p, h, used[pid]) < generationRouteScore(ps[best], hs[best], used[best]) {
			best = pid
		}
	}
	if best == 0 {
		return nil, ErrNoTestProxy
	}
	if used[best] >= accountsPerProxy {
		victim := victims[best]
		if _, err = tx.ExecContext(ctx, `UPDATE accounts SET proxy_id=NULL,proxy_fallback_origin_id=NULL,updated_at=NOW() WHERE id=$1`, victim); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO proxy_failover_events(account_id,from_proxy_id,to_proxy_id) VALUES($1,$2,0)`, victim, best); err != nil {
			return nil, err
		}
		if err = allocationOutbox(ctx, tx, []int64{victim}); err != nil {
			return nil, err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO adobe_proxy_408_holds(account_id,resume_schedulable)
 SELECT id,$2::boolean FROM accounts WHERE id=$1
 ON CONFLICT(account_id) DO NOTHING`, id, originalResume); err != nil {
		return nil, err
	}
	random := make([]byte, 16)
	if _, err = rand.Read(random); err != nil {
		return nil, err
	}
	owner := hex.EncodeToString(random)
	var returnSchedulable *bool
	if !blocked {
		returnSchedulable = &originalResume
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO adobe_proxy_test_leases(account_id,owner,proxy_id,proxy_updated_at,credential_hash,scope,expires_at,return_schedulable)
 SELECT a.id,$2,$3,$4,`+credentialHashSQL("a.")+`,$5,NOW()+INTERVAL '5 minutes',$6::boolean FROM accounts a WHERE a.id=$1`, id, owner, best, ps[best].UpdatedAt, scope, returnSchedulable); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT set_config('sub2api.proxy_allocator','on',true)`); err != nil {
		return nil, err
	}
	var previous int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(proxy_id,0) FROM accounts WHERE id=$1`, id).Scan(&previous); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE accounts SET proxy_id=$2,proxy_fallback_origin_id=NULL,schedulable=false,updated_at=NOW() WHERE id=$1`, id, best); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO proxy_failover_events(account_id,from_proxy_id,to_proxy_id) VALUES($1,$2,$3)`, id, previous, best); err != nil {
		return nil, err
	}
	if err = allocationOutbox(ctx, tx, []int64{id}); err != nil {
		return nil, err
	}
	if err = Refresh408Metadata(ctx, tx, id); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &TestLease{AccountID: id, Owner: owner, SourceID: routeSource(ps[best])}, nil
}

func (m *Manager) FinishTestProxy(ctx context.Context, lease TestLease, success, allowResume bool) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = lockTestRouting(ctx, tx)
	if err != nil {
		return err
	}
	var pid int64
	var scope, hash string
	var acquired, proxyUpdated, expires time.Time
	var returnSchedulable sql.NullBool
	err = tx.QueryRowContext(ctx, `SELECT proxy_id,scope,credential_hash,acquired_at,proxy_updated_at,expires_at,return_schedulable FROM adobe_proxy_test_leases WHERE account_id=$1 AND owner=$2 FOR UPDATE`, lease.AccountID, lease.Owner).Scan(&pid, &scope, &hash, &acquired, &proxyUpdated, &expires, &returnSchedulable)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// Keep the proxy revision stable until the routing decision commits.
	if _, err = tx.ExecContext(ctx, `SELECT id FROM proxies WHERE id=$1 FOR SHARE`, pid); err != nil {
		return err
	}
	var current, proved bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounts a JOIN proxies p ON p.id=a.proxy_id WHERE a.id=$1 AND a.deleted_at IS NULL
 AND a.proxy_id=$2 AND `+credentialHashSQL("a.")+`=$3 AND p.updated_at=$4 AND p.deleted_at IS NULL AND p.status='active'
 AND (p.expires_at IS NULL OR p.expires_at>NOW())),
 NOT EXISTS(SELECT 1 FROM adobe_proxy_408_blocks WHERE account_id=$1)
 AND EXISTS(SELECT 1 FROM adobe_account_health WHERE account_id=$1 AND scope=$5 AND success_started_at>=$6
 AND (failed_at IS NULL OR success_started_at>failed_at))`, lease.AccountID, pid, hash, proxyUpdated, scope, acquired).Scan(&current, &proved)
	if err != nil {
		return err
	}
	ps, err := loadProxies(ctx, tx)
	if err != nil {
		return err
	}
	keep := success && time.Now().Before(expires) && current && proved && ps[pid].available(time.Now())
	if current {
		if _, err = tx.ExecContext(ctx, `SELECT set_config('sub2api.proxy_allocator','on',true)`); err != nil {
			return err
		}
		if !keep {
			_, err = tx.ExecContext(ctx, `UPDATE accounts SET proxy_id=NULL,proxy_fallback_origin_id=NULL,schedulable=false,updated_at=NOW() WHERE id=$1`, lease.AccountID)
			if err == nil {
				_, err = tx.ExecContext(ctx, `INSERT INTO proxy_failover_events(account_id,from_proxy_id,to_proxy_id) VALUES($1,$2,0)`, lease.AccountID, pid)
			}
		} else {
			// The test has reconciled this route. Retaining allocator pause
			// ownership would let a later round bypass a disabled recovery policy.
			_, err = tx.ExecContext(ctx, `UPDATE accounts SET proxy_auto_paused=false,updated_at=NOW() WHERE id=$1 AND proxy_auto_paused`, lease.AccountID)
			if err == nil && allowResume {
				_, err = tx.ExecContext(ctx, `UPDATE accounts a SET schedulable=true,updated_at=NOW()
 WHERE a.id=$1 AND a.status='active' AND COALESCE(a.error_message,'')='' AND NOT a.proxy_auto_paused
 AND (a.expires_at IS NULL OR a.expires_at>NOW()) AND a.extra->'okad_manual_pause' IS DISTINCT FROM 'true'::jsonb
 AND (a.extra->'okad_connection_test'->>'gated' IS DISTINCT FROM 'true' OR a.extra->'okad_connection_test'->>'state'='success')
 AND (a.extra->'okad_connection_test'->>'require_verification' IS DISTINCT FROM 'true' OR a.extra->'okad'->>'verification'='verified')
 AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at<=NOW()) AND (a.overload_until IS NULL OR a.overload_until<=NOW())
 AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until<=NOW())
 AND (EXISTS(SELECT 1 FROM adobe_proxy_408_holds h WHERE h.account_id=a.id AND h.resume_schedulable)
 OR (a.extra->'okad_connection_test'->>'gated'='true' AND a.extra->'okad_connection_test'->>'resume'='true'))`, lease.AccountID)
			}
		}
		if err != nil {
			return err
		}
		if !keep && returnSchedulable.Valid {
			// Undo only a pause owned by a non-408 review. Cooldown timestamps
			// remain intact: a 429 still waits its normal 15 seconds. New 408,
			// credentials, verification gates and operator pauses never bypass it.
			_, err = tx.ExecContext(ctx, `UPDATE accounts a SET schedulable=$2,updated_at=NOW()
 WHERE a.id=$1 AND NOT EXISTS(SELECT 1 FROM adobe_proxy_408_blocks WHERE account_id=a.id)
 AND a.status='active' AND COALESCE(a.error_message,'')='' AND NOT a.proxy_auto_paused
 AND (a.expires_at IS NULL OR a.expires_at>NOW()) AND a.extra->'okad_manual_pause' IS DISTINCT FROM 'true'::jsonb
 AND (a.extra->'okad_connection_test'->>'gated' IS DISTINCT FROM 'true' OR a.extra->'okad_connection_test'->>'state'='success')
 AND (a.extra->'okad_connection_test'->>'require_verification' IS DISTINCT FROM 'true' OR a.extra->'okad'->>'verification'='verified')`, lease.AccountID, returnSchedulable.Bool)
			if err != nil {
				return err
			}
		}
		if err = allocationOutbox(ctx, tx, []int64{lease.AccountID}); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM adobe_proxy_test_leases WHERE account_id=$1 AND owner=$2`, lease.AccountID, lease.Owner); err != nil {
		return err
	}
	if keep {
		if _, err = tx.ExecContext(ctx, `DELETE FROM adobe_proxy_408_holds WHERE account_id=$1`, lease.AccountID); err != nil {
			return err
		}
	}
	if returnSchedulable.Valid {
		if _, err = tx.ExecContext(ctx, `DELETE FROM adobe_proxy_408_holds h WHERE account_id=$1
 AND NOT EXISTS(SELECT 1 FROM adobe_proxy_408_blocks b WHERE b.account_id=h.account_id)`, lease.AccountID); err != nil {
			return err
		}
	}
	if err = Refresh408Metadata(ctx, tx, lease.AccountID); err != nil {
		return err
	}
	return tx.Commit()
}

// Test ownership must expire even if automatic allocation was disabled while a
// process was interrupted. Only the still-owned temporary route is released.
func (m *Manager) releaseExpiredTests(ctx context.Context) error {
	rows, err := m.db.QueryContext(ctx, `SELECT account_id,owner FROM adobe_proxy_test_leases WHERE expires_at<=NOW()`)
	if err != nil {
		return err
	}
	leases := []TestLease{}
	for rows.Next() {
		var l TestLease
		if err = rows.Scan(&l.AccountID, &l.Owner); err != nil {
			rows.Close()
			return err
		}
		leases = append(leases, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, l := range leases {
		if err = m.FinishTestProxy(ctx, l, false, false); err != nil {
			return err
		}
	}
	return nil
}
