-- 408 is an observation, not durable permission to disable an account.
-- Keep health/error history and immutable older migrations. Only return pauses
-- whose ownership was recorded by the retired 408 hold or connection-test gate.
LOCK TABLE accounts IN SHARE ROW EXCLUSIVE MODE;

-- A timeout on an already identity-verified Cookie is not a failed credential.
-- Match the exact Cookie and completed test; never release a running/new test.
-- Older manual completions have no started_at. Only accept their closed queue
-- window (queued_at <= result <= finished_at); missing evidence stays gated.
-- A retired hold can own an earlier pause even when the later test captured
-- resume=false while that account was already waiting for its proxy.
WITH changed AS (
 UPDATE accounts a SET
 extra=jsonb_set(a.extra,'{okad_connection_test}',a.extra->'okad_connection_test'||'{"gated":false}'::jsonb),
 status=CASE WHEN a.status='error' THEN 'active' ELSE a.status END,
 error_message=NULL,
 schedulable=CASE WHEN (a.extra->'okad_connection_test'->'resume'='true'::jsonb
 OR EXISTS(SELECT 1 FROM adobe_proxy_408_holds h WHERE h.account_id=a.id AND h.resume_schedulable))
 AND NOT EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=a.id) THEN true ELSE a.schedulable END,
 updated_at=NOW()
 WHERE a.platform='adobe' AND a.deleted_at IS NULL
 AND a.extra->'okad_connection_test'->'gated'='true'::jsonb
 AND a.extra->'okad_connection_test'->>'state'='failed'
 AND a.extra->>'last_check_status_code'='408' AND a.extra->'last_check_success'='false'::jsonb
 AND a.extra->'okad_manual_pause' IS DISTINCT FROM 'true'::jsonb
 AND (a.extra->>'last_check_at')::timestamptz<=NOW()
 AND CASE WHEN a.extra->'okad_connection_test'->>'started_at' IS NOT NULL
 THEN (a.extra->>'last_check_at')::timestamptz >= (a.extra->'okad_connection_test'->>'started_at')::timestamptz
 ELSE (a.extra->>'last_check_at')::timestamptz >= (a.extra->'okad_connection_test'->>'queued_at')::timestamptz
 AND (a.extra->>'last_check_at')::timestamptz <= (a.extra->'okad_connection_test'->>'finished_at')::timestamptz END
 AND a.extra->'okad_connection_test'->>'key'=encode(sha256(convert_to(COALESCE(a.credentials->>'cookie',''),'UTF8')),'hex')
 AND NULLIF(BTRIM(a.credentials->>'cookie'),'') IS NOT NULL
 AND (a.extra->'okad_connection_test'->'require_verification'='false'::jsonb OR a.extra->'okad'->>'verification'='verified')
 AND a.extra->'okad'->>'verification' IS DISTINCT FROM 'quota_blocked'
 AND (a.status='active' AND COALESCE(a.error_message,'')=''
 OR a.status='error' AND a.error_message='Okad credentials pending connection test'
 OR a.status='error' AND a.error_message='自动代理分配：等待可用节点（每个节点最多 3 个账号）'
 AND EXISTS(SELECT 1 FROM adobe_proxy_408_holds h WHERE h.account_id=a.id AND h.resume_schedulable))
 AND (a.expires_at IS NULL OR a.expires_at>NOW())
 RETURNING id)
INSERT INTO scheduler_outbox(event_type,account_id) SELECT 'account_changed',id FROM changed;

-- In-flight temporary routes keep their owner, expiry and credential snapshot.
UPDATE adobe_proxy_test_leases l SET return_schedulable=COALESCE(l.return_schedulable,h.resume_schedulable)
FROM adobe_proxy_408_holds h WHERE h.account_id=l.account_id;

WITH changed AS (
 UPDATE accounts a SET schedulable=true,updated_at=NOW()
 FROM adobe_proxy_408_holds h WHERE h.account_id=a.id AND h.resume_schedulable
 AND a.platform='adobe' AND a.deleted_at IS NULL AND NOT a.schedulable
 AND a.extra->'okad_manual_pause' IS DISTINCT FROM 'true'::jsonb
 AND (a.expires_at IS NULL OR a.expires_at>NOW())
 AND (a.status='active' AND COALESCE(a.error_message,'')='' OR
 a.status='error' AND a.proxy_auto_paused AND a.error_message='自动代理分配：等待可用节点（每个节点最多 3 个账号）')
 AND (a.extra->'okad_connection_test'->'gated' IS DISTINCT FROM 'true'::jsonb OR a.extra->'okad_connection_test'->>'state'='success')
 AND (a.extra->'okad_connection_test'->'require_verification' IS DISTINCT FROM 'true'::jsonb OR a.extra->'okad'->>'verification'='verified')
 AND a.extra->'okad'->>'verification' IS DISTINCT FROM 'quota_blocked'
 AND NOT EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=a.id)
 RETURNING a.id)
INSERT INTO scheduler_outbox(event_type,account_id) SELECT 'account_changed',id FROM changed;

-- No account/proxy/group/credential is deleted or rebound here. The ordinary
-- allocator handles missing routes and capacity using its existing protections.
UPDATE accounts SET extra=extra-'adobe_proxy_408',updated_at=NOW() WHERE extra ? 'adobe_proxy_408';
DELETE FROM adobe_proxy_408_blocks;
DELETE FROM adobe_proxy_408_holds;
