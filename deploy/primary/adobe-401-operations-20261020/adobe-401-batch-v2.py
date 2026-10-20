"""Resume only untouched accounts; existing per-account login markers prevent replay."""
import ast,datetime,importlib.util,json,os,pathlib,subprocess,sys
os.umask(0o077)
root=pathlib.Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020');ops=pathlib.Path(__file__).parent
s=importlib.util.spec_from_file_location('audit',root/'audit.py');a=importlib.util.module_from_spec(s);s.loader.exec_module(a)
ids=[int(r['id'])for r in json.loads((root/'401-preflight-private.json').read_text())['accounts']if not r.get('manual_pause')]
rows=json.loads(a.sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,COALESCE(credentials->>'email','') email,extra->'okad'->>'source_key' source_key,md5(credentials->>'cookie') source_hash FROM accounts WHERE deleted_at IS NULL AND platform='adobe' AND type='oauth' AND status='error' AND COALESCE(extra->>'okad_manual_pause','false')<>'true' AND (expires_at IS NULL OR expires_at>NOW()) AND id IN ("+','.join(map(str,ids or[0]))+"))t"))
batch=root/'401-manual-batch-v2';batch.mkdir(mode=0o700,exist_ok=True)
if(batch/'started.json').exists():print(json.dumps({'already_started':True}));sys.exit(0)
tree=ast.parse((ops/'adobe-401-pilot.py').read_text());program=None
for node in tree.body:
 if isinstance(node,ast.Assign)and any(isinstance(x,ast.Name)and x.id=='program'for x in node.targets):program=ast.literal_eval(node.value)
assert program
program+="\nprint('401_REPAIR_RESULT '+json.dumps(results),flush=True)\n";ast.parse(program)
(batch/'login-worker-private.py').write_text(program);(batch/'input-private.json').write_text(json.dumps(rows))
with(batch/'host-private.log').open('w')as log:p=subprocess.Popen([sys.executable,str(ops/'adobe-401-host-worker.py')],stdin=subprocess.DEVNULL,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'pid':p.pid,'selected':len(rows),'workers':3,'automatic_attempts_reset':False};(batch/'started.json').write_text(json.dumps(out));print(json.dumps(out),flush=True)
