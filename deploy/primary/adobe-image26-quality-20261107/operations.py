"""Image 2.6 quality and names release using preserved deployment guards."""
import base64,hashlib,json,os,pathlib,re,subprocess,sys
phase,sha,build=sys.argv[1:]
assert phase in ['preflight','prepare','start','public-test','status','verify','archive']
assert re.fullmatch('[0-9a-f]{40}',sha) and build.isdigit()
root='/opt/adobe-relay/releases/adobe-image26-quality-20261107'
options=['-i',os.environ.get('PRIMARY_SSH_IDENTITY',str(pathlib.Path.home()/'.ssh/deploy_key')),'-o','BatchMode=yes','-o','StrictHostKeyChecking=yes','-o','ConnectTimeout=20','-o','ServerAliveInterval=15','-o','ServerAliveCountMax=3']
target='root@173.234.15.150'
def remote(script):return subprocess.run(['ssh',*options,target,'python3 -'],input=script,text=True,check=True,timeout=900)
def copy(paths,dest=root+'/'):subprocess.run(['scp',*options,*map(str,paths),target+':'+dest],check=True,timeout=240)
if phase=='preflight':
 remote("""import fcntl,json,pathlib,subprocess
held=[]
for path in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(path,'r');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
containers={}
for name in ['adobe-relay-app-1','adobe-team-cookie-app-1','adobe-relay-postgres-1','adobe-relay-redis-1','adobe-relay-mihomo-1']:
 d=json.loads(subprocess.check_output(['docker','inspect',name]))[0]
 containers[name]={'image':d['Config']['Image'],'image_id':d['Image'],'id':d['Id'],'health':d['State'].get('Health',{}).get('Status'),'restarts':d['RestartCount'],'memory':d['HostConfig']['Memory']}
lease=subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
assert not lease, 'Another rolling release is active'
print(json.dumps({'containers':containers,'deployment_locks_available':True,'rolling_lease_empty':True,'read_only':True}))
""")
elif phase=='prepare':
 archive=pathlib.Path('release-artifact/release-images.tar.gz');digest=hashlib.sha256(archive.read_bytes()).hexdigest()
 assert pathlib.Path(str(archive)+'.sha256').read_text().split()==[digest,archive.name]
 proof=pathlib.Path('release-artifact/ci-proof.json');proof.write_text(json.dumps({'application_sha':sha,'workflow_run':int(build),'conclusion':'success','archive_sha256':digest}))
 remote(f"import pathlib; r=pathlib.Path('{root}'); assert not (r/'progress.json').exists(), 'Already started'; (r/'artifact').mkdir(parents=True,exist_ok=True,mode=0o700)")
 copy(pathlib.Path(__file__).parent.glob('*.py'))
 copy([pathlib.Path(__file__).parent/'backend-change-manifest.json', pathlib.Path('deploy/diagnostics/image26-quality-20261107/verification.json')])
 copy(pathlib.Path('release-artifact').glob('*'),root+'/artifact/')
 release_hash=hashlib.sha256(pathlib.Path('deploy/primary/release.sh').read_bytes()).hexdigest()
 remote(f"""import fcntl,hashlib,json,pathlib,subprocess,sys
orchestration=open('/opt/adobe-relay/shared/.adobe-deploy.lock','a');fcntl.flock(orchestration,fcntl.LOCK_EX|fcntl.LOCK_NB)
r=pathlib.Path('{root}')
contract=json.loads((r/'verification.json').read_text())
assert len(contract['native_candidate_results'])==4 and all(row['success'] and row['png_fully_decoded'] and row['expected_detail_level'] in (6,7) for row in contract['native_candidate_results']), 'Prior native quality tests must pass before release'
assert hashlib.sha256(pathlib.Path('/opt/adobe-relay/deploy/primary/release.sh').read_bytes()).hexdigest()=='{release_hash}'
(r/'expected-sha').write_text('{sha}');(r/'expected-run').write_text('{build}')
if not (r/'before.json').exists():subprocess.run([sys.executable,str(r/'audit.py'),'before'],check=True)
if not (r/'installed-migrations-before.json').exists():
 import importlib.util
 spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
 a.save('installed-migrations-before.json',json.loads(a.sql("SELECT json_object_agg(filename,checksum) FROM schema_migrations")))
subprocess.run([sys.executable,str(r/'load-candidate.py')],check=True)
""")
elif phase=='start':
 remote(f"""import json,pathlib,subprocess
r=pathlib.Path('{root}')
assert (r/'expected-sha').read_text().strip()=='{sha}'
assert json.loads((r/'image-loaded.json').read_text())['sha']=='{sha}'
assert not (r/'progress.json').exists(), 'Already started; inspect existing evidence'
subprocess.run(['systemd-run','--unit=adobe-image26-quality-release-20261107','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/driver.log','--property=StandardError=append:{root}/driver.log','/usr/bin/python3',str(r/'driver.py')],check=True)
print(json.dumps({{'driver_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='public-test':
 copy([pathlib.Path(__file__).parent/name for name in ['public-test.py','public-test-access.py','public_contract_guard.py']])
 remote(f"""import json,os,pathlib,subprocess
r=pathlib.Path('{root}')
assert (r/'expected-sha').read_text().strip()=='{sha}'
assert (r/'rolling-finished.json').exists() and (r/'runtime-verification.json').exists()
with os.fdopen(os.open(r/'public-test-dispatched.json',os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600),'w') as f:json.dump({{'application_sha':'{sha}','single_dispatch':True,'cases':4}},f)
subprocess.run(['systemd-run','--unit=adobe-image26-quality-public-20261107','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/public-test.log','--property=StandardError=append:{root}/public-test.log','/usr/bin/python3',str(r/'public-test-access.py')],check=True)
print(json.dumps({{'public_test_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='verify':
 copy([pathlib.Path(__file__).parent/name for name in ['final-verify.py','public_contract_guard.py']])
 remote(f"""import pathlib,subprocess,sys
r=pathlib.Path('{root}');assert (r/'expected-sha').read_text().strip()=='{sha}'
assert (r/'rolling-finished.json').exists()
for script,output in [('route-guard-review.py','route-guard-review.json'),('business-verify.py','business-verification.json'),('review-credentials.py','credential-change-review.json'),('observe.py','observation.json'),('final-verify.py','final-verification.json')]:
 if not (r/output).exists():subprocess.run([sys.executable,str(r/script)],check=True)
""")
elif phase=='archive':
 documents={'release-notes.md':pathlib.Path('deploy/primary/adobe-image26-quality-20261107.md'),'primary-README.md':pathlib.Path('deploy/primary/README.md')}
 payload={name:base64.b64encode(path.read_bytes()).decode() for name,path in documents.items()}
 remote(f"""import base64,datetime,fcntl,importlib.util,json,pathlib
r=pathlib.Path('{root}');held=[]
for path in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(path,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
container=a.inspect(a.APP);assert container['Config']['Image']=='adobe-relay:{sha}'
assert container['Image']==json.loads((r/'image-loaded.json').read_text())['image_id']
core=json.loads((r/'final-verification.json').read_text());assert core['application_sha']=='{sha}'
before=json.loads((r/'before.json').read_text());current=a.snapshot()
checks=a.check_preserved(before,current);a.validate_rollback(before,current,'{sha}',a.inspect(a.ROLLBACK_IMAGE)['Id'])
files={{}}
for name,value in {payload!r}.items():
 data=base64.b64decode(value);assert b'PENDING' not in data
 path=r/name;path.write_bytes(data);path.chmod(0o600);files[name]=a.digest(data)
 if name=='primary-README.md':pathlib.Path('/opt/adobe-relay/deploy/primary/README.md').write_bytes(data)
out={{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':'{sha}','image_id':container['Image'],'core_verified_at':core['at'],'published_document_hashes':files,'preserved':checks,'rollback_guard':{{'passed':True,'mutated':False}},'cutover_backup':json.loads((r/'after.json').read_text())['backup'],'public_contract':core['public_contract'],'mail_sent':False}}
a.save('release-completion.json',out);print(json.dumps(out))
""")
else:
 evidence=pathlib.Path('primary-release-evidence');evidence.mkdir(exist_ok=True)
 script=f"""import json,pathlib,re,subprocess
r=pathlib.Path('{root}');out={{'application_sha':'{sha}','files':{{}},'current_containers':{{}}}}
for name in ['adobe-relay-app-1','adobe-relay-memory-canary']:
 p=subprocess.run(['docker','inspect',name],text=True,capture_output=True)
 if p.returncode==0:
  d=json.loads(p.stdout)[0];out['current_containers'][name]={{'image':d['Config']['Image'],'health':d['State'].get('Health',{{}}).get('Status'),'oom':d['State']['OOMKilled'],'restarts':d['RestartCount']}}
for name in ['progress.json','image-loaded.json','rolling-route-canary.json','rolling-route-canonical.json','rolling-finished.json','runtime-verification.json','business-verification.json','credential-change-review.json','post-release-tests.json','business-key-created.json','business-access-cleanup.json','final-verification.json','release-completion.json']:
 if (r/name).exists():out['files'][name]=json.loads((r/name).read_text())
for side in ['old','canary']:
 p=r/('rolling-'+side+'-drain-progress.json')
 if p.exists():out.setdefault('recent_drain_samples',{{}})[side]=json.loads(p.read_text())['samples'][-6:]
out['public_cases']=[json.loads(p.read_text()) for p in sorted((r/'public').glob('*-result.json'))]
if (r/'after.json').exists():out['cutover_backup']=json.loads((r/'after.json').read_text())['backup']
for name in ['driver.log','public-test.log']:
 p=r/name
 if p.exists():out[name+'_errors']=[line for line in p.read_text().splitlines() if line.startswith(('AssertionError:','RuntimeError:','subprocess.CalledProcessError:'))][-8:]
if 'adobe-relay-memory-canary' in out['current_containers']:
 p=subprocess.run(['docker','logs','--since','20m','--tail','8000','adobe-relay-memory-canary'],capture_output=True,text=True,timeout=20)
 events=[]
 for line in (p.stdout+chr(10)+p.stderr).splitlines():
  try:d=json.loads(line)
  except ValueError:
   match=re.search(r'\\t(adobe_images\\.[a-z_]+)\\t(\\{{.*\\}})$',line)
   if not match:continue
   try:d=json.loads(match[2]);d['msg']=match[1];d['time']=line.split(chr(9))[0]
   except ValueError:continue
  if isinstance(d,dict) and str(d.get('msg','')).startswith('adobe_images.'):
   events.append({{k:d[k] for k in ['time','msg','processing_ms','elapsed_ms','stage','upstream_status','image_count'] if k in d}})
 out['canary_recent_events']=events[-12:]
 d=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-memory-canary']))[0]
 p=subprocess.run(['nsenter','-t',str(d['State']['Pid']),'-n','ss','-nti','state','established','sport = :8080'],capture_output=True,text=True,timeout=10)
 out['canary_socket_counters']=[{{k:int(v) for k,v in re.findall(r'(bytes_sent|bytes_received|lastsnd|lastrcv):(\\d+)',block)}} for block in re.split(r'\\n(?=\\S)',p.stdout)[1:]]
print(json.dumps(out))
"""
 p=subprocess.run(['ssh',*options,target,'python3 -'],input=script,text=True,capture_output=True,check=True,timeout=90)
 (evidence/'release-status.json').write_text(p.stdout);print(p.stdout)
