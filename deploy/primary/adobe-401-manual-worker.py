"""One explicit manual repair per account. Normal coordinator/delivery own every write."""
import base64,collections,concurrent.futures,hashlib,json,os,pathlib,threading,time
from app.db.session import SessionLocal
from app.models.external_member import ExternalMember
from app.services import external_login,external_sub2,firefly,sub2_client
os.umask(0o077)
rows=json.loads(__import__('sys').stdin.readline())
root=pathlib.Path('/app/data/operations/401-manual-repair-20261020');root.mkdir(parents=True,exist_ok=True)
with SessionLocal()as db:cfg=external_sub2.config(db)
base=sub2_client.api_base(cfg['base_url'])+'/admin/accounts/'
thread=threading.local();original=firefly.register_account

def no_completion(*args,**kwargs):raise firefly._adm.AdminError('reauthorization does not complete profiles or change passwords')
firefly._adm.complete_sub_account=no_completion

def guarded(*args,**kwargs):
 result=original(*args,**kwargs)
 if result.get('user_id')!=thread.expected:raise firefly._adm.AdminError('reauthorization identity mismatch; session not stored')
 return result
firefly.register_account=guarded

def request(row,path):
 status,raw=sub2_client._request('GET',base+str(row['id'])+path,cfg['admin_token'],timeout=110)
 if status!=200:raise RuntimeError('admin_api_http_'+str(status))
 return json.loads(raw).get('data')or{}

def finish(marker,out):
 marker.write_text(json.dumps(out));marker.chmod(0o600);return out

def repair(row):
 source=row.get('source_key')or'';marker=root/('account-'+str(row['id'])+'.json')
 if marker.exists():return json.loads(marker.read_text())
 out={'account_id':row['id'],'ok':False,'code':'not_started','at':time.time()}
 try:
  latest=request(row,'')
  if latest.get('status')!='error' or (latest.get('extra')or{}).get('okad_manual_pause')is True:return {**out,'code':'account_changed_or_paused'}
  cookie=(latest.get('credentials')or{}).get('cookie')or''
  if hashlib.md5(cookie.encode()).hexdigest()!=row['source_hash']:return {**out,'code':'source_changed'}
  usage=request(row,'/usage?source=active&force=true')
  if not usage.get('needs_reauth') and usage.get('error_code')!='unauthenticated':
   if not usage.get('error_code') and not usage.get('error'):return finish(marker,{**out,'ok':True,'code':'auth_valid_no_login'})
   return {**out,'code':'temporary_or_non_auth_failure'}
  latest=request(row,'')
  if latest.get('status')!='error' or (latest.get('extra')or{}).get('okad_manual_pause')is True:return {**out,'code':'account_changed_or_paused'}
  if hashlib.md5(((latest.get('credentials')or{}).get('cookie')or'').encode()).hexdigest()!=row['source_hash']:return {**out,'code':'source_changed'}
  if not source.startswith('external:'):return {**out,'code':'unsupported_source'}
  mid=int(source.split(':')[1])
  with SessionLocal()as db:
   member=db.get(ExternalMember,mid)
   if not member or member.email.lower()!=row['email'].lower() or member.otp_mode=='manual':return {**out,'code':'manual_or_identity_guard'}
   profile=member.account_profile or{};expected=profile.get('current_profile_id')or''
   if not expected and member.access_token:
    try:expected=json.loads(base64.urlsafe_b64decode(member.access_token.split('.')[1]+'===')).get('user_id')or''
    except Exception:pass
   if not expected:return {**out,'code':'missing_previous_identity'}
   preference=member.profile_preference or profile.get('current_kind')or''
   if preference not in ['personal','organization']:preference=''
  thread.expected=expected
  fd=os.open(str(marker),os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
  os.write(fd,json.dumps({**out,'code':'started_outcome_unknown'}).encode());os.close(fd)
  result=external_login.login_and_store(mid,automatic=False,network_retries=0,profile_preference=preference,log=lambda message:None)
  return finish(marker,{**out,'ok':bool(result.get('ok')),'code':result.get('code')or('success'if result.get('ok')else'failed'),'at':time.time(),'credits_known':result.get('credits_available')is not None})
 except Exception as error:
  # Never retry an unknown login outcome. No response body or credentials are emitted.
  if marker.exists():return finish(marker,{**out,'code':type(error).__name__})
  return {**out,'code':type(error).__name__}

results=[]
with concurrent.futures.ThreadPoolExecutor(max_workers=3)as pool:
 for future in concurrent.futures.as_completed([pool.submit(repair,r)for r in rows]):
  results.append(future.result())
  summary={'completed':len(results),'selected':len(rows),'success':sum(bool(r.get('ok'))for r in results),'codes':dict(collections.Counter(r.get('code')for r in results)),'automatic_attempts_reset':False,'at':time.time()}
  print('401_REPAIR_PROGRESS '+json.dumps(summary),flush=True)
print('401_REPAIR_FINISHED '+json.dumps(summary if results else{'completed':0,'selected':0}),flush=True)
