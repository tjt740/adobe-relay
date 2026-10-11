package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Optional replay input contains only pseudonymous emails and operational state.
// SQL runs in a disposable schema; it never calls Adobe or a production endpoint.
func TestOkadProductionShapeReplayPostgres(t *testing.T) {
	path := os.Getenv("OKAD_REPLAY_FIXTURE")
	if path == "" {
		t.Skip("set OKAD_REPLAY_FIXTURE to a sanitized snapshot")
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var fixture struct {
		Accounts []struct {
			ID          int64  `json:"id"`
			Email       string `json:"email"`
			Status      string `json:"status"`
			Schedulable bool   `json:"schedulable"`
			ErrorClass  string `json:"error_class"`
		} `json:"sub2"`
	}
	require.NoError(t, json.Unmarshal(raw, &fixture))
	require.NotEmpty(t, fixture.Accounts)
	db := newOkadRecoveryPostgres(t)
	db.SetMaxOpenConns(16)
	repo := &accountRepository{sql: db}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	for _, row := range fixture.Accounts {
		message := ""
		if row.ErrorClass == "auth_401" {
			message = "Authentication failed (401): fixture"
		} else if row.ErrorClass != "" {
			message = "fixture non-auth error"
		}
		creds, _ := json.Marshal(map[string]any{"email": row.Email, "cookie": "fixture-original"})
		_, err := db.ExecContext(ctx, `INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,updated_at) VALUES($1,'adobe','oauth',$2,$3,$4,$5,'{}',NOW())`, row.ID, row.Status, message, row.Schedulable, string(creds))
		require.NoError(t, err)
	}
	read := func(id int64) (*service.Account, error) {
		a := &service.Account{ID: id, Platform: "adobe", Type: "oauth"}
		var c, e []byte
		err := db.QueryRowContext(ctx, `SELECT credentials,extra,status,COALESCE(error_message,''),schedulable,updated_at,proxy_id FROM accounts WHERE id=$1`, id).Scan(&c, &e, &a.Status, &a.ErrorMessage, &a.Schedulable, &a.UpdatedAt, &a.ProxyID)
		if err == nil {
			err = json.Unmarshal(c, &a.Credentials)
		}
		if err == nil {
			err = json.Unmarshal(e, &a.Extra)
		}
		return a, err
	}
	start := time.Now()
	var wait sync.WaitGroup
	slots := make(chan struct{}, 16)
	for _, row := range fixture.Accounts {
		for _, version := range []int64{4, 1, 3, 2} {
			id, email, v := row.ID, row.Email, version
			slots <- struct{}{}
			wait.Add(1)
			go func() {
				defer func() { <-slots; wait.Done() }()
				for retry := 0; retry < 20; retry++ {
					a, err := read(id)
					require.NoError(t, err)
					state, _ := a.Extra["okad"].(map[string]any)
					if n, ok := state["version"].(float64); ok && int64(n) >= v {
						return
					}
					applied, err := repo.ApplyOkadCredential(ctx, a, service.OkadCredentialUpdate{Email: email, Issuer: "replay", SourceKey: fmt.Sprintf("external:%d", id), Version: v, Cookie: fmt.Sprintf("fixture-v%d", v)})
					require.NoError(t, err)
					if applied {
						return
					}
				}
				t.Errorf("account %d did not converge", id)
			}()
		}
	}
	wait.Wait()
	for i, row := range fixture.Accounts {
		a, err := read(row.ID)
		require.NoError(t, err)
		require.Equal(t, "fixture-v4", a.GetCredential("cookie"))
		if i%5 == 0 {
			require.NoError(t, repo.SetSchedulable(ctx, row.ID, false))
			applied, err := repo.CompleteOkadVerification(ctx, a, nil, "verified", "fixture")
			require.NoError(t, err)
			require.False(t, applied)
			a, err = read(row.ID)
			require.NoError(t, err)
			applied, err = repo.CompleteOkadVerification(ctx, a, nil, "verified", "fixture")
			require.NoError(t, err)
			require.True(t, applied)
			a, err = read(row.ID)
			require.NoError(t, err)
			require.False(t, a.Schedulable)
		}
	}
	t.Logf("replayed %d accounts, %d out-of-order submissions, concurrency 16, elapsed %s", len(fixture.Accounts), len(fixture.Accounts)*4, time.Since(start))
}
