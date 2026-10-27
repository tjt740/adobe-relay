import json,subprocess,pathlib,datetime,time,sqlite3,hashlib,urllib.request,urllib.error,urllib.parse,os,sys,traceback,signal,threading
ROOT=pathlib.Path('/opt/adobe-relay/releases/expanded-20261027');IDS=[1843, 1815, 1842, 1833, 1807, 1817];MEMBERS={1843: 2, 1815: 75, 1842: 3, 1833: 20, 1807: 111, 1817: 70};EXPECTED='adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c';os.umask(0o077)
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def log(kind,**kw):
 with (ROOT/'controller.jsonl').open('a') as f:f.write(json.dumps({'at':now(),'event':kind,**kw},ensure_ascii=False)+'\n')
def sql(q,raw=False):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='8s';"+q+';COMMIT;',capture_output=True,text=True,timeout=15)
 if r.returncode:raise RuntimeError('database_read_failed')
 return r.stdout.strip() if raw else [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]
def api(method,path,body=None,timeout=35):
 assert path=='/api/v1/admin/accounts/bulk-update' or any(path==f'/api/v1/admin/accounts/{i}'+suffix for i in IDS for suffix in ('','/schedulable','/test'))
 if path.endswith('bulk-update'):assert set(body['account_ids'])<=set(IDS) and 'filters' not in body
 key=sql("SELECT value FROM settings WHERE key='admin_api_key'",True)
 req=urllib.request.Request('http://127.0.0.1:6666'+path,data=json.dumps(body).encode() if body is not None else None,headers={'x-api-key':key,'Content-Type':'application/json','X-Request-ID':'session-controlled-20261027'},method=method)
 with urllib.request.urlopen(req,timeout=timeout) as r:
  if path.endswith('/test'):
   result=[]
   for line in r:
    if line.startswith(b'data: '):
     d=json.loads(line[6:]);result.append({k:d[k] for k in ['type','success','status_code'] if k in d})
   return result
  d=json.load(r);return {'code':d.get('code'),'success_ids':(d.get('data') or {}).get('success_ids')} if isinstance(d,dict) else {}
def rows(private=False):
 q="SELECT json_build_object('id',a.id,'email',a.name,'status',a.status,'schedulable',a.schedulable,'concurrency',a.concurrency,'manual_pause',a.extra->'okad_manual_pause','manual_proxy',a.extra->'manual_proxy_enabled','proxy_id',a.proxy_id,'proxy_updated_at',p.updated_at,'proxy_status',p.status,'proxy_excluded',EXISTS(SELECT 1 FROM adobe_route_exclusions e WHERE e.proxy_id=a.proxy_id),'other_bindings',(SELECT count(*) FROM accounts b WHERE b.deleted_at IS NULL AND b.proxy_id=a.proxy_id AND b.id<>a.id),'active_lease',EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=a.id AND l.expires_at>NOW()),'version',a.extra->'okad'->'version','verification',a.extra->'okad'->'verification','test',a.extra->'okad_connection_test','error',a.error_message,'cookie_hash',md5(a.credentials->>'cookie'),'runtime_cookie_hash',md5(a.credentials->>'adobe_session_cookie'),'token_hash',md5(a.credentials->>'access_token'),'token',a.credentials->>'access_token','arp_session_id',a.credentials->>'arp_session_id','proxy',json_build_object('host',p.host,'port',p.port,'protocol',p.protocol,'username',p.username,'password',p.password)) FROM accounts a LEFT JOIN proxies p ON p.id=a.proxy_id WHERE a.deleted_at IS NULL AND a.id IN ("+','.join(map(str,IDS))+') ORDER BY a.id'
 data=sql(q)
 if not private:
  for a in data:
   for k in ['token','arp_session_id','proxy']:a.pop(k,None)
 return data
def save(name,data):
 p=ROOT/name;tmp=ROOT/(name+'.tmp');tmp.write_text(json.dumps(data,ensure_ascii=False,indent=2));tmp.replace(p)
def others():
 return sql("SELECT json_build_object('id',id,'config_hash',md5(json_build_object('concurrency',concurrency,'priority',priority,'groups',(SELECT json_agg(group_id ORDER BY group_id) FROM account_groups WHERE account_id=accounts.id),'manual_pause',extra->'okad_manual_pause','manual_proxy',extra->'manual_proxy_enabled')::text),'cookie_hash',md5(credentials->>'cookie')) FROM accounts WHERE deleted_at IS NULL AND id NOT IN ("+','.join(map(str,IDS))+') ORDER BY id')


