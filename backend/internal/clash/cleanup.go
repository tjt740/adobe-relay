package clash

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/lib/pq"
)

// retireProxies removes only IDs owned by the saved subscription. Accounts
// must stop scheduling before their proxy is detached, since nil means DIRECT.
// A cached old endpoint also fails closed: reused ports get new credentials.
func retireProxies(ctx context.Context, tx *sql.Tx, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	retired := pq.Array(ids)
	rows, err := tx.QueryContext(ctx, `
 UPDATE accounts SET
   schedulable=CASE WHEN proxy_id=ANY($1) THEN false ELSE schedulable END,
   status=CASE WHEN proxy_id=ANY($1) THEN 'error' ELSE status END,
   error_message=CASE WHEN proxy_id=ANY($1) THEN 'Clash 订阅节点已移除，请重新选择代理并恢复账号调度' ELSE error_message END,
   proxy_id=CASE WHEN proxy_id=ANY($1) THEN NULL ELSE proxy_id END,
   proxy_fallback_origin_id=CASE WHEN proxy_fallback_origin_id=ANY($1) THEN NULL ELSE proxy_fallback_origin_id END,
   extra=COALESCE(extra,'{}'::jsonb)-'upstream_billing_probe'-'ollama_cloud_usage_snapshot',
   updated_at=NOW()
 WHERE proxy_id=ANY($1) OR proxy_fallback_origin_id=ANY($1)
 RETURNING id`, retired)
	if err != nil {
		return err
	}
	accountIDs := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		accountIDs = append(accountIDs, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// A removed primary invalidates its policy. Other policies retain their
	// remaining backups in priority order, and stale concurrent checks lose CAS.
	if _, err = tx.ExecContext(ctx, `DELETE FROM account_proxy_failover WHERE primary_proxy_id=ANY($1)`, retired); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
 UPDATE account_proxy_failover SET
   backup_proxy_ids=COALESCE((SELECT jsonb_agg(value ORDER BY ordinal)
     FROM jsonb_array_elements(backup_proxy_ids) WITH ORDINALITY AS b(value,ordinal)
     WHERE NOT (value::text::bigint=ANY($1))), '[]'::jsonb),
   revision=revision+1,updated_at=NOW()
 WHERE EXISTS (SELECT 1 FROM jsonb_array_elements_text(backup_proxy_ids) b(id) WHERE id::bigint=ANY($1))`, retired); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE proxies SET backup_proxy_id=NULL,fallback_mode='none',updated_at=NOW() WHERE backup_proxy_id=ANY($1)`, retired); err != nil {
		return err
	}
	// Physical removal also cascades proxy health records. No retired configs,
	// credentials or proxy rows are kept in the pool.
	if _, err = tx.ExecContext(ctx, `DELETE FROM proxies WHERE id=ANY($1)`, retired); err != nil {
		return err
	}
	return notifyAccounts(ctx, tx, accountIDs)
}

func notifyAccounts(ctx context.Context, tx *sql.Tx, ids []int64) error {
	for start := 0; start < len(ids); start += 500 {
		payload, _ := json.Marshal(map[string]any{"account_ids": ids[start:min(start+500, len(ids))]})
		if _, err := tx.ExecContext(ctx, `INSERT INTO scheduler_outbox(event_type,payload) VALUES('account_bulk_changed',$1)`, string(payload)); err != nil {
			return err
		}
	}
	return nil
}
