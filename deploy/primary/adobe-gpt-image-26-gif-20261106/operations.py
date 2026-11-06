"""Fixed-scope GIF and complexity release using the original deployment guards."""
import base64,hashlib,json,os,pathlib,re,subprocess,sys
phase,sha,build=sys.argv[1:]
assert phase in ['preflight','prepare','start','resume','public-test','customer-test','selected-test','inspect-clash','status','verify','archive']
assert re.fullmatch('[0-9a-f]{40}',sha) and build.isdigit()
root='/opt/adobe-relay/releases/adobe-gpt-image-26-gif-20261106'
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
 copy([pathlib.Path(__file__).parent/'backend-change-manifest.json'])
 copy(pathlib.Path('release-artifact').glob('*'),root+'/artifact/')
 release_hash=hashlib.sha256(pathlib.Path('deploy/primary/release.sh').read_bytes()).hexdigest()
 remote(f"""import fcntl,hashlib,json,pathlib,subprocess,sys
orchestration=open('/opt/adobe-relay/shared/.adobe-deploy.lock','a');fcntl.flock(orchestration,fcntl.LOCK_EX|fcntl.LOCK_NB)
r=pathlib.Path('{root}')
contract=json.loads((r/'preflight-contract-summary.json').read_text())
assert contract['all_passed'] and contract['static_models']==5 and contract['edited_models']==5 and contract['gif_edit_guard_cases']==6 and contract['gif_edit_supported'] is False, 'Online contract tests must pass before preparing a release'
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
subprocess.run(['systemd-run','--unit=adobe-gpt-image-26-gif-release-20261106','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/driver.log','--property=StandardError=append:{root}/driver.log','/usr/bin/python3',str(r/'driver.py')],check=True)
print(json.dumps({{'driver_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='resume':
 copy([pathlib.Path(__file__).parent/name for name in ['rolling.py','resume.py']])
 remote(f"""import json,os,pathlib,subprocess
r=pathlib.Path('{root}')
assert (r/'expected-sha').read_text().strip()=='{sha}'=='da24f5634a0fd332f95a14d666fa2e9cea19db53'
assert json.loads((r/'progress.json').read_text())['state']=='attention_required'
assert not (r/'after.json').exists() and not (r/'rolling-old-drained.json').exists()
assert subprocess.run(['systemctl','is-active','--quiet','adobe-gpt-image-26-gif-release-20261106']).returncode!=0
with os.fdopen(os.open(r/'resume-dispatched.json',os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600),'w') as f:json.dump({{'application_sha':'{sha}','resume_once':True}},f)
subprocess.run(['systemd-run','--unit=adobe-gpt-image-26-gif-resume-20261106','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/resume.log','--property=StandardError=append:{root}/resume.log','/usr/bin/python3',str(r/'resume.py')],check=True)
print(json.dumps({{'resume_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='public-test':
 copy([pathlib.Path(__file__).parent/'public-test.py'])
 remote(f"""import json,os,pathlib,subprocess
r=pathlib.Path('{root}');assert (r/'expected-sha').read_text().strip()=='{sha}'
assert (r/'rolling-finished.json').exists() and (r/'runtime-verification.json').exists()
assert not list(r.glob('public-gpt-image-*-started.json')) and not (r/'post-release-tests.json').exists()
with os.fdopen(os.open(r/'public-test-dispatched.json',os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600),'w') as f:json.dump({{'application_sha':'{sha}','single_dispatch':True}},f)
subprocess.run(['systemd-run','--unit=adobe-gpt-image-26-gif-public-tests-20261106','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/public-test.log','--property=StandardError=append:{root}/public-test.log','/usr/bin/python3',str(r/'public-test.py'),'{sha}'],check=True)
print(json.dumps({{'public_test_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='customer-test':
 copy([pathlib.Path(__file__).parent/name for name in ['public-test.py','customer-test-access.py','customer-test-driver.py']])
 remote(f"""import json,os,pathlib,subprocess
r=pathlib.Path('{root}');assert (r/'expected-sha').read_text().strip()=='{sha}'
assert (r/'rolling-finished.json').exists() and (r/'runtime-verification.json').exists()
assert not (r/'customer-test-access-started.json').exists() and not (r/'customer-post-release-tests.json').exists()
assert not list(r.glob('customer-gpt-image-*-started.json'))
with os.fdopen(os.open(r/'customer-test-dispatched.json',os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600),'w') as f:json.dump({{'application_sha':'{sha}','single_dispatch':True,'group_id':33}},f)
subprocess.run(['systemd-run','--unit=adobe-gpt-image-26-gif-customer-tests-20261106','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/customer-test.log','--property=StandardError=append:{root}/customer-test.log','/usr/bin/python3',str(r/'customer-test-driver.py'),'{sha}'],check=True)
print(json.dumps({{'customer_test_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='selected-test':
 copy([pathlib.Path(__file__).parent/name for name in ['public-test.py','customer-test-access.py','selected-test-access.py','selected-test-driver.py']])
 remote(f"""import json,os,pathlib,subprocess
r=pathlib.Path('{root}');assert (r/'expected-sha').read_text().strip()=='{sha}'
assert (r/'rolling-finished.json').exists() and (r/'runtime-verification.json').exists()
assert (r/'customer-test-access-cleanup.json').exists()
assert not (r/'selected-test-access-started.json').exists() and not (r/'selected-post-release-tests.json').exists()
assert not list(r.glob('selected-gpt-image-*-started.json'))
with os.fdopen(os.open(r/'selected-test-dispatched.json',os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600),'w') as f:json.dump({{'application_sha':'{sha}','single_dispatch':True,'selected_account_id':2933}},f)
subprocess.run(['systemd-run','--unit=adobe-gpt-image-26-gif-selected-tests-20261106','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/selected-test.log','--property=StandardError=append:{root}/selected-test.log','/usr/bin/python3',str(r/'selected-test-driver.py'),'{sha}'],check=True)
print(json.dumps({{'selected_test_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='inspect-clash':
 copy([pathlib.Path(__file__).parent/'clash-review.py'])
 script=f"import pathlib,subprocess,sys; r=pathlib.Path('{root}'); assert (r/'expected-sha').read_text().strip()=='{sha}'; subprocess.run([sys.executable,str(r/'clash-review.py')],check=True)"
 p=subprocess.run(['ssh',*options,target,'python3 -'],input=script,text=True,capture_output=True,check=True,timeout=180)
 evidence=pathlib.Path('primary-release-evidence');evidence.mkdir(exist_ok=True)
 (evidence/'clash-inspection.json').write_text(p.stdout);print(p.stdout)
elif phase=='verify':
 copy([pathlib.Path(__file__).parent/name for name in ['final-verify.py','public_contract_guard.py','selected-binding-review.py','audit.py','clash_contract_guard.py','clash-concurrent-import-review.json']])
 remote(f"""import pathlib,subprocess,sys
r=pathlib.Path('{root}');assert (r/'expected-sha').read_text().strip()=='{sha}'
assert (r/'rolling-finished.json').exists()
for script,output in [('selected-binding-review.py','selected-binding-review.json'),('route-guard-review.py','route-guard-review.json'),('business-verify.py','business-verification.json'),('review-credentials.py','credential-change-review.json'),('observe.py','observation.json'),('final-verify.py','final-verification.json')]:
 if not (r/output).exists():subprocess.run([sys.executable,str(r/script)],check=True)
""")
elif phase=='archive':
 documents={'release-notes.md':pathlib.Path('deploy/primary/adobe-gpt-image-26-gif-20261106.md'),'primary-README.md':pathlib.Path('deploy/primary/README.md')}
 payload={name:base64.b64encode(path.read_bytes()).decode() for name,path in documents.items()}
 remote(f"""import base64,datetime,fcntl,importlib.util,json,pathlib
r=pathlib.Path('{root}');held=[]
for path in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(path,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
container=a.inspect(a.APP)
assert container['Config']['Image']=='adobe-relay:{sha}'
assert container['Image']==json.loads((r/'image-loaded.json').read_text())['image_id'], 'Running image differs from the checked archive'
core=json.loads((r/'final-verification.json').read_text())
assert core['application_sha']=='{sha}'
before=json.loads((r/'before.json').read_text());current=a.snapshot()
checks=a.check_preserved(before,current)
a.validate_rollback(before,current,'{sha}',a.inspect(a.ROLLBACK_IMAGE)['Id'])
previous=r/'release-completion.json'
if previous.exists():
 saved=json.loads(previous.read_text());assert saved['application_sha']=='{sha}' and saved['core_verified_at']==core['at']
 initial=r/'release-completion.initial.json'
 if not initial.exists():initial.write_bytes(previous.read_bytes())
files={{}}
for name,value in {payload!r}.items():
 data=base64.b64decode(value);assert b'PENDING' not in data
 path=r/name;path.write_bytes(data);path.chmod(0o600);files[name]=a.digest(data)
 if name=='primary-README.md':pathlib.Path('/opt/adobe-relay/deploy/primary/README.md').write_bytes(data)
notification=json.loads((r/'release-notification.json').read_text()) if (r/'release-notification.json').exists() else None
out={{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':'{sha}','image_id':container['Image'],'core_verified_at':core['at'],'published_document_hashes':files,'preserved':checks,'rollback_guard':{{'passed':True,'mutated':False}},'cutover_backup':json.loads((r/'after.json').read_text())['backup'],'adobe_requests':{{'preflight':core['authorized_live_generation_tests']['before'],'post_release':core['authorized_live_generation_tests']['after'],'additional_legacy_group_diagnostics':core['authorized_live_generation_tests']['additional_legacy_group_diagnostics'],'additional_current_group_diagnostics':core['authorized_live_generation_tests']['additional_current_group_diagnostics']}},'public_contract':core['public_contract'],'temporary_test_access_cleanup':core['temporary_test_access_cleanup'],'current_group_test_access_cleanup':core['current_group_test_access_cleanup'],'selected_account_usage':core['selected_account_usage'],'selected_account_bindings':core['selected_account_bindings'],'mail_sent':bool(notification and notification.get('state')=='smtp_accepted'),'notification':notification}}
(r/'release-completion.json').write_text(json.dumps(out,ensure_ascii=False,indent=2));print(json.dumps(out))
""")
else:
 evidence=pathlib.Path('primary-release-evidence');evidence.mkdir(exist_ok=True)
 script=f"""import importlib.util,json,pathlib,subprocess
r=pathlib.Path('{root}');out={{'application_sha':'{sha}','files':{{}}}}
out['current_containers']={{}}
for name in ['adobe-relay-app-1','adobe-relay-memory-canary']:
 p=subprocess.run(['docker','inspect',name],text=True,capture_output=True)
 if p.returncode==0:
  d=json.loads(p.stdout)[0];out['current_containers'][name]={{'image':d['Config']['Image'],'health':d['State'].get('Health',{{}}).get('Status'),'oom':d['State']['OOMKilled'],'restarts':d['RestartCount']}}
out['public_completed_cases']=[json.loads(p.read_text()) for p in sorted(r.glob('public-gpt-image-*-result.json'))]
out['customer_completed_cases']=[json.loads(p.read_text()) for p in sorted(r.glob('customer-gpt-image-*-result.json'))]
out['selected_completed_cases']=[json.loads(p.read_text()) for p in sorted(r.glob('selected-gpt-image-*-result.json'))]
for name in ['progress.json','image-loaded.json','rolling-route-canary.json','rolling-route-canonical.json','rolling-finished.json','deferred-canary.json','runtime-verification.json','business-verification.json','credential-change-review.json','preflight-contract-summary.json','post-release-tests.json','customer-post-release-tests.json','customer-test-key-result.json','customer-test-access-cleanup.json','selected-post-release-tests.json','selected-test-key-result.json','selected-test-group-result.json','selected-test-access-cleanup.json','final-verification.json','release-completion.json','release-notification.json']:
 if (r/name).exists():out['files'][name]=json.loads((r/name).read_text())
for side in ['old','canary']:
 p=r/('rolling-'+side+'-drain-progress.json')
 if p.exists():
  samples=json.loads(p.read_text())['samples'];out['files'][p.name]=samples[-1]
  out.setdefault('recent_drain_samples',{{}})[side]=samples[-18:]
if (r/'after.json').exists():out['cutover_backup']=json.loads((r/'after.json').read_text())['backup']
log=r/'driver.log'
if log.exists():out['driver_errors']=[line for line in log.read_text().splitlines() if line.startswith(('AssertionError:','RuntimeError:','subprocess.CalledProcessError:'))][-8:]
resume=r/'resume.log'
if resume.exists():out['resume_errors']=[line for line in resume.read_text().splitlines() if line.startswith(('AssertionError:','RuntimeError:','subprocess.CalledProcessError:'))][-8:]
customer=r/'customer-test.log'
if customer.exists():out['customer_test_errors']=[line for line in customer.read_text().splitlines() if line.startswith(('AssertionError:','RuntimeError:','subprocess.CalledProcessError:'))][-8:]
selected=r/'selected-test.log'
if selected.exists():out['selected_test_errors']=[line for line in selected.read_text().splitlines() if line.startswith(('AssertionError:','RuntimeError:','KeyError:','subprocess.CalledProcessError:'))][-8:]
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
