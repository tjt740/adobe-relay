"""Operate the authorized release with exact CI artifacts and preserved guards."""
import datetime,hashlib,json,os,pathlib,re,subprocess,sys
phase,sha,build=sys.argv[1:]
assert phase in ['preflight','prepare','start','status','verify','live-billing','resume','defer-old','defer-canary']
assert re.fullmatch('[0-9a-f]{40}',sha) and build.isdigit()
root='/opt/adobe-relay/releases/adobe-webp-20261111'
options=['-i',os.environ.get('PRIMARY_SSH_IDENTITY',str(pathlib.Path.home()/'.ssh/deploy_key')),'-o','BatchMode=yes','-o','StrictHostKeyChecking=yes','-o','ConnectTimeout=20','-o','ServerAliveInterval=15','-o','ServerAliveCountMax=3']
target='root@173.234.15.150'
def remote(script):return subprocess.run(['ssh',*options,target,'python3 -'],input=script,text=True,check=True,timeout=900)
def copy(paths,dest=root+'/'):subprocess.run(['scp',*options,*map(str,paths),target+':'+dest],check=True,timeout=300)
if phase=='preflight':
 remote("""import fcntl,json,subprocess
held=[]
for path in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(path,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
d=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
assert d['Config']['Image']=='adobe-relay:77e6ece37dc357728699211b9ab2799d4298de00' and d['State']['Health']['Status']=='healthy'
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
print(json.dumps({'read_only':True,'locks_available':True,'rolling_lease_empty':True,'image':d['Config']['Image']}))
""")
elif phase=='prepare':
 ci=json.loads(subprocess.check_output(['gh','api','repos/tjt740/adobe-relay/actions/runs/'+build]))
 assert ci['head_sha']==sha and ci['status']=='completed' and ci['conclusion']=='success' and ci['path']=='.github/workflows/deploy-adobe-relay.yml'
 subprocess.run([sys.executable,str(pathlib.Path(__file__).with_name('source_guard.py')),sha],check=True)
 artifact=pathlib.Path(os.environ.get('PRIMARY_RELEASE_ARTIFACT','release-artifact'))
 archive=artifact/'release-images.tar.gz';digest=hashlib.sha256(archive.read_bytes()).hexdigest()
 assert pathlib.Path(str(archive)+'.sha256').read_text().split()==[digest,archive.name]
 (artifact/'ci-proof.json').write_text(json.dumps({'application_sha':sha,'workflow_run':int(build),'conclusion':'success','archive_sha256':digest}))
 remote(f"import pathlib; r=pathlib.Path('{root}'); assert not (r/'progress.json').exists(); (r/'artifact').mkdir(parents=True,exist_ok=True,mode=0o700)")
 copy(pathlib.Path(__file__).parent.glob('*.py'));copy(pathlib.Path(__file__).parent.glob('*.json'))
 copy([pathlib.Path(os.environ.get('PRIMARY_WEBP_VALIDATOR','release-tooling/webp-validate'))])
 copy(artifact.glob('*'),root+'/artifact/')
 release_hash=hashlib.sha256(pathlib.Path('deploy/primary/release.sh').read_bytes()).hexdigest()
 remote(f"""import fcntl,hashlib,importlib.util,json,pathlib,subprocess,sys,datetime
orchestration=open('/opt/adobe-relay/shared/.adobe-deploy.lock','a');fcntl.flock(orchestration,fcntl.LOCK_EX|fcntl.LOCK_NB)
r=pathlib.Path('{root}')
assert hashlib.sha256(pathlib.Path('/opt/adobe-relay/deploy/primary/release.sh').read_bytes()).hexdigest()=='{release_hash}'
(r/'expected-sha').write_text('{sha}');(r/'expected-run').write_text('{build}')
if not (r/'before.json').exists():subprocess.run([sys.executable,str(r/'audit.py'),'before'],check=True)
spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
if not (r/'installed-migrations-before.json').exists():a.save('installed-migrations-before.json',json.loads(a.sql("SELECT json_object_agg(filename,checksum) FROM schema_migrations")))
if not (r/'premigration-backup.json').exists():
 lock=open('/opt/adobe-relay/.release.lock','a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
 d=pathlib.Path('/opt/adobe-relay/backups')/(datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')+'-before-adobe-webp-20261111');d.mkdir(mode=0o700)
 with (d/'database.dump').open('wb') as f:subprocess.run(['docker','exec','adobe-relay-postgres-1','pg_dump','-U','sub2api','-d','sub2api','-Fc'],stdout=f,check=True,timeout=180)
 with (d/'database.dump').open('rb') as f:subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--list'],stdin=f,stdout=subprocess.DEVNULL,check=True,timeout=60)
 a.save('premigration-backup.json',{{'path':str(d),'verified':True,'sha256':hashlib.sha256((d/'database.dump').read_bytes()).hexdigest()}})
 lock.close()
if not (r/'image-loaded.json').exists():subprocess.run([sys.executable,str(r/'load-candidate.py')],check=True)
""")
elif phase=='start':
 remote(f"""import json,pathlib,subprocess
r=pathlib.Path('{root}')
assert (r/'expected-sha').read_text().strip()=='{sha}' and json.loads((r/'image-loaded.json').read_text())['sha']=='{sha}'
assert json.loads((r/'premigration-backup.json').read_text())['verified']
assert not (r/'progress.json').exists(), 'Already started; inspect saved evidence'
subprocess.run(['systemd-run','--unit=adobe-webp-release-20261111','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/driver.log','--property=StandardError=append:{root}/driver.log','/usr/bin/python3',str(r/'driver.py')],check=True)
print(json.dumps({{'driver_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='status':
 remote(f"""import pathlib,json
r=pathlib.Path('{root}')
for name in ['progress.json','rolling-old-drain-progress.json','rolling-canary-drain-progress.json','after.json','api-verification.json','deployment-verification.json','live-billing-verification.json','live-billing-cleanup.json','live-flare-default-result.json','live-sunburst-transparent-25-result.json','deferred-old.json','deferred-canary.json']:
 p=r/name
 if p.exists():
  d=json.loads(p.read_text())
  if name=='after.json':d={{'backup':d['backup'],'preserved':d['preserved']}}
  if name.startswith('rolling-') and 'samples' in d:d={{'sample_count':len(d['samples']),'last_samples':d['samples'][-2:]}}
  print(name,json.dumps(d))
p=r/'driver.log'
if p.exists():print(p.read_text()[-3500:])
""")
elif phase=='verify':
 remote(f"""import pathlib,subprocess,sys
r=pathlib.Path('{root}')
assert (r/'expected-sha').read_text().strip()=='{sha}'
for script,out in [('business-verify.py','business-verification.json'),('review-credentials.py','credential-change-review.json'),('route-guard-review.py','route-guard-review.json'),('api-verify.py','api-verification.json'),('observe.py','observation.json'),('verify-deployment.py','deployment-verification.json')]:
 if not (r/out).exists():subprocess.run([sys.executable,str(r/script)],check=True,timeout=240)
""")

elif phase=='live-billing':
 copy([pathlib.Path(__file__).with_name(n) for n in ['live-billing.py','live_validation.py']])
 remote(f"""import pathlib,subprocess,json
r=pathlib.Path('{root}')
assert (r/'expected-sha').read_text().strip()=='{sha}'
assert (r/'deployment-verification.json').exists()
assert not (r/'live-billing-started.json').exists(), 'Already started; no replay'
subprocess.run(['systemd-run','--unit=adobe-webp-billing-20261111','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/live-billing.log','--property=StandardError=append:{root}/live-billing.log','/usr/bin/python3',str(r/'live-billing.py')],check=True)
print(json.dumps({{'single_live_billing_dispatch':True}}))
""")

elif phase=='resume':
 # Resume only a proven pre-migration failure, with no canary/lease/route created.
 remote(f"""import pathlib,json,subprocess
r=pathlib.Path('{root}')
assert (r/'expected-sha').read_text().strip()=='{sha}'
p=json.loads((r/'progress.json').read_text())
assert p['step']=='failed' and 'prepare' in p['error']
assert not (r/'rolling-prepared.json').exists() and not (r/'rolling-route-canary.json').exists()
d=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
assert d['Id']==json.loads((r/'before.json').read_text())['containers']['adobe-relay-app-1']['id']
assert d['Config']['Image']=='adobe-relay:77e6ece37dc357728699211b9ab2799d4298de00'
assert subprocess.run(['docker','inspect','adobe-relay-memory-canary'],capture_output=True).returncode!=0
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
assert not (r/'progress-attempt1-failed.json').exists()
""")
 copy([pathlib.Path(__file__).with_name(n) for n in ['audit.py','operations.py']])
 remote(f"""import pathlib,subprocess,json
r=pathlib.Path('{root}')
(r/'progress.json').rename(r/'progress-attempt1-failed.json')
subprocess.run(['systemd-run','--unit=adobe-webp-release-attempt2-20261111','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/driver.log','--property=StandardError=append:{root}/driver.log','/usr/bin/python3',str(r/'driver.py')],check=True)
print(json.dumps({{'guarded_resume_dispatched':True,'original_failure_retained':True}}))
""")

elif phase in ['defer-old','defer-canary']:
 part=phase.removeprefix('defer-')
 remote(f"""import pathlib,subprocess,sys
r=pathlib.Path('{root}')
assert (r/'expected-sha').read_text().strip()=='{sha}'
subprocess.run([sys.executable,str(r/'defer-pending-tests.py'),'{part}'],check=True,timeout=310)
""")