SEL=[{'id': 1843, 'email': 'BrianSchmidtfY445921@outlook.com', 'member_id': 2, 'proxy_id': 1375, 'arm': 'web_low', 'history': [{'account_id': 1843, 'model': 'gpt-image-2.5-sunburst', 'n': 21, 'first_at': '2026-10-27T12:02:23.155276+08:00', 'last_at': '2026-10-27T17:21:58.118611+08:00'}, {'account_id': 1843, 'model': 'gpt-image-2.5-flare', 'n': 9, 'first_at': '2026-10-27T14:29:54.375954+08:00', 'last_at': '2026-10-27T17:06:59.083552+08:00'}]}, {'id': 1815, 'email': 'BergesenVillafranca062@outlook.com', 'member_id': 75, 'proxy_id': 1547, 'arm': 'web_low', 'history': [{'account_id': 1815, 'model': 'gpt-image-2.5-sunburst', 'n': 64, 'first_at': '2026-10-27T14:25:10.314159+08:00', 'last_at': '2026-10-27T17:18:10.480596+08:00'}, {'account_id': 1815, 'model': 'gpt-image-2.5-flare', 'n': 4, 'first_at': '2026-10-27T13:35:16.088473+08:00', 'last_at': '2026-10-27T14:54:11.785867+08:00'}]}, {'id': 1842, 'email': 'PatrickCaseyUT43130@outlook.com', 'member_id': 3, 'proxy_id': 1174, 'arm': 'sdk_low', 'history': [{'account_id': 1842, 'model': 'gpt-image-2.5-sunburst', 'n': 31, 'first_at': '2026-10-27T12:03:15.647674+08:00', 'last_at': '2026-10-27T17:17:45.981765+08:00'}, {'account_id': 1842, 'model': 'gpt-image-2.5-flare', 'n': 9, 'first_at': '2026-10-27T12:02:59.694364+08:00', 'last_at': '2026-10-27T15:17:58.602714+08:00'}]}, {'id': 1833, 'email': 'McnicholasBergquist89@hotmail.com', 'member_id': 20, 'proxy_id': 1210, 'arm': 'sdk_low', 'history': [{'account_id': 1833, 'model': 'gpt-image-2.5-sunburst', 'n': 17, 'first_at': '2026-10-27T12:32:42.07581+08:00', 'last_at': '2026-10-27T17:17:21.712888+08:00'}, {'account_id': 1833, 'model': 'gpt-image-2.5-flare', 'n': 9, 'first_at': '2026-10-27T14:08:27.261911+08:00', 'last_at': '2026-10-27T16:42:59.884809+08:00'}]}, {'id': 1807, 'email': 'DubieWrightsel775@outlook.com', 'member_id': 111, 'proxy_id': 1425, 'arm': 'sdk_existing', 'history': [{'account_id': 1807, 'model': 'gpt-image-2.5-sunburst', 'n': 29, 'first_at': '2026-10-27T14:26:32.228061+08:00', 'last_at': '2026-10-27T17:15:09.476444+08:00'}, {'account_id': 1807, 'model': 'gpt-image-2.5-flare', 'n': 1, 'first_at': '2026-10-27T16:28:23.606021+08:00', 'last_at': '2026-10-27T16:28:23.606021+08:00'}]}, {'id': 1817, 'email': 'MaltbaKinsolving20@outlook.com', 'member_id': 70, 'proxy_id': 1448, 'arm': 'sdk_existing', 'history': [{'account_id': 1817, 'model': 'gpt-image-2.5-sunburst', 'n': 9, 'first_at': '2026-10-27T12:00:58.434466+08:00', 'last_at': '2026-10-27T17:14:29.273189+08:00'}, {'account_id': 1817, 'model': 'gpt-image-2.5-flare', 'n': 17, 'first_at': '2026-10-27T12:01:18.993459+08:00', 'last_at': '2026-10-27T17:14:09.023472+08:00'}]}]
c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);c.row_factory=sqlite3.Row
members=[]
for a in SEL:
 r=c.execute('SELECT id,email,last_login_at FROM external_members WHERE id=?',(a['member_id'],)).fetchone();assert r and r['email'].lower()==a['email'].lower();members.append(dict(r))
print(json.dumps({'at':now(),'kind':'members','data':members}))
tables=[r[0] for r in c.execute("SELECT name FROM sqlite_master WHERE type='table' AND (name LIKE '%setting%' OR name LIKE '%config%')")];print(json.dumps({'at':now(),'kind':'okad_settings_tables','data':tables}))
print(json.dumps({'at':now(),'kind':'resources','data':{'available_memory_kib':next(int(l.split()[1]) for l in pathlib.Path('/proc/meminfo').read_text().splitlines() if l.startswith('MemAvailable:'))}}))
for kind,query in [
 ('recent_success',"SELECT row_to_json(t) FROM (SELECT u.account_id,u.created_at,u.model,u.requested_model,u.upstream_model,u.image_count,u.image_size,u.duration_ms FROM usage_logs u WHERE u.account_id IN ("+','.join(map(str,IDS))+") AND u.created_at>='2026-10-27T16:00:00+08:00' ORDER BY u.created_at DESC LIMIT 100) t"),
 ('errors_after_16',"SELECT row_to_json(t) FROM (SELECT e.account_id,e.created_at,e.model,e.status_code,e.upstream_status_code,e.error_phase,left(e.error_message,180) AS error,left(e.upstream_error_message,180) AS upstream FROM ops_error_logs e WHERE e.account_id IN ("+','.join(map(str,IDS))+") AND e.created_at>='2026-10-27T16:00:00+08:00' ORDER BY e.created_at DESC LIMIT 160) t")]:
 data=sql(query)
 for n in range(0,len(data),10):print(json.dumps({'at':now(),'kind':kind,'offset':n,'data':data[n:n+10]},ensure_ascii=False))
print(json.dumps({'at':now(),'kind':'selected_current','data':rows()},ensure_ascii=False))
