import json,subprocess,pathlib,datetime,time,sqlite3,hashlib,urllib.request,urllib.error,urllib.parse,os,sys,traceback,signal,threading
ROOT=pathlib.Path('/opt/adobe-relay/releases/three-arm-20261027');IDS=[1739,1741,1743];MEMBERS={1742:247,1744:245,1749:240,1758:231};EXPECTED='adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c';os.umask(0o077)
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




code="import json,sys,hashlib,urllib.parse,concurrent.futures\nfrom app.services import proxy_pool\nfrom curl_cffi import requests\ncfg=json.load(sys.stdin)\nraw=cfg['proxy_raw'] if cfg['proxy_enabled'] else ''\nnodes=proxy_pool._split(raw)\ndef hashv(v):return hashlib.sha256(v.encode()).hexdigest()[:24]\nselected=cfg['selected'];out={'production_proxy_enabled':cfg['proxy_enabled'],'pool_size':len(nodes),'selected_proxy_endpoint_in_login_pool':{str(a['id']):a['proxy'] in nodes for a in selected},'login_selection':'random_proxy independently of Sub2 account proxy','ip_samples':[]}\ndef one(proxy):\n x={'proxy_endpoint_hash':hashv(proxy or 'direct')}\n try:\n  s=requests.Session(proxies={'http':proxy,'https':proxy} if proxy else {},impersonate='chrome',timeout=10)\n  r=s.get('https://api.ipify.org?format=json');x['ip_hash']=hashv(r.json()['ip']);x['status']=r.status_code\n except Exception as e:x['error_class']=type(e).__name__\n return x\nsample=nodes[:3] if nodes else ['']\nwith concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:out['ip_samples']=list(pool.map(one,sample))\nprint(json.dumps(out))\n"

db=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);db.execute('PRAGMA query_only=ON')
settings=dict(db.execute("SELECT key,value FROM settings WHERE key IN ('proxy_enabled','proxy_url')"))
def proxyurl(p):
 auth=(urllib.parse.quote(p.get('username') or '',safe='')+':'+urllib.parse.quote(p.get('password') or '',safe='')+'@') if p.get('username') else ''
 return p['protocol']+'://'+auth+p['host']+':'+str(p['port'])
selected=[{'id':a['id'],'proxy':proxyurl(a['proxy'])} for a in rows(True)]
cfg={'proxy_enabled':str(settings.get('proxy_enabled','false')).lower()=='true','proxy_raw':settings.get('proxy_url',''),'selected':selected}
r=subprocess.run(['docker','run','--rm','-i','--network','adobe-relay_default','--memory','256m','--cpus','0.5','--read-only','--tmpfs','/tmp:rw,size=16m','--entrypoint','python','adobe-team-cookie:79382b0f79d559575956e023dc9f52fbf07dbfde','-c',code],input=json.dumps(cfg),text=True,capture_output=True,timeout=35)
# Current production image working directory already /app/backend for python -c.
out={'at':now(),'production_login_exit':json.loads(r.stdout) if r.returncode==0 else None,'returncode':r.returncode,'error_tail':r.stderr[-250:] if r.returncode else None}
save('production-login-exit-comparison.json',out);print(json.dumps(out))

import json,subprocess,pathlib,datetime,time,sqlite3,hashlib,urllib.request,urllib.error,urllib.parse,os,sys,traceback,signal,threading
ROOT=pathlib.Path('/opt/adobe-relay/releases/three-arm-20261027');IDS=[1739,1741,1743];MEMBERS={1742:247,1744:245,1749:240,1758:231};EXPECTED='adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c';os.umask(0o077)
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




before={a['id']:a for a in json.loads((ROOT/'before.json').read_text())};current={a['id']:a for a in rows()}
keys=['status','schedulable','concurrency','manual_pause','manual_proxy','proxy_id','version','verification','test','error','cookie_hash','runtime_cookie_hash','token_hash']
differences={i:{k:{'before':before[i].get(k),'after':current.get(i,{}).get(k)} for k in keys if before[i].get(k)!=current.get(i,{}).get(k)} for i in IDS}
others_before={a['id']:a for a in json.loads((ROOT/'other-accounts-before.json').read_text())};others_after={a['id']:a for a in others()}
other_differences={i:{k:{'before':others_before.get(i,{}).get(k),'after':others_after.get(i,{}).get(k)} for k in ['config_hash','cookie_hash'] if others_before.get(i,{}).get(k)!=others_after.get(i,{}).get(k)} for i in set(others_before)|set(others_after)}
other_differences={i:d for i,d in other_differences.items() if d}
settings_equal=sql("SELECT json_build_object('key',key,'hash',md5(value)) FROM settings ORDER BY key")==json.loads((ROOT/'settings-before.json').read_text())
containers=[]
for name in ['adobe-relay-app-1','adobe-team-cookie-app-1']:
 d=json.loads(subprocess.check_output(['docker','inspect',name],text=True))[0]
 containers.append({'name':name,'image':d['Config']['Image'],'started_at':d['State']['StartedAt'],'restarts':d['RestartCount'],'memory_limit':d['HostConfig']['Memory'],'cpu_shares':d['HostConfig']['CpuShares']})
running=[n for n in subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines() if n.startswith('adobe-three-arm-')]
out={'at':now(),'selected_differences':differences,'other_accounts_before':len(others_before),'other_accounts_after':len(others_after),'other_account_differences':other_differences,'settings_equal':settings_equal,'private_tmpfs_exists':pathlib.Path('/dev/shm/adobe-three-arm-20261027-private.json').exists(),'diagnostic_containers':running,'production_containers':containers}
for name in ['restored.json','trial-finished.json']:
 if (ROOT/name).exists():out[name]=json.loads((ROOT/name).read_text())
save('final-preservation-check.json',out);print(json.dumps(out,ensure_ascii=False))
