"""Fixed-scope primary release bridge for unreliable local SSH connections."""
import base64,hashlib,inspect,json,pathlib,re,subprocess,sys
from candidate_history import archive_candidate
phase,sha,build=sys.argv[1:]
assert phase in ['prepare','start','status','verify','resume-preflight','scoped-test','archive','export']
assert re.fullmatch('[0-9a-f]{40}',sha) and build.isdigit()
root='/opt/adobe-relay/releases/adobe-route-health-20261023'
options=['-i',str(pathlib.Path.home()/'.ssh/deploy_key'),'-o','BatchMode=yes','-o','StrictHostKeyChecking=yes','-o','ConnectTimeout=20','-o','ServerAliveInterval=15','-o','ServerAliveCountMax=3']
target='root@173.234.15.150'
def remote(script):
 return subprocess.run(['ssh',*options,target,'python3 -'],input=script,text=True,check=True,timeout=900)
def copy(paths,dest=root+'/'):
 subprocess.run(['scp',*options,*map(str,paths),target+':'+dest],check=True,timeout=240)
if phase=='prepare':
 archive=pathlib.Path('release-artifact/release-images.tar.gz')
 digest=hashlib.sha256(archive.read_bytes()).hexdigest()
 assert pathlib.Path(str(archive)+'.sha256').read_text().split()==[digest,archive.name]
 proof=pathlib.Path('release-artifact/ci-proof.json');proof.write_text(json.dumps({'application_sha':sha,'workflow_run':int(build),'conclusion':'success','archive_sha256':digest}))
 remote(inspect.getsource(archive_candidate)+'\narchive_candidate('+repr(root)+','+repr(sha)+')\n')
 copy(pathlib.Path(__file__).parent.glob('*.py'))
 copy([pathlib.Path(__file__).parent/'expected-new-migrations.json'])
 copy(pathlib.Path('release-artifact').glob('*'),root+'/artifact/')
 release_hash=hashlib.sha256(pathlib.Path('deploy/primary/release.sh').read_bytes()).hexdigest()
 remote(f"""import hashlib,json,pathlib,subprocess,sys
r=pathlib.Path('{root}')
assert hashlib.sha256(pathlib.Path('/opt/adobe-relay/deploy/primary/release.sh').read_bytes()).hexdigest()=='{release_hash}', 'Canonical release guard changed; inspect before use'
(r/'expected-sha').write_text('{sha}');(r/'expected-run').write_text('{build}')
if not (r/'before.json').exists():subprocess.run([sys.executable,str(r/'audit.py'),'before'],check=True)
if not (r/'premigration-backup.json').exists():subprocess.run([sys.executable,str(r/'pre-schema.py')],check=True)
assert json.loads((r/'premigration-backup.json').read_text())['dump_verified']
subprocess.run([sys.executable,str(r/'load-candidate.py')],check=True)
""")
elif phase=='start':
 remote(f"""import json,pathlib,subprocess
r=pathlib.Path('{root}')
assert (r/'expected-sha').read_text().strip()=='{sha}'
assert json.loads((r/'image-loaded.json').read_text())['sha']=='{sha}'
assert not (r/'progress.json').exists(), 'Driver already started; inspect its evidence'
subprocess.run(['systemd-run','--unit=adobe-route-health-release-20261023','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/driver.log','--property=StandardError=append:{root}/driver.log','/usr/bin/python3',str(r/'driver.py')],check=True)
print(json.dumps({{'driver_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='resume-preflight':
 copy([pathlib.Path(__file__).parent/name for name in ['audit.py','resume-preflight.py']])
 remote(f"""import pathlib,subprocess,sys
r=pathlib.Path('{root}');assert (r/'expected-sha').read_text().strip()=='{sha}'
subprocess.run([sys.executable,str(r/'resume-preflight.py')],check=True)
""")
elif phase=='archive':
 documents={'release-notes.md':pathlib.Path('deploy/primary/adobe-route-health-20261023.md'),'primary-README.md':pathlib.Path('deploy/primary/README.md')}
 payload={name:base64.b64encode(path.read_bytes()).decode() for name,path in documents.items()}
 remote(f"""import base64,datetime,fcntl,importlib.util,json,pathlib,subprocess
r=pathlib.Path('{root}');held=[]
for path in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(path,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:{sha}'
core=json.loads((r/'final-verification.json').read_text());test=json.loads((r/'scoped-live-test.json').read_text())
assert core['application_sha']==test['application_sha']=='{sha}'
assert test['manual_pause_preserved'] and test['cookie_unchanged'] and not test['retried_outer_post']
checks=a.check_preserved(json.loads((r/'before.json').read_text()),a.snapshot())
assert not (r/'release-completion.json').exists(), 'Archive already completed; inspect its evidence'
files={{}}
for name,value in {payload!r}.items():
 data=base64.b64decode(value);assert b'PENDING' not in data
 path=r/name;path.write_bytes(data);path.chmod(0o600);files[name]=a.digest(data)
 if name=='primary-README.md':pathlib.Path('/opt/adobe-relay/deploy/primary/README.md').write_bytes(data)
a.save('release-completion.json',{{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':'{sha}','core_verified_at':core['at'],'scoped_test_at':test['at'],'scoped_test_success':test['success'],'scoped_test_actual_adobe_request':test['actual_adobe_test'],'active_outer_test_requests':1,'mail_sent':False,'preserved':checks,'published_document_hashes':files,'cutover_backup':json.loads((r/'after.json').read_text())['backup']}})
print((r/'release-completion.json').read_text())
""")
elif phase=='scoped-test':
 remote(f"""import hashlib,json,pathlib,subprocess
r=pathlib.Path('{root}')
assert (r/'expected-sha').read_text().strip()=='{sha}'
assert json.loads((r/'final-verification.json').read_text())['application_sha']=='{sha}'
assert hashlib.sha256((r/'scoped-live-test.py').read_bytes()).hexdigest()=='f5cf0228788aff3907eb84d4ca510076e5b7a2d09cc17d57a8ac34148a850fb5'
assert not (r/'scoped-live-test-started.json').exists(), 'Already started; inspect the result without replaying'
subprocess.run(['systemd-run','--unit=adobe-route-scoped-test-20261023','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/scoped-live-test.log','--property=StandardError=append:{root}/scoped-live-test.log','/usr/bin/python3',str(r/'scoped-live-test.py')],check=True)
print(json.dumps({{'single_test_dispatched':True,'application_sha':'{sha}','account_id':1172,'model':'gpt-image-2'}}))
""")
elif phase=='verify':
 remote(f"""import json,pathlib,subprocess,sys
r=pathlib.Path('{root}');assert (r/'expected-sha').read_text().strip()=='{sha}'
assert (r/'rolling-finished.json').exists()
for script,output in [('observe.py','observation.json'),('capacity.py','capacity-release.json'),('business-verify.py','business-verification.json'),('review-credentials.py','credential-change-review.json'),('final-verify.py','final-verification.json')]:
 if not (r/output).exists():subprocess.run([sys.executable,str(r/script)]+(['after'] if script=='capacity.py' else []),check=True)
subprocess.run([sys.executable,str(r/'audit.py'),'check-rollback'],check=True)
""")
else:
 evidence=pathlib.Path('primary-release-evidence');evidence.mkdir(exist_ok=True)
 script=f"""import json,pathlib,subprocess
r=pathlib.Path('{root}')
out={{'application_sha':'{sha}','files':{{}}}}
for name in ['progress.json','premigration-backup.json','image-loaded.json','rolling-route-canary.json','rolling-route-canonical.json','rolling-finished.json','runtime-verification.json','business-verification.json','credential-change-review.json','final-verification.json','scoped-live-test-started.json','scoped-live-test-response.json','scoped-live-test.json','release-completion.json']:
 if (r/name).exists():out['files'][name]=json.loads((r/name).read_text())
for side in ['old','canary']:
 p=r/('rolling-'+side+'-drain-progress.json')
 if p.exists():out['files'][p.name]=json.loads(p.read_text())['samples'][-1]
test_log=r/'scoped-live-test.log'
if test_log.exists():out['scoped_test_errors']=[line for line in test_log.read_text().splitlines() if line.startswith(('AssertionError','RuntimeError','KeyError','TypeError','ValueError','urllib.error.')) or line.startswith('    assert')][-8:]
if (r/'scoped-live-test.log').exists() and not (r/'scoped-live-test-started.json').exists():
 import importlib.util
 spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
 out['scoped_test_preconditions']=json.loads(a.sql("SELECT json_build_object('id',id,'present',deleted_at IS NULL,'status',status,'schedulable',schedulable,'manual_pause',extra->'okad_manual_pause','proxy_id',proxy_id) FROM accounts WHERE id=1172"))
log=r/'driver.log'
if log.exists():out['driver_errors']=[line for line in log.read_text().splitlines() if line.startswith(('AssertionError:','RuntimeError:','subprocess.CalledProcessError:'))][-8:]
if (r/'progress.json').exists() and json.loads((r/'progress.json').read_text()).get('state')=='attention_required':
 import importlib.util
 spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
 before=json.loads((r/'before.json').read_text());current=a.snapshot()
 out['preservation_diff']={{k:{{'before':before[k],'current':current[k]}} for k in ['settings_hashes','config_hashes','env_without_image_hash','okad_settings_hash','clash_contract_hash','rules_hash'] if before[k]!=current[k]}}
 out['container_state']={{k:{{'id':v['id'],'image':v['image'],'health':v['health'],'restarts':v['restarts']}} for k,v in current['containers'].items()}}
print(json.dumps(out,ensure_ascii=False))
"""
 p=subprocess.run(['ssh',*options,target,'python3 -'],input=script,text=True,capture_output=True,check=True,timeout=90)
 (evidence/'release-status.json').write_text(p.stdout)
 print(p.stdout)
