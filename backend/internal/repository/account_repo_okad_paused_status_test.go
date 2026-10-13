package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOkadPausedCookieDoesNotKeepObsoleteAuthenticationFailure(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	repo := &accountRepository{sql: db}
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO proxies(id) VALUES(7)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,proxy_id,updated_at)
 VALUES(1,'adobe','oauth','error',$1,false,
 '{"cookie":"old","email":"one@example.test","model_mapping":{"original":"keep"}}',
 '{"okad_manual_pause":true,"okad":{"issuer":"test","source_key":"external:1","version":3,"resume":true,"verification":"auth_failed"}}',7,NOW());`, service.AdobeCookieRecoveryExhausted)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO adobe_cookie_recovery_attempts(account_id,attempt_id,attempts,last_attempt_at,next_attempt_at,finished_at) VALUES(1,'existing-budget',2,NOW()-INTERVAL '1 hour',NOW()-INTERVAL '45 minutes',NOW()-INTERVAL '1 hour')`)
	require.NoError(t, err)
	read := func() *service.Account {
		a := &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth}
		var credentials, extra []byte
		require.NoError(t, db.QueryRow(`SELECT status,COALESCE(error_message,''),schedulable,credentials,extra,proxy_id,updated_at FROM accounts WHERE id=1`).Scan(&a.Status, &a.ErrorMessage, &a.Schedulable, &credentials, &extra, &a.ProxyID, &a.UpdatedAt))
		require.NoError(t, json.Unmarshal(credentials, &a.Credentials))
		require.NoError(t, json.Unmarshal(extra, &a.Extra))
		return a
	}
	applied, err := repo.ApplyOkadCredential(ctx, read(), service.OkadCredentialUpdate{Email: "one@example.test", Issuer: "test", SourceKey: "external:1", Version: 4, Cookie: "new"})
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, service.OkadVerificationPending, read().ErrorMessage)
	require.False(t, read().Schedulable)
	// Failed token verification remains an actual 401 even when operator-paused.
	authFailure := "Authentication failed (401): Adobe rejected the Cookie during token exchange"
	applied, err = repo.CompleteOkadVerification(ctx, read(), nil, "auth_failed", authFailure)
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, authFailure, read().ErrorMessage)
	// A fresh authenticated probe must supersede that 401 without enabling traffic.
	applied, err = repo.CompleteOkadVerification(ctx, read(), map[string]any{"access_token": "verified-token"}, "verified", "credentials verified")
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, service.OkadConnectionTestPending, read().ErrorMessage)
	require.False(t, read().Schedulable)
	job, err := repo.ClaimOkadConnectionTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, job)
	_, err = repo.RecordConnectionTestResultIfCurrent(ctx, read(), false, 408)
	require.NoError(t, err)
	require.NoError(t, repo.FinishOkadConnectionTest(ctx, *job, "failed"))
	a := read()
	require.False(t, a.Schedulable)
	require.NotContains(t, a.ErrorMessage, "401")
	require.Equal(t, float64(408), a.Extra["last_check_status_code"])
	// A later explicit full success clears obsolete errors but retains the pause.
	job.Manual = true
	require.NoError(t, repo.FinishOkadConnectionTest(ctx, *job, "success"))
	a = read()
	require.Equal(t, service.StatusActive, a.Status)
	require.Empty(t, a.ErrorMessage)
	require.False(t, a.Schedulable)
	require.Equal(t, true, a.Extra["okad_manual_pause"])
	require.EqualValues(t, 7, *a.ProxyID)
	require.Equal(t, "new", a.GetCredential("cookie"))
	require.Contains(t, a.Credentials, "model_mapping")
	var attempts int
	require.NoError(t, db.QueryRow(`SELECT attempts FROM adobe_cookie_recovery_attempts WHERE account_id=1`).Scan(&attempts))
	require.Equal(t, 2, attempts, "a push or test result cannot independently erase the login budget")
}

func TestOkadAuthenticationOwnershipExcludesConfigurationQuotaAndEntitlement(t *testing.T) {
	for _, message := range []string{
		"Authentication failed (401): taste_exhausted",
		"Authentication failed (401): model_not_entitled",
		"Authentication failed (401): quota exhausted",
		"Authentication failed (401): invalid_client",
		"Token refresh failed (non-retryable): Adobe cookie is no longer valid invalid_scope",
		"proxy unavailable", "account disabled",
	} {
		require.False(t, okadOwnsAuthenticationError(message), message)
	}
	require.True(t, okadOwnsAuthenticationError(service.AdobeCookieRecoveryExhausted))
}

func TestOkadNewVersionPreservesPendingResumeAndOperatorPause(t *testing.T) {
	for _, paused := range []bool{false, true} {
		db := newOkadRecoveryPostgres(t)
		repo := &accountRepository{sql: db}
		extra := map[string]any{"okad_manual_pause": paused, "okad": map[string]any{"issuer": "test", "source_key": "external:1", "version": 3, "resume": true}}
		raw, err := json.Marshal(extra)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,updated_at)
 VALUES(1,'adobe','oauth','error',$1,false,'{"cookie":"old","email":"one@example.test"}',$2,NOW())`, service.OkadConnectionTestPending, string(raw))
		require.NoError(t, err)
		a := &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth, Status: service.StatusError, ErrorMessage: service.OkadConnectionTestPending, Credentials: map[string]any{"cookie": "old", "email": "one@example.test"}, Extra: extra}
		require.NoError(t, db.QueryRow(`SELECT updated_at FROM accounts WHERE id=1`).Scan(&a.UpdatedAt))
		applied, err := repo.ApplyOkadCredential(context.Background(), a, service.OkadCredentialUpdate{Email: "one@example.test", Issuer: "test", SourceKey: "external:1", Version: 4, Cookie: "new"})
		require.NoError(t, err)
		require.True(t, applied)
		var resume, schedulable bool
		require.NoError(t, db.QueryRow(`SELECT (extra->'okad'->>'resume')::boolean,schedulable FROM accounts WHERE id=1`).Scan(&resume, &schedulable))
		require.Equal(t, !paused, resume)
		require.False(t, schedulable)
	}
}
