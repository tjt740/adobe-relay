import pathlib,json,datetime,subprocess,time
root_wait=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028-r7')
deadline=time.time()+480
while not (root_wait/'cleaned.json').exists() and time.time()<deadline:time.sleep(5)
time.sleep(15)
assert (root_wait/'cleaned.json').exists(),'bounded wait expired; inspect running trial, do not replay login'

names=['prepared.json','ready.json','login-started.json','login-result.json','trial-started.json','trial-finished.json','trial-error.json','first-auth-failure.json','latest-state.json','controller.jsonl','events.jsonl','database-journal.json','usage.json','errors.json','route-events.json','final-state.json','credential-observations.jsonl','egress-samples.jsonl','proxy-hash-map.json','production-before.json','production-after.json','cleaned.json','handoff.json','egress-continuous.jsonl','egress-observer-started.json','egress-observer-done.json','continuation-started.json','continuation-error.json']
for suffix in ['-r7']:
 root=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028'+suffix)
 for name in names:
  f=root/name
  if not f.exists():continue
  data=[json.loads(x) for x in f.read_text().splitlines()] if name.endswith('.jsonl') else json.loads(f.read_text())
  print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'export','root':root.name,'file':name,'data':data},ensure_ascii=False))
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'containers','data':subprocess.check_output(['docker','ps','-a','--filter','name=crawford-fullchain','--format','{{.Names}} {{.Status}}'],text=True).splitlines()}))

# Read-only audit of possible concurrent production login activity.
import sqlite3,re
c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);c.row_factory=sqlite3.Row
cols=[r[1] for r in c.execute('PRAGMA table_info(external_members)')]
allow=[x for x in ['id','email','status','last_login_at','updated_at','sub2_account_id','sub2_sync_version'] if x in cols]
rows=[dict(r) for r in c.execute('SELECT '+','.join(allow)+' FROM external_members WHERE lower(email)=?',('crawfordheather6275@hotmail.com',))]
print(json.dumps({'kind':'okad_same_email','data':rows},ensure_ascii=False))
lease_tables=[]
for r in c.execute("SELECT name FROM sqlite_master WHERE type='table' AND (name LIKE '%login%' OR name LIKE '%lease%')"):
 name=r[0];fields=[x[1] for x in c.execute('PRAGMA table_info("'+name.replace('"','""')+'")')]
 lease_tables.append({'name':name,'columns':fields})
print(json.dumps({'kind':'okad_login_tables','data':lease_tables},ensure_ascii=False));c.close()
r=subprocess.run(['docker','logs','--since','2026-10-27T18:18:35Z','--until','2026-10-27T18:21:10Z','--tail','10000','adobe-team-cookie-app-1'],capture_output=True,text=True,timeout=30)
lines=(r.stdout+'\n'+r.stderr).splitlines();hits=[]
for line in lines:
 if 'crawfordheather6275' in line.lower() or re.search(r'\b1138\b',line):
  ts=re.search(r'20\d\d-\d\d-\d\d[ T]\d\d:\d\d:\d\d(?:[.,]\d+)?',line)
  hits.append({'at':ts.group(0) if ts else None,'mentions_email':'crawfordheather6275' in line.lower(),'markers':[x for x in ['login','refresh','push','lease','登录','刷新','推送','成功','失败','error','warning'] if x in line.lower()]})
print(json.dumps({'kind':'production_okad_activity_r5_window','data':{'returncode':r.returncode,'window':'18:18:35Z--18:21:10Z','scanned_lines':len(lines),'selected_hits':hits}},ensure_ascii=False))
