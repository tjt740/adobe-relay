"""Fixed-scope primary release operations using the original deployment guards."""
import base64,hashlib,json,os,pathlib,re,subprocess,sys
phase,sha,build=sys.argv[1:]
assert phase in ['prepare','start','status','verify','archive']
assert re.fullmatch('[0-9a-f]{40}',sha) and build.isdigit()
root='/opt/adobe-relay/releases/cookie-recovery-revert-20261026'
options=['-i',os.environ.get('PRIMARY_SSH_IDENTITY',str(pathlib.Path.home()/'.ssh/deploy_key')),'-o','BatchMode=yes','-o','StrictHostKeyChecking=yes','-o','ConnectTimeout=20','-o','ServerAliveInterval=15','-o','ServerAliveCountMax=3']
target='root@173.234.15.150'
def remote(script):return subprocess.run(['ssh',*options,target,'python3 -'],input=script,text=True,check=True,timeout=900)
def copy(paths,dest=root+'/'):subprocess.run(['scp',*options,*map(str,paths),target+':'+dest],check=True,timeout=240)
if phase=='prepare':
 archive=pathlib.Path('release-artifact/release-images.tar.gz');digest=hashlib.sha256(archive.read_bytes()).hexdigest()
 assert pathlib.Path(str(archive)+'.sha256').read_text().split()==[digest,archive.name]
 proof=pathlib.Path('release-artifact/ci-proof.json');proof.write_text(json.dumps({'application_sha':sha,'workflow_run':int(build),'conclusion':'success','archive_sha256':digest}))
 remote(f"import pathlib; r=pathlib.Path('{root}'); assert not (r/'progress.json').exists(), 'Already started'; (r/'artifact').mkdir(parents=True,exist_ok=True,mode=0o700)")
 copy(pathlib.Path(__file__).parent.glob('*.py'))
 copy(pathlib.Path('release-artifact').glob('*'),root+'/artifact/')
 release_hash=hashlib.sha256(pathlib.Path('deploy/primary/release.sh').read_bytes()).hexdigest()
 remote(f"""import fcntl,hashlib,json,pathlib,subprocess,sys
orchestration=open('/opt/adobe-relay/shared/.adobe-deploy.lock','a');fcntl.flock(orchestration,fcntl.LOCK_EX|fcntl.LOCK_NB)
r=pathlib.Path('{root}')
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
subprocess.run(['systemd-run','--unit=cookie-recovery-release-20261026','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/driver.log','--property=StandardError=append:{root}/driver.log','/usr/bin/python3',str(r/'driver.py')],check=True)
print(json.dumps({{'driver_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='verify':
 remote(f"""import pathlib,subprocess,sys
r=pathlib.Path('{root}');assert (r/'expected-sha').read_text().strip()=='{sha}'
assert (r/'rolling-finished.json').exists()
for script,output in [('route-guard-review.py','route-guard-review.json'),('business-verify.py','business-verification.json'),('review-credentials.py','credential-change-review.json'),('observe.py','observation.json'),('final-verify.py','final-verification.json')]:
 if not (r/output).exists():subprocess.run([sys.executable,str(r/script)],check=True)
""")
elif phase=='archive':
 documents={'release-notes.md':pathlib.Path('deploy/primary/cookie-recovery-revert-20261026.md'),'primary-README.md':pathlib.Path('deploy/primary/README.md')}
 payload={name:base64.b64encode(path.read_bytes()).decode() for name,path in documents.items()}
 remote(f"""import base64,datetime,fcntl,importlib.util,json,pathlib
r=pathlib.Path('{root}');held=[]
for path in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(path,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:{sha}'
core=json.loads((r/'final-verification.json').read_text())
assert core['application_sha']=='{sha}'
checks=a.check_preserved(json.loads((r/'before.json').read_text()),a.snapshot())
assert not (r/'release-completion.json').exists(),'Already archived; inspect evidence'
files={{}}
for name,value in {payload!r}.items():
 data=base64.b64decode(value);assert b'PENDING' not in data
 path=r/name;path.write_bytes(data);path.chmod(0o600);files[name]=a.digest(data)
 if name=='primary-README.md':pathlib.Path('/opt/adobe-relay/deploy/primary/README.md').write_bytes(data)
out={{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':'{sha}','core_verified_at':core['at'],'published_document_hashes':files,'preserved':checks,'cutover_backup':json.loads((r/'after.json').read_text())['backup'],'adobe_requests':False,'mail_sent':False}}
a.save('release-completion.json',out);print(json.dumps(out))
""")
else:
 evidence=pathlib.Path('primary-release-evidence');evidence.mkdir(exist_ok=True)
 script=f"""import importlib.util,json,pathlib
r=pathlib.Path('{root}');out={{'application_sha':'{sha}','files':{{}}}}
for name in ['progress.json','image-loaded.json','rolling-route-canary.json','rolling-route-canonical.json','rolling-finished.json','runtime-verification.json','business-verification.json','credential-change-review.json','final-verification.json','release-completion.json']:
 if (r/name).exists():out['files'][name]=json.loads((r/name).read_text())
for side in ['old','canary']:
 p=r/('rolling-'+side+'-drain-progress.json')
 if p.exists():out['files'][p.name]=json.loads(p.read_text())['samples'][-1]
if (r/'after.json').exists():out['cutover_backup']=json.loads((r/'after.json').read_text())['backup']
log=r/'driver.log'
if log.exists():out['driver_errors']=[line for line in log.read_text().splitlines() if line.startswith(('AssertionError:','RuntimeError:','subprocess.CalledProcessError:'))][-8:]
if (r/'progress.json').exists() and json.loads((r/'progress.json').read_text()).get('state')=='attention_required':
 spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
 before=json.loads((r/'before.json').read_text());current=a.snapshot()
 old=before['settings_hashes'];new=current['settings_hashes']
 out['changed_setting_hashes']={{k:{{'before':old.get(k),'current':new.get(k)}} for k in old.keys()|new.keys() if old.get(k)!=new.get(k)}}
 out['other_protected_changes']=[k for k in ['config_hashes','env_without_image_hash','okad_settings_hash','clash_contract_hash','rules_hash'] if before[k]!=current[k]]
print(json.dumps(out,ensure_ascii=False))
"""
 copy([pathlib.Path(__file__).parent/'route-guard-review.py'])
 remote(f"import pathlib,subprocess,sys; r=pathlib.Path('{root}'); subprocess.run([sys.executable,str(r/'route-guard-review.py')],check=True) if (r/'after.json').exists() else None")
 p=subprocess.run(['ssh',*options,target,'python3 -'],input=script,text=True,capture_output=True,check=True,timeout=90)
 (evidence/'release-status.json').write_text(p.stdout);print(p.stdout)
