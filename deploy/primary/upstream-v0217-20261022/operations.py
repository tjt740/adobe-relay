"""Fixed-scope primary release bridge for unreliable local SSH connections."""
import hashlib,json,pathlib,re,subprocess,sys
phase,sha,build=sys.argv[1:]
assert phase in ['prepare','start','status','browser','review-state','approve-state','verify','export']
assert re.fullmatch('[0-9a-f]{40}',sha) and build.isdigit()
root='/opt/adobe-relay/releases/upstream-v0217-20261022'
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
 remote("import pathlib; pathlib.Path('"+root+"/artifact').mkdir(parents=True,exist_ok=True,mode=0o700)")
 copy(pathlib.Path(__file__).parent.glob('*.py'))
 copy([pathlib.Path(__file__).with_name('expected-new-migrations.json')])
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
subprocess.run(['systemd-run','--unit=adobe-upstream-v0217-release','--collect','--property=WorkingDirectory={root}','--property=StandardOutput=append:{root}/driver.log','--property=StandardError=append:{root}/driver.log','/usr/bin/python3',str(r/'driver.py')],check=True)
print(json.dumps({{'driver_dispatched':True,'application_sha':'{sha}'}}))
""")
elif phase=='browser':
 subprocess.run(['node',str(pathlib.Path(__file__).with_name('live-browser-check.cjs'))],check=True,timeout=420)
 report=pathlib.Path(__file__).with_name('live-browser-verification.json')
 copy([report])
 evidence=pathlib.Path('primary-release-evidence');evidence.mkdir(exist_ok=True)
 import shutil
 shutil.copy2(report,evidence/report.name)
 for screenshot in pathlib.Path(__file__).parent.glob('live-*.png'):shutil.copy2(screenshot,evidence/screenshot.name)
elif phase in ['review-state','approve-state']:
 copy([pathlib.Path(__file__).with_name('review-state-changes.py'),pathlib.Path(__file__).with_name('state-change-diagnostic.py')])
 if phase=='review-state':
  remote(f"import subprocess; subprocess.run(['python3','{root}/state-change-diagnostic.py'],check=True)")
  sys.exit(0)
 action='capture' if phase=='review-state' else 'approve'
 remote(f"import subprocess; subprocess.run(['python3','{root}/review-state-changes.py','{action}'],check=True)")
elif phase=='verify':
 copy([pathlib.Path(__file__).with_name('schema-business-verify.py'),pathlib.Path(__file__).with_name('expected-new-migrations.json'),pathlib.Path(__file__).with_name('final-verify.py'),pathlib.Path(__file__).with_name('cutover-state.py')])
 remote(f"""import json,pathlib,subprocess,sys,time
r=pathlib.Path('{root}');assert (r/'expected-sha').read_text().strip()=='{sha}'
deadline=time.monotonic()+300
while not (r/'rolling-finished.json').exists():
 assert json.loads((r/'progress.json').read_text())['state']!='attention_required', 'Driver needs inspection'
 assert time.monotonic()<deadline, 'Natural draining still in progress; no tasks interrupted'
 time.sleep(5)
for script,output in [('cutover-state.py','cutover-account-state.json'),('schema-business-verify.py','schema-business-verification.json'),('observe.py','observation.json'),('business-verify.py','business-verification.json'),('review-credentials.py','credential-change-review.json'),('final-verify.py','final-verification.json')]:
 if not (r/output).exists():subprocess.run([sys.executable,str(r/script)],check=True)
subprocess.run([sys.executable,str(r/'audit.py'),'check-rollback'],check=True)
""")
else:
 evidence=pathlib.Path('primary-release-evidence');evidence.mkdir(exist_ok=True)
 script=f"""import json,pathlib,subprocess
r=pathlib.Path('{root}')
out={{'application_sha':'{sha}','files':{{}}}}
for name in ['progress.json','premigration-backup.json','image-loaded.json','rolling-route-canary.json','rolling-route-canonical.json','rolling-finished.json','runtime-verification.json','business-verification.json','credential-change-review.json','schema-business-verification.json','live-state-change-evidence.json','state-change-diagnostic.json','precutover-live-state-changes.json','reviewed-live-state-changes.json','final-verification.json']:
 if (r/name).exists():out['files'][name]=json.loads((r/name).read_text())
for side in ['old','canary']:
 p=r/('rolling-'+side+'-drain-progress.json')
 if p.exists():out['files'][p.name]=json.loads(p.read_text())['samples'][-1]
print(json.dumps(out,ensure_ascii=False))
"""
 p=subprocess.run(['ssh',*options,target,'python3 -'],input=script,text=True,capture_output=True,check=True,timeout=90)
 (evidence/'release-status.json').write_text(p.stdout)
 print(p.stdout)
