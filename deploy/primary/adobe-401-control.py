"""Narrow controls for this reviewed release, retaining current data and configuration."""
import argparse,datetime,hashlib,json,os,pathlib,subprocess,sys
os.umask(0o077)
root=pathlib.Path('/opt/adobe-relay/releases/adobe-401-repair-20261020')
parser=argparse.ArgumentParser();parser.add_argument('action',choices=['prepare','release','verify','defer-old','defer-canary','recheck']);args=parser.parse_args()
def run(name,*extra):return subprocess.run([sys.executable,str(root/name),*extra],check=True,timeout=360)
if args.action=='prepare':
 assert hashlib.sha256(pathlib.Path('/opt/adobe-relay/deploy/primary/release.sh').read_bytes()).hexdigest()=='4bf42b5920d67c0f1c11e4c096080e65af3c999a56f310a48c45c8d0e8ae1154'
 if not(root/'before.json').exists():run('audit.py','before')
 run('load-candidate.py')
elif args.action=='release':
 assert (root/'image-loaded.json').exists()
 marker=root/'driver-started.json'
 if marker.exists():print(json.dumps({'already_started':True}));sys.exit(0)
 fd=os.open(str(marker),os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600);os.write(fd,json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat()}).encode());os.close(fd)
 with(root/'driver-private.log').open('w')as log:p=subprocess.Popen([sys.executable,str(root/'driver.py')],stdin=subprocess.DEVNULL,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
 print(json.dumps({'driver_started':True,'pid':p.pid}),flush=True)
elif args.action=='recheck':
 batch=pathlib.Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020/401-manual-batch-v2')
 assert(batch/'finished.json').exists(),'Wait for the original batch to finish'
 marker=batch/'recheck-started.json'
 if marker.exists():print(json.dumps({'recheck_already_started':True}));sys.exit(0)
 fd=os.open(str(marker),os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600);os.write(fd,b'{}');os.close(fd)
 with(batch/'recheck-private.log').open('w')as log:subprocess.run([sys.executable,str(pathlib.Path(__file__).with_name('adobe-401-host-worker.py'))],stdout=log,stderr=subprocess.STDOUT,check=True,timeout=500)
 result=json.loads((batch/'finished.json').read_text());(batch/'recheck-finished.json').write_text(json.dumps(result));print(json.dumps(result),flush=True)
 failures=[]
 for p in batch.glob('result-*-private.json'):
  d=json.loads(p.read_text())
  if not d.get('ok'):failures.append(d)
 print(json.dumps({'remaining_outcome_codes':{c:sum(r.get('code')==c for r in failures)for c in sorted(set(r.get('code')for r in failures))},'remaining_error_kinds':{c:sum(r.get('kind')==c for r in failures)for c in sorted(set(r.get('kind','unspecified')for r in failures))}}),flush=True)
elif args.action.startswith('defer-'):
 phase=args.action.split('-',1)[1]
 with(root/('defer-'+phase+'-private.log')).open('w')as log:p=subprocess.Popen([sys.executable,str(root/'defer-pending-tests.py'),phase],stdin=subprocess.DEVNULL,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
 print(json.dumps({'deferred_phase_started':phase,'pid':p.pid}),flush=True)
else:
 assert(root/'after.json').exists()and(root/'rolling-finished.json').exists()
 for filename,output in [('observe.py','observation.json'),('business-verify.py','business-verification.json'),('review-credentials.py','credential-change-review.json')]:
  if filename!='observe.py' or not(root/output).exists():
   # Detailed operational data remains private on the primary.
   with(root/(filename+'.private.log')).open('w')as log:subprocess.run([sys.executable,str(root/filename)],check=True,stdout=log,stderr=subprocess.STDOUT,timeout=300)
 run('audit.py','check-rollback')
 run('audit.py','verify')
 run('final-verify.py')
 quota_helper=pathlib.Path('/opt/adobe-relay/releases/adobe-session-renewal-20261020/verify-native-host.py')
 result=subprocess.run([sys.executable,str(quota_helper),'--port','6666','--quota-only'],capture_output=True,text=True,timeout=150)
 (root/'original-primary-quota-private.jsonl').write_text(result.stdout)
 quota=[json.loads(line)for line in result.stdout.splitlines()if line.startswith('{')]
 valid=result.returncode==0 and any(r.get('phase')=='primary_quota'and r.get('http')==200 and not r.get('has_error')for r in quota)
 print(json.dumps({'original_primary_quota_validated':valid}),flush=True)
 assert valid,'Original primary quota check failed; inspect its private result before declaring completion'
 print(json.dumps({'final_verified':True,'at':json.loads((root/'final-verification.json').read_text())['at']}),flush=True)
