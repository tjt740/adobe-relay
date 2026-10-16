"""Identical-load comparison with today's incident sizes; disposable CI only."""
import datetime, hashlib, json, os, re, statistics, subprocess, sys, time
from pathlib import Path

assert os.environ.get('GITHUB_ACTIONS') == 'true'
root=Path(sys.argv[1]).resolve()
production=json.loads((root/'production-limits.json').read_text())
assert production['memory_bytes']==12*1024**3
assert production['environment']=={'GOMEMLIMIT':'9216MiB','GOMAXPROCS':'6'}
assert production['pids_max']=='max' or int(production['pids_max'])>=2048
trace=json.loads((root/'trace.json').read_text())
assert trace['window_start'].startswith('2026-10-16') and len(trace['requests'])>400
subprocess.run(['docker','pull','alpine:3.21'],check=True)
results=[]
regressions=[]
cases=[('burst-400-4m','12g',400,4,False,True),('burst-600-8m','12g',600,8,False,True),('paced-incident-4m','12g',None,4,True,True),('json-burst-400-4m','12g',400,4,False,True),('json-paced-incident-4m','12g',None,4,True,True),('full-64-4m','12g',64,4,False,False),('low-limit-32-64m','1536m',32,64,False,False)]
repeats=int(os.environ.get('ADOBE_MEMORY_BURST_REPEATS','1'))
assert repeats in (1,5)
if repeats==5:
 cases=[c for c in cases if c[0] in ('burst-400-4m','burst-600-8m','json-burst-400-4m')]
 plan=[(baseline,repeat,case) for repeat in range(5) for case in cases for baseline in ([True,False] if repeat%2==0 else [False,True])]
else:
 plan=[(baseline,0,case) for baseline in [True,False] for case in cases]
for baseline,repeat,case_spec in plan:
  case,limit,total,output,paced,compact=case_spec
  label=('baseline-' if baseline else 'mapped-')+case+('-repeat-'+str(repeat) if repeats>1 else '')
  binary='memory-baseline.test' if baseline else 'memory.test'
  name='sub2-real-'+label
  temp=root/('tmp-'+label);temp.mkdir()
  args=['docker','run','-d','--name',name,'--network=none','--read-only','--cpus=4','--memory='+limit,'--memory-swap='+limit,'--pids-limit=2048',
        '--mount',f'type=bind,src={root},dst=/input,readonly','--mount',f'type=bind,src={temp},dst=/tmp',
        '-e','GOMEMLIMIT=9216MiB','-e','GOMAXPROCS=6','-e','ADOBE_HISTORY_TRACE=/input/trace.json',
        '-e','ADOBE_HISTORY_TIME_SCALE=0.15','-e','ADOBE_HISTORY_OUTPUT_MIB='+str(output)]
  if compact:args+=['-e','ADOBE_HISTORY_COMPACT_RESERVATION=1']
  if total:args+=['-e','ADOBE_HISTORY_TOTAL='+str(total)]
  if paced:args+=['-e','ADOBE_HISTORY_PACED=1']
  if compact:args+=['-e','ADOBE_HISTORY_MIXED=1','-e','ADOBE_HISTORY_LEGACY_PERCENT=40','-e','ADOBE_HISTORY_OBSERVED_PATHS=1']
  if case.startswith('json-'):args+=['-e','ADOBE_HISTORY_INPUT_FORMAT=json']
  args+=['alpine:3.21','sh','-c','/input/'+binary+' -test.run=^TestImageMemoryHistoricalBurst$ -test.v -test.timeout=9m; code=$?; sleep 2; exit "$code"']
  subprocess.run(args,check=True,stdout=subprocess.DEVNULL)
  started=time.monotonic(); peak=0;events={}
  try:
   while True:
    d=json.loads(subprocess.check_output(['docker','inspect',name]))[0]
    if not d['State']['Running']:break
    try:
     p=Path('/sys/fs/cgroup')/Path(f"/proc/{d['State']['Pid']}/cgroup").read_text().strip().split(':')[-1].lstrip('/')
     peak=max(peak,int((p/'memory.peak').read_text()))
     events={k:int(v) for k,v in (line.split() for line in (p/'memory.events').read_text().splitlines())}
    except FileNotFoundError:pass
    assert time.monotonic()-started<570
    time.sleep(1)
   log=subprocess.check_output(['docker','logs',name],stderr=subprocess.STDOUT,text=True)
   (root/(label+'.log')).write_text(log)
   row={'case':case,'repeat':repeat,'baseline':baseline,'binary_sha256':hashlib.sha256((root/binary).read_bytes()).hexdigest(),'hard_limit':limit,'compact_disk_reservation':compact,
        'elapsed_seconds':round(time.monotonic()-started,2),'exit_code':d['State']['ExitCode'],'oom_killed':d['State']['OOMKilled'],'memory_peak':peak,'memory_events':events,'temporary_files_left':len(list(temp.iterdir()))}
   match=re.search(r'historical_burst requested=(\d+) accepted=(\d+) rejected=(\d+) peak=(\d+)',log)
   if match:row.update(zip(['requested','accepted','rejected','peak_inflight'],map(int,match.groups())))
   results.append(row)
   (root/'linux-validation.json').write_text(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'trace_requests':len(trace['requests']),'trace_sha256':hashlib.sha256((root/'trace.json').read_bytes()).hexdigest(),'production_limits':production,'results':results},indent=2))
   print(json.dumps(row),flush=True)
   if row['exit_code']!=0:print(log[-4000:],flush=True)
   assert row['exit_code']==0 and not row['oom_killed'] and events.get('oom',0)==0 and events.get('oom_kill',0)==0, label
   assert row['temporary_files_left']==0,label
   if not baseline and repeats==1:
    previous=next(r for r in results if r['case']==case and r['baseline'])
    if row.get('accepted',0)<previous.get('accepted',0):
     regressions.append({'case':case,'baseline_accepted':previous.get('accepted',0),'candidate_accepted':row.get('accepted',0)})
    if not compact:assert row['accepted']==row['requested'], 'Full physical reservation/low limit must complete all accepted-size requests'
  finally:
   subprocess.run(['docker','rm','-f',name],stdout=subprocess.DEVNULL)

if repeats>1:
 medians={}
 for case in cases:
  name=case[0]
  before=[r['accepted'] for r in results if r['case']==name and r['baseline']]
  after=[r['accepted'] for r in results if r['case']==name and not r['baseline']]
  assert len(before)==len(after)==5
  medians[name]={'baseline_samples':before,'candidate_samples':after,'baseline_median':statistics.median(before),'candidate_median':statistics.median(after)}
  if statistics.median(after)<statistics.median(before):regressions.append(medians[name])
 data=json.loads((root/'linux-validation.json').read_text());data['comparison_medians']=medians
 (root/'linux-validation.json').write_text(json.dumps(data,indent=2))
 print(json.dumps({'comparison_medians':medians}),flush=True)
assert not regressions, "Admission regressed: "+json.dumps(regressions)
