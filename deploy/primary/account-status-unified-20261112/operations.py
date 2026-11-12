"""Operate the authorized release with exact CI artifacts and preserved guards."""
import base64,datetime,hashlib,json,os,pathlib,re,subprocess,sys
phase,sha,build=sys.argv[1:]
assert phase in ['preflight','prepare','start','resume-drain','review-operator-edits','docs-access','status','verify','archive']
assert re.fullmatch('[0-9a-f]{40}',sha) and build.isdigit()
root='/opt/adobe-relay/releases/account-status-unified-20261112'
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
assert d['Config']['Image']=='adobe-relay:e1c9b83ace0472ad7e97b35d4bf2c4dd5dc1574b' and d['State']['Health']['Status']=='healthy'
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
 d=pathlib.Path('/opt/adobe-relay/backups')/(datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')+'-before-account-status-unified-20261112');d.mkdir(mode=0o700)
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
subprocess.run(['systemd-run','--unit=account-status-unified-release-20261112','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/driver.log','--property=StandardError=append:{root}/driver.log','/usr/bin/python3',str(r/'driver.py')],check=True)
print(json.dumps({{'driver_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='resume-drain':
 copy([pathlib.Path(__file__).with_name('resume-drain.py')])
 remote(f"""import subprocess,pathlib,json
r=pathlib.Path('{root}');p=json.loads((r/'progress.json').read_text())
assert p['step']=='failed' and "'drain-canary'" in p['error']
assert not (r/'canary-drain-retry-1.json').exists()
subprocess.run(['systemd-run','--unit=account-status-unified-drain-20261112-retry1','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/driver.log','--property=StandardError=append:{root}/driver.log','/usr/bin/python3',str(r/'resume-drain.py')],check=True)
print('Existing canary natural drain continuation dispatched')
""")
elif phase=='review-operator-edits':
 copy([pathlib.Path(__file__).with_name('review-operator-edits.py'),pathlib.Path(__file__).with_name('verify-deployment.py')])
 remote(f"import subprocess,pathlib; r=pathlib.Path('{root}'); assert not (r/'operator-changes-review.json').exists(); subprocess.run(['/usr/bin/python3',str(r/'review-operator-edits.py')],check=True)")
elif phase=='docs-access':
 remote(r"""import collections,datetime,hashlib,json,pathlib,re
start=datetime.datetime(2026,11,12,6,53,tzinfo=datetime.timezone.utc)
end=datetime.datetime(2026,11,12,7,2,tzinfo=datetime.timezone.utc)
rows=[]
for line in pathlib.Path('/var/log/nginx/creative-docs-access.log').read_bytes()[-4000000:].decode(errors='replace').splitlines():
 m=re.match(r'^(\S+) \[([^]]+)\] "(\S+) (\S+) [^"]+" (\d+) ',line)
 if not m:continue
 at=datetime.datetime.strptime(m[2],'%d/%b/%Y:%H:%M:%S %z')
 if start<=at<=end:rows.append({'at':at.isoformat(),'method':m[3],'path':m[4],'status':int(m[5]),'client_fingerprint':hashlib.sha256(m[1].encode()).hexdigest()[:12]})
print(json.dumps({'read_only':True,'window_utc':[start.isoformat(),end.isoformat()],'counts':dict(collections.Counter(str(r['status']) for r in rows)),'errors':[r for r in rows if r['status']>=400],'successful_independent_reading':[r for r in rows if r['status']==200 and datetime.datetime.fromisoformat(r['at'])>=datetime.datetime(2026,11,12,7,0,tzinfo=datetime.timezone.utc) and r['path'] in ['/docs/session','/docs/session/guide.md']]}))
""")
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
 remote(f"""import pathlib
r=pathlib.Path('{root}'); assert (r/'operator-changes-review.json').exists()
for name in ['review-operator-edits.py','verify-deployment.py']:
 original=r/(name+'.before-live-audit')
 if not original.exists():original.write_bytes((r/name).read_bytes());original.chmod(0o600)
""")
 copy([pathlib.Path(__file__).with_name('review-operator-edits.py'),pathlib.Path(__file__).with_name('verify-deployment.py')])
 browser=pathlib.Path(__file__).with_name('live-browser-verification.json')
 if browser.exists():
  proof=json.loads(browser.read_text());assert proof['application_sha']==sha and proof['authenticated'] and proof['case_count']>=4 and not proof['attempted_writes'] and not proof['runtime_errors']
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
 documents={'release-notes.md':pathlib.Path('deploy/primary/account-status-unified-20261112.md'),'primary-README.md':pathlib.Path('deploy/primary/README.md'),'release-verification.json':pathlib.Path(__file__).with_name('release-verification.json')}
 payload={name:base64.b64encode(path.read_bytes()).decode() for name,path in documents.items()}
 source=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()
 remote(f"""import base64,datetime,fcntl,hashlib,importlib.util,json,pathlib,subprocess
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
assert metadata['application_sha']=='{sha}' and metadata['status']=='deployed_account_status_filters_verified'
before=json.loads((r/'before.json').read_text());current=a.snapshot()
checks=a.check_preserved(before,current);a.validate_rollback(before,current,'{sha}',a.inspect(a.ROLLBACK_IMAGE)['Id'])
live_operator_review=None
if (r/'operator-changes-review.json').exists():
 live_operator_review=json.loads(subprocess.check_output(['/usr/bin/python3',str(r/'review-operator-edits.py'),'--validate-only'],text=True,timeout=150));a.save('operator-changes-review-archive.json',live_operator_review)
assert all(current['containers'][name]['started']==item['started'] and current['containers'][name]['restarts']==item['restarts'] for name,item in before['containers'].items() if name!=a.APP)
files={{}}
for name,value in {payload!r}.items():
 data=base64.b64decode(value);assert b'PENDING' not in data
 path=r/name;path.write_bytes(data);path.chmod(0o600);files[name]=a.digest(data)
 if name=='primary-README.md':pathlib.Path('/opt/adobe-relay/deploy/primary/README.md').write_bytes(data)
out={{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'status':'deployed_account_status_filters_verified','application_sha':'{sha}','archive_source':'{source}','image_id':container['Image'],'core_verified_at':core['at'],'process_preservation':processes,'published_document_hashes':files,'preserved':checks,'live_operator_review':live_operator_review,'rollback_guard':{{'passed':True,'mutated':False}},'cutover_backup':json.loads((r/'after.json').read_text())['backup'],'active_generation':False,'production_priority_changed':False,'mail_sent':False}}
a.save('release-completion.json',out);print(json.dumps(out))
""")
