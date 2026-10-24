"""Reapply the exact idempotent data migration after every legacy writer exits."""
import datetime,fcntl,hashlib,importlib.util,json,subprocess
from pathlib import Path
ROOT=Path('/opt/adobe-relay/releases/adobe-408-timeout-scheduling-20261024')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
held=[]
for path in ['/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(path,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
sha=(ROOT/'expected-sha').read_text().strip();before=json.loads((ROOT/'before.json').read_text())
assert a.inspect(a.APP)['Id']!=before['containers'][a.APP]['id']
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
assert a.inspect('adobe-relay-memory-canary')['Config']['Image']=='adobe-relay:'+sha
assert (ROOT/'rolling-old-drained.json').exists()
for raw in subprocess.check_output(['docker','ps','--format','{{json .}}'],text=True).splitlines():
 row=json.loads(raw)
 assert row['Image']!=a.OLD_IMAGE,'Legacy writer still running; no final cleanup'
name='254_adobe_408_timeout_scheduling.sql';sql=(ROOT/name).read_text();digest=hashlib.sha256(sql.strip().encode()).hexdigest()
assert json.loads((ROOT/'migration-manifest.json').read_text())=={name:digest}
installed=json.loads(a.sql('SELECT json_object_agg(filename,checksum) FROM schema_migrations'))
expected=json.loads((ROOT/'installed-migrations-before.json').read_text());assert name not in expected
assert installed==dict(expected,**{name:digest}),'Unexpected migration or old checksum change'
cmd=['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1']
query="BEGIN; SET LOCAL lock_timeout='15s'; SET LOCAL statement_timeout='60s'; "+sql+"; SELECT json_build_object('holds',(SELECT count(*) FROM adobe_proxy_408_holds),'blocks',(SELECT count(*) FROM adobe_proxy_408_blocks)); COMMIT;"
p=subprocess.run(cmd,input=query,text=True,capture_output=True,check=True,timeout=90)
result=json.loads(p.stdout.strip());assert result=={'holds':0,'blocks':0}
a.save('migration-final-recheck.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'legacy_writers_stopped':True,'migration':name,'checksum':digest,'old_checksums_preserved':True,**result})
print(json.dumps({'migration_recheck_passed':True,'legacy_writers_stopped':True}))
