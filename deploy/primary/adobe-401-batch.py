"""Launch one durable, explicitly authorized manual batch. No attempt-budget reset."""
import ast,datetime,importlib.util,json,os,pathlib,subprocess,sys
os.umask(0o077)
root=pathlib.Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020')
s=importlib.util.spec_from_file_location('audit',root/'audit.py');a=importlib.util.module_from_spec(s);s.loader.exec_module(a)
# The original read-only snapshot fixes this repair's scope; later unrelated accounts are excluded.
ids=[int(r['id'])for r in json.loads((root/'401-preflight-private.json').read_text())['accounts']if not r.get('manual_pause')]
q="SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,COALESCE(credentials->>'email','') email,extra->'okad'->>'source_key' source_key,md5(credentials->>'cookie') source_hash FROM accounts WHERE deleted_at IS NULL AND platform='adobe' AND type='oauth' AND status='error' AND COALESCE(extra->>'okad_manual_pause','false')<>'true' AND (expires_at IS NULL OR expires_at>NOW()) AND id IN ("+','.join(map(str,ids or[0]))+"))t"
rows=json.loads(a.sql(q));batch=root/'401-manual-batch';batch.mkdir(mode=0o700,exist_ok=True)
if (batch/'started.json').exists():print(json.dumps({'batch_already_started':True,'no_duplicate_launch':True}));sys.exit(0)
worker=(pathlib.Path(__file__).with_name('adobe-401-manual-worker.py')).read_text();ast.parse(worker)
(batch/'input-private.json').write_text(json.dumps(rows));(batch/'worker-private.py').write_text(worker)
runner="""import json,pathlib,subprocess,time,os
os.umask(0o077)
r=pathlib.Path(ROOT)
with (r/'stderr-private.log').open('w')as err:
 p=subprocess.Popen(['docker','exec','-i','adobe-team-cookie-app-1','python','-u','-c',(r/'worker-private.py').read_text()],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=err,text=True)
 p.stdin.write((r/'input-private.json').read_text()+'\\n');p.stdin.close()
 with (r/'stdout-private.log').open('w')as log:
  for line in p.stdout:
   log.write(line);log.flush()
   if line.startswith('401_REPAIR_PROGRESS '):(r/'progress.json').write_text(line.split(' ',1)[1])
   if line.startswith('401_REPAIR_FINISHED '):(r/'finished.json').write_text(line.split(' ',1)[1])
 code=p.wait();(r/'process-exit.json').write_text(json.dumps({'exit':code,'at':time.time()}))
""".replace('ROOT',repr(str(batch)))
(batch/'runner.py').write_text(runner)
with (batch/'launcher-private.log').open('w')as log:p=subprocess.Popen([sys.executable,str(batch/'runner.py')],stdin=subprocess.DEVNULL,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'pid':p.pid,'selected':len(rows),'original_unpaused_scope':len(ids),'workers':3,'automatic_attempts_reset':False}
(batch/'started.json').write_text(json.dumps(out));print(json.dumps(out),flush=True)
