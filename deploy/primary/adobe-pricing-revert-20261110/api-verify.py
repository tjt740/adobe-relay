"""Verify all 30 Image 2.6 quality cells via an isolated unbound admin group; never generate."""
import datetime,importlib.util,json,pathlib,urllib.request,urllib.error
ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-pricing-revert-20261110')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
sha=(ROOT/'expected-sha').read_text().strip()
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
name='Adobe Verification [20261110] - isolated'
key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'");assert key
assert a.sql("SELECT count(*) FROM groups WHERE name='"+name+"'")=='0', 'One-shot group already exists; inspect evidence'
def request(method,path,payload=None,expected=200):
 req=urllib.request.Request('http://127.0.0.1:6666/api/v1'+path,data=json.dumps(payload).encode() if payload is not None else None,headers={'x-api-key':key,'Content-Type':'application/json'},method=method)
 try:
  with urllib.request.urlopen(req,timeout=20) as res:
   assert res.status==expected
   return json.load(res).get('data')
 except urllib.error.HTTPError as e:
  if e.code==expected:return None
  raise RuntimeError('Verification API HTTP '+str(e.code)) from None
models=['gpt-image-2.6-flare','gpt-image-2.6-sunburst']
prices={m:{size:{q:(mi+si+qi)/100 for qi,q in enumerate(['low','medium','high','xhigh','max'])} for si,size in enumerate(['1K','2K','4K'])} for mi,m in enumerate(models)}
proof={'application_sha':sha,'models':2,'cells':30,'active_generation':False,'existing_group_changes':False}
try:
 group=request('POST','/admin/groups',{'name':name,'platform':'adobe','rate_multiplier':1,'is_exclusive':True,'subscription_type':'standard','allow_image_generation':False,'image_quality_prices':prices})
 gid=group['id'];proof['temporary_group_id']=gid;a.save('api-verification-started.json',proof)
 assert group['image_quality_prices']==prices
 request('PUT',f'/admin/groups/{gid}',{'status':'inactive'})
 assert a.sql('SELECT count(*) FROM account_groups WHERE group_id='+str(gid))=='0'
 assert request('PUT',f'/admin/groups/{gid}',{'description':'isolated price verification'})['image_quality_prices']==prices
 assert not request('PUT',f'/admin/groups/{gid}',{'image_quality_prices':{}}).get('image_quality_prices')
 assert request('PUT',f'/admin/groups/{gid}',{'image_quality_prices':prices})['image_quality_prices']==prices
 assert request('GET',f'/admin/groups/{gid}')['image_quality_prices']==prices
 for invalid in [ {'gpt-image-2':{'1K':{'low':1}}}, {'gpt-image-2.5-flare':{'1K':{'low':1}}}, {'gpt-image-2.6-flare':{'8K':{'high':1}}}, {'gpt-image-2.6-flare':{'1K':{'low':-1}}} ]:
  request('PUT',f'/admin/groups/{gid}',{'image_quality_prices':invalid},400)
 assert request('GET',f'/admin/groups/{gid}')['image_quality_prices']==prices
 rows=request('GET','/admin/groups?adobe_model=Verification%20%5B20261110%5D&page=1&page_size=1')
 assert rows['total']==1 and rows['items'][0]['id']==gid
 rows2=request('GET','/admin/groups?adobe_model=Verification%20%5B20261110%5D&page=2&page_size=1')
 assert rows2['total']==1 and rows2['items']==[]
 try:
  directory=request('GET','/admin/groups/adobe-models')
  assert directory['source']=='adobe' and directory['models'];proof['adobe_directory']='live'
 except RuntimeError:
  proof['adobe_directory']='unavailable; frontend fallback applies'
 proof.update(omit_preserves=True,empty_object_clears=True,zero_preserved=True,reload_matches=True,invalid_rejected_without_changes=True,filter_before_pagination=True,passed=True)
finally:
 ids=json.loads(a.sql("SELECT coalesce(json_agg(id),'[]') FROM groups WHERE name='"+name+"' AND deleted_at IS NULL"));assert len(ids)<=1
 for gid in ids:
  assert a.sql('SELECT count(*) FROM account_groups WHERE group_id='+str(gid))=='0'
  assert a.sql('SELECT count(*) FROM api_keys WHERE group_id='+str(gid))=='0'
  request('DELETE',f'/admin/groups/{gid}')
 assert a.sql("SELECT count(*) FROM groups WHERE name='"+name+"' AND deleted_at IS NULL")=='0'
 proof['temporary_group_deleted']=True
 proof['at']=datetime.datetime.now(datetime.timezone.utc).isoformat()
 a.save('api-verification.json',proof)
print(json.dumps(proof))
