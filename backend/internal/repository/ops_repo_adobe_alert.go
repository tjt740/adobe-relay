package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Use each saved upstream attempt, including failed accounts followed by a
// successful failover. Never attribute every attempt to the final account.
// Legacy rows without an attempt array fall back to an explicit upstream code;
// client-facing 401s (bad relay API keys) are deliberately excluded.
const adobeErrorAccountsSQL = `
WITH recent AS MATERIALIZED (
 SELECT id, created_at, account_id, platform, upstream_status_code,
        upstream_error_message, upstream_errors, error_owner, error_source
 FROM ops_error_logs
 WHERE created_at >= $1 AND created_at < $2
   AND ($3::bigint IS NULL OR group_id = $3)
), expanded AS (
 SELECT r.id, r.created_at,
   CASE WHEN ev->>'account_id' ~ '^[0-9]{1,18}$' THEN (ev->>'account_id')::bigint
        WHEN ev->>'account_id' IS NULL THEN r.account_id ELSE NULL END AS account_id,
   COALESCE(NULLIF(ev->>'platform',''),r.platform) AS platform,
   CASE WHEN ev->>'upstream_status_code' IN ('401','408') THEN (ev->>'upstream_status_code')::int ELSE 0 END AS code,
   COALESCE(ev->>'reason','') AS reason,
   LOWER(COALESCE(ev->>'message','')) AS message
 FROM recent r CROSS JOIN LATERAL jsonb_array_elements(
   CASE WHEN jsonb_typeof(r.upstream_errors) = 'array' THEN r.upstream_errors ELSE '[]'::jsonb END
 ) ev
 UNION ALL
 SELECT r.id, r.created_at, r.account_id, r.platform, r.upstream_status_code, '', LOWER(COALESCE(r.upstream_error_message,''))
 FROM recent r
 WHERE (r.error_owner = 'provider' OR r.error_source = 'upstream_http')
   AND r.upstream_status_code IN (401,408)
   AND CASE WHEN jsonb_typeof(r.upstream_errors) = 'array' THEN jsonb_array_length(r.upstream_errors) = 0 ELSE TRUE END
), classified AS (
 SELECT *, CASE
   WHEN reason = 'adobe_quota_exhausted' OR message LIKE '%quota exhausted%' OR message LIKE '%credits exhausted%' OR message LIKE '%taste_exhausted%' THEN 'quota'
   WHEN reason = 'adobe_not_entitled' OR message LIKE '%not entitled%' OR message LIKE '%model_not_entitled%' OR message LIKE '%user_not_entitled%' THEN 'entitled'
   WHEN reason = 'adobe_auth' OR message LIKE '%token invalid or expired%' OR message LIKE '%invalid_credentials%' OR message LIKE '%cookie is no longer valid%' THEN 'auth'
   ELSE 'unknown' END AS category
 FROM expanded WHERE platform = 'adobe' AND code IN (401,408) AND account_id > 0
)
SELECT e.account_id, COALESCE(a.name,''), COALESCE(a.status,'deleted'),
 COUNT(*) FILTER (WHERE code=401), COUNT(*) FILTER (WHERE code=408),
 COUNT(*) FILTER (WHERE code=401 AND category='auth'),
 COUNT(*) FILTER (WHERE code=401 AND category='quota'),
 COUNT(*) FILTER (WHERE code=401 AND category='entitled'),
 COUNT(*) FILTER (WHERE code=401 AND category='unknown'),
 MIN(e.created_at), MAX(e.created_at)
FROM classified e LEFT JOIN accounts a ON a.id=e.account_id
GROUP BY e.account_id, a.name, a.status
ORDER BY COUNT(*) DESC, e.account_id`

func (r *opsRepository) GetAdobeErrorAccounts(ctx context.Context, start, end time.Time, groupID *int64) ([]service.OpsAdobeAccountErrors, error) {
	if start.IsZero() || !end.After(start) || end.Sub(start) > 24*time.Hour {
		return nil, fmt.Errorf("Adobe alert window must be between 0 and 24 hours")
	}
	rows, err := r.db.QueryContext(ctx, adobeErrorAccountsSQL, start.UTC(), end.UTC(), groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := make([]service.OpsAdobeAccountErrors, 0)
	for rows.Next() {
		var a service.OpsAdobeAccountErrors
		if err := rows.Scan(&a.AccountID, &a.Name, &a.Status, &a.Count401, &a.Count408, &a.Auth401, &a.Quota401, &a.Entitled401, &a.Unknown401, &a.FirstSeen, &a.LastSeen); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}
