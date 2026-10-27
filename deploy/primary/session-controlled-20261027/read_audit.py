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


name='crawfordheather6275@hotmail.com'
a=sql("SELECT json_build_object('id',id,'email',name,'status',status,'schedulable',schedulable,'concurrency',concurrency,'proxy_id',proxy_id,'manual_pause',extra->'okad_manual_pause','manual_proxy',extra->'manual_proxy_enabled','version',extra->'okad'->'version','source_key',extra->'okad'->>'source_key','test',extra->'okad_connection_test','error',error_message) FROM accounts WHERE deleted_at IS NULL AND lower(name)='"+name+"'")
assert len(a)==1
IDS[:]=[a[0]['id']]
current=rows()[0]
print(json.dumps({'at':now(),'kind':'crawford_current','data':current},ensure_ascii=False))
c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);c.row_factory=sqlite3.Row;c.execute('PRAGMA query_only=ON')
m=c.execute('SELECT id,email,last_login_at,login_status,profile_preference,credits_available,credits_total,credits_error,account_profile FROM external_members WHERE lower(email)=?',(name,)).fetchone();assert m
m=dict(m)
try:profile=json.loads(m.pop('account_profile') or '{}')
except Exception:profile={}
m['profile_keys']=list(profile) if isinstance(profile,dict) else []
m['profile_safe']={k:v for k,v in profile.items() if k in ['current_kind','account_type','requested_preference','fallback','fallback_used','profiles_complete']} if isinstance(profile,dict) else {}
print(json.dumps({'at':now(),'kind':'crawford_member','data':m},ensure_ascii=False));c.close()
for kind,q in [
 ('crawford_usage',"SELECT row_to_json(t) FROM (SELECT created_at,model,requested_model,upstream_model,image_count,image_size,duration_ms FROM usage_logs WHERE account_id="+str(IDS[0])+" AND created_at>='2026-10-27T00:00:00+08:00' ORDER BY created_at DESC LIMIT 20) t"),
 ('crawford_errors',"SELECT row_to_json(t) FROM (SELECT created_at,model,status_code,upstream_status_code,error_phase,left(error_message,220) AS error,left(upstream_error_message,220) AS upstream FROM ops_error_logs WHERE account_id="+str(IDS[0])+" AND created_at>='2026-10-27T00:00:00+08:00' ORDER BY created_at DESC LIMIT 20) t")]:
 data=sql(q)
 for off in range(0,len(data),10):print(json.dumps({'at':now(),'kind':kind,'offset':off,'data':data[off:off+10]},ensure_ascii=False))
