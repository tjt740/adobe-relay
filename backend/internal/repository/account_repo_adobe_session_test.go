package repository

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAdobeSessionRotationPostgres(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	repo := &accountRepository{sql: db}
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra,updated_at)
 VALUES(1,'adobe','oauth','inactive',false,'{"cookie":"imported","model_mapping":{"x":"operator-edit"}}','{"okad_manual_pause":true}',NOW())`)
	require.NoError(t, err)
	snapshot := service.AdobeSessionSnapshot{Cookie: "imported"}
	fields := map[string]any{"access_token": "token-a", "adobe_session_cookie": "renewed-a", "adobe_session_cookie_source_hash": "source-hash", "cookie": "must-not-overwrite-source"}
	applied, err := repo.UpdateAdobeSessionIfUnchanged(ctx, 1, snapshot, fields)
	require.NoError(t, err)
	require.True(t, applied)
	read := func() map[string]any {
		var raw []byte
		var status string
		var sched bool
		require.NoError(t, db.QueryRow(`SELECT credentials,status,schedulable FROM accounts WHERE id=1`).Scan(&raw, &status, &sched))
		require.Equal(t, "inactive", status)
		require.False(t, sched)
		var result map[string]any
		require.NoError(t, json.Unmarshal(raw, &result))
		return result
	}
	got := read()
	require.Equal(t, "imported", got["cookie"])
	require.Equal(t, "renewed-a", got["adobe_session_cookie"])
	require.Equal(t, map[string]any{"x": "operator-edit"}, got["model_mapping"])
	// Same source Cookie does not authorize a stale response to replace a newer renewal.
	applied, err = repo.UpdateAdobeSessionIfUnchanged(ctx, 1, snapshot, map[string]any{"access_token": "stale"})
	require.NoError(t, err)
	require.False(t, applied)
	require.Equal(t, "token-a", read()["access_token"])
	// Error results use the same runtime-session fence and preserve the manual pause.
	stale := &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth, Status: "inactive", Schedulable: false, Credentials: map[string]any{"cookie": "imported"}, Extra: map[string]any{"okad_manual_pause": true}}
	applied, err = repo.SetAdobeErrorIfCurrent(ctx, stale, "stale rejection")
	require.NoError(t, err)
	require.False(t, applied)
	// A current renewal merges with a concurrently edited model mapping.
	snapshot.SessionCookie = "renewed-a"
	snapshot.SourceHash = "source-hash"
	_, err = db.Exec(`UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping}','{"x":"newer-edit"}') WHERE id=1`)
	require.NoError(t, err)
	applied, err = repo.UpdateAdobeSessionIfUnchanged(ctx, 1, snapshot, map[string]any{"access_token": "token-b", "adobe_session_cookie": "renewed-b"})
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, map[string]any{"x": "newer-edit"}, read()["model_mapping"])
	// A source replacement fences an in-flight refresh, including its outbox event.
	_, err = db.Exec(`UPDATE accounts SET credentials=jsonb_set(credentials,'{cookie}','"replacement"') WHERE id=1`)
	require.NoError(t, err)
	snapshot.SessionCookie = "renewed-b"
	applied, err = repo.UpdateAdobeSessionIfUnchanged(ctx, 1, snapshot, map[string]any{"access_token": "stale"})
	require.NoError(t, err)
	require.False(t, applied)
	var events int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM scheduler_outbox`).Scan(&events))
	require.Equal(t, 2, events)
}

func TestAdobeSessionRotationPostgresFencesProxyAndRenewal(t *testing.T) {
	for _, mutation := range []string{"runtime", "binding", "proxy", "proxy_edit"} {
		t.Run(mutation, func(t *testing.T) {
			db := newOkadRecoveryPostgres(t)
			repo := &accountRepository{sql: db}
			_, err := db.Exec(`INSERT INTO proxies(id) VALUES(7),(8);
 INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra,proxy_id,updated_at)
 VALUES(1,'adobe','oauth','active',true,'{"cookie":"source","adobe_session_cookie":"current","adobe_session_cookie_source_hash":"binding"}','{}',7,NOW())`)
			require.NoError(t, err)
			pid := int64(7)
			var stamp time.Time
			require.NoError(t, db.QueryRow(`SELECT updated_at FROM proxies WHERE id=7`).Scan(&stamp))
			before := service.AdobeSessionSnapshot{Cookie: "source", SessionCookie: "current", SourceHash: "binding", ProxyID: &pid, ProxyUpdatedAt: &stamp}
			mutations := map[string]string{"runtime": `UPDATE accounts SET credentials=jsonb_set(credentials,'{adobe_session_cookie}','"newer"')`, "binding": `UPDATE accounts SET credentials=jsonb_set(credentials,'{adobe_session_cookie_source_hash}','"newer"')`, "proxy": `UPDATE accounts SET proxy_id=8`, "proxy_edit": `UPDATE proxies SET updated_at=NOW()+INTERVAL '1 second' WHERE id=7`}
			_, err = db.Exec(mutations[mutation])
			require.NoError(t, err)
			applied, err := repo.UpdateAdobeSessionIfUnchanged(context.Background(), 1, before, map[string]any{"access_token": "stale"})
			require.NoError(t, err)
			require.False(t, applied)
		})
	}
}
