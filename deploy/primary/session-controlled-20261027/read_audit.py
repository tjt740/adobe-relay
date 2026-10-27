import json,datetime,subprocess,sqlite3,hashlib,pathlib,shutil

def emit(kind,data):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':kind,'data':data},ensure_ascii=False))
def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='12s';"+q+';COMMIT;',text=True,capture_output=True,timeout=20)
 if r.returncode:raise RuntimeError('read query failed')
 return [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]

c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);c.row_factory=sqlite3.Row;r=dict(c.execute('SELECT email,profile_preference,last_login_at,account_profile FROM external_members WHERE id=1138').fetchone());c.close();d=json.loads(r.pop('account_profile') or '{}');r['cleanup']=d.get('organization_cleanup');r['profile_kind']=d.get('current_kind');r['personal_org_count']=len(d.get('personal_organizations') or []);r['profile_descriptors']=[{k:x.get(k) for k in ['kind','status','disabled']} for x in d.get('profiles',[]) if isinstance(x,dict)];emit('production_profile_context',r)
code="import inspect,json;from app.services import firefly;print(json.dumps({'source':inspect.getsource(firefly._cleanup_profile_organizations)}))"
z=subprocess.run(['docker','exec','adobe-team-cookie-app-1','python','-c',code],capture_output=True,text=True,timeout=20)
if z.returncode==0:emit('cleanup_function',json.loads(z.stdout))
