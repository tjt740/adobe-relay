"""Prepare immutable preservation and backup evidence without starting an app."""
import datetime,fcntl,json,pathlib,subprocess,sys
ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-route-health-20261023')
lock=open('/opt/adobe-relay/shared/.adobe-deploy.lock','a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
for name,args,output in [('audit.py',['before'],'before.json'),('pre-schema.py',[],'premigration-backup.json')]:
 if (ROOT/output).exists():continue
 subprocess.run([sys.executable,str(ROOT/name),*args],check=True,timeout=700)
(ROOT/'preflight-complete.json').write_text(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_started':False,'backup_verified':json.loads((ROOT/'premigration-backup.json').read_text())['dump_verified']}))
print('pre-migration backup and preservation baseline complete; runtime unchanged')
