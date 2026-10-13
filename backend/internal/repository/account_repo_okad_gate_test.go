package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOkadCookieGatePostgresPreservesConfigurationAndRaces(t *testing.T) {
	for _, mutation := range []string{"", "manual_pause", "disable", "proxy_change", "proxy_edit", "cookie_change", "expired"} {
		t.Run(mutation, func(t *testing.T) {
			db := newOkadRecoveryPostgres(t)
			repo := &accountRepository{sql: db}
			ctx := context.Background()
			_, err := db.Exec(`INSERT INTO proxies(id) VALUES(7),(8);
 INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,proxy_id,updated_at)
 VALUES(1,'adobe','oauth','error','Authentication failed (401): Okad recovery pending',false,
 '{"cookie":"old","email":"one@example.test","access_token":"old-token","model_mapping":{"original":"keep"}}',
 '{"adobeteam_external_email":"one@example.test","base_rpm":12,"last_check_success":true,"last_check_status_code":200}',7,NOW())`)
			require.NoError(t, err)
			read := func() *service.Account {
				a := &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth}
				var creds, extra []byte
				require.NoError(t, db.QueryRow(`SELECT status,COALESCE(error_message,''),schedulable,credentials,extra,proxy_id,updated_at,expires_at FROM accounts WHERE id=1`).Scan(&a.Status, &a.ErrorMessage, &a.Schedulable, &creds, &extra, &a.ProxyID, &a.UpdatedAt, &a.ExpiresAt))
				require.NoError(t, json.Unmarshal(creds, &a.Credentials))
				require.NoError(t, json.Unmarshal(extra, &a.Extra))
				if a.ProxyID != nil {
					a.Proxy = &service.Proxy{ID: *a.ProxyID}
					require.NoError(t, db.QueryRow(`SELECT updated_at FROM proxies WHERE id=$1`, *a.ProxyID).Scan(&a.Proxy.UpdatedAt))
				}
				return a
			}
			initial := read()
			applied, err := repo.StageOkadCookie(ctx, initial, "new")
			require.NoError(t, err)
			require.True(t, applied)
			a := read()
			require.False(t, a.Schedulable)
			require.Equal(t, service.OkadConnectionTestPending, a.ErrorMessage)
			require.NotContains(t, a.Credentials, "access_token")
			require.NotContains(t, a.Extra, "last_check_success")
			require.Equal(t, float64(12), a.Extra["base_rpm"])
			require.EqualValues(t, 7, *a.ProxyID)
			require.Contains(t, a.Credentials, "model_mapping")
			require.NoError(t, repo.ClearError(ctx, 1))
			require.False(t, read().Schedulable)
			job, err := repo.ClaimOkadConnectionTest(ctx)
			require.NoError(t, err)
			require.NotNil(t, job)
			// Duplicate writes and restarts cannot replay a generation.
			applied, err = repo.StageOkadCookie(ctx, read(), "new")
			require.NoError(t, err)
			require.False(t, applied)
			duplicate, err := (&accountRepository{sql: db}).ClaimOkadConnectionTest(ctx)
			require.NoError(t, err)
			require.Nil(t, duplicate)
			snapshot := read()
			switch mutation {
			case "manual_pause":
				require.NoError(t, repo.SetSchedulable(ctx, 1, false))
			case "disable":
				_, err = db.Exec(`UPDATE accounts SET status='disabled' WHERE id=1`)
			case "proxy_change":
				_, err = db.Exec(`UPDATE accounts SET proxy_id=8 WHERE id=1`)
			case "proxy_edit":
				_, err = db.Exec(`UPDATE proxies SET updated_at=NOW()+INTERVAL '1 second' WHERE id=7`)
			case "cookie_change":
				_, err = repo.StageOkadCookie(ctx, read(), "newer")
			case "expired":
				_, err = db.Exec(`UPDATE accounts SET expires_at=NOW()-INTERVAL '1 second' WHERE id=1`)
			}
			require.NoError(t, err)
			recorded, err := repo.RecordConnectionTestResultIfCurrent(ctx, snapshot, true, 200)
			require.NoError(t, err)
			if mutation == "proxy_change" || mutation == "proxy_edit" || mutation == "cookie_change" {
				require.False(t, recorded)
			}
			require.NoError(t, repo.FinishOkadConnectionTest(ctx, *job, "success"))
			require.Equal(t, mutation == "", read().Schedulable)
			if mutation == "manual_pause" || mutation == "disable" || mutation == "proxy_change" || mutation == "proxy_edit" || mutation == "cookie_change" {
				applied, err := repo.SetAdobeErrorIfCurrent(ctx, snapshot, "old IMS rejection")
				require.NoError(t, err)
				require.False(t, applied)
			}
			if mutation == "cookie_change" {
				require.Equal(t, "newer", read().GetCredential("cookie"))
			}
		})
	}
}

