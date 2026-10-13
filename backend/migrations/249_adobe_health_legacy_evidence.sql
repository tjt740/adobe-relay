-- Preserve pre-upgrade failures across the first quiet evaluation window.
WITH cutoff AS (
 SELECT LEAST(NOW()-INTERVAL '10 minutes', COALESCE(MIN(e.fired_at-make_interval(mins=>r.window_minutes)),NOW())) AS at
 FROM ops_alert_events e JOIN ops_alert_rules r ON r.id=e.rule_id
 WHERE e.status='firing' AND r.metric_type='adobe_401_408_accounts'
), evidence AS (
 SELECT account_id,created_at,status_code FROM adobe_account_test_errors WHERE created_at >= (SELECT at FROM cutoff)
 UNION ALL
 SELECT r.account_id,r.created_at,r.upstream_status_code FROM ops_error_logs r
 WHERE r.created_at >= (SELECT at FROM cutoff) AND r.platform='adobe' AND r.upstream_status_code IN (401,408)
 AND (r.error_owner='provider' OR r.error_source='upstream_http')
 AND (jsonb_typeof(r.upstream_errors) IS DISTINCT FROM 'array' OR r.upstream_errors='[]'::jsonb)
 UNION ALL
 SELECT CASE WHEN ev->>'account_id' ~ '^[0-9]{1,18}$' THEN (ev->>'account_id')::bigint
 WHEN ev->>'account_id' IS NULL THEN r.account_id ELSE NULL END,
 r.created_at,(ev->>'upstream_status_code')::int FROM ops_error_logs r
 CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(r.upstream_errors)='array' THEN r.upstream_errors ELSE '[]'::jsonb END) ev
 WHERE r.created_at >= (SELECT at FROM cutoff) AND COALESCE(NULLIF(ev->>'platform',''),r.platform)='adobe'
 AND ev->>'upstream_status_code' IN ('401','408')
), latest AS (
 SELECT DISTINCT ON (e.account_id) e.* FROM evidence e JOIN accounts a ON a.id=e.account_id
 WHERE a.platform='adobe' AND a.deleted_at IS NULL ORDER BY e.account_id,e.created_at DESC
)
INSERT INTO adobe_account_health(account_id,scope,failed_at,alert_failed_at,status_code,reason,source)
SELECT account_id,'',created_at,created_at,status_code,'legacy','legacy' FROM latest
ON CONFLICT(account_id,scope) DO NOTHING;

-- Surface the carried-over uncertainty on the account page as well.
UPDATE accounts a SET extra=COALESCE(a.extra,'{}'::jsonb)||jsonb_build_object('adobe_health',
 jsonb_build_object('pending',true,'latest',jsonb_build_object('success',false,'code',h.status_code,
 'reason','legacy','stage','','source','legacy','scope','','started_at',h.failed_at,'checked_at',h.failed_at))),updated_at=NOW()
FROM adobe_account_health h WHERE h.account_id=a.id AND h.scope='' AND h.reason='legacy'
 AND h.success_at IS NULL AND NOT COALESCE(a.extra,'{}'::jsonb) ? 'adobe_health';
