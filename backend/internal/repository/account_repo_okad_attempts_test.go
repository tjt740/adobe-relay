package repository

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOkadRecoveryPostgresDurableBudget(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	ctx := context.Background()
	repo := &accountRepository{sql: db}
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,updated_at)
 VALUES(1,'adobe','oauth','active','',true,'{"cookie":"old","email":"fixture@example.test"}','{}',NOW())`)
	require.NoError(t, err)
	a := &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": "old", "email": "fixture@example.test"}}
	// Database arbitration, independently of singleflight, permits one caller.
	var granted atomic.Int32
	var wg sync.WaitGroup
	var first *service.AdobeCookieRecoveryAttempt
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claim, claimErr := (&accountRepository{sql: db}).ClaimAdobeCookieRecovery(ctx, a, 5*time.Minute)
			if claimErr != nil {
				t.Errorf("claim failed: %v", claimErr)
				return
			}
			if claim.Allowed {
				granted.Add(1)
				first = claim
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, granted.Load())
	require.Equal(t, 1, first.Count)
	var remaining float64
	require.NoError(t, db.QueryRow(`SELECT EXTRACT(EPOCH FROM next_attempt_at-NOW()) FROM adobe_cookie_recovery_attempts`).Scan(&remaining))
	require.Greater(t, remaining, 599.0)
	require.NoError(t, repo.FinishAdobeCookieRecovery(ctx, a.ID, first.ID, false))
	require.NoError(t, db.QueryRow(`SELECT EXTRACT(EPOCH FROM next_attempt_at-NOW()) FROM adobe_cookie_recovery_attempts`).Scan(&remaining))
	require.InDelta(t, 300.0, remaining, 2.0)
	second, err := (&accountRepository{sql: db}).ClaimAdobeCookieRecovery(ctx, a, time.Minute)
	require.NoError(t, err)
	require.False(t, second.Allowed)
	require.False(t, second.InFlight)
	// A rotated cookie and recreated repository do not reset cooldown/count.
	_, err = db.Exec(`UPDATE accounts SET credentials = credentials || '{"cookie":"new"}'; UPDATE adobe_cookie_recovery_attempts SET next_attempt_at=NOW()-INTERVAL '1 second'`)
	require.NoError(t, err)
	a.Credentials["cookie"] = "new"
	second, err = (&accountRepository{sql: db}).ClaimAdobeCookieRecovery(ctx, a, time.Minute)
	require.NoError(t, err)
	require.True(t, second.Allowed)
	require.Equal(t, 2, second.Count)
	// A late first result cannot finish or unlock the second reservation.
	require.NoError(t, repo.FinishAdobeCookieRecovery(ctx, a.ID, first.ID, true))
	var inFlight bool
	require.NoError(t, db.QueryRow(`SELECT finished_at IS NULL FROM adobe_cookie_recovery_attempts`).Scan(&inFlight))
	require.True(t, inFlight)
	require.NoError(t, repo.FinishAdobeCookieRecovery(ctx, a.ID, second.ID, true))
	require.NoError(t, db.QueryRow(`SELECT EXTRACT(EPOCH FROM next_attempt_at-NOW()) FROM adobe_cookie_recovery_attempts`).Scan(&remaining))
	require.InDelta(t, 900.0, remaining, 2.0)
	// Even success does not renew the budget or allow an immediate third login.
	require.NoError(t, repo.ConfirmAdobeCookieRecovery(ctx, a))
	_, err = db.Exec(`UPDATE adobe_cookie_recovery_attempts SET next_attempt_at=NOW()-INTERVAL '1 day'`)
	require.NoError(t, err)
	third, err := (&accountRepository{sql: db}).ClaimAdobeCookieRecovery(ctx, a, time.Minute)
	require.NoError(t, err)
	require.False(t, third.Allowed)
	require.Equal(t, 2, third.Count)
	// A stale successful response must never clear the current incident.
	_, err = db.Exec(`UPDATE adobe_cookie_recovery_attempts SET finished_at=NOW()-INTERVAL '16 minutes'`)
	require.NoError(t, err)
	stale := *a
	stale.Credentials = map[string]any{"cookie": "old", "email": "fixture@example.test"}
	require.NoError(t, repo.ConfirmAdobeCookieRecovery(ctx, &stale))
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_cookie_recovery_attempts`).Scan(&count))
	require.Equal(t, 1, count)
	// A verified response after stabilization closes the incident.
	require.NoError(t, repo.ConfirmAdobeCookieRecovery(ctx, a))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_cookie_recovery_attempts`).Scan(&count))
	require.Zero(t, count)
	nextIncident, err := repo.ClaimAdobeCookieRecovery(ctx, a, time.Minute)
	require.NoError(t, err)
	require.True(t, nextIncident.Allowed)
	require.Equal(t, 1, nextIncident.Count)
	// Process death keeps its reservation. Only a real success after the stale
	// lease plus stabilization can close an incident without a completion row.
	require.NoError(t, repo.ConfirmAdobeCookieRecovery(ctx, a))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_cookie_recovery_attempts`).Scan(&count))
	require.Equal(t, 1, count)
	_, err = db.Exec(`UPDATE adobe_cookie_recovery_attempts SET next_attempt_at=NOW()-INTERVAL '16 minutes'`)
	require.NoError(t, err)
	require.NoError(t, repo.ConfirmAdobeCookieRecovery(ctx, a))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_cookie_recovery_attempts`).Scan(&count))
	require.Zero(t, count)
}

func TestOkadRecoveryPostgresLimitPreservesConcurrentEdits(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	ctx := context.Background()
	repo := &accountRepository{sql: db}
	a := &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth, Status: service.StatusError, ErrorMessage: service.AdobeCookieRecoveryPending, Credentials: map[string]any{"cookie": "old", "email": "fixture@example.test"}}
	for _, mutation := range []string{"", "UPDATE accounts SET status='disabled'", "UPDATE accounts SET credentials=credentials || '{\"cookie\":\"manual\"}'", "UPDATE accounts SET proxy_id=3"} {
		_, err := db.Exec(`TRUNCATE accounts, scheduler_outbox CASCADE`)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,updated_at)
 VALUES(1,'adobe','oauth','error',$1,false,'{"cookie":"old","email":"fixture@example.test"}','{}',NOW())`, a.ErrorMessage)
		require.NoError(t, err)
		if mutation != "" {
			_, err = db.Exec(mutation)
			require.NoError(t, err)
		}
		applied, err := repo.SetAdobeCookieRecoveryErrorIfUnchanged(ctx, a, service.AdobeCookieRecoveryExhausted)
		require.NoError(t, err)
		require.Equal(t, mutation == "", applied, mutation)
	}
}
