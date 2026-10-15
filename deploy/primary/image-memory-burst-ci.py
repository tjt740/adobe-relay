"""Run offline image burst replay in disposable Linux containers, never on production."""
import datetime, hashlib, json, os
from pathlib import Path
import re, subprocess, sys, time

assert os.environ.get('GITHUB_ACTIONS') == 'true', 'This harness is for a disposable CI runner only'
root=Path(sys.argv[1]).resolve()
results=[]
meminfo=Path('/proc/meminfo').read_text()
memtotal=int(re.search(r'MemTotal:\s+(\d+)',meminfo)[1])*1024
assert memtotal >= 14*1024**3, 'Need a runner with at least 14 GiB RAM'
subprocess.run(['docker','pull','alpine:3.21'],check=True)

def inspect(name):
 return json.loads(subprocess.check_output(['docker','inspect',name]))[0]

cases=[('burst-400-4m','12g',400,4,False),('paced-3075-4m','12g',None,4,True),('burst-400-8m','12g',400,8,False),('large-output-32-64m','1536m',32,64,False)]
for label,limit,total,output,paced in cases:
 name='sub2-offline-'+label
 temp=root/('tmp-'+label);temp.mkdir()
 args=['docker','run','-d','--name',name,'--network=none','--read-only','--cpus=4','--memory='+limit,'--memory-swap='+limit,
       '--pids-limit=512','--mount',f'type=bind,src={root},dst=/input,readonly',
       '--mount',f'type=bind,src={temp},dst=/tmp','-e','GOMEMLIMIT=9216MiB',
       '-e','ADOBE_HISTORY_TRACE=/input/trace.json','-e','ADOBE_HISTORY_COMPACT_RESERVATION=1',
       '-e','ADOBE_HISTORY_TIME_SCALE=0.2','-e','ADOBE_HISTORY_OUTPUT_MIB='+str(output)]
 if total:args+=['-e','ADOBE_HISTORY_TOTAL='+str(total)]
 if paced:args+=['-e','ADOBE_HISTORY_PACED=1']
 args+=['alpine:3.21','sh','-c','/input/memory.test -test.run=^TestImageMemoryHistoricalBurst$ -test.v -test.timeout=9m; code=$?; sleep 3; exit "$code"']
 subprocess.run(args,check=True,stdout=subprocess.DEVNULL)
 peak=0; events={}; samples=[]; deadline=time.monotonic()+600
 try:
  while True:
   d=inspect(name)
   if not d['State']['Running']:break
   pid=d['State']['Pid']
   try:
    path=Path('/sys/fs/cgroup')/Path(f'/proc/{pid}/cgroup').read_text().strip().split(':')[-1].lstrip('/')
    current=int((path/'memory.current').read_text())
    peak=max(peak,int((path/'memory.peak').read_text()))
    events={k:int(v) for k,v in (row.split() for row in (path/'memory.events').read_text().splitlines())}
    samples.append(current)
   except FileNotFoundError: pass
   assert time.monotonic()<deadline,'Replay timed out'
   time.sleep(1)
  log=subprocess.check_output(['docker','logs',name],stderr=subprocess.STDOUT,text=True)
  (root/(label+'.log')).write_text(log)
  row={'case':label,'hard_limit':limit,'exit_code':d['State']['ExitCode'],'oom_killed':d['State']['OOMKilled'],'memory_peak':peak,'memory_events':events,'sample_count':len(samples),'temporary_files_left':len(list(temp.iterdir()))}
  match=re.search(r'historical_burst requested=(\d+) accepted=(\d+) rejected=(\d+) peak=(\d+)',log)
  if match:row.update(zip(['requested','accepted','rejected','peak_inflight'],map(int,match.groups())))
  results.append(row)
  (root/'linux-validation.json').write_text(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'binary_sha256':hashlib.sha256((root/'memory.test').read_bytes()).hexdigest(),'runner_memory':memtotal,'compact_synthetic_disk_reservations':True,'results':results},indent=2))
  print(json.dumps(row),flush=True)
  assert d['State']['ExitCode']==0 and not row['oom_killed'] and events.get('oom',0)==0 and events.get('oom_kill',0)==0,log[-5000:]
  assert row['temporary_files_left']==0,'Temporary files leaked'
  assert row.get('accepted',0)>=row.get('requested',1)*0.99,'More than 1% rejected; inspect before release'
 finally:
  subprocess.run(['docker','rm','-f',name],check=False,stdout=subprocess.DEVNULL)
