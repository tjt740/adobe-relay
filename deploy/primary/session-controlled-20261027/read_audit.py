import pathlib,subprocess,json,datetime,time,hashlib,socket,urllib.request,urllib.parse,ipaddress

def emit(k,d):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':k,'data':d},ensure_ascii=False),flush=True)
def sha(x):return hashlib.sha256(str(x).encode()).hexdigest()
def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='10s';"+q+';COMMIT;',text=True,capture_output=True,timeout=20)
 assert r.returncode==0,'readonly query failed'
 return [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]
def resolve(h):
 try:socket.getaddrinfo(h,None);return h
 except socket.gaierror:
  r=subprocess.run(['docker','exec','adobe-relay-app-1','getent','hosts',h],capture_output=True,text=True,timeout=5);assert r.returncode==0
  return r.stdout.split()[0]
rows=sql("SELECT json_build_object('proxy',to_jsonb(p)) FROM proxies p WHERE id IN (1335,1438,1447)")
state=sql("SELECT json_build_object('state',document) FROM clash_subscription WHERE id=1")[0]['state'];state=json.loads(state) if isinstance(state,str) else state
emit('subscription_revision',{'revision':state.get('revision'),'updated_at':state.get('updated_at')})
nodes={x['proxy_id']:x for x in state.get('nodes',[]) if x.get('proxy_id') in (1335,1438,1447)}
for x in rows:
 p=x['proxy'];n=nodes.get(p['id'],{});cfg=n.get('config',{})
 emit('route_definition',{'proxy_id':p['id'],'active':p['status']=='active','deleted':bool(p.get('deleted_at')),'updated_at':p.get('updated_at'),'protocol':p['protocol'],'host_sha256':sha(p['host']),'port':p['port'],'node_type':n.get('type'),'enabled':n.get('enabled'),'server_sha256':sha(cfg.get('server','')),'server_port':cfg.get('port'),'config_keys':sorted(cfg),'source_id_sha256':sha(n.get('source_id',''))})
events=sql("SELECT json_build_object('at',created_at,'from',from_proxy_id,'to',to_proxy_id) FROM proxy_failover_events WHERE account_id=1738 AND created_at>='2026-10-26T00:00:00Z' ORDER BY created_at")
emit('production_route_events',events)
# Read actual running Mihomo routing metadata without changing selected groups or config.
try:
 ins=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1'],text=True))[0];env=dict(x.split('=',1) for x in ins['Config']['Env'] if '=' in x);u=urllib.parse.urlsplit(env['CLASH_CONTROLLER_URL']);host=resolve(u.hostname);netloc=('['+host+']' if ':' in host else host)+(':'+str(u.port) if u.port else '')
 url=urllib.parse.urlunsplit((u.scheme,netloc,'/proxies','',''));req=urllib.request.Request(url,headers={'Authorization':'Bearer '+env['CLASH_CONTROLLER_SECRET']})
 with urllib.request.urlopen(req,timeout=10) as r:data=json.load(r)
 for ident in nodes:
  x=data.get('proxies',{}).get('sub2api-node-'+str(ident),{});emit('running_mihomo_route',{'proxy_id':ident,'present':bool(x),'type':x.get('type'),'is_group':bool(x.get('all')),'selected_child_sha256':sha(x.get('now',''))})
except Exception as ex:emit('running_mihomo_route_unknown',{'error_class':type(ex).__name__})
start=time.monotonic()
for sample in range(5):
 time.sleep(max(0,start+sample*30-time.monotonic()))
 for x in rows:
  p=x['proxy']
  if p.get('deleted_at') or p['status']!='active':continue
  try:
   host=resolve(p['host']);auth=(urllib.parse.quote(p.get('username') or '',safe='')+':'+urllib.parse.quote(p.get('password') or '',safe='')+'@') if p.get('username') else '';proxy=p['protocol']+'://'+auth+host+':'+str(p['port'])
   op=urllib.request.build_opener(urllib.request.ProxyHandler({'http':proxy,'https':proxy}))
   for endpoint in ['https://api.ipify.org?format=json']+(['https://api64.ipify.org?format=json'] if sample in [0,4] else []):
    try:
     with op.open(endpoint,timeout=12) as r:ip=json.load(r)['ip']
     emit('exit_sample',{'proxy_id':p['id'],'sample':sample,'probe_host':urllib.parse.urlsplit(endpoint).hostname,'ip_sha256':sha(ip),'address_family':ipaddress.ip_address(ip).version})
    except Exception as ex:emit('exit_unknown',{'proxy_id':p['id'],'sample':sample,'probe_host':urllib.parse.urlsplit(endpoint).hostname,'error_class':type(ex).__name__})
  except Exception as ex:emit('exit_unknown',{'proxy_id':p['id'],'sample':sample,'error_class':type(ex).__name__})
emit('observer_complete',{'production_writes':False,'samples_per_active_route':5,'duration_seconds':round(time.monotonic()-start,1)})
