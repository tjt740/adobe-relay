import sqlite3,json,hashlib,pathlib,datetime,subprocess
c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True)
rows=[list(x) for x in c.execute('SELECT * FROM settings ORDER BY rowid')];c.close()
def h(v):return hashlib.sha256(json.dumps(v,default=str).encode()).hexdigest()
def emit(k,d):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':k,'data':d}))
targets={'craw_a_before':'a6c4762df32c7454694dc3d43b532cddde7ec43b7345261f5569df9012d857e5','craw_a_after':'29059b29969d7c28f4f6e991fd041bf5febe8b4507b7fcfe687337e7454ce306'}
values=set(r[2] for r in rows if r[1].startswith('supply_mail_binding:'))
results=[]
for name,target in targets.items():
 if h(rows)==target:results.append({'target':name,'kind':'current_exact'});continue
 for i,r in enumerate(rows):
  if not r[1].startswith('supply_mail_binding:'):continue
  if h(rows[:i]+rows[i+1:])==target:results.append({'target':name,'kind':'one_added_binding','key':r[1]})
  for value in values:
   if value==r[2]:continue
   cp=[list(x) for x in rows];cp[i][2]=value
   if h(cp)==target:results.append({'target':name,'kind':'one_changed_binding','key':r[1],'old_value_sha256':hashlib.sha256(str(value).encode()).hexdigest(),'new_value_sha256':hashlib.sha256(str(r[2]).encode()).hexdigest()})
emit('settings_hash_reconciliation',{'current_hash':h(rows),'matches':results,'binding_value_count':len(values)})
health=[]
for row in rows:
 if row[1].startswith('supply_mail_health'):
  try:v=json.loads(row[2])
  except Exception:v={}
  safe={k:w for k,w in v.items() if k in ['updated_at','last_success_at','last_failure_at','consecutive_failures','disabled_until','cooldown_until','checked_at','status'] and isinstance(w,(str,int,float,bool,type(None)))} if isinstance(v,dict) else {}
  health.append({'key':row[1],'field_names':list(v) if isinstance(v,dict) else [],'metadata':safe})
emit('mail_health_metadata',health)
# Read only names of source files that own the known mail binding update.
cmd=['docker','exec','adobe-team-cookie-app-1','python','-c',"import pathlib,json; r=pathlib.Path('/app/backend/app'); print(json.dumps([str(p.relative_to(r)) for p in r.rglob('*.py') if 'supply_mail_binding:' in p.read_text(errors='ignore')]))"]
emit('binding_source_paths',json.loads(subprocess.check_output(cmd,text=True)))

root=pathlib.Path('/opt/adobe-relay/releases/408-causal-20261028/donald-pa')
for name in ['experiment-runtime.json','login-result.json','trial-error.json','first-auth-failure.json','latest-state.json','cleaned.json']:
 f=root/name
 if f.exists():emit('donald_progress_'+name,json.loads(f.read_text()))
f=root/'controller.jsonl'
if f.exists():emit('donald_controller_tail',[json.loads(x) for x in f.read_text().splitlines()[-4:]])
