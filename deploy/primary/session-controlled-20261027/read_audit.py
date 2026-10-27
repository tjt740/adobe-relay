import json,datetime,subprocess,sqlite3,hashlib,pathlib

def emit(k,d):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':k,'data':d},ensure_ascii=False))
c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);c.row_factory=sqlite3.Row
rows=[dict(x) for x in c.execute('SELECT * FROM settings ORDER BY rowid')];c.close()
clean=[]
for x in rows:
 clean.append({k:(hashlib.sha256(str(v).encode()).hexdigest() if k not in ['id','key','name','created_at','updated_at'] else v) for k,v in x.items()})
emit('current_okad_settings_hashes',clean)

health=[]
for x in rows:
 if str(x['key']).startswith('supply_mail_health'):
  try:d=json.loads(x['value'])
  except:continue
  if isinstance(d,dict):health.append({'key':x['key'],'fields':{k:v for k,v in d.items() if any(w in k.lower() for w in ['time','checked','updated','status','success','code']) and isinstance(v,(int,float,str,bool,type(None)))}})
emit('mail_health_operational_metadata',health)
