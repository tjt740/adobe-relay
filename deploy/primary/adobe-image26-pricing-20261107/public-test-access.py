"""Use an isolated administrator-owned group and key; never change customer prices."""
import datetime,hashlib,json,os,pathlib,subprocess,sys,urllib.request,urllib.error
from public_contract_guard import PRICES
os.umask(0o077)
ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-image26-pricing-20261107')
NAME='release-image26-pricing-20261107-temporary';USER=1;SOURCE_GROUP=2
CONFIRMATION=len(sys.argv)==2 and sys.argv[1]=='confirmation'
assert len(sys.argv)==1 or CONFIRMATION
if CONFIRMATION:
 NAME+='-confirmation'
 review=json.loads((ROOT/'explicit-rejection-review.json').read_text())
 assert review['explicit_preacceptance_rejection'] and not review['accepted_or_unknown'] and review['usage_count']==0
 assert hashlib.sha256((ROOT/'public/flare-xhigh-result.json').read_bytes()).hexdigest()==review['original_result_sha256']
 assert not (ROOT/'public-confirmation').exists(),'Confirmation is one-shot'
SHA=(ROOT/'expected-sha').read_text().strip()
def sql(q):
 return subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input='BEGIN READ ONLY; '+q+'; COMMIT;',capture_output=True,text=True,check=True,timeout=25).stdout.strip()
def groups():return json.loads(sql("SELECT coalesce(json_agg(group_id ORDER BY group_id),'[]') FROM user_allowed_groups WHERE user_id=1"))
def save(name,data):
 if CONFIRMATION:name=name.replace('business-','business-confirmation-',1).replace('pricing-api-roundtrip.json','pricing-api-roundtrip-confirmation.json')
 with (ROOT/name).open('x') as f:json.dump(data,f,indent=2)
def request(method,path,payload=None,token=None):
 headers={'Content-Type':'application/json'}
 if token:headers['Authorization']='Bearer '+token
 req=urllib.request.Request('http://127.0.0.1:6666/api/v1'+path,data=json.dumps(payload).encode() if payload is not None else None,headers=headers,method=method)
 try:
  with urllib.request.urlopen(req,timeout=35) as h:return json.load(h).get('data')
 except urllib.error.HTTPError as e:raise RuntimeError('Admin API failed with HTTP '+str(e.code)) from None
c=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
assert c['Config']['Image']=='adobe-relay:'+SHA and c['State']['Health']['Status']=='healthy'
assert sql("SELECT count(*) FROM groups WHERE name='"+NAME+"'")=='0'
assert sql("SELECT count(*) FROM api_keys WHERE user_id=1 AND name='"+NAME+"'")=='0'
assert int(sql("SELECT count(*) FROM accounts a JOIN account_groups ag ON a.id=ag.account_id WHERE ag.group_id=2 AND a.platform='adobe' AND a.status='active' AND a.schedulable AND a.deleted_at IS NULL"))>0
original=groups()
env=dict(v.split('=',1) for v in c['Config']['Env'])
email=sql("SELECT email FROM users WHERE id=1 AND role='admin' AND status='active' AND deleted_at IS NULL")
assert email==env['ADMIN_EMAIL']
data=request('POST','/auth/login',{'email':email,'password':env['ADMIN_PASSWORD']})
assert not data.get('requires_2fa') and data['user']['id']==USER and data['user']['role']=='admin'
token=data['access_token'];env.clear()
save('business-access-started.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'source_group_id':SOURCE_GROUP,'original_allowed_groups':original,'quota':2,'cleanup_required':True,'customer_prices_changed':False})
GROUP=None;outcome=None
try:
 payload={'name':NAME,'platform':'adobe','rate_multiplier':1.5,'is_exclusive':True,'subscription_type':'standard','allow_image_generation':True,'image_rate_independent':True,'image_rate_multiplier':2,'image_price_1k':0.09,'image_price_2k':0.09,'image_price_4k':0.09,'image_quality_prices':PRICES,'copy_accounts_from_group_ids':[SOURCE_GROUP]}
 group=request('POST','/admin/groups',payload,token);GROUP=group['id']
 assert group['name']==NAME and group['image_quality_prices']==PRICES
 save('business-group-created.json',{'id':GROUP,'name':NAME,'source_group_id':SOURCE_GROUP,'price_cells':sum(len(q) for sizes in PRICES.values() for q in sizes.values()),'customer_prices_changed':False})
 # Exercise omit, explicit clear, numeric zero and read-after-update via the real admin API.
 assert request('PUT','/admin/groups/'+str(GROUP),{'description':'isolated release verification'},token)['image_quality_prices']==PRICES
 assert not request('PUT','/admin/groups/'+str(GROUP),{'image_quality_prices':{}},token).get('image_quality_prices')
 assert request('PUT','/admin/groups/'+str(GROUP),{'image_quality_prices':PRICES},token)['image_quality_prices']==PRICES
 assert request('GET','/admin/groups/'+str(GROUP),token=token)['image_quality_prices']==PRICES
 save('pricing-api-roundtrip.json',{'models':2,'configured_cells':29,'omit_preserves':True,'empty_object_clears':True,'zero_preserved':True,'reload_matches':True,'group_id':GROUP})
 request('PUT','/admin/users/1',{'allowed_groups':original+[GROUP]},token)
 assert groups()==sorted(original+[GROUP])
 key=request('POST','/keys',{'name':NAME,'group_id':GROUP,'quota':2,'expires_in_days':1},token)
 assert key['user_id']==USER and key['group_id']==GROUP and key['quota']==2 and key['expires_at']
 save('business-key-created.json',{k:key[k] for k in ['id','user_id','group_id','name','quota','expires_at']})
 key_id=key['id'];key.clear()
 outcome=subprocess.run(['python3',str(ROOT/'public-test.py'),str(key_id)]+(['confirmation'] if CONFIRMATION else []))
finally:
 # Lookup by our unique name also covers a create whose HTTP outcome was unclear.
 rows=json.loads(sql("SELECT coalesce(json_agg(json_build_object('id',id,'group_id',group_id)),'[]') FROM api_keys WHERE user_id=1 AND name='"+NAME+"' AND deleted_at IS NULL"))
 assert len(rows)<=1
 for k in rows:request('DELETE','/keys/'+str(k['id']),token=token)
 assert sql("SELECT count(*) FROM api_keys WHERE user_id=1 AND name='"+NAME+"' AND deleted_at IS NULL")=='0'
 owned=json.loads(sql("SELECT coalesce(json_agg(id),'[]') FROM groups WHERE name='"+NAME+"' AND deleted_at IS NULL"));assert len(owned)<=1
 for gid in owned:
  assert sql('SELECT count(*) FROM api_keys WHERE group_id='+str(gid)+' AND deleted_at IS NULL')=='0'
  current=groups();remaining=[g for g in current if g!=gid]
  assert set(original).issubset(remaining)
  if remaining!=current:request('PUT','/admin/users/1',{'allowed_groups':remaining},token)
  request('DELETE','/admin/groups/'+str(gid),token=token)
  assert sql('SELECT count(*) FROM account_groups WHERE group_id='+str(gid))=='0'
 assert sql("SELECT count(*) FROM groups WHERE name='"+NAME+"' AND deleted_at IS NULL")=='0'
 save('business-access-cleanup.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'temporary_key_deleted':True,'temporary_group_deleted':True,'temporary_grant_removed':True,'original_allowed_groups':original,'current_allowed_groups':groups(),'credentials_persisted':False,'customer_prices_changed':False})
assert outcome and outcome.returncode==0,'Failure or unknown result; preserve evidence, no replay'
