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

func TestAdobeDeliveryPostgresConcurrentClaimsCooldownAndRestart(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`CREATE TABLE ops_alert_events(id bigint PRIMARY KEY,rule_id bigint,status text,email_sent boolean,fired_at timestamptz,dimensions jsonb,title text,description text);
 INSERT INTO ops_alert_events VALUES (1,9,'firing',false,NOW(),'{}','',''),(2,9,'firing',false,NOW(),'{}','','');`)
	require.NoError(t, err)
	ctx := context.Background()
	now := time.Now().UTC()
	repo := &opsRepository{db: db}
	var claims atomic.Int64
	var failures atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.ClaimAdobeAlertEmail(ctx, 1, 9, now, time.Hour)
			if err != nil {
				failures.Add(1)
			}
			if ok {
				claims.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Zero(t, failures.Load())
	require.EqualValues(t, 1, claims.Load())
	// A stale dashboard read must not erase an in-flight durable receipt.
	require.NoError(t, repo.UpdateAdobeAlertReport(ctx, &service.OpsAlertEvent{ID: 1, Dimensions: map[string]any{"adobe_health": map[string]any{"pending": 5}, "adobe_incident_accounts": []int64{1}}}))
	restarted := &opsRepository{db: db}
	ok, err := restarted.ClaimAdobeAlertEmail(ctx, 1, 9, now.Add(2*time.Hour), time.Hour)
	require.NoError(t, err)
	require.False(t, ok, "uncertain in-flight delivery remains suppressed after restart")
	require.NoError(t, repo.FinishAdobeAlertEmail(ctx, 1, false))
	ok, err = repo.ClaimAdobeAlertEmail(ctx, 1, 9, now.Add(59*time.Minute), time.Hour)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = repo.ClaimAdobeAlertEmail(ctx, 1, 9, now.Add(time.Hour), time.Hour)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, repo.FinishAdobeAlertEmail(ctx, 1, true))
	ok, err = repo.ClaimAdobeAlertEmail(ctx, 1, 9, now.Add(3*time.Hour), time.Hour)
	require.NoError(t, err)
	require.False(t, ok, "delivered incident never repeats")
	ok, err = repo.ClaimAdobeAlertEmail(ctx, 2, 9, now.Add(90*time.Minute), time.Hour)
	require.NoError(t, err)
	require.False(t, ok, "new incident uses last attempt, not only fired_at")
	ok, err = repo.ClaimAdobeAlertEmail(ctx, 2, 9, now.Add(2*time.Hour), time.Hour)
	require.NoError(t, err)
	require.True(t, ok)
}
