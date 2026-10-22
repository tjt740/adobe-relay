"""Read only classifications and audit metadata for observed live state changes."""
import datetime,importlib.util,json,pathlib
ROOT=pathlib.Path('/opt/adobe-relay/releases/upstream-v0217-20261022')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
accounts=json.loads(a.sql("SELECT json_agg(json_build_object('id',id,'status',status,'schedulable',schedulable,'updated_at',updated_at,'error_message',CASE WHEN error_message IN ('Okad credentials pending connection test','Okad credentials pending validation','自动代理分配：等待可用节点（每个节点最多 3 个账号）') THEN error_message ELSE 'classified_or_other' END,'error_prefix',CASE WHEN error_message LIKE 'Token refresh failed (non-retryable):%' THEN 'Token refresh failed' WHEN error_message LIKE 'Authentication failed (401):%' THEN 'Authentication failed (401)' ELSE 'other' END,'error_hash',md5(coalesce(error_message,'')),'last_check_at',extra->>'last_check_at','last_check_status_code',extra->'last_check_status_code','last_check_success',extra->'last_check_success','manual_pause',extra->'okad_manual_pause','gate',extra->'okad_connection_test','okad',jsonb_build_object('version',extra->'okad'->'version','received_at',extra->'okad'->'received_at','verification',extra->'okad'->'verification'))) FROM accounts WHERE id IN(1150,1151,1153,1154,1156,1157,1158,1159,1160,1162,1163)"))
health=json.loads(a.sql("SELECT coalesce(json_agg(to_jsonb(h)),'[]') FROM adobe_account_health h WHERE account_id IN(1150,1151,1153,1154,1156,1157,1158,1159,1160,1162,1163)"))
audit=json.loads(a.sql("SELECT coalesce(json_agg(x ORDER BY id),'[]') FROM (SELECT id,created_at,actor_role,auth_method,action,method,path,status_code,request_body FROM audit_logs WHERE created_at>='2026-10-22T08:22:29Z' AND path LIKE '%accounts%' ORDER BY id DESC LIMIT 80)x"))
for row in audit:
 body=row.pop('request_body')
 try:
  body=json.loads(body);row['body_keys']=sorted(body.keys()) if isinstance(body,dict) else []
  if isinstance(body,dict):row['requested_state']={k:body[k] for k in ['status','schedulable'] if k in body}
 except Exception:row['body_keys']=['truncated_or_nonjson']
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'accounts':accounts,'health':health,'audit_requests':audit,'business_writes':False,'raw_credentials_in_output':False}
a.save('state-change-diagnostic-latest.json',result);print(json.dumps(result,ensure_ascii=False))