func TestOkadCookieGatePostgresFailureAndManualRetry(t *testing.T) {
	for _, code := range []int{401, 408, 0} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			db := newOkadRecoveryPostgres(t)
			repo := &accountRepository{sql: db}
			ctx := context.Background()
			_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,updated_at) VALUES(1,'adobe','oauth','active','',true,'{"cookie":"old","email":"one@example.test"}','{}',NOW())`)
			require.NoError(t, err)
			a := &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": "old", "email": "one@example.test"}}
			_, err = repo.StageOkadCookie(ctx, a, "new")
			require.NoError(t, err)
			a.Credentials["cookie"] = "new"
			job, err := repo.ClaimOkadConnectionTest(ctx)
			require.NoError(t, err)
			require.NotNil(t, job)
			_, err = repo.RecordConnectionTestResultIfCurrent(ctx, a, false, code)
			require.NoError(t, err)
			require.NoError(t, repo.FinishOkadConnectionTest(ctx, *job, "failed"))
			var sched bool
			require.NoError(t, db.QueryRow(`SELECT schedulable FROM accounts WHERE id=1`).Scan(&sched))
			require.False(t, sched)
			next, err := repo.ClaimOkadConnectionTest(ctx)
			require.NoError(t, err)
			require.Nil(t, next)
			// An explicit successful full retest can release the same quarantined Cookie.
			job.Manual = true
			require.NoError(t, repo.FinishOkadConnectionTest(ctx, *job, "success"))
			require.NoError(t, db.QueryRow(`SELECT schedulable FROM accounts WHERE id=1`).Scan(&sched))
			require.True(t, sched)
		})
	}
}

func TestOkadCookieGateInterruptedCheckIsNotReplayed(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	repo := &accountRepository{sql: db}
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,updated_at) VALUES(1,'adobe','oauth','active','',true,'{"cookie":"old","email":"one@example.test"}','{}',NOW())`)
	require.NoError(t, err)
	a := &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": "old", "email": "one@example.test"}}
	_, err = repo.StageOkadCookie(ctx, a, "new")
	require.NoError(t, err)
	job, err := repo.ClaimOkadConnectionTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, job)
	_, err = db.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{okad_connection_test,started_at}',to_jsonb(NOW()-INTERVAL '6 minutes'))`)
	require.NoError(t, err)
	next, err := repo.ClaimOkadConnectionTest(ctx)
	require.NoError(t, err)
	require.Nil(t, next)
	var state string
	var sched bool
	require.NoError(t, db.QueryRow(`SELECT extra->'okad_connection_test'->>'state',schedulable FROM accounts WHERE id=1`).Scan(&state, &sched))
	require.Equal(t, "interrupted", state)
	require.False(t, sched)
	// A late process cannot turn an interrupted generation into a successful one.
	require.NoError(t, repo.FinishOkadConnectionTest(ctx, *job, "success"))
	require.NoError(t, db.QueryRow(`SELECT schedulable FROM accounts WHERE id=1`).Scan(&sched))
	require.False(t, sched)
}

func TestOkadCookieGateManualCheckCannotBypassIdentityVerification(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	repo := &accountRepository{sql: db}
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,updated_at)
 VALUES(1,'adobe','oauth','active','',true,'{"cookie":"old","email":"one@example.test"}','{}',NOW())`)
	require.NoError(t, err)
	a := &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": "old", "email": "one@example.test"}}
	_, err = repo.StageOkadCookie(ctx, a, "new")
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{okad_connection_test,require_verification}','true')`)
	require.NoError(t, err)
	a.Credentials["cookie"] = "new"
	require.NoError(t, repo.FinishOkadConnectionTest(ctx, service.OkadConnectionTestJob{AccountID: 1, Key: service.OkadConnectionTestKey(a), Manual: true}, "success"))
	var sched bool
	require.NoError(t, db.QueryRow(`SELECT schedulable FROM accounts WHERE id=1`).Scan(&sched))
	require.False(t, sched)
}
