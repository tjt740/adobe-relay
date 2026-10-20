"""Refresh the three pilot sessions after idle time; persist through the normal guarded API."""
import collections,concurrent.futures,datetime,importlib.util,json,pathlib,urllib.request
root=pathlib.Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020')
s=importlib.util.spec_from_file_location('audit',root/'audit.py');a=importlib.util.module_from_spec(s);s.loader.exec_module(a)
p=root/'401-pilot-stabilization-private.json'
if p.exists():
 data=json.loads(p.read_text());print(json.dumps({'prior_stabilization':True,'at':data['at'],'results':dict(collections.Counter(r['verification']for r in data['results']))}));raise SystemExit()
ids=[int(r['id'])for r in json.loads((root/'401-auth-sample-private.json').read_text())if r.get('verification')=='auth_failed'][:3]
key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
def call(i):
 try:
  query="SELECT json_build_object('paused',extra->'okad_manual_pause','cookie_hash',md5(credentials->>'cookie')) FROM accounts WHERE deleted_at IS NULL AND id="+str(i)
  before=json.loads(a.sql(query))
  if before.get('paused')is True:return {'id':i,'verification':'operator_paused_skipped'}
  req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/'+str(i)+'/okad-verify',data=b'{}',headers={'x-api-key':key,'Content-Type':'application/json'})
  with urllib.request.urlopen(req,timeout=80)as r:data=json.loads(r.read()).get('data')or{}
  after=json.loads(a.sql(query))
  return {'id':i,'verification':(data.get('integration')or{}).get('verification'),'status':data.get('status'),'schedulable':data.get('schedulable'),'source_cookie_and_pause_preserved':before==after}
 except Exception as error:return {'id':i,'verification':type(error).__name__}
with concurrent.futures.ThreadPoolExecutor(max_workers=2)as pool:results=list(pool.map(call,ids))
data={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'results':results,'login_or_image_requests':False};p.write_text(json.dumps(data));p.chmod(0o600)
print(json.dumps({'stabilization_at':data['at'],'pilot_count':len(results),'results':dict(collections.Counter(r['verification']for r in results)),'source_cookie_and_pause_preserved':all(r.get('source_cookie_and_pause_preserved')for r in results),'logins_or_images':False}),flush=True)
