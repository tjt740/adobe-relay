"""Operate the authorized release with exact CI artifacts and preserved guards."""
import base64,datetime,hashlib,json,os,pathlib,re,subprocess,sys
phase,sha,build=sys.argv[1:]
assert phase in ['preflight','prepare','start','resume-precutover','resume-drain','review-operator-edits','inspect-subscription','review-subscription','status','verify','archive']
assert re.fullmatch('[0-9a-f]{40}',sha) and build.isdigit()
root='/opt/adobe-relay/releases/clash-ui-combined-20261112'
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
assert d['Config']['Image']=='adobe-relay:47c5c46ead01a03cbfa7c547e4da971a561f86c7' and d['State']['Health']['Status']=='healthy'
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
 d=pathlib.Path('/opt/adobe-relay/backups')/(datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')+'-before-clash-ui-combined-20261112');d.mkdir(mode=0o700)
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
subprocess.run(['systemd-run','--unit=clash-ui-combined-release-20261112','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/driver.log','--property=StandardError=append:{root}/driver.log','/usr/bin/python3',str(r/'driver.py')],check=True)
print(json.dumps({{'driver_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='resume-precutover':
 remote(pathlib.Path(__file__).with_name('review_concurrent_okad.py').read_text()+'\n'+pathlib.Path(__file__).with_name('resume-precutover.py').read_text())
elif phase=='resume-drain':
 copy([pathlib.Path(__file__).with_name('resume-drain.py')])
 remote(f"""import subprocess,pathlib,json
r=pathlib.Path('{root}');p=json.loads((r/'progress.json').read_text())
assert p['step']=='failed' and "'drain-canary'" in p['error']
assert not (r/'canary-drain-retry-1.json').exists()
subprocess.run(['systemd-run','--unit=clash-ui-combined-drain-20261112-retry1','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/driver.log','--property=StandardError=append:{root}/driver.log','/usr/bin/python3',str(r/'resume-drain.py')],check=True)
print('Existing canary natural drain continuation dispatched')
""")
elif phase=='review-operator-edits':
 copy([pathlib.Path(__file__).with_name('review-operator-edits.py'),pathlib.Path(__file__).with_name('verify-deployment.py')])
 remote(f"import subprocess,pathlib; r=pathlib.Path('{root}'); assert not (r/'operator-changes-review.json').exists(); subprocess.run(['/usr/bin/python3',str(r/'review-operator-edits.py')],check=True)")
elif phase=='inspect-subscription':
 remote(pathlib.Path(__file__).with_name('inspect-subscription.py').read_text())
elif phase=='review-subscription':
 copy([pathlib.Path(__file__).with_name(name) for name in ['audit.py','api-verify.py','review-subscription.py','subscription_review_guard.py']])
 remote(f"import subprocess,pathlib; r=pathlib.Path('{root}'); assert not (r/'reviewed-operator-subscription.json').exists(); subprocess.run(['/usr/bin/python3',str(r/'review-subscription.py')],check=True)")
elif phase=='status':
 remote(fr"""import pathlib,json,subprocess,re,hashlib
r=pathlib.Path('{root}')
for name in ['operator-changes-review.json','canary-drain-initial-failure.json','canary-drain-retry-1.json','image-loaded.json','premigration-backup.json','initial-start-failure.json','reviewed-concurrent-okad.json','precutover-retry-1.json','process-preservation.json','rolling-route-canary.json','rolling-route-canonical.json','rolling-finished.json','progress.json','rolling-old-drain-progress.json','rolling-canary-drain-progress.json','after.json','api-verification.json','deployment-verification.json','live-billing-verification.json','live-billing-cleanup.json','live-flare-default-result.json','live-sunburst-transparent-25-result.json','deferred-old.json','deferred-canary.json']:
 p=r/name
 if p.exists():
  d=json.loads(p.read_text())
  if name=='after.json':d={{'backup':d['backup'],'preserved':d['preserved']}}
  if name.startswith('rolling-') and 'samples' in d:d={{'sample_count':len(d['samples']),'last_samples':d['samples'][-2:]}}
  print(name,json.dumps(d))
p=r/'driver.log'
if p.exists():print(p.read_text()[-3500:])
progress=json.loads((r/'progress.json').read_text()) if (r/'progress.json').exists() else {{}}
if progress.get('step')=='drain-canary':
 d=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-memory-canary']))[0]
 raw=subprocess.check_output(['nsenter','-t',str(d['State']['Pid']),'-n','ss','-nti','state','established','sport = :8080'],text=True)
 pending=[]
 for block in re.split(r'\n(?=\S)',raw)[1:]:
  sent=re.search(r'bytes_sent:(\d+)',block)
  if sent is None or int(sent[1])<=128:
   first=block.splitlines()[0].split()
   pending.append({{'connection_fingerprint':hashlib.sha256(' '.join(first[2:]).encode()).hexdigest()[:12],'receive_queue':first[0],'send_queue':first[1],'counters':dict(re.findall(r'(bytes_sent|bytes_received|lastsnd|lastrcv|lastack|segs_in|segs_out):([0-9]+)',block))}})
 print('pending-input-metadata',json.dumps(pending))
""")
elif phase=='verify':
 browser=pathlib.Path(__file__).with_name('live-browser-verification.json')
 if browser.exists():
  proof=json.loads(browser.read_text());assert proof['application_sha']==sha and proof['authenticated'] and proof['case_count']==6 and not proof['attempted_writes'] and not proof['runtime_errors']
  encoded=base64.b64encode(browser.read_bytes()).decode()
  remote(f"import pathlib,base64,json; r=pathlib.Path('{root}'); assert (r/'expected-sha').read_text().strip()=='{sha}'; p=r/'live-browser-verification.json'; data=base64.b64decode('{encoded}'); assert not p.exists() or p.read_bytes()==data; p.write_bytes(data); p.chmod(0o600)")
 remote(f"""import pathlib,subprocess,sys,json,importlib.util,datetime
r=pathlib.Path('{root}')
assert (r/'expected-sha').read_text().strip()=='{sha}'
assert json.loads((r/'progress.json').read_text())['step']=='complete'
spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
before=json.loads((r/'before.json').read_text());current=a.snapshot();preserved=a.check_preserved(before,current)
others={{name:{{'pid':a.inspect(name)['State']['Pid'],'started':item['started'],'restarts':item['restarts'],'image':item['image'],'resources':item['resources']}} for name,item in current['containers'].items() if name!=a.APP}}
assert all(item['started']==before['containers'][name]['started'] and item['restarts']==before['containers'][name]['restarts'] for name,item in others.items())
if not (r/'process-preservation.json').exists():a.save('process-preservation.json',{{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'passed':True,'other_processes':others,'docs_process':current['docs_process'],'preserved':preserved}})
for script,out in [('business-verify.py','business-verification.json'),('review-credentials.py','credential-change-review.json'),('route-guard-review.py','route-guard-review.json'),('api-verify.py','api-verification.json'),('observe.py','observation.json'),('verify-deployment.py','deployment-verification.json')]:
 if not (r/out).exists():subprocess.run([sys.executable,str(r/script)],check=True,timeout=240)
""")

elif phase=='archive':
 documents={'release-notes.md':pathlib.Path('deploy/primary/clash-ui-combined-20261112.md'),'primary-README.md':pathlib.Path('deploy/primary/README.md'),'release-verification.json':pathlib.Path(__file__).with_name('release-verification.json')}
 payload={name:base64.b64encode(path.read_bytes()).decode() for name,path in documents.items()}
 source=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()
 remote(f"""import base64,datetime,fcntl,hashlib,importlib.util,json,pathlib
r=pathlib.Path('{root}');held=[]
for path in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(path,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
assert not (r/'release-completion.json').exists(), 'Completion already recorded; inspect without replay'
spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
container=a.inspect(a.APP);assert container['Config']['Image']=='adobe-relay:{sha}'
assert container['Image']==json.loads((r/'image-loaded.json').read_text())['image_id']
core=json.loads((r/'deployment-verification.json').read_text());assert core['application_sha']=='{sha}' and core['status']=='deployed_api_verified'
processes=json.loads((r/'process-preservation.json').read_text());assert processes['passed']
metadata=json.loads(base64.b64decode({payload!r}['release-verification.json']))
assert metadata['application_sha']=='{sha}' and metadata['status']=='deployed_subscription_settings_verified'
before=json.loads((r/'before.json').read_text());current=a.snapshot()
checks=a.check_preserved(before,current);a.validate_rollback(before,current,'{sha}',a.inspect(a.ROLLBACK_IMAGE)['Id'])
assert all(current['containers'][name]['started']==item['started'] and current['containers'][name]['restarts']==item['restarts'] for name,item in before['containers'].items() if name!=a.APP)
files={{}}
for name,value in {payload!r}.items():
 data=base64.b64decode(value);assert b'PENDING' not in data
 path=r/name;path.write_bytes(data);path.chmod(0o600);files[name]=a.digest(data)
 if name=='primary-README.md':pathlib.Path('/opt/adobe-relay/deploy/primary/README.md').write_bytes(data)
out={{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'status':'deployed_subscription_settings_verified','application_sha':'{sha}','archive_source':'{source}','image_id':container['Image'],'core_verified_at':core['at'],'process_preservation':processes,'published_document_hashes':files,'preserved':checks,'rollback_guard':{{'passed':True,'mutated':False}},'cutover_backup':json.loads((r/'after.json').read_text())['backup'],'active_generation':False,'production_priority_changed':False,'mail_sent':False}}
a.save('release-completion.json',out);print(json.dumps(out))
""")
