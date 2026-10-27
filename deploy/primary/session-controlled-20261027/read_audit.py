import json,subprocess,pathlib,datetime,time,sqlite3,hashlib,urllib.request,urllib.error,urllib.parse,os,sys,traceback,signal,threading
ROOT=pathlib.Path('/opt/adobe-relay/releases/flare-confirmed-20261027');IDS=[1815,1813];MEMBERS={1815:75,1813:90};EXPECTED='adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c';os.umask(0o077)
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


# Read-only attribution of concurrent credential updates; no Adobe calls or account writes.
check=json.loads((ROOT/'final-preservation-check.json').read_text())
changed=sorted(int(i) for i in check['other_account_differences'])
print(json.dumps({'at':now(),'kind':'scope','changed_ids':changed,'snapshot_at':check['at']}))
if changed:
 data=sql("SELECT json_build_object('id',id,'cookie_hash',md5(credentials->>'cookie'),'source_key',extra->'okad'->>'source_key','version',extra->'okad'->'version','received_at',extra->'okad'->'received_at','created_at',created_at,'updated_at',updated_at) FROM accounts WHERE id IN ("+','.join(map(str,changed))+') ORDER BY id')
 c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);c.row_factory=sqlite3.Row;c.execute('PRAGMA query_only=ON')
 for a in data:
  source=(a.get('source_key') or '').split(':')
  if len(source)==2 and source[0]=='external' and source[1].isdigit():
   m=c.execute('SELECT cookie,last_login_at,updated_at,sub2_links FROM external_members WHERE id=?',(int(source[1]),)).fetchone()
   if m:
    a['okad_cookie_matches']=hashlib.md5((m['cookie'] or '').encode()).hexdigest()==a['cookie_hash'];a['okad_last_login_at']=m['last_login_at'];a['okad_updated_at']=m['updated_at']
    try:links=json.loads(m['sub2_links'] or '{}')
    except Exception:links={}
    a['link_shape']=list(links) if isinstance(links,dict) else type(links).__name__
  print(json.dumps({'at':now(),'kind':'concurrent_sync_state','data':a},ensure_ascii=False))
 c.close()
# Management requests in the bounded trial window, with only non-secret routing / version fields.
audit=sql("SELECT row_to_json(t) FROM (SELECT id,created_at,method,path,status_code,auth_method,request_id,request_body FROM audit_logs WHERE created_at>='2026-10-27T13:38:50Z' AND created_at<='2026-10-27T13:53:10Z' AND (path LIKE '%okad%' OR path LIKE '%accounts%') ORDER BY id LIMIT 700) t")
for a in audit:
 raw=a.pop('request_body',None)
 try:b=json.loads(raw or '{}')
 except Exception:b={}
 if isinstance(b,dict):
  a['body_fields']=list(b)
  a['safe_body']={k:b[k] for k in ['account_ids','account_id','source_key','version','resume','schedulable'] if k in b}
 print(json.dumps({'at':now(),'kind':'management_audit','data':a},ensure_ascii=False))
print(json.dumps({'at':now(),'kind':'audit_complete','audit_rows':len(audit),'limit_reached':len(audit)==700}))
