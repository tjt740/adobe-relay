import json,datetime,subprocess,sqlite3,hashlib,pathlib

def emit(k,d):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':k,'data':d},ensure_ascii=False))
c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);c.row_factory=sqlite3.Row
rows=[dict(x) for x in c.execute('SELECT * FROM settings ORDER BY rowid')];c.close()
clean=[]
for x in rows:
 clean.append({k:(hashlib.sha256(str(v).encode()).hexdigest() if k not in ['id','key','name','created_at','updated_at'] else v) for k,v in x.items()})
emit('current_okad_settings_hashes',clean)
code="import inspect,json;from app.services import external_login;print(json.dumps({'login_and_store':inspect.getsource(external_login.login_and_store)}))"
r=subprocess.run(['docker','exec','adobe-team-cookie-app-1','python','-c',code],capture_output=True,text=True,timeout=20)
if r.returncode==0:emit('login_coordinator_write_boundary',json.loads(r.stdout))
