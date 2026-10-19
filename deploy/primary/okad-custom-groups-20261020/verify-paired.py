"""Final paired-release checks; configuration values and credentials are only compared by digest."""
import datetime,hashlib,importlib.util,json,re,sqlite3,subprocess
from pathlib import Path
root=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('audit',root/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
before=json.loads((root/'before.json').read_text());current=a.snapshot()
okad=json.loads((root/'okad'/'after.json').read_text());manifest=json.loads((root/'okad'/'manifest.json').read_text())
sha=(root/'expected-sha').read_text().strip()
assert current['containers'][a.APP]['image']=='adobe-relay:'+sha
assert current['containers']['adobe-team-cookie-app-1']['image']=='adobe-team-cookie:'+manifest['commit']
apps={a.APP,'adobe-team-cookie-app-1'}
checks={
 'other_services_retained':all(current['containers'][name]['id']==item['id'] and current['containers'][name]['image_id']==item['image_id'] for name,item in before['containers'].items() if name not in apps),
 'all_resources_retained':all(current['containers'][name]['resources']==item['resources'] and current['containers'][name]['go_env']==item['go_env'] for name,item in before['containers'].items()),
 'sub2_environment_retained':current['env_without_image_hash']==before['env_without_image_hash'],
 'sub2_settings_and_smtp_retained':current['settings_hashes']==before['settings_hashes'],
 'alert_rules_retained':current['rules_hash']==before['rules_hash'],
 'okad_settings_retained':current['okad_settings_hash']==before['okad_settings_hash'],
 'apps_healthy':all(current['containers'][name]['health']=='healthy' and current['containers'][name]['restarts']==0 for name in apps),
}
allowed='/opt/adobe-team-cookie/compose.yml'
checks['configuration_retained_except_image']=all(current['config_hashes'][name]==digest for name,digest in before['config_hashes'].items() if name!=allowed)
old=(root/'okad'/'backup'/'compose.yml').read_text();now=Path(allowed).read_text()
expected,count=re.subn(r'(?m)^(    image:)\s*adobe-team-cookie:[^\s]+[ \t]*$',lambda m:m[1]+' adobe-team-cookie:'+manifest['commit'],old)
checks['okad_compose_only_image_changed']=count==1 and now==expected
sp=importlib.util.spec_from_file_location('okad_release',root/'okad'/'okad-release.py');om=importlib.util.module_from_spec(sp);sp.loader.exec_module(om)
checks['original_okad_records_exact']=om.database_summary()==okad['database']
state=json.loads((root/'group-test-fixture.json').read_text());checks['temporary_group_fixtures_cleaned']=state.get('cleaned') is True
with sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True) as db:
 checks['sqlite_integrity']=db.execute('PRAGMA integrity_check').fetchone()[0]=='ok'
 checks['test_groups_removed']=db.execute("SELECT count(*) FROM external_member_groups WHERE name LIKE '发布验证-20261020-%'").fetchone()[0]==0
 checks['test_external_account_removed']=db.execute('SELECT count(*) FROM external_members WHERE email=?',(state['email'],)).fetchone()[0]==0
 checks['original_okad_ids_retained']=not set(okad['database']['external_member_ids'])-{r[0] for r in db.execute('SELECT id FROM external_members')}
checks['test_sub2_account_removed']=a.sql(f"SELECT count(*) FROM accounts WHERE id={int(state['remote_id'])} AND deleted_at IS NULL")=='0'
lease=subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
checks['rolling_lease_removed']=not lease
checks['temporary_container_removed']=subprocess.run(['docker','inspect','adobe-relay-memory-canary'],capture_output=True).returncode!=0
runtime=a.runtime_verification(sha)
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sub2_application_sha':sha,'okad_application_sha':manifest['commit'],'checks':checks,'runtime':runtime,'database_restored':False}
(root/'paired-verification.json').write_text(json.dumps(out,ensure_ascii=False,indent=2));(root/'paired-verification.json').chmod(0o600)
print(json.dumps(out,ensure_ascii=False))
assert all(checks.values()),'Review failed preservation checks without restoring stale data'
