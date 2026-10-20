import importlib.util,json,os,subprocess,uuid
from pathlib import Path
p=Path(__file__).with_name('reconcile.py');spec=importlib.util.spec_from_file_location('reconcile',p);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
dsn=os.environ['OKAD_RECOVERY_TEST_DATABASE_URL'];schema='reconcile_'+uuid.uuid4().hex

def sql(q):
 p=subprocess.run(['psql',dsn,'-Atq','-v','ON_ERROR_STOP=1'],input='SET search_path='+schema+';'+q,capture_output=True,text=True,check=True)
 return p.stdout.strip()
try:
 sql('CREATE SCHEMA '+schema+''';CREATE TABLE proxies(id bigint,status text,deleted_at timestamptz,updated_at timestamptz);
 CREATE TABLE accounts(id bigint,credentials jsonb,proxy_id bigint,status text,error_message text,schedulable bool,temp_unschedulable_until timestamptz,temp_unschedulable_reason text,extra jsonb,updated_at timestamptz,deleted_at timestamptz,platform text,type text,proxy_auto_paused bool,expires_at timestamptz,rate_limit_reset_at timestamptz,overload_until timestamptz);
 CREATE TABLE proxy_failover_events(account_id bigint,created_at timestamptz);
 CREATE TABLE adobe_account_health(account_id bigint,failed_at timestamptz);
 CREATE TABLE scheduler_outbox(event_type text,account_id bigint);
 INSERT INTO proxies VALUES(7,'active',NULL,NOW()-INTERVAL '1 day');
 INSERT INTO accounts(id,credentials,proxy_id,status,error_message,schedulable,extra,updated_at,platform,type,proxy_auto_paused)
 SELECT n,'{"cookie":"c"}',7,'error','Clash 订阅节点已移除，请重新选择代理并恢复账号调度',true,
 jsonb_build_object('last_check_success',true,'last_check_at',NOW()-INTERVAL '1 minute','okad_connection_test',jsonb_build_object('state','success','key',encode(sha256(convert_to('c','UTF8')),'hex')),'adobe_health',jsonb_build_object('last_success',jsonb_build_object('source','test','started_at',NOW()-INTERVAL '2 minutes','checked_at',NOW()-INTERVAL '1 minute'))),NOW(),'adobe','oauth',false
 FROM generate_series(1,7) n;
 UPDATE accounts SET extra=extra||'{"okad_manual_pause":true}' WHERE id=2;
 UPDATE accounts SET temp_unschedulable_until=NOW()+INTERVAL '1 day' WHERE id=3;
 UPDATE accounts SET credentials='{"cookie":"new"}' WHERE id=4;
 INSERT INTO proxy_failover_events VALUES(5,NOW());
 INSERT INTO adobe_account_health VALUES(6,NOW());''')
 rows=json.loads(sql("SELECT coalesce(json_agg(t),'[]') FROM ("+m.ELIGIBLE+") t"));assert sorted(r['id'] for r in rows)==[1,7],rows
 sql("UPDATE accounts SET updated_at=NOW(),schedulable=false WHERE id=7")
 values=','.join('('+str(r['id'])+",'"+r['fingerprint']+"')" for r in rows)
 q="WITH eligible AS ("+m.ELIGIBLE+"), planned(id,fingerprint) AS (VALUES "+values+"), updated AS (UPDATE accounts a SET status='active',error_message=NULL,updated_at=NOW() FROM eligible e,planned p WHERE a.id=e.id AND e.id=p.id AND e.fingerprint=p.fingerprint RETURNING a.id), event AS (INSERT INTO scheduler_outbox(event_type,account_id) SELECT 'account_changed',id FROM updated) SELECT coalesce(json_agg(id),'[]') FROM updated"
 assert json.loads(sql(q))==[1]
 assert sql("SELECT count(*) FROM scheduler_outbox")=="1"
 assert sql("SELECT status||':'||schedulable::text FROM accounts WHERE id=7")=="error:false"
 print(json.dumps({'eligible_before_race':[1,7],'recovered':[1],'preserved_manual_pause_quota_credentials_proxy_new_failure_and_race':True}))
finally:sql('DROP SCHEMA '+schema+' CASCADE')
