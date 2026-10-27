import pathlib,json,datetime,subprocess,sys
ROOT=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028-r5')
def emit(kind,data):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':kind,'data':data},ensure_ascii=False))
for n in ['prepared.json','prepare-error.json','cleaned.json','login-started.json','login-result.json','trial-started.json','trial-finished.json','trial-error.json','first-auth-failure.json','latest-state.json']:
 if (ROOT/n).exists():emit(n,json.loads((ROOT/n).read_text()))
for n in ['controller.jsonl','events.jsonl','egress-continuous.jsonl']:
 if (ROOT/n).exists():
  a=[]
  for l in (ROOT/n).read_text().splitlines():
   try:a.append(json.loads(l))
   except:pass
  from collections import Counter
  emit(n,{'count':len(a),'events':dict(Counter(x.get('event') for x in a)),'http_statuses':dict(Counter(str(x.get('status')) for x in a if x.get('event')=='http_finish')),'tail':a[-8:]})
if (ROOT/'prepared.json').exists() and not (ROOT/'cleaned.json').exists():
 sys.path.insert(0,str(ROOT));from replica import clone,crows,selected,command
 emit('isolation_check',crows("SELECT json_build_object('other_accounts_with_credentials',count(*) FILTER(WHERE id<>1738 AND credentials<>'{}'::jsonb),'other_schedulable',count(*) FILTER(WHERE id<>1738 AND schedulable),'target',count(*) FILTER(WHERE id=1738)) FROM accounts"))
 emit('selected',selected())
 obs=crows("SELECT json_build_object('at',clock_timestamp(),'account_id',id,'token_sha256',diag_hash(credentials->>'access_token'),'source_sha256',diag_hash(credentials->>'cookie'),'runtime_sha256',diag_hash(credentials->>'adobe_session_cookie'),'runtime_source_sha256',credentials->>'adobe_session_cookie_source_hash','expires_at',credentials->>'expires_at','token_version',credentials->'_token_version','proxy_id',proxy_id) FROM accounts WHERE id=1738")[0]
 with (ROOT/'credential-observations.jsonl').open('a') as f:f.write(json.dumps(obs)+'\n')
 emit('credential_observation',obs)

 emit('migrations',crows("SELECT json_build_object('count',count(*),'digest',md5(string_agg(filename||checksum,',' ORDER BY filename))) FROM schema_migrations"))
 try:
  import socket,urllib.request,urllib.parse,hashlib
  row=crows("SELECT json_build_object('proxy',to_jsonb(p)) FROM accounts a JOIN proxies p ON p.id=a.proxy_id WHERE a.id=1738")[0]['proxy'];host=row['host']
  try:socket.getaddrinfo(host,row['port'])
  except socket.gaierror:host=command(['docker','exec','crawford-fullchain-app','getent','hosts',host]).split()[0]
  auth=(urllib.parse.quote(row.get('username') or '',safe='')+':'+urllib.parse.quote(row.get('password') or '',safe='')+'@') if row.get('username') else ''
  proxy=row['protocol']+'://'+auth+host+':'+str(row['port'])
  opener=urllib.request.build_opener(urllib.request.ProxyHandler({'http':proxy,'https':proxy}))
  with opener.open('https://api.ipify.org?format=json',timeout=15) as r:ip=json.load(r)['ip']
  sample={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'proxy_id':row['id'],'ip_sha256':hashlib.sha256(ip.encode()).hexdigest()}
  with (ROOT/'egress-samples.jsonl').open('a') as f:f.write(json.dumps(sample)+'\n')
  emit('egress',sample)
 except Exception as e:emit('egress_check_unknown',{'error_class':type(e).__name__})

emit('containers',subprocess.check_output(['docker','ps','-a','--filter','name=crawford-fullchain','--format','{{.Names}} {{.Status}}'],text=True).splitlines())
