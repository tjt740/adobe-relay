"""One guarded manual authorization for the original primary account."""
import ast,datetime,importlib.util,json,os,pathlib,re,subprocess,urllib.request
os.umask(0o077)
root=pathlib.Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020');new=root.parent/'adobe-401-repair-20261020';ops=pathlib.Path(__file__).parent
s=importlib.util.spec_from_file_location('audit',root/'audit.py');a=importlib.util.module_from_spec(s);s.loader.exec_module(a)
helper=root.parent/'adobe-session-renewal-20261020'/'verify-native-host.py';i=int(re.search(r'/accounts/(\d+)',helper.read_text())[1])
def record():
 return json.loads(a.sql("SELECT json_build_object('id',id,'email',COALESCE(credentials->>'email',''),'source_key',extra->'okad'->>'source_key','source_hash',md5(credentials->>'cookie'),'status',status,'paused',extra->'okad_manual_pause','expired',expires_at IS NOT NULL AND expires_at<=NOW(),'version',extra->'okad'->>'version') FROM accounts WHERE deleted_at IS NULL AND platform='adobe' AND type='oauth' AND id="+str(i)))
row=record();assert row['status']=='error'and row['paused']is not True and not row['expired']and row['source_key'].startswith('external:')
key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
def quota():
 req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/'+str(i)+'/usage?source=active&force=true',headers={'x-api-key':key})
 with urllib.request.urlopen(req,timeout=115)as r:return json.loads(r.read()).get('data')or{}
before=quota()
if not(before.get('needs_reauth')or before.get('error_code')=='unauthenticated'):
 print(json.dumps({'primary_authorization_started':False,'quota_has_error':bool(before.get('error')),'error_code':before.get('error_code')}),flush=True)
 raise SystemExit(0)
assert record()==row,'Account changed before authorization'
program=None
for node in ast.parse((ops/'adobe-401-pilot.py').read_text()).body:
 if isinstance(node,ast.Assign)and any(isinstance(x,ast.Name)and x.id=='program'for x in node.targets):program=ast.literal_eval(node.value)
assert program
program+="\nprint('401_REPAIR_RESULT '+json.dumps(results),flush=True)\n"
p=subprocess.run(['docker','exec','-i','adobe-team-cookie-app-1','python','-u','-c',program],input=json.dumps([row])+'\n',capture_output=True,text=True,timeout=430)
(new/'original-primary-reauth-stdout-private.log').write_text(p.stdout);(new/'original-primary-reauth-stderr-private.log').write_text(p.stderr)
results=[]
for line in p.stdout.splitlines():
 if line.startswith('401_REPAIR_RESULT '):results=json.loads(line.split(' ',1)[1])
print(json.dumps({'primary_authorization_process_exit':p.returncode,'primary_authorization_success':len(results)==1 and bool(results[0].get('ok')),'code':results[0].get('code')if len(results)==1 else'unknown','automatic_attempts_reset':False,'at':datetime.datetime.now(datetime.timezone.utc).isoformat()}),flush=True)
assert p.returncode==0 and len(results)==1 and results[0].get('ok'),'Inspect private result; do not repeat an unknown authorization'
