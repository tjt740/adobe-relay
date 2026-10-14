"""Verify/enable the second Adobe mailbox, or restore the preceding code safely."""
import argparse,datetime,fcntl,hashlib,json,subprocess,urllib.request
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('action',choices=['activate','verify','check-rollback','rollback']);args=parser.parse_args()
root=Path('/opt/adobe-relay/releases/adobe-second-alert-mail-20261014')
before=json.loads((root/'before.json').read_text());expected=(root/'expected-sha').read_text().strip()
lock=open('/opt/adobe-relay/.release.lock','a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
def sql(q):return subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input='BEGIN READ ONLY; '+q+'; COMMIT;',text=True,capture_output=True,check=True,timeout=30).stdout.strip()
def digest(v):return hashlib.sha256(json.dumps(v,sort_keys=True).encode()).hexdigest()
def inspect(name):return json.loads(subprocess.check_output(['docker','inspect',name]))[0]
app=inspect('adobe-relay-app-1');assert app['Config']['Image']=='adobe-relay:'+expected
assert app['State']['Health']['Status']=='healthy'
for name,identity in before['other_containers'].items():assert inspect(name)['Id']==identity, 'Other service changed: '+name
for path,h in before['config_hashes'].items():assert hashlib.sha256(Path(path).read_bytes()).hexdigest()==h, 'Config changed: '+path
assert sql("SELECT md5(COALESCE(string_agg(key||':'||value,',' ORDER BY key),'')) FROM settings WHERE key<>'ops_adobe_alert_mail_private'")==before['other_settings_hash'],'Other settings changed'
mail=json.loads(sql("SELECT value FROM settings WHERE key='ops_adobe_alert_mail_private'"))
assert digest({k:v for k,v in mail.items() if k!='additional'})==before['primary_transport_hash']
second=[a for a in mail.get('additional',[]) if a.get('recipient')=='lhaiming65@gmail.com'];assert len(second)==1 and digest(second[0])==before['additional_transport_hash']
rule9=json.loads(sql('SELECT row_to_json(r) FROM ops_alert_rules r WHERE id=9'))
assert all(rule9[k]==v for k,v in before['rule9'].items() if k not in ('updated_at','last_triggered_at')),'Primary rule changed'
rule=json.loads((root/'rule.json').read_text());ruleid=int(rule['id'])
current=json.loads(sql('SELECT row_to_json(r) FROM ops_alert_rules r WHERE id='+str(ruleid)))
assert all(current[k]==v for k,v in rule.items() if k not in ('enabled','updated_at','last_triggered_at')),'Second rule changed'
key=sql("SELECT value FROM settings WHERE key='admin_api_key'")
def request(path,body=None):
 req=urllib.request.Request('http://127.0.0.1:6666'+path,data=None if body is None else json.dumps(body).encode(),headers={'x-api-key':key,'Content-Type':'application/json'},method='GET' if body is None else 'PUT')
 with urllib.request.urlopen(req,timeout=30) as response:return json.load(response)['data']
for url in ['https://creative.cheap/health','https://creative.cheap/okad/api/health']:
 with urllib.request.urlopen(url,timeout=30) as response:assert response.status==200
report=request('/api/v1/admin/ops/adobe-health');assert report['quiet_notifications'] and report['notification_cooldown_minutes']==60 and report['threshold']==20 and report['minimum']==5
if args.action=='activate':
 assert not current['enabled'],'Already enabled; verify instead'
 current['enabled']=True;updated=request('/api/v1/admin/ops/alert-rules/'+str(ruleid),current);assert updated['enabled']
 (root/'enabled.json').write_text(json.dumps(updated,ensure_ascii=False,indent=2))
elif args.action=='verify':
 assert current['enabled']
 heartbeat=json.loads(sql("SELECT json_build_object('healthy',last_success_at>now()-interval '5 minutes' AND (last_error_at IS NULL OR last_success_at>last_error_at),'last_success_at',last_success_at) FROM ops_job_heartbeats WHERE job_name='ops_alert_evaluator'"));assert heartbeat['healthy']
 out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sha':expected,'health':'ok','primary_transport_and_rule_preserved':True,'other_services_and_settings_preserved':True,'second_rule':ruleid,'second_recipient':second[0]['recipient'],'report_total':report['total'],'heartbeat':heartbeat}
 (root/'verification.json').write_text(json.dumps(out,ensure_ascii=False,indent=2));print(json.dumps(out,ensure_ascii=False))
else:
 rollback='adobe-relay:before-adobe-second-alert-mail-20261014'
 assert inspect(rollback)['Id']==before['app_image_id']
 if args.action=='rollback':
  current['enabled']=False;updated=request('/api/v1/admin/ops/alert-rules/'+str(ruleid),current);assert not updated['enabled']
  fcntl.flock(lock,fcntl.LOCK_UN)
  subprocess.run(['bash','/opt/adobe-relay/deploy/primary/release.sh',rollback,'adobe-relay:'+expected],check=True)
  print('Previous code restored; second rule disabled; all current data and private transports retained')
 else:print('Rollback guards passed; no mutation performed')
