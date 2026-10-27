import pathlib,json,subprocess,datetime
ROOT=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028-r5')
assert (ROOT/'prepared.json').exists() and not (ROOT/'cleaned.json').exists()
assert not (ROOT/'egress-observer-started.json').exists()
SOURCE='''import sys,pathlib,json,time,socket,urllib.parse,urllib.request,ipaddress,hashlib,datetime
R=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028-r5');sys.path.insert(0,str(R))
from replica import crows,command
for iteration in range(45):
 if (R/'cleaned.json').exists():break
 start=time.monotonic()
 try:
  p=crows("SELECT json_build_object('proxy',to_jsonb(p)) FROM accounts a JOIN proxies p ON p.id=a.proxy_id WHERE a.id=1738")[0]['proxy'];host=p['host']
  try:socket.getaddrinfo(host,None)
  except socket.gaierror:host=command(['docker','exec','crawford-fullchain-app','getent','hosts',host]).split()[0]
  auth=(urllib.parse.quote(p.get('username') or '',safe='')+':'+urllib.parse.quote(p.get('password') or '',safe='')+'@') if p.get('username') else ''
  proxy=p['protocol']+'://'+auth+host+':'+str(p['port']);op=urllib.request.build_opener(urllib.request.ProxyHandler({'http':proxy,'https':proxy}))
  for endpoint in ['https://api.ipify.org?format=json','https://api64.ipify.org?format=json']:
   record={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'proxy_id':p['id'],'probe_host':urllib.parse.urlsplit(endpoint).hostname,'iteration':iteration}
   try:
    with op.open(endpoint,timeout=8) as r:ip=json.load(r)['ip']
    record.update(ip_sha256=hashlib.sha256(ip.encode()).hexdigest(),address_family=ipaddress.ip_address(ip).version)
   except Exception as e:record['error_class']=type(e).__name__
   with (R/'egress-continuous.jsonl').open('a') as f:f.write(json.dumps(record)+'\\n')
 except Exception as e:
  if (R/'cleaned.json').exists():break
  with (R/'egress-continuous.jsonl').open('a') as f:f.write(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'error_class':type(e).__name__})+'\\n')
 time.sleep(max(0,14-time.monotonic()+start))
'''
compile(SOURCE,'egress-observer.py','exec');(ROOT/'egress-observer.py').write_text(SOURCE);(ROOT/'egress-observer-started.json').write_text(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'iterations_limit':45,'interval_seconds':14,'production_writes':False}))
f=open(ROOT/'egress-observer-process.log','a');p=subprocess.Popen(['python3',str(ROOT/'egress-observer.py')],stdout=f,stderr=f,start_new_session=True)
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'egress_observer_launched','pid':p.pid,'production_writes':False}))
