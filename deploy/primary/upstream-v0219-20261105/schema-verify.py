"""Verify the exact additive schema upgrade without writing business data."""
import datetime,importlib.util,json
from pathlib import Path
ROOT=Path('/opt/adobe-relay/releases/upstream-v0219-20261105')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
baseline=json.loads((ROOT/'installed-migrations-before.json').read_text())
manifest=json.loads((ROOT/'migration-manifest.json').read_text())
installed=json.loads(a.sql('SELECT json_object_agg(filename,checksum) FROM schema_migrations'))
assert all(installed.get(name)==value for name,value in baseline.items()),'Installed migration changed'
assert {name:value for name,value in installed.items() if name not in baseline}==manifest,'Unexpected new migration set'
contracts=json.loads((ROOT/'business-contracts-before.json').read_text())
group_hash=a.sql("SELECT md5(coalesce(jsonb_agg(to_jsonb(g)-'updated_at'-'image_count_billing' ORDER BY id)::text,'')) FROM groups g")
assert group_hash==contracts['groups'],'Group configuration changed; review before routing'
new_switches=int(a.sql('SELECT count(*) FROM groups WHERE image_count_billing'))
assert new_switches==0,'No existing group may be opted into per-image billing by upgrade'
admin_keys=int(a.sql('SELECT count(*) FROM scoped_admin_api_keys'))
assert admin_keys==0,'Upgrade must not create scoped administrator keys'
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'historical_migrations_preserved':len(baseline),
 'new_migrations':manifest,'new_switch_defaults_off':True,'groups_preserved':True,'no_scoped_keys_created':True,'read_only':True}
target=ROOT/'schema-verification.json'
if not target.exists():a.save(target.name,result)
print(json.dumps(result))
