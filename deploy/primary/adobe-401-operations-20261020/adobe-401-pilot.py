"""One manual authorization per confirmed, unpaused sample. Never reset auto budgets."""
import datetime,importlib.util,json,pathlib,subprocess,sys
root=pathlib.Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020')
s=importlib.util.spec_from_file_location('audit',root/'audit.py');a=importlib.util.module_from_spec(s);s.loader.exec_module(a)
ids=[int(r['id'])for r in json.loads((root/'401-auth-sample-private.json').read_text())if r.get('verification')=='auth_failed'][:3]
q="SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,COALESCE(credentials->>'email','') email,extra->'okad'->>'source_key' source_key,md5(credentials->>'cookie') source_hash FROM accounts WHERE deleted_at IS NULL AND platform='adobe' AND type='oauth' AND status='error' AND COALESCE(extra->>'okad_manual_pause','false')<>'true' AND (expires_at IS NULL OR expires_at>NOW()) AND id IN ("+','.join(map(str,ids or[0]))+"))t"
rows=json.loads(a.sql(q));print(json.dumps({'pilot_selected':len(rows),'source_kinds':sorted(set((r.get('source_key')or'').split(':')[0]for r in rows)),'automatic_attempts_reset':False}),flush=True)
program=r'''
import base64,concurrent.futures,datetime,json,os,pathlib,threading,time
from app.db.session import SessionLocal
from app.models.external_member import ExternalMember
from app.services import external_login,firefly
os.umask(0o077)
rows=json.loads(__import__('sys').stdin.readline())
root=pathlib.Path('/app/data/operations/401-manual-repair-20261020');root.mkdir(parents=True,exist_ok=True)
thread=threading.local();original=firefly.register_account

def no_completion(*args,**kwargs):raise firefly._adm.AdminError('reauthorization does not complete profiles or change passwords')
firefly._adm.complete_sub_account=no_completion

def guarded(*args,**kwargs):
 result=original(*args,**kwargs)
 if result.get('user_id')!=thread.expected:raise firefly._adm.AdminError('reauthorization identity mismatch; session not stored')
 return result
firefly.register_account=guarded

def repair(row):
 source=row.get('source_key')or''
 if not source.startswith('external:'):return {'code':'unsupported_source','ok':False}
 mid=int(source.split(':')[1]);marker=root/('account-'+str(row['id'])+'.json')
 if marker.exists():return json.loads(marker.read_text())
 with SessionLocal()as db:
  member=db.get(ExternalMember,mid)
  if not member or member.email.lower()!=row['email'].lower() or member.otp_mode=='manual':return {'ok':False,'code':'manual_or_identity_guard'}
  profile=member.account_profile or{};expected=profile.get('current_profile_id')or''
  if not expected and member.access_token:
   try:expected=json.loads(base64.urlsafe_b64decode(member.access_token.split('.')[1]+'===' )).get('user_id')or''
   except Exception:pass
  if not expected:return {'ok':False,'code':'missing_previous_identity'}
  preference=member.profile_preference or profile.get('current_kind')or''
  if preference not in ['personal','organization']:preference=''
 thread.expected=expected
 initial={'ok':False,'code':'started_outcome_unknown','at':time.time(),'account_id':row['id']}
 fd=os.open(str(marker),os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600);os.write(fd,json.dumps(initial).encode());os.close(fd)
 try:
  result=external_login.login_and_store(mid,automatic=False,network_retries=0,profile_preference=preference,log=lambda message:None)
  out={'account_id':row['id'],'ok':bool(result.get('ok')),'code':result.get('code')or('success'if result.get('ok')else'failed'),'at':time.time(),'credits_known':result.get('credits_available')is not None}
 except Exception as error:out={'account_id':row['id'],'ok':False,'code':type(error).__name__,'at':time.time()}
 marker.write_text(json.dumps(out));return out
with concurrent.futures.ThreadPoolExecutor(max_workers=3)as pool:results=list(pool.map(repair,rows))
print('401_REPAIR_PROGRESS '+json.dumps({'pilot_completed':len(results),'success':sum(bool(r.get('ok'))for r in results),'codes':{c:sum(r.get('code')==c for r in results)for c in sorted(set(r.get('code')for r in results))},'automatic_attempts_reset':False}),flush=True)
'''
# The source and inputs remain private on the primary; only aggregate outcomes enter CI logs.
source=root/'401-manual-pilot-private.py';source.write_text(program);source.chmod(0o600)
command=['docker','exec','-i','adobe-team-cookie-app-1','python','-u','-c',program]
p=subprocess.run(command,input=json.dumps(rows)+'\n',capture_output=True,text=True,timeout=430)
(root/'401-manual-pilot-stdout-private.log').write_text(p.stdout);(root/'401-manual-pilot-stderr-private.log').write_text(p.stderr)
for line in p.stdout.splitlines():
 if line.startswith('401_REPAIR_PROGRESS '):print(line,flush=True)
print(json.dumps({'pilot_process_exit':p.returncode,'at':datetime.datetime.now(datetime.timezone.utc).isoformat()}),flush=True)
assert p.returncode==0,'Pilot process failed; inspect private server logs without repeating unknown attempts'
