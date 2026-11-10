"""Two one-shot public generations verify actual on/off pricing and database debits."""
import base64,datetime,importlib.util,json,os,pathlib,subprocess,time,urllib.request,urllib.error
from decimal import Decimal
from live_validation import validate_png,parse_pipeline_line,pipeline_matches,pipeline_evidence
os.umask(0o077)
ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-multi-pricing-20261111')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
SHA=(ROOT/'expected-sha').read_text().strip();NAME='release-quality-mode-20261111-temporary';USER=1
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+SHA
assert not (ROOT/'live-billing-started.json').exists(),'One-shot verification already started; no replay'
key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
def admin(method,path,payload=None,token=None):
 headers={'Content-Type':'application/json','x-api-key':key}
 if token:headers={'Content-Type':'application/json','Authorization':'Bearer '+token}
 req=urllib.request.Request('http://127.0.0.1:6666/api/v1'+path,data=json.dumps(payload).encode() if payload is not None else None,headers=headers,method=method)
 try:
  with urllib.request.urlopen(req,timeout=35) as h:return json.load(h).get('data')
 except urllib.error.HTTPError as e:raise RuntimeError('Admin API HTTP '+str(e.code)) from None
def groups():return json.loads(a.sql("SELECT coalesce(json_agg(group_id ORDER BY group_id),'[]') FROM user_allowed_groups WHERE user_id=1"))
source=a.sql("SELECT ag.group_id FROM account_groups ag JOIN accounts a ON a.id=ag.account_id JOIN groups g ON g.id=ag.group_id WHERE a.id=1848 AND a.status='active' AND a.schedulable AND a.deleted_at IS NULL AND g.deleted_at IS NULL ORDER BY ag.group_id LIMIT 1")
assert source,'Known successful account is no longer available; no test submitted'
source=int(source)
assert a.sql("SELECT count(*) FROM groups WHERE name='"+NAME+"'")=='0'
assert a.sql("SELECT count(*) FROM api_keys WHERE name='"+NAME+"' AND user_id=1")=='0'
c=a.inspect(a.APP);env=dict(v.split('=',1) for v in c['Config']['Env']);email=a.sql("SELECT email FROM users WHERE id=1 AND role='admin' AND status='active' AND deleted_at IS NULL")
assert email==env['ADMIN_EMAIL']
login=admin('POST','/auth/login',{'email':email,'password':env['ADMIN_PASSWORD']});env.clear()
assert not login.get('requires_2fa') and login['user']['id']==USER and login['user']['role']=='admin'
token=login['access_token'];original=groups();gid=None;kid=None;cases=[]
a.save('live-billing-started.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':SHA,'source_group_id':source,'original_allowed_groups':original,'planned_outer_posts':2,'never_replay':True,'cleanup_required':True})
try:
 prices={'gpt-image-2.6-flare':{'1K':{'medium':0.0123}}}
 group=admin('POST','/admin/groups',{'name':NAME,'platform':'adobe','rate_multiplier':1.5,'is_exclusive':True,'subscription_type':'standard','allow_image_generation':True,'image_rate_independent':True,'image_rate_multiplier':1.5,'image_price_1k':0.0234,'image_quality_pricing_enabled':True,'image_quality_prices':prices,'copy_accounts_from_group_ids':[source]})
 gid=group['id'];assert group['image_quality_pricing_enabled'] is True and group['image_quality_prices']==prices
 a.save('live-billing-group.json',{'id':gid,'source_group_id':source,'customer_prices_changed':False})
 admin('PUT','/admin/users/1',{'allowed_groups':original+[gid]});assert groups()==sorted(original+[gid])
 created=admin('POST','/keys',{'name':NAME,'group_id':gid,'quota':0.2,'expires_in_days':1},token)
 kid=created['id'];assert created['user_id']==USER and created['group_id']==gid and created['quota']==0.2
 a.save('live-billing-key.json',{x:created[x] for x in ['id','user_id','group_id','quota','expires_at']});created.clear()
 secret=a.sql('SELECT key FROM api_keys WHERE id='+str(kid));headers={'Authorization':'Bearer '+secret,'Content-Type':'application/json'}
 for name,enabled,price in [('new',True,Decimal('0.0123')),('legacy',False,Decimal('0.0234'))]:
  saved=admin('PUT','/admin/groups/'+str(gid),{'image_quality_pricing_enabled':enabled});assert saved['image_quality_pricing_enabled'] is enabled and saved['image_quality_prices']==prices
  started=datetime.datetime.now(datetime.timezone.utc).isoformat();balance=Decimal(a.sql('SELECT balance FROM users WHERE id=1'));quota=Decimal(a.sql('SELECT quota_used FROM api_keys WHERE id='+str(kid)))
  payload={'model':'gpt-image-2.6-flare','quality':'medium','size':'1024x1024','n':1,'response_format':'b64_json','output_format':'png','prompt':'A blue ceramic cup beside an orange on a pale wooden table, natural window light. Billing mode verification 20261111 '+name+'.'}
  a.save('live-'+name+'-started.json',{'at':started,'model':payload['model'],'quality':'medium','outer_posts':1,'mode_enabled':enabled,'never_replay':True})
  row={'name':name,'application_sha':SHA,'mode_enabled':enabled,'outer_posts':1,'success':False,'at':started};began=time.monotonic()
  try:
   req=urllib.request.Request('https://creative.cheap/v1/images/generations',data=json.dumps(payload).encode(),headers=headers,method='POST')
   with urllib.request.urlopen(req,timeout=420) as res:row['http_status']=res.status;row['http_request_id']=res.headers.get('X-Request-ID');body=json.load(res)
   assert body.get('quality')=='medium' and len(body['data'])==1
   data=base64.b64decode(body['data'][0]['b64_json'],validate=True);row['image']=validate_png(data,(1024,1024));(ROOT/('live-'+name+'.png')).write_bytes(data)
   usage=[]
   for _ in range(15):
    usage=json.loads(a.sql("SELECT coalesce(json_agg(x),'[]') FROM (SELECT request_id,account_id,model,upstream_model,image_count,image_size,image_quality,total_cost,actual_cost,rate_multiplier,created_at FROM usage_logs WHERE api_key_id="+str(kid)+" AND created_at>='"+started+"'::timestamptz) x"))
    if usage:break
    time.sleep(1)
   assert len(usage)==1;u=usage[0];row['usage']=u
   assert u['model']=='gpt-image-2.6-flare' and u['image_quality']=='medium' and u['image_size']=='1K' and u['image_count']==1
   expected=price*Decimal('1.5');assert abs(Decimal(str(u['total_cost']))-price)<Decimal('0.00000001') and abs(Decimal(str(u['actual_cost']))-expected)<Decimal('0.00000001') and u['rate_multiplier']==1.5
   delta=quota-Decimal(a.sql('SELECT quota_used FROM api_keys WHERE id='+str(kid)));assert abs(-delta-expected)<Decimal('0.00000001');row['key_quota_debit']=float(-delta)
   other=int(a.sql("SELECT count(*) FROM usage_logs WHERE user_id=1 AND api_key_id<>"+str(kid)+" AND created_at>='"+started+"'::timestamptz"))
   actual_delta=balance-Decimal(a.sql('SELECT balance FROM users WHERE id=1'));row['admin_balance_delta']=float(actual_delta);row['other_admin_usage_count']=other
   assert other==0 and abs(actual_delta-expected)<Decimal('0.00000001'),'Balance changed concurrently or debit did not match; do not replay'
   assert a.sql("SELECT count(*) FROM usage_billing_dedup WHERE api_key_id="+str(kid)+" AND request_id='"+u['request_id'].replace("'","''")+"'")=='1'
   found=None
   for _ in range(10):
    chunks=[]
    for cmd in [['docker','exec',a.APP,'tail','-c','16777216','/app/data/logs/sub2api.log'],['docker','logs','--since','15m','--tail','12000',a.APP]]:
     r=subprocess.run(cmd,capture_output=True,text=True,timeout=30)
     if r.returncode==0:chunks.extend([r.stdout,r.stderr])
    for chunk in chunks:
     for line in chunk.splitlines():
      e=parse_pipeline_line(line)
      if pipeline_matches(e,u,row['http_request_id'],kid):found=pipeline_evidence(e);break
     if found:break
    if found:break
    time.sleep(1)
   assert found;row['pipeline']=found;applied,=found['applied_submissions'];assert applied['submit_attempts']==1 and applied['submitted_detail_level']==3 and applied['submitted_model_version']=='gpt-image-2.6-star-swirl'
   row['success']=True
  except urllib.error.HTTPError as e:
   row['http_status']=e.code
   try:err=json.loads(e.read(65536)).get('error',{});row['error']={k:v for k,v in err.items() if k in ['type','code','param','stage','source','upstream_status','request_id']}
   except Exception:pass
  except Exception as e:row['exception_type']=type(e).__name__
  finally:
   row['seconds']=round(time.monotonic()-began,3);a.save('live-'+name+'-result.json',row);cases.append(row);print(json.dumps(row),flush=True)
  assert row['success'],'Failure or unknown result; no automatic replay'
 restored=admin('PUT','/admin/groups/'+str(gid),{'image_quality_pricing_enabled':True});assert restored['image_quality_prices']==prices
 a.save('live-billing-verification.json',{'application_sha':SHA,'cases':cases,'live_billing_verified':True,'real_database_debits_verified':True,'saved_prices_preserved':True,'replayed_requests':0})
