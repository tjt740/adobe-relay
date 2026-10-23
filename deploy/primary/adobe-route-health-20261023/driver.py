"""Run the already-reviewed capacity handoff, retaining every accepted request."""
import datetime, fcntl, json, os, pathlib, subprocess, sys, traceback

ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-route-health-20261023')
SHA=(ROOT/'expected-sha').read_text().strip()
def progress(step,state,**extra):
 obj={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'step':step,'state':state,**extra}
 p=ROOT/'progress.json';p.write_text(json.dumps(obj,ensure_ascii=False));p.chmod(0o600)
 print(json.dumps(obj,ensure_ascii=False),flush=True)
def run_step(name,args,timeout=1700):
 progress(name,'running')
 subprocess.run(args,cwd=ROOT,check=True,timeout=timeout)
 progress(name,'done')
if __name__=='__main__':
 os.umask(0o077)
 orchestration=open('/opt/adobe-relay/shared/.adobe-deploy.lock','a');fcntl.flock(orchestration,fcntl.LOCK_EX|fcntl.LOCK_NB)
 lock=open(ROOT/'driver.lock','a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
 assert not (ROOT/'after.json').exists(),'Already complete'
 assert json.loads((ROOT/'premigration-backup.json').read_text())['dump_verified'], 'A verified pre-migration backup is required'
 assert (ROOT/'capacity-before.json').exists(), 'Capacity baseline must precede the new process'
 heartbeat=None
 try:
  run_step('capacity-before-candidate',[sys.executable,str(ROOT/'capacity.py'),'pre-switch'],timeout=90)
  run_step('prepare-candidate',[sys.executable,str(ROOT/'rolling.py'),'prepare','--sha',SHA])
  run_step('seed-adobe-exclusion',[sys.executable,str(ROOT/'seed-exclusion.py')],timeout=60)
  heartbeat=subprocess.Popen([sys.executable,str(ROOT/'rolling.py'),'heartbeat'])
  for step in ['route-canary','drain-old']:
   run_step(step,[sys.executable,str(ROOT/'rolling.py'),step])
  run_step('canonical-recreate',['bash','/opt/adobe-relay/deploy/primary/release.sh','adobe-relay:'+SHA,'adobe-relay:3d0f78a461e37b8a9bbaea8a807bfede57b2a60e','--keep-canary-on-failure'],timeout=600)
  for step in ['route-canonical','drain-canary','cleanup']:
   run_step(step,[sys.executable,str(ROOT/'rolling.py'),step])
  run_step('preservation-verification',[sys.executable,str(ROOT/'audit.py'),'after'],timeout=180)
  run_step('runtime-verification',[sys.executable,str(ROOT/'audit.py'),'verify'],timeout=180)
  progress('complete','done')
 except Exception as e:
  progress('failed','attention_required',error=str(e)[:400]);traceback.print_exc();raise
 finally:
  # On a failed handoff leave heartbeat alive to protect accepted canary slots.
  if heartbeat is not None and (ROOT/'rolling-finished.json').exists():heartbeat.terminate();heartbeat.wait(timeout=10)
