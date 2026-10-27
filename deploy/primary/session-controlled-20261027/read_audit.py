import json,subprocess,pathlib,datetime,time,sqlite3,hashlib,urllib.request,urllib.error,urllib.parse,os,sys,traceback,signal,threading
ROOT=pathlib.Path('/opt/adobe-relay/releases/native-20261027');IDS=[1843, 1815];MEMBERS={1843: 2, 1815: 75};EXPECTED='adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c';os.umask(0o077)
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
running=[n for n in subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines() if n.startswith('adobe-native-')]
out={'at':now(),'selected_differences':differences,'other_accounts_before':len(others_before),'other_accounts_after':len(others_after),'other_account_differences':other_differences,'settings_equal':settings_equal,'private_tmpfs_exists':bool(list(pathlib.Path('/dev/shm').glob('adobe-native-20261027-*.json'))),'diagnostic_containers':running,'production_containers':containers}
for name in ['restored.json','finished.json']:
 if (ROOT/name).exists():out[name]=json.loads((ROOT/name).read_text())
save('final-preservation-check.json',out);print(json.dumps(out,ensure_ascii=False))

for f in sorted(ROOT.glob('*.jsonl')):
 rows=[]
 for l in f.read_text().splitlines():
  try:rows.append(json.loads(l))
  except ValueError:pass
 for start in range(0,len(rows),5):print(json.dumps({'at':now(),'evidence_file':f.name,'offset':start,'records':rows[start:start+5]},ensure_ascii=False))
for name in ['started.json','finished.json']:
 f=ROOT/name
 if f.exists():print(json.dumps({'at':now(),'evidence_file':name,'document':json.loads(f.read_text())},ensure_ascii=False))
print(json.dumps({'at':now(),'export_finished':True,'phase_done':(ROOT/'finished.json').exists()}))
