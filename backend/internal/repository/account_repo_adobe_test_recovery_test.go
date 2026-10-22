package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAdobeSuccessfulTestRecoveryPostgres(t *testing.T) {
	for _, mutation := range []string{"", "manual_pause", "disabled", "cookie", "proxy", "proxy_edit", "new_failure", "old_408_other_model", "new_cooldown", "identity_pending", "already_paused", "allocator_paused"} {
		t.Run(mutation, func(t *testing.T) {
			db := newOkadRecoveryPostgres(t)
			_, err := db.Exec(`ALTER TABLE accounts ADD COLUMN rate_limited_at timestamptz;
 ALTER TABLE proxies ADD COLUMN status text DEFAULT 'active',ADD COLUMN deleted_at timestamptz;
 INSERT INTO proxies(id) VALUES(7),(8);
 INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,proxy_id,updated_at,temp_unschedulable_until,temp_unschedulable_reason)
 VALUES(1,'adobe','oauth','error','Clash 订阅节点已移除，请重新选择代理并恢复账号调度',true,'{"cookie":"c","model_mapping":{"original":"keep"}}','{"keep":true}',7,NOW(),NOW()+INTERVAL '1 day','adobe_credits_exhausted')`)
			require.NoError(t, err)
			if mutation == "already_paused" {
				_, err = db.Exec(`UPDATE accounts SET schedulable=false,extra=extra||'{"okad_manual_pause":true}'`)
				require.NoError(t, err)
			}
			if mutation == "allocator_paused" {
				_, err = db.Exec(`UPDATE accounts SET schedulable=false,proxy_auto_paused=true,error_message='自动代理分配：等待可用节点（每个节点最多 3 个账号）'`)
				require.NoError(t, err)
			}
			a := &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth}
			var raw []byte
			require.NoError(t, db.QueryRow(`SELECT status,error_message,schedulable,credentials,proxy_id,temp_unschedulable_until,temp_unschedulable_reason FROM accounts WHERE id=1`).Scan(&a.Status, &a.ErrorMessage, &a.Schedulable, &raw, &a.ProxyID, &a.TempUnschedulableUntil, &a.TempUnschedulableReason))
			require.NoError(t, json.Unmarshal(raw, &a.Credentials))
			a.Proxy = &service.Proxy{ID: 7}
			require.NoError(t, db.QueryRow(`SELECT updated_at FROM proxies WHERE id=7`).Scan(&a.Proxy.UpdatedAt))
			started := time.Now().Add(-time.Minute)
			mutations := map[string]string{
				"manual_pause":        `UPDATE accounts SET schedulable=false,extra=extra||'{"okad_manual_pause":true}'`,
				"disabled":            `UPDATE accounts SET status='disabled'`,
				"cookie":              `UPDATE accounts SET credentials=jsonb_set(credentials,'{cookie}','"new"')`,
				"proxy":               `UPDATE accounts SET proxy_id=8`,
				"proxy_edit":          `UPDATE proxies SET updated_at=NOW()+INTERVAL '1 second' WHERE id=7`,
				"new_failure":         `INSERT INTO adobe_account_health(account_id,scope,failed_at,status_code) VALUES(1,'another-model',NOW(),408)`,
				"old_408_other_model": `INSERT INTO adobe_proxy_408_blocks VALUES(1,'another-model',NOW()-INTERVAL '1 day')`,
				"new_cooldown":        `UPDATE accounts SET temp_unschedulable_until=NOW()+INTERVAL '2 days'`,
				"identity_pending":    `UPDATE accounts SET extra=extra||'{"okad_connection_test":{"require_verification":true},"okad":{"verification":"pending"}}'`,
			}
			if q := mutations[mutation]; q != "" {
				_, err = db.Exec(q)
				require.NoError(t, err)
			}
			applied, err := (&accountRepository{sql: db}).RecoverAdobeAfterSuccessfulTestIfCurrent(context.Background(), a, started)
			require.NoError(t, err)
			expected := mutation == "" || mutation == "already_paused" || mutation == "allocator_paused"
			require.Equal(t, expected, applied)
			var status, cookie string
			var schedulable bool
			var until *time.Time
			var proxy int64
			var outbox int
			require.NoError(t, db.QueryRow(`SELECT status,schedulable,credentials->>'cookie',proxy_id,temp_unschedulable_until FROM accounts WHERE id=1`).Scan(&status, &schedulable, &cookie, &proxy, &until))
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM scheduler_outbox`).Scan(&outbox))
			if expected {
				require.Equal(t, "active", status)
				require.Nil(t, until)
				require.Equal(t, mutation != "already_paused" && mutation != "allocator_paused", schedulable)
				require.Equal(t, 1, outbox)
				require.Equal(t, "c", cookie)
				require.EqualValues(t, 7, proxy)
			} else {
				require.NotNil(t, until)
				require.Zero(t, outbox)
			}
		})
	}
}
