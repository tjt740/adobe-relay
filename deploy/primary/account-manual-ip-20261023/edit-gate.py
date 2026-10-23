"""Keep manual edits closed until only the verified new worker remains."""
import datetime,fcntl,importlib.util,json,os,pathlib,subprocess,sys,urllib.error,urllib.request

ROOT=pathlib.Path('/opt/adobe-relay/releases/account-manual-ip-20261023')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
stage=sys.argv[1];assert stage in ['protect','check','activate']
held=[]
for path in ['/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(path,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
sha=(ROOT/'expected-sha').read_text().strip()
app=a.inspect(a.APP)
mount=next(m for m in app['Mounts'] if m['Destination']=='/app/data')
assert mount['Type']=='bind' and mount['RW']
marker=pathlib.Path(mount['Source'])/'.manual-proxy-rollout-in-progress'
owner={'release':'account-manual-ip-20261023','application_sha':sha}
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def store_marker():
 fd=os.open(marker,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
 with os.fdopen(fd,'w') as f:json.dump(owner,f)
def request_status():
 # A nonexistent positive ID tests the new parameter and release guard without
 # selecting or editing any live account. No retry or external request occurs.
 account_id=9223372036854775806
 assert a.sql('SELECT EXISTS(SELECT 1 FROM accounts WHERE id='+str(account_id)+')')=='f'
 key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
 req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/'+str(account_id),method='PUT',data=b'{"manual_proxy_enabled":false}',headers={'x-api-key':key,'Content-Type':'application/json'})
 try:
  with urllib.request.urlopen(req,timeout=20) as response:status=response.status
 except urllib.error.HTTPError as error:status=error.code
 assert a.sql('SELECT EXISTS(SELECT 1 FROM accounts WHERE id='+str(account_id)+')')=='f'
 return status
if stage=='protect':
 before=json.loads((ROOT/'before.json').read_text())
 assert app['Config']['Image']==a.OLD_IMAGE and app['Id']==before['containers'][a.APP]['id']
 assert not a.snapshot()['manual_proxy_account_ids'], 'Review pre-existing manual ownership before running older workers'
 assert not marker.exists(), 'Another rollout already owns the manual edit gate'
 store_marker()
 a.save('edit-gate-protected.json',{'at':now(),**owner,'manual_edits_blocked':True,'persistent_configuration_changed':False})
 print(json.dumps({'manual_edits_protected':True}))
else:
 assert app['Config']['Image']=='adobe-relay:'+sha
 assert marker.is_file() and json.loads(marker.read_text())==owner
 if stage=='check':
  status=request_status();assert status==409,'The new worker must reject manual mode edits until release verification'
  out={'at':now(),'http_status':status,'used_nonexistent_account':True,'adobe_requests':False,'account_changes':False}
  a.save('edit-gate-verified.json',out);print(json.dumps(out))
 else:
  core=json.loads((ROOT/'final-verification.json').read_text());cleanup=json.loads((ROOT/'rolling-finished.json').read_text())
  assert core['application_sha']==sha and core['manual_mode_default_off']
  assert all(cleanup[k] for k in ['temporary_container_removed','temporary_nat_rule_removed','rolling_lease_removed'])
  assert subprocess.run(['docker','inspect','adobe-relay-memory-canary'],capture_output=True).returncode!=0
  assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
  checks=a.check_preserved(json.loads((ROOT/'before.json').read_text()),a.snapshot())
  marker.unlink()
  try:
   status=request_status();assert status==404,'Manual edit gate did not open normally'
  except Exception:
   store_marker();raise
  out={'at':now(),'application_sha':sha,'http_status':status,'manual_edits_available':True,'gate_marker_removed':True,'used_nonexistent_account':True,'account_changes':False,'adobe_requests':False,'preserved':checks}
  a.save('manual-edit-activation.json',out);print(json.dumps(out))
