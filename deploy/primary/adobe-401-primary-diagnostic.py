"""Classify the original account failure, session identity and incident counter."""
import base64,collections,importlib.util,json,pathlib,re,subprocess,urllib.request
root=pathlib.Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020');new=root.parent/'adobe-401-repair-20261020'
s=importlib.util.spec_from_file_location('audit',root/'audit.py');a=importlib.util.module_from_spec(s);s.loader.exec_module(a)
helper=root.parent/'adobe-session-renewal-20261020'/'verify-native-host.py';i=int(re.search(r'/accounts/(\d+)',helper.read_text())[1])
def record():return json.loads(a.sql("SELECT json_build_object('id',a.id,'status',a.status,'schedulable',a.schedulable,'error',a.error_message,'token',a.credentials->>'access_token','cookie',a.credentials->>'cookie','runtime',a.credentials->>'adobe_session_cookie','source_key',a.extra->'okad'->>'source_key','version',a.extra->'okad'->>'version','received_at',a.extra->'okad'->>'received_at','latest_image_code',a.extra->'adobe_health'->'latest'->>'code','attempts',r.attempts,'last_attempt_at',r.last_attempt_at,'finished_at',r.finished_at) FROM accounts a LEFT JOIN adobe_cookie_recovery_attempts r ON r.account_id=a.id WHERE a.id="+str(i)))
before=record();key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/'+str(i)+'/usage?source=active&force=true',headers={'x-api-key':key})
with urllib.request.urlopen(req,timeout=115)as response:data=json.loads(response.read()).get('data')or{}
after=record();message=str(data.get('error')or'').lower()
kind='identity_mismatch'if any(s in message for s in['identity changed','identity does not match'])else'guest'if'guest'in message else'session_rejected'if any(s in message for s in['no longer valid','invalid_credentials','session is unavailable'])else'token_rejected'if any(s in message for s in['invalid bearer','invalid access token','credits request failed: 401'])else'other'
program="""import json,sys
from app.db.session import SessionLocal
from app.models.external_member import ExternalMember
key=json.loads(sys.stdin.readline())
with SessionLocal()as db:
 m=db.get(ExternalMember,int(key.split(':')[1]));p=m.account_profile or{}
 print(json.dumps({'expected_id':p.get('current_profile_id'),'kind':p.get('current_kind')}))
"""
identity={}
if str(before.get('source_key')or'').startswith('external:'):
 p=subprocess.run(['docker','exec','-i','adobe-team-cookie-app-1','python','-c',program],input=json.dumps(before['source_key'])+'\n',capture_output=True,text=True,timeout=30)
 if p.returncode==0:identity=json.loads(p.stdout)
uid=None
try:uid=json.loads(base64.urlsafe_b64decode((after.get('token')or'').split('.')[1]+'===')).get('user_id')
except Exception:pass
(new/'original-primary-diagnostic-private.json').write_text(json.dumps({'before':before,'after':after,'quota':data,'identity':identity}));(new/'original-primary-diagnostic-private.json').chmod(0o600)
print(json.dumps({'quota_error_code':data.get('error_code'),'quota_needs_reauth':data.get('needs_reauth'),'quota_has_error':bool(data.get('error')),'failure_kind':kind,'status':after['status'],'latest_image_code':after['latest_image_code'],'automatic_attempts':after['attempts'],'incident_last_attempt':after['last_attempt_at'],'incident_finished_at':after['finished_at'],'source_version':after['version'],'source_received_at':after['received_at'],'expected_profile_kind':identity.get('kind'),'token_identity_matches_source':bool(uid)and uid==identity.get('expected_id'),'source_ims_sid_count':sum(part.strip().startswith('ims_sid=')for part in(before.get('cookie')or'').split(';')),'runtime_ims_sid_count':sum(part.strip().startswith('ims_sid=')for part in(before.get('runtime')or'').split(';'))}),flush=True)