finally:
 rows=json.loads(a.sql("SELECT coalesce(json_agg(id),'[]') FROM api_keys WHERE user_id=1 AND name='"+NAME+"' AND deleted_at IS NULL"));assert len(rows)<=1
 for k in rows:admin('DELETE','/keys/'+str(k),token=token)
 owned=json.loads(a.sql("SELECT coalesce(json_agg(id),'[]') FROM groups WHERE name='"+NAME+"' AND deleted_at IS NULL"));assert len(owned)<=1
 for g in owned:
  current=groups();remaining=[v for v in current if v!=g];assert set(original).issubset(remaining)
  if current!=remaining:admin('PUT','/admin/users/1',{'allowed_groups':remaining})
  admin('DELETE','/admin/groups/'+str(g))
  assert a.sql('SELECT count(*) FROM account_groups WHERE group_id='+str(g))=='0'
 assert a.sql("SELECT count(*) FROM api_keys WHERE user_id=1 AND name='"+NAME+"' AND deleted_at IS NULL")=='0'
 assert a.sql("SELECT count(*) FROM groups WHERE name='"+NAME+"' AND deleted_at IS NULL")=='0'
 a.save('live-billing-cleanup.json',{'temporary_key_deleted':True,'temporary_group_deleted':True,'temporary_grant_removed':True,'original_allowed_groups':original,'current_allowed_groups':groups(),'customer_prices_changed':False})
