"""Read-only installed migration, binding and admin gateway verification."""
import datetime,hashlib,importlib.util,json,pathlib,urllib.request
ROOT=pathlib.Path('/opt/adobe-relay/releases/upstream-v0217-20261022')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
old=json.loads((ROOT/'installed-migrations-before.json').read_text())
current=json.loads(a.sql("SELECT json_object_agg(filename,checksum) FROM schema_migrations"))
expected=json.loads((ROOT/'expected-new-migrations.json').read_text())
assert all(current.get(k)==v for k,v in old.items()), 'Installed migration history changed'
assert {k:v for k,v in current.items() if k not in old}==expected, 'Only the seven reviewed migrations may be applied'
queries={
 'groups':"SELECT md5(coalesce(jsonb_agg(to_jsonb(g)-'updated_at'-'i18n' ORDER BY id)::text,'')) FROM groups g",
 'account_groups':"SELECT md5(coalesce(jsonb_agg(to_jsonb(g) ORDER BY account_id,group_id)::text,'')) FROM account_groups g",
 'apikey_binding':"SELECT md5(coalesce(jsonb_agg(jsonb_build_object('id',id,'key',key,'user_id',user_id,'group_id',group_id,'status',status,'deleted_at',deleted_at) ORDER BY id)::text,'')) FROM api_keys",
 'users_identity':"SELECT md5(coalesce(jsonb_agg(jsonb_build_object('id',id,'email',email,'password_hash',password_hash,'role',role,'status',status,'deleted_at',deleted_at) ORDER BY id)::text,'')) FROM users",
 'user_quotas':"SELECT md5(coalesce(jsonb_agg(to_jsonb(q) ORDER BY user_id,platform)::text,'')) FROM user_platform_quotas q"
}
before=json.loads((ROOT/'business-contracts-before.json').read_text())
checks={k:a.sql(q)==before[k] for k,q in queries.items()}
assert all(checks.values()), 'Live business contract changed; attribute it, never restore old data: '+json.dumps(checks)
key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
def get(path):
 req=urllib.request.Request('http://127.0.0.1:6666'+path,headers={'x-api-key':key})
 with urllib.request.urlopen(req,timeout=30) as r:return json.load(r)['data']
version=get('/api/v1/admin/system/version');assert version['version'].lstrip('v')=='0.2.17'
groups=get('/api/v1/admin/groups/all');assert isinstance(groups,list)
assert all(any(g['id']==i and g['platform']=='adobe' for g in groups) for i in [2,7]), 'Adobe groups 2/7 must retain their platform'
accounts=get('/api/v1/admin/accounts?page=1&page_size=1')
assert accounts['items'] and accounts['items'][0]['platform']=='adobe'
account_id=accounts['items'][0]['id']
models=get('/api/v1/admin/accounts/'+str(account_id)+'/models')
assert models and 'claude' not in json.dumps(models).lower(), 'Adobe account model listing must retain Adobe models'
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'installed_before':len(old),'installed_after':len(current),'reviewed_migrations':sorted(expected),'history_checksums_preserved':True,'business_contracts':checks,'application_version':version['version'],'adobe_groups_2_7_preserved':True,'adobe_accounts_total':accounts['total'],'adobe_model_listing_nonempty':True,'active_upstream_requests':False,'restored_old_data':False}
a.save('schema-business-verification.json',result);print(json.dumps(result))
