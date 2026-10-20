"""Reconcile only still-current full-generation proof, preserving user settings."""
import argparse,datetime,fcntl,hashlib,json,re,subprocess
from pathlib import Path
ROOT=Path('/opt/adobe-relay/releases/account-check-state-20261020')
ELIGIBLE="""
 SELECT a.id,md5(jsonb_build_array(md5(a.credentials::text),a.proxy_id,a.status,a.error_message,a.schedulable,a.temp_unschedulable_until,a.extra->'last_check_at',a.extra->'okad_connection_test',a.updated_at)::text) fingerprint
 FROM accounts a JOIN proxies p ON p.id=a.proxy_id
 WHERE a.deleted_at IS NULL AND a.platform='adobe' AND a.type='oauth' AND a.status='error'
 AND a.error_message='Clash 订阅节点已移除，请重新选择代理并恢复账号调度'
 AND a.schedulable AND coalesce(a.extra->>'okad_manual_pause','false')<>'true' AND NOT coalesce(a.proxy_auto_paused,false)
 AND p.deleted_at IS NULL AND p.status='active'
 AND (a.expires_at IS NULL OR a.expires_at>now())
 AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until<=now())
 AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at<=now()) AND (a.overload_until IS NULL OR a.overload_until<=now())
 AND a.extra->>'last_check_success'='true'
 AND a.extra->'okad_connection_test'->>'state'='success'
 AND a.extra->'okad_connection_test'->>'key'=encode(sha256(convert_to(coalesce(a.credentials->>'cookie',''),'UTF8')),'hex')
 AND a.extra->'adobe_health'->'last_success'->>'source' IN ('test','okad')
 AND ABS(EXTRACT(EPOCH FROM ((a.extra->>'last_check_at')::timestamptz-(a.extra->'adobe_health'->'last_success'->>'checked_at')::timestamptz)))<2
 AND p.updated_at<=(a.extra->'adobe_health'->'last_success'->>'started_at')::timestamptz
 AND NOT EXISTS(SELECT 1 FROM proxy_failover_events e WHERE e.account_id=a.id AND e.created_at>(a.extra->'adobe_health'->'last_success'->>'started_at')::timestamptz)
 AND NOT EXISTS(SELECT 1 FROM adobe_account_health h WHERE h.account_id=a.id AND h.failed_at>=(a.extra->'adobe_health'->'last_success'->>'started_at')::timestamptz)
"""
def sql(q,write=False):
 p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input=('BEGIN;' if write else 'BEGIN READ ONLY;')+"SET LOCAL statement_timeout='15s'; "+q+'; COMMIT;',capture_output=True,text=True,check=True,timeout=25)
 return json.loads(p.stdout)
def main():
 action=argparse.ArgumentParser();action.add_argument('mode',choices=['plan','apply']);action.add_argument('--label',default='reconciliation');args=action.parse_args()
 assert re.fullmatch('[a-z][a-z-]{0,60}',args.label)
 plan_path=ROOT/(args.label+'-plan.json');applied_path=ROOT/(args.label+'-applied.json')
 if args.mode=='plan':
  rows=sql("SELECT coalesce(json_agg(t),'[]') FROM ("+ELIGIBLE+") t")
  path=plan_path;assert not path.exists();path.write_text(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'accounts':rows},indent=2));path.chmod(0o600)
  print(json.dumps({'eligible':len(rows),'ids':[r['id'] for r in rows]}));return
 assert not applied_path.exists(),'Reconciliation already applied'
 held=[]
 for p in ['/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
  f=open(p,'r');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
 image=subprocess.check_output(['docker','inspect','--format','{{.Config.Image}}','adobe-relay-app-1'],text=True).strip()
 assert image in ('adobe-relay:9baadf2d1b525c318f9a7be0d6f2bf37381d2fb2','adobe-relay:'+(ROOT/'expected-sha').read_text().strip()),'A later application exists; review before changing account state'
 lease=subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
 assert not lease,'Wait for the rolling handoff to finish'
 plan=json.loads(plan_path.read_text())['accounts']
 if not plan:print(json.dumps({'recovered':0}));return
 values=','.join('('+str(row['id'])+",'"+row['fingerprint']+"')" for row in plan)
 q="WITH eligible AS ("+ELIGIBLE+"), planned(id,fingerprint) AS (VALUES "+values+"), updated AS (UPDATE accounts a SET status='active',error_message=NULL,updated_at=NOW() FROM eligible e,planned p WHERE a.id=e.id AND e.id=p.id AND e.fingerprint=p.fingerprint RETURNING a.id), event AS (INSERT INTO scheduler_outbox(event_type,account_id) SELECT 'account_changed',id FROM updated) SELECT coalesce(json_agg(id),'[]') FROM updated"
 rows=sql(q,True)
 path=applied_path;assert not path.exists();path.write_text(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'recovered':rows,'scope':'status and stale proxy message only; current scheduling, credentials, proxy, quota and health history retained'},indent=2));path.chmod(0o600)
 print(json.dumps({'recovered':len(rows),'ids':rows}))
if __name__=='__main__':main()
