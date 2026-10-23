"""Fixed-scope primary release bridge for unreliable local SSH connections."""
import hashlib,json,pathlib,re,subprocess,sys
phase,sha,build=sys.argv[1:]
assert phase in ['prepare','start','status','verify','export']
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
 remote("import pathlib; pathlib.Path('"+root+"/artifact').mkdir(parents=True,exist_ok=True,mode=0o700)")
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
for name in ['progress.json','premigration-backup.json','image-loaded.json','rolling-route-canary.json','rolling-route-canonical.json','rolling-finished.json','runtime-verification.json','business-verification.json','credential-change-review.json','final-verification.json']:
 if (r/name).exists():out['files'][name]=json.loads((r/name).read_text())
for side in ['old','canary']:
 p=r/('rolling-'+side+'-drain-progress.json')
 if p.exists():out['files'][p.name]=json.loads(p.read_text())['samples'][-1]
print(json.dumps(out,ensure_ascii=False))
"""
 p=subprocess.run(['ssh',*options,target,'python3 -'],input=script,text=True,capture_output=True,check=True,timeout=90)
 (evidence/'release-status.json').write_text(p.stdout)
 print(p.stdout)
