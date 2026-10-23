package proxyfailover

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/clash"
	"slices"
	"time"
)

type TestRouteOptions struct {
	ExcludeSources []string
	RequireClash   bool
}
type testRouteOptionsKey struct{}

func WithTestRouteOptions(ctx context.Context, opts TestRouteOptions) context.Context {
	return context.WithValue(ctx, testRouteOptionsKey{}, opts)
}
func TestRouteOptionsFromContext(ctx context.Context) TestRouteOptions {
	opts, _ := ctx.Value(testRouteOptionsKey{}).(TestRouteOptions)
	return opts
}
func routeSource(p proxy) string {
	if p.sourceID != "" {
		return p.sourceID
	}
	return fmt.Sprintf("proxy:%d", p.ID)
}
func testRouteAvailable(p proxy, opts TestRouteOptions, now time.Time) bool {
	return p.networkAvailable(now) && !p.excluded && !p.retryAfter.After(now) &&
		(!opts.RequireClash || p.sourceID != "") && !slices.Contains(opts.ExcludeSources, routeSource(p))
}

// HEAD reachability remains independent. An open generation circuit can only
// close on a newer real success, and after cooldown admits one test lease.
func loadGenerationHealth(ctx context.Context, q queryer, ps map[int64]proxy) error {
	rows, err := q.QueryContext(ctx, `SELECT p.id,
 EXISTS(SELECT 1 FROM adobe_route_exclusions e WHERE e.proxy_id=p.id),
 EXISTS(SELECT 1 FROM adobe_route_health h WHERE h.proxy_id=p.id AND h.proxy_updated_at=p.updated_at AND h.retry_after IS NOT NULL),
 (SELECT max(h.retry_after) FROM adobe_route_health h WHERE h.proxy_id=p.id AND h.proxy_updated_at=p.updated_at),
 EXISTS(SELECT 1 FROM adobe_route_health h WHERE h.proxy_id=p.id AND h.proxy_updated_at=p.updated_at AND h.success_started_at>NOW()-INTERVAL '30 minutes' AND (h.failed_at IS NULL OR h.success_started_at>h.failed_at))
 FROM proxies p`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var excluded, blocked, proven bool
		var retry sql.NullTime
		if err = rows.Scan(&id, &excluded, &blocked, &retry, &proven); err != nil {
			return err
		}
		p := ps[id]
		p.generationProven = proven
		p.excluded = excluded
		p.generationBlocked = blocked
		p.retryAfter = retry.Time
		ps[id] = p
	}
	return rows.Err()
}

// RouteReady is a read-only guard immediately before native Adobe submission.
// It never changes account credentials, pause state or proxy configuration.
func (m *Manager) RouteReady(ctx context.Context, id int64, expected *time.Time) (string, bool, error) {
	var ready bool
	err := m.db.QueryRowContext(ctx, `SELECT deleted_at IS NULL AND status='active' AND (expires_at IS NULL OR expires_at>NOW())
 AND ($2::timestamptz IS NULL OR updated_at=$2)
 AND NOT EXISTS(SELECT 1 FROM adobe_route_exclusions e WHERE e.proxy_id=p.id)
 AND NOT EXISTS(SELECT 1 FROM adobe_route_health h WHERE h.proxy_id=p.id AND h.proxy_updated_at=p.updated_at AND h.retry_after IS NOT NULL)
 FROM proxies p WHERE id=$1`, id, expected).Scan(&ready)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	source, err := clash.LoadProxySource(ctx, m.db, id)
	if err != nil {
		return "", false, err
	}
	key := source.ID
	if key == "" {
		key = fmt.Sprintf("proxy:%d", id)
	}
	return key, ready && !source.Blocked, nil
}

func RecordGeneration(ctx context.Context, tx *sql.Tx, pid int64, revision time.Time, source string, accountID int64, cookie, apiKey, scope string, success bool, code int, stage string, rejected bool, started, checked time.Time) (string, error) {
	// Serialize a node/model's evidence across accounts and instances.
	_, err := tx.ExecContext(ctx, `INSERT INTO adobe_route_health(proxy_id,proxy_updated_at,scope) VALUES($1,$2,$3)
 ON CONFLICT(proxy_id,scope) DO NOTHING`, pid, revision, scope)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `SELECT proxy_id FROM adobe_route_health WHERE proxy_id=$1 AND scope=$2 FOR UPDATE`, pid, scope)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO adobe_route_observations(proxy_id,proxy_updated_at,source_id,account_id,credential_hash,scope,success,status_code,stage,submission_rejected,started_at,checked_at)
 VALUES($1,$2,$3,$4,encode(sha256(convert_to(jsonb_build_array($5::text,$6::text)::text,'UTF8')),'hex'),$7,$8,$9,$10,$11,$12,$13)`, pid, revision, source, accountID, cookie, apiKey, scope, success, code, stage, rejected, started, checked)
	if err != nil {
		return "", err
	}
	if success {
		_, err = tx.ExecContext(ctx, `UPDATE adobe_route_health SET proxy_updated_at=$2,success_started_at=$4,
 retry_after=CASE WHEN proxy_updated_at<>$2 OR failed_at<$4 THEN NULL ELSE retry_after END
 WHERE proxy_id=$1 AND scope=$3 AND (proxy_updated_at<>$2 OR success_started_at IS NULL OR success_started_at<$4)`, pid, revision, scope, started)
	} else if code == 408 && (stage == "submit" || stage == "poll" || stage == "download") {
		_, err = tx.ExecContext(ctx, `UPDATE adobe_route_health SET proxy_updated_at=$2,failed_at=$4,
 success_started_at=CASE WHEN proxy_updated_at=$2 THEN success_started_at ELSE NULL END,
 retry_after=CASE WHEN (proxy_updated_at<>$2 OR success_started_at IS NULL OR success_started_at<=$5)
 AND (SELECT count(DISTINCT account_id) FROM adobe_route_observations WHERE proxy_id=$1 AND proxy_updated_at=$2 AND scope=$3 AND NOT success AND status_code=408 AND stage IN ('submit','poll','download')
 AND checked_at>=$4-INTERVAL '5 minutes' AND checked_at<=$4
 AND (adobe_route_health.success_started_at IS NULL OR started_at>=adobe_route_health.success_started_at))>=3 THEN $4+INTERVAL '5 minutes'
 WHEN proxy_updated_at=$2 THEN retry_after ELSE NULL END
 WHERE proxy_id=$1 AND scope=$3 AND (proxy_updated_at<>$2 OR failed_at IS NULL OR failed_at<$4)`, pid, revision, scope, checked, started)
	}
	return source, err
}

func allocationRouteAvailable(p proxy, now time.Time, adobe bool) bool {
	if adobe {
		return p.available(now)
	}
	return p.networkAvailable(now)
}
