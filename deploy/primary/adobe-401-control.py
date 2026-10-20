"""Narrow controls for this reviewed release, retaining current data and configuration."""
import argparse,datetime,hashlib,json,os,pathlib,subprocess,sys
os.umask(0o077)
root=pathlib.Path('/opt/adobe-relay/releases/adobe-401-repair-20261020')
parser=argparse.ArgumentParser();parser.add_argument('action',choices=['prepare','release','verify','defer-old','defer-canary']);args=parser.parse_args()
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
elif args.action.startswith('defer-'):
 phase=args.action.split('-',1)[1]
 with(root/('defer-'+phase+'-private.log')).open('w')as log:p=subprocess.Popen([sys.executable,str(root/'defer-pending-tests.py'),phase],stdin=subprocess.DEVNULL,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
 print(json.dumps({'deferred_phase_started':phase,'pid':p.pid}),flush=True)
else:
 assert(root/'after.json').exists()and(root/'rolling-finished.json').exists()
 for filename,output in [('observe.py','observation.json'),('business-verify.py','business-verification.json'),('review-credentials.py','credential-change-review.json')]:
  if not(root/output).exists():
   # Detailed operational data remains private on the primary.
   with(root/(filename+'.private.log')).open('w')as log:subprocess.run([sys.executable,str(root/filename)],check=True,stdout=log,stderr=subprocess.STDOUT,timeout=300)
 run('audit.py','check-rollback')
 if not(root/'final-verification.json').exists():run('final-verify.py')
 print(json.dumps({'final_verified':True,'at':json.loads((root/'final-verification.json').read_text())['at']}),flush=True)
