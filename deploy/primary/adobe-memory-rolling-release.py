"""Guarded temporary rolling handoff; no business records or persistent config edits."""
import argparse, datetime, fcntl, hashlib, importlib.util, json, os, re, subprocess, time, urllib.request
from pathlib import Path

RID='adobe-memory-real-load-20261016'
ROOT=Path('/opt/adobe-relay/releases')/RID
APP='adobe-relay-app-1'
CANARY='adobe-relay-memory-canary'
LEASE='concurrency:rolling_deployment'
RULE=['-d','127.0.0.1/32','-p','tcp','--dport','6666','-m','comment','--comment',RID,'-j','REDIRECT','--to-ports','6667']

def run(args,timeout=35,input=None,check=True):
 p=subprocess.run(args,input=input,text=True,capture_output=True,timeout=timeout)
 if check and p.returncode:raise RuntimeError(p.stderr[:250])
 return p.stdout.strip()
def inspect(name):return json.loads(run(['docker','inspect',name]))[0]
def save(name,obj):
 p=ROOT/name;p.write_text(json.dumps(obj,ensure_ascii=False,indent=2));p.chmod(0o600)
def load(name):return json.loads((ROOT/name).read_text())
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def redis(*args):return run(['docker','exec','adobe-relay-redis-1','redis-cli','--raw',*map(str,args)])
def own_lease():
 owner=redis('GET',LEASE)
 assert not owner or owner==RID,'Another rolling handoff holds the lease'
 if not owner:assert redis('SET',LEASE,RID,'NX','EX',1800)=='OK'
 else:assert redis('EXPIRE',LEASE,1800)=='1'
def slot_prefixes():
 lua="""local out={} local cursor='0' repeat local r=redis.call('SCAN',cursor,'MATCH','concurrency:*','COUNT',300) cursor=r[1] for _,k in ipairs(r[2]) do if redis.call('TYPE',k).ok=='zset' then for _,m in ipairs(redis.call('ZRANGE',k,0,-1)) do local p=string.match(m,'^(r[a-z0-9]+)%-') if p then out[p]=(out[p] or 0)+1 end end end end until cursor=='0' return cjson.encode(out)"""
 return json.loads(redis('EVAL',lua,0)) or {}
def available():return int(next(l.split()[1] for l in Path('/proc/meminfo').read_text().splitlines() if l.startswith('MemAvailable:')))*1024
def health(port):
 with urllib.request.urlopen('http://127.0.0.1:'+str(port)+'/health',timeout=5) as r:return r.status==200
def rule_present():return subprocess.run(['iptables','-t','nat','-C','OUTPUT',*RULE],capture_output=True).returncode==0
def locks():
 result=[]
 for name in ['/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
  f=open(name,'r');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);result.append(f)
 return result
def prepare(sha):
 assert re.fullmatch('[0-9a-f]{40}',sha)
 held=locks()
 old=inspect(APP)
 assert old['Config']['Image']=='adobe-relay:c574dcc0dc6369cd05d94b25f75d4ea66c1e4bb1'
 assert old['State']['Health']['Status']=='healthy' and available()>5*1024**3
 assert not rule_present()
 spec=importlib.util.spec_from_file_location('release_guard',ROOT/'adobe-memory-real-load-release.py')
 guard=importlib.util.module_from_spec(spec);spec.loader.exec_module(guard);r=guard.release
 if not (ROOT/'before.json').exists():
  r.save('before.json',r.snapshot())
  run(['docker','tag',old['Image'],r.ROLLBACK_IMAGE])
 own_lease()
 prefixes=slot_prefixes()
 assert prefixes,'No tracked source process; inspect its work before starting a handoff'
 env=dict(v.split('=',1) for v in old['Config']['Env'])
 env['LOG_OUTPUT_TO_FILE']='false';env['LOG_OUTPUT_TO_STDOUT']='true'
 private=ROOT/'canary-environment'
 private.write_text('\n'.join(k+'='+v for k,v in env.items())+'\n');private.chmod(0o600)
 args=['docker','create','--name',CANARY,'--restart','unless-stopped','--memory',str(old['HostConfig']['Memory']),'--memory-swap',str(old['HostConfig']['MemorySwap']),
       '--network',next(iter(old['NetworkSettings']['Networks'])),'--publish','127.0.0.1:6667:8080','--env-file',str(private)]
 for mount in old['Mounts']:
  assert mount['Type']=='bind'
  args+=['--mount','type=bind,src='+mount['Source']+',dst='+mount['Destination']+('' if mount['RW'] else ',readonly')]
 for u in old['HostConfig'].get('Ulimits') or []:args+=['--ulimit',f"{u['Name']}={u['Soft']}:{u['Hard']}"]
 args+=['adobe-relay:'+sha]
 try:
  run(args)
  for network in list(old['NetworkSettings']['Networks'])[1:]:run(['docker','network','connect',network,CANARY])
  run(['docker','start',CANARY])
 finally:private.unlink(missing_ok=True)
 deadline=time.monotonic()+150
 while True:
  try:
   if health(6667) and inspect(CANARY)['State'].get('Health',{}).get('Status')=='healthy':break
  except Exception:pass
  assert time.monotonic()<deadline,'Canary health failed; no traffic has switched'
  time.sleep(2)
 assert inspect(APP)['Id']==old['Id'],'Source container changed during preparation'
 state={'at':now(),'candidate_sha':sha,'old_id':old['Id'],'old_prefixes':list(prefixes),'canary_id':inspect(CANARY)['Id'],'nginx_files':{str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in Path('/etc/nginx/conf.d').glob('*.conf')}}
 save('rolling-prepared.json',state)
 print(json.dumps({'prepared':True,'served_customer_traffic':False,'candidate_sha':sha,'source_prefix_count':len(prefixes)}))
