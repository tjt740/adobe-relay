"""One explicitly authorized GPT Image 2 test; never replay a lost response."""
import base64,datetime,hashlib,importlib.util,json,os,pathlib,re,subprocess,urllib.request
ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-route-health-20261023')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
os.umask(0o077)
assert (ROOT/'rolling-finished.json').exists()
sha=(ROOT/'expected-sha').read_text().strip()
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
assert json.loads((ROOT/'artifact'/'ci-proof.json').read_text())['application_sha']==sha
before=json.loads(a.sql("SELECT json_build_object('id',id,'present',deleted_at IS NULL,'status',status,'schedulable',schedulable,'manual_pause',extra->'okad_manual_pause','cookie_hash',md5(COALESCE(credentials->>'cookie','')),'proxy_id',proxy_id,'quarantined',EXISTS(SELECT 1 FROM adobe_proxy_408_blocks b WHERE b.account_id=1172) OR EXISTS(SELECT 1 FROM adobe_proxy_408_holds h WHERE h.account_id=1172)) FROM accounts WHERE id=1172"))
assert before and before['present'] and before['status']=='active' and (before['schedulable'] or before['quarantined']) and before['manual_pause'] is not True
assert a.sql("SELECT EXISTS(SELECT 1 FROM adobe_route_exclusions WHERE proxy_id=1290)")=='t'
assert a.sql("SELECT EXISTS(SELECT 1 FROM adobe_proxy_test_leases WHERE account_id=1172 AND expires_at>NOW())")=='f'
start=datetime.datetime.now(datetime.timezone.utc).isoformat()
marker=ROOT/'scoped-live-test-started.json'
fd=os.open(marker,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
with os.fdopen(fd,'w') as f:json.dump({'at':start,'application_sha':sha,'account_id':1172,'model':'gpt-image-2','max_distinct_sources':3,'never_replay_if_interrupted':True},f)
key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
request=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/1172/test',data=json.dumps({'model_id':'gpt-image-2'}).encode(),headers={'x-api-key':key,'Content-Type':'application/json'},method='POST')
events=[];images=[];success=False;problem=None
try:
 with urllib.request.urlopen(request,timeout=210) as response:
  for line in response:
   if not line.startswith(b'data: '):continue
   event=json.loads(line[6:]);events.append(event.get('type'))
   if event.get('type')=='image':
    value=event.get('image_url','')
    if value.startswith('data:image/') and ';base64,' in value:
     data=base64.b64decode(value.split(';base64,',1)[1],validate=True)
     images.append({'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest(),'mime':value.split(';',1)[0][5:]})
   if event.get('type')=='test_complete':success=event.get('success') is True
except Exception as error:
 problem=type(error).__name__ # Never persist URLs, response bodies or credentials.
post=json.loads(a.sql("SELECT json_build_object('status',status,'schedulable',schedulable,'manual_pause',extra->'okad_manual_pause','cookie_hash',md5(COALESCE(credentials->>'cookie','')),'proxy_id',proxy_id,'review',extra->'adobe_route_review') FROM accounts WHERE id=1172"))
observations=json.loads(a.sql("SELECT COALESCE(json_agg(json_build_object('proxy_id',proxy_id,'source_id',source_id,'model',scope,'success',success,'code',status_code,'stage',stage,'submission_rejected',submission_rejected) ORDER BY id),'[]') FROM adobe_route_observations WHERE account_id=1172 AND started_at>='"+start+"'::timestamptz"))
assert all(o['proxy_id']!=1290 for o in observations)
if post['review']:assert post['review']['attempts']<=3
preserved={}
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'account_id':1172,'model':'gpt-image-2','success':success,'images':images,'events':events,'transport_exception':problem,'route_observations_in_window':observations,'review':post['review'],'cookie_unchanged':post['cookie_hash']==before['cookie_hash'],'manual_pause_preserved':post['manual_pause']==before['manual_pause'],'before_status':before['status'],'status':post['status'],'before_schedulable':before['schedulable'],'schedulable':post['schedulable'],'proxy_id':post['proxy_id'],'preserved':preserved,'test_endpoint_posted':True,'actual_adobe_test':bool(images) or any(o['stage'] in ('submit','poll','download') for o in observations),'login_requested':False,'mail_sent':False,'retried_outer_post':False}
a.save('scoped-live-test-response.json',result)
result['preserved']=a.check_preserved(json.loads((ROOT/'before.json').read_text()),a.snapshot())
a.save('scoped-live-test.json',result)
print(json.dumps(result,ensure_ascii=False))
assert result['cookie_unchanged'] and result['manual_pause_preserved']
if success:assert len(images)==1 and images[0]['bytes']>0 and any(o['success'] for o in observations)
