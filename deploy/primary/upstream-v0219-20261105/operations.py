"""Operate the checked upstream upgrade through the unchanged primary release guards."""
import base64,hashlib,json,os,re,subprocess,sys
from pathlib import Path
phase,sha,build=sys.argv[1:]
assert phase in ['preflight','prepare','start','resume','status','verify','archive','defer-old','defer-canary']
assert re.fullmatch('[0-9a-f]{40}',sha) and build.isdigit()
ROOT='/opt/adobe-relay/releases/upstream-v0219-20261105'
HERE=Path(__file__).parent
options=['-i',os.environ.get('PRIMARY_SSH_IDENTITY',str(Path.home()/'.ssh/deploy_key')),'-o','BatchMode=yes','-o','StrictHostKeyChecking=yes','-o','ConnectTimeout=20','-o','ServerAliveInterval=15','-o','ServerAliveCountMax=3']
target='root@173.234.15.150'
def remote(script,timeout=900,capture=False):
 return subprocess.run(['ssh',*options,target,'python3 -'],input=script,text=True,check=True,timeout=timeout,capture_output=capture)
def copy(paths,dest=ROOT+'/'):
 subprocess.run(['scp',*options,*map(str,paths),target+':'+dest],check=True,timeout=240)
if phase=='preflight':
 remote("""import fcntl,json,subprocess
held=[]
for p in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(p,'r');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
d=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
assert d['Config']['Image']=='adobe-relay:4569e1bd2e1e8957d1a6bbba677f67d76d56b47e'
print(json.dumps({'locks_available':True,'rolling_lease_empty':True,'current_image':d['Config']['Image'],'health':d['State']['Health']['Status'],'read_only':True}))
""")
elif phase=='prepare':
 archive=Path('release-artifact/release-images.tar.gz');digest=hashlib.sha256(archive.read_bytes()).hexdigest()
 assert Path(str(archive)+'.sha256').read_text().split()==[digest,archive.name]
 Path('release-artifact/ci-proof.json').write_text(json.dumps({'application_sha':sha,'workflow_run':int(build),'conclusion':'success','archive_sha256':digest}))
 remote(f"from pathlib import Path; r=Path('{ROOT}'); assert not (r/'progress.json').exists(); (r/'artifact').mkdir(parents=True,exist_ok=True,mode=0o700)")
 copy([*HERE.glob('*.py'),HERE/'migration-manifest.json'])
 copy(Path('release-artifact').glob('*'),ROOT+'/artifact/')
 release_hash=hashlib.sha256(Path('deploy/primary/release.sh').read_bytes()).hexdigest()
 remote(f"""import fcntl,hashlib,json,subprocess,sys
from pathlib import Path
lock=open('/opt/adobe-relay/shared/.adobe-deploy.lock','a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
r=Path('{ROOT}')
assert hashlib.sha256(Path('/opt/adobe-relay/deploy/primary/release.sh').read_bytes()).hexdigest()=='{release_hash}'
(r/'expected-sha').write_text('{sha}');(r/'expected-run').write_text('{build}')
if not (r/'before.json').exists():subprocess.run([sys.executable,str(r/'audit.py'),'before'],check=True)
if not (r/'premigration-backup.json').exists():subprocess.run([sys.executable,str(r/'pre-schema.py')],check=True)
assert json.loads((r/'premigration-backup.json').read_text())['dump_verified']
if not (r/'image-loaded.json').exists():subprocess.run([sys.executable,str(r/'load-candidate.py')],check=True)
""")
elif phase=='start':
 remote(f"""import json,subprocess
from pathlib import Path
r=Path('{ROOT}');assert (r/'expected-sha').read_text().strip()=='{sha}'
assert json.loads((r/'image-loaded.json').read_text())['sha']=='{sha}'
assert not (r/'progress.json').exists(),'Driver already started; inspect its evidence'
subprocess.run(['systemd-run','--unit=adobe-upstream-v0219-release-20261105','--collect','--property=WorkingDirectory={ROOT}','--property=StandardOutput=append:{ROOT}/driver.log','--property=StandardError=append:{ROOT}/driver.log','/usr/bin/python3',str(r/'driver.py')],check=True)
print(json.dumps({{'driver_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='resume':
 remote(f"""import fcntl,json,subprocess
from pathlib import Path
r=Path('{ROOT}');assert (r/'expected-sha').read_text().strip()=='{sha}'
lock=open(r/'driver.lock','a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
p=json.loads((r/'progress.json').read_text());assert p['step']=='failed' and p['state']=='attention_required'
assert 'schema-verify.py' in p['error']
assert not (r/'rolling-route-canary.json').exists() and not (r/'resume-dispatched.json').exists()
for name in ['progress.json','migration-manifest.json']:
 dst=r/('initial-'+name);assert not dst.exists();dst.write_bytes((r/name).read_bytes());dst.chmod(0o600)
""")
 copy([HERE/'driver.py',HERE/'resume_guard.py',HERE/'migration-manifest.json'])
 remote(f"""import datetime,fcntl,json,subprocess
from pathlib import Path
r=Path('{ROOT}');lock=open(r/'driver.lock','a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
assert not (r/'resume-dispatched.json').exists()
subprocess.run(['systemd-run','--unit=adobe-upstream-v0219-resume-20261105','--collect','--property=WorkingDirectory={ROOT}','--property=StandardOutput=append:{ROOT}/driver.log','--property=StandardError=append:{ROOT}/driver.log','/usr/bin/python3',str(r/'driver.py'),'--resume-schema-check'],check=True)
(r/'resume-dispatched.json').write_text(json.dumps({{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':'{sha}','unchanged_application_artifact':True}}))
print(json.dumps({{'resume_dispatched':True}}))
""")
elif phase=='verify':
 remote(f"""import subprocess,sys
from pathlib import Path
r=Path('{ROOT}');assert (r/'expected-sha').read_text().strip()=='{sha}'
assert (r/'rolling-finished.json').exists()
for script,output in [('route-guard-review.py','route-guard-review.json'),('business-verify.py','business-verification.json'),('review-credentials.py','credential-change-review.json'),('observe.py','observation.json'),('final-verify.py','final-verification.json')]:
 if not (r/output).exists():subprocess.run([sys.executable,str(r/script)],check=True)
subprocess.run([sys.executable,str(r/'audit.py'),'check-rollback'],check=True)
""")
elif phase.startswith('defer-'):
 which=phase.removeprefix('defer-')
 copy([HERE/'defer-pending-tests.py'])
 remote(f"from pathlib import Path;import subprocess,sys;r=Path('{ROOT}');assert (r/'expected-sha').read_text().strip()=='{sha}';subprocess.run([sys.executable,str(r/'defer-pending-tests.py'),'{which}'],check=True,timeout=330)")
elif phase=='archive':
 documents={'release-notes.md':Path('deploy/primary/upstream-v0.2.19-upgrade-20261105.md'),'primary-README.md':Path('deploy/primary/README.md')}
 payload={k:base64.b64encode(p.read_bytes()).decode() for k,p in documents.items()}
 remote(f"""import base64,datetime,fcntl,hashlib,json,subprocess
from pathlib import Path
r=Path('{ROOT}');held=[]
for p in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(p,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
d=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
assert d['Config']['Image']=='adobe-relay:{sha}'
assert d['Image']==json.loads((r/'image-loaded.json').read_text())['image_id']
assert json.loads((r/'final-verification.json').read_text())['application_sha']=='{sha}'
files={{}}
for name,value in {payload!r}.items():
 data=base64.b64decode(value);assert b'PENDING' not in data;(r/name).write_bytes(data);(r/name).chmod(0o600);files[name]=hashlib.sha256(data).hexdigest()
 if name=='primary-README.md':Path('/opt/adobe-relay/deploy/primary/README.md').write_bytes(data)
out={{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':'{sha}','image_id':d['Image'],'document_hashes':files,'mail_sent':False}}
(r/'release-completion.json').write_text(json.dumps(out,indent=2));print(json.dumps(out))
""")
else:
 script=f"""import json
from pathlib import Path
r=Path('{ROOT}');out={{'application_sha':'{sha}','files':{{}}}}
import importlib.util
spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
b=json.loads((r/'installed-migrations-before.json').read_text());i=json.loads(a.sql('SELECT json_object_agg(filename,checksum) FROM schema_migrations'))
out['schema_diagnostic']={{'old_checksums_preserved':all(i.get(k)==v for k,v in b.items()),'new_installed':{{k:v for k,v in i.items() if k not in b}},'current_image':a.inspect(a.APP)['Config']['Image']}}

for name in ['progress.json','initial-progress.json','resume-validation.json','image-loaded.json','premigration-backup.json','schema-verification.json','rolling-route-canary.json','rolling-route-canonical.json','rolling-finished.json','deferred-old.json','deferred-canary.json','runtime-verification.json','business-verification.json','credential-change-review.json','final-verification.json','release-completion.json']:
 if (r/name).exists():out['files'][name]=json.loads((r/name).read_text())
for side in ['old','canary']:
 p=r/('rolling-'+side+'-drain-progress.json')
 if p.exists():out['files'][p.name]=json.loads(p.read_text())['samples'][-1]
if (r/'after.json').exists():out['cutover_backup']=json.loads((r/'after.json').read_text())['backup']
if (r/'driver.log').exists():out['driver_errors']=[x for x in (r/'driver.log').read_text().splitlines() if x.startswith(('AssertionError:','RuntimeError:','subprocess.CalledProcessError:'))][-8:]
print(json.dumps(out,ensure_ascii=False))
"""
 result=remote(script,timeout=90,capture=True)
 evidence=Path('primary-release-evidence');evidence.mkdir(exist_ok=True)
 (evidence/'release-status.json').write_text(result.stdout);print(result.stdout)