def switch(on):
 held=locks();own_lease()
 if on:
  state=load('rolling-prepared.json');assert inspect(APP)['Id']==state['old_id']
  assert health(6667) and inspect(CANARY)['State']['Health']['Status']=='healthy'
  if not rule_present():run(['iptables','-t','nat','-I','OUTPUT','1',*RULE])
 else:
  expected=(ROOT/'expected-sha').read_text().strip()
  assert inspect(APP)['Config']['Image']=='adobe-relay:'+expected and inspect(APP)['State'].get('Health',{}).get('Status')=='healthy'
  if rule_present():run(['iptables','-t','nat','-D','OUTPUT',*RULE])
 try:assert health(6666)
 except Exception:
  if on and rule_present():run(['iptables','-t','nat','-D','OUTPUT',*RULE])
  if not on and not rule_present():run(['iptables','-t','nat','-I','OUTPUT','1',*RULE])
  raise
 # Reload unchanged Nginx config to retire idle old upstream connections.
 run(['nginx','-t']);run(['systemctl','reload','nginx'])
 save('rolling-route-'+('canary' if on else 'canonical')+'.json',{'at':now(),'temporary_redirect':on,'source_and_canary_healthy':True})
 print(json.dumps({'at':now(),'new_connections':'canary' if on else 'canonical','existing_connections':'retained'}))
def pending_uploads(name):
 pid=inspect(name)['State']['Pid']
 # Metadata only: no packet contents. No-response sockets may be reading an
 # upload/audit before a user slot exists; do not drain them by slot count alone.
 raw=run(['nsenter','-t',str(pid),'-n','ss','-nti','state','established','sport = :8080'])
 blocks=re.split(r'\n(?=\S)',raw)[1:]
 pending=0
 for block in blocks:
  sent=re.search(r'bytes_sent:(\d+)',block)
  if sent is None or int(sent[1])<=128:pending+=1
 return pending
def drain(which):
 state=load('rolling-prepared.json');own_lease()
 if which=='old':prefixes=state['old_prefixes'];name=APP
 else:prefixes=load('rolling-old-drained.json')['canary_prefixes'];name=CANARY
 expected_id=state['old_id'] if which=='old' else state['canary_id']
 assert inspect(name)['Id']==expected_id,'Draining container changed; inspect the later release first'
 assert prefixes,'Unknown process ownership; do not stop an untracked process'
 zero_since=None;end=time.monotonic()+1500;samples=[]
 while True:
  assert inspect(name)['Id']==expected_id,'Draining container changed during observation'
  own_lease();current=slot_prefixes();count=sum(current.get(p,0) for p in prefixes);pending=pending_uploads(name)
  row={'at':now(),'remaining_tracked_slots':count,'pending_input_sockets':pending,'host_available_bytes':available()}
  samples.append(row);save('rolling-'+which+'-drain-progress.json',{'samples':samples})
  if available()<2*1024**3:
   if rule_present():run(['iptables','-t','nat','-D','OUTPUT',*RULE])
   raise RuntimeError('Host headroom too low; restored source routing without stopping accepted tasks')
  if count==0 and pending==0:
   if zero_since is None:zero_since=time.monotonic()
   if time.monotonic()-zero_since>=45:
    value={'at':now(),'drained':which,'stable_zero_seconds':45,'samples':samples}
    if which=='old':
     value['canary_prefixes']=[p for p in current if p not in prefixes]
     assert value['canary_prefixes'],'Canary has not taken real traffic; do not infer a new owner'
    save('rolling-'+which+'-drained.json',value)
    print(json.dumps({'drained':which,'stable_zero_seconds':45,'samples':len(samples)}));return
  else:zero_since=None
  assert time.monotonic()<end,'Requests remain active; no container stopped'
  print(json.dumps(row),flush=True);time.sleep(5)
def cleanup():
 held=locks();assert not rule_present();assert (ROOT/'rolling-canary-drained.json').exists()
 state=load('rolling-prepared.json')
 assert inspect(CANARY)['Id']==state['canary_id'],'A different canary exists; do not remove it'
 assert redis('GET',LEASE)==RID
 assert inspect(APP)['Config']['Image']=='adobe-relay:'+(ROOT/'expected-sha').read_text().strip()
 run(['docker','logs','--tail','300',CANARY],check=False) # Never print request logs.
 run(['docker','stop','--time','30',CANARY],timeout=40);run(['docker','rm',CANARY])
 redis('DEL',LEASE)
 assert state['nginx_files']=={str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in Path('/etc/nginx/conf.d').glob('*.conf')}
 save('rolling-finished.json',{'at':now(),'temporary_container_removed':True,'temporary_nat_rule_removed':True,'rolling_lease_removed':True,'nginx_configuration_unchanged':True})
 print(json.dumps(load('rolling-finished.json')))

if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('action',choices=['prepare','route-canary','drain-old','route-canonical','drain-canary','cleanup']);p.add_argument('--sha');a=p.parse_args()
 if a.action=='prepare':prepare(a.sha)
 elif a.action=='route-canary':switch(True)
 elif a.action=='route-canonical':switch(False)
 elif a.action=='drain-old':drain('old')
 elif a.action=='drain-canary':drain('canary')
 else:cleanup()
