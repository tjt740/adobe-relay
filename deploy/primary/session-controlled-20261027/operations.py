"""Transport only: bounded session experiment, no application deployment."""
import pathlib,subprocess,sys,json,hashlib,os
PHASE=sys.argv[1];assert PHASE in ('session-start','session-status','session-export','session-restore')
HOST='root@173.234.15.150';ROOT='/opt/adobe-relay/releases/session-controlled-20261027'
SSH=['ssh','-i',os.path.expanduser('~/.ssh/deploy_key'),'-o','BatchMode=yes','-o','StrictHostKeyChecking=yes','-o','ConnectTimeout=20','-o','ServerAliveInterval=10','-o','ServerAliveCountMax=3',HOST,'python3 -']
def remote(code):
 r=subprocess.run(SSH,input=code,text=True,capture_output=True,timeout=90)
 if r.returncode:raise RuntimeError(r.stderr[-1200:])
 return r.stdout
if PHASE=='session-start':
 binary=pathlib.Path('/tmp/session-probe');digest=hashlib.sha256(binary.read_bytes()).hexdigest()
 print(remote("import pathlib; p=pathlib.Path("+repr(ROOT)+");p.mkdir(mode=0o700,exist_ok=True);assert not (p/'started.json').exists();print('preflight: not started')"))
 for src,dest in [(binary,'probe'),(pathlib.Path('deploy/primary/session-controlled-20261027/controller.py'),'controller.py')]:
  subprocess.run(['scp','-i',os.path.expanduser('~/.ssh/deploy_key'),'-o','BatchMode=yes','-o','StrictHostKeyChecking=yes','-o','ConnectTimeout=20',str(src),HOST+':'+ROOT+'/'+dest],check=True,timeout=180)
 print(remote("import pathlib,hashlib,os,subprocess,json; p=pathlib.Path("+repr(ROOT)+");assert not (p/'started.json').exists();assert hashlib.sha256((p/'probe').read_bytes()).hexdigest()=="+repr(digest)+";os.chmod(p/'probe',0o700);os.chmod(p/'controller.py',0o600);log=open(p/'execution.log','a');proc=subprocess.Popen(['python3',str(p/'controller.py')],stdout=log,stderr=log,start_new_session=True);print(json.dumps({'pid':proc.pid,'binary_sha256':"+repr(digest)+",'ids':[1742,1744,1749,1758]}))"))
elif PHASE=='session-restore':
 print(remote("import pathlib,subprocess; p=pathlib.Path("+repr(ROOT)+");assert not any(x.startswith('adobe-session-controlled-r') for x in subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines());subprocess.run(['python3',str(p/'controller.py'),'restore'],check=True);print((p/'restored.json').read_text())"))
else:
 code="""import pathlib,json,collections
p=pathlib.Path(ROOT)
for name in ['started.json','completed.json','failed.json','restored.json']:
 if (p/name).exists():print(json.dumps({'file':name,'data':json.loads((p/name).read_text())}))
if (p/'controller.jsonl').exists():print(json.dumps({'controller':(p/'controller.jsonl').read_text().splitlines()[-12:]}))
for n in [1,2]:
 f=p/f'round-{n}.jsonl'
 if not f.exists():continue
 rows=[json.loads(l) for l in f.read_text().splitlines() if l.strip()]
 for i in [1742,1744,1749,1758]:
  a=[r for r in rows if r.get('account_id')==i];jobs=[r for r in a if r.get('event')=='job_finish'];validations=[r for r in a if r.get('event')=='validate'];progress=[r for r in a if r.get('event') in ('progress','account_done')]
  print(json.dumps({'round':n,'account_id':i,'jobs':len(jobs),'images':sum(bool(j.get('success')) for j in jobs),'submit_attempts':sum(r.get('event')=='http_finish' and r.get('stage')=='submit' for r in a),'upstream_codes':dict(collections.Counter(r.get('status') for r in a if r.get('event')=='http_finish' and r.get('stage')=='submit')),'latest_validation':validations[-1] if validations else None,'progress':progress[-1] if progress else None}))
if (p/'execution.log').exists():print(json.dumps({'execution_log':(p/'execution.log').read_text()[-2000:]}))
""".replace('ROOT',repr(ROOT))
 output=remote(code);print(output);pathlib.Path('session-evidence').mkdir(exist_ok=True);pathlib.Path('session-evidence/status.jsonl').write_text(output)
 if PHASE=='session-export':
  audit=remote(pathlib.Path('deploy/primary/session-controlled-20261027/read_audit.py').read_text())
  pathlib.Path('session-evidence/read-only-audit.json').write_text(audit)
  # Only sanitized result files; tokens live exclusively in process memory/stdin.
  for pattern in ['*.json','*.jsonl','*.log']:
   subprocess.run(['scp','-i',os.path.expanduser('~/.ssh/deploy_key'),'-o','BatchMode=yes','-o','StrictHostKeyChecking=yes',HOST+':'+ROOT+'/'+pattern,'session-evidence/'],check=True,timeout=180)
