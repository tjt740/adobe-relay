import datetime, fcntl, hashlib, json, pathlib, sqlite3, subprocess, sys
from urllib.parse import quote

ROOT = pathlib.Path('/opt/adobe-relay/releases/account-status-cookie-fix-20261014')
ROOT.mkdir(parents=True, exist_ok=True)
phase = sys.argv[1]
assert phase in ('before', 'after')
expected_sha = sys.argv[2] if len(sys.argv)>2 else ''
output = ROOT / (phase + '.json')
if output.exists() and phase == 'before':
    print(output.read_text()); sys.exit(0)
if output.exists():
    output = ROOT / ('after-' + datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '.json')
locks = []
for path in ['/opt/adobe-relay/.release.lock', '/opt/adobe-team-cookie/.release.lock']:
    lock = open(path, 'a')
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    locks.append(lock)

def sql(query):
    result = subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'], input="BEGIN READ ONLY; SET LOCAL statement_timeout='10s'; "+query+"; COMMIT;", text=True, capture_output=True, check=True, timeout=20)
    return result.stdout.strip()

def digest(value):
    return hashlib.sha256(value).hexdigest()

out = {'at':datetime.datetime.now(datetime.timezone.utc).isoformat(), 'containers':{}, 'config_hashes':{}}
for name in ['adobe-relay-app-1','adobe-team-cookie-app-1','adobe-relay-postgres-1','adobe-relay-redis-1','adobe-relay-mihomo-1']:
    data = json.loads(subprocess.check_output(['docker','inspect',name]))[0]
    out['containers'][name] = {'id':data['Id'],'image':data['Config']['Image'],'image_id':data['Image'],'started':data['State']['StartedAt'],'health':data['State'].get('Health',{}).get('Status'),'restarts':data['RestartCount']}
for path in ['/opt/adobe-relay/shared/.env','/opt/adobe-relay/shared/compose.yml','/opt/adobe-relay/shared/data/config.yaml','/opt/adobe-relay/shared/clash.yml','/opt/adobe-relay/shared/resources.yml','/opt/adobe-team-cookie/.env','/opt/adobe-team-cookie/compose.yml']:
    out['config_hashes'][path] = digest(pathlib.Path(path).read_bytes())
# Read the same mounted SQLite database without importing Okad's application
# inside its memory-limited container. Read-only mode retains WAL visibility.
okad = json.loads(subprocess.check_output(['docker','inspect','adobe-team-cookie-app-1']))[0]
db_url = next(value.split('=',1)[1] for value in okad['Config']['Env'] if value.startswith('DATABASE_URL='))
assert db_url.startswith('sqlite:///'), 'Expected deployed Okad SQLite storage'
db_path = pathlib.PurePosixPath(db_url[len('sqlite:///'):])
assert db_path.is_absolute()
mounts = sorted(okad['Mounts'], key=lambda m:len(m['Destination']), reverse=True)
mount = next(m for m in mounts if db_path.is_relative_to(m['Destination']))
host_path = pathlib.Path(mount['Source']) / str(db_path.relative_to(mount['Destination']))
with sqlite3.connect('file:'+quote(str(host_path))+'?mode=ro', uri=True, timeout=5) as connection:
    rows = connection.execute("SELECT key,value FROM settings WHERE key LIKE 'sub2_%' OR key LIKE '%integration%' ORDER BY key").fetchall()
out['okad_link_hash'] = digest(json.dumps(rows,ensure_ascii=False).encode())
out['sub2_env_without_image'] = digest('\n'.join(line for line in pathlib.Path('/opt/adobe-relay/shared/.env').read_text().splitlines() if not line.startswith('SUB2API_IMAGE=')).encode())
out['pagination'] = json.loads(sql("SELECT COALESCE(json_object_agg(key,value),'{}') FROM settings WHERE key IN ('table_default_page_size','table_page_size_options')"))
out['mail_hash'] = sql("SELECT md5(COALESCE(string_agg(key||':'||value,',' ORDER BY key),'')) FROM settings WHERE key LIKE 'smtp_%' OR key IN ('ops_adobe_alert_mail_private','ops_email_notification_config','email_verify_enabled','ops_alert_runtime_settings')")
out['policy'] = json.loads(sql("SELECT json_build_object('ratio',filters->'adobe_error_ratio_percent','minimum',threshold,'window',window_minutes,'cooldown',cooldown_minutes,'enabled',enabled) FROM ops_alert_rules WHERE id=9"))
assert out['policy']['ratio'] == 20 and out['policy']['minimum'] == 5 and out['policy']['enabled']
out['rule'] = json.loads(sql("SELECT row_to_json(r) FROM ops_alert_rules r WHERE id=9"))
out['rule_hash'] = sql("SELECT md5(row_to_json(r)::text) FROM ops_alert_rules r WHERE id=9")
out['accounts'] = json.loads(sql("SELECT json_build_object('count',count(*),'adobe',count(*) FILTER(WHERE platform='adobe'),'mock',count(*) FILTER(WHERE extra->>'mock_dataset'='account-filter-mocks-20261013'),'id_proxy_hash',md5(string_agg(id::text||':'||COALESCE(proxy_id::text,'null'),',' ORDER BY id))) FROM accounts WHERE deleted_at IS NULL"))
out['account_state'] = json.loads(sql("SELECT COALESCE(json_agg(json_build_object('id',id,'credentials_hash',md5(credentials::text),'proxy_id',proxy_id,'status',status,'schedulable',schedulable,'manual_pause',extra->>'okad_manual_pause','cookie_version',extra->'okad'->>'version') ORDER BY id),'[]') FROM accounts WHERE deleted_at IS NULL"))
out['automatic_tests'] = json.loads(sql("SELECT COALESCE(json_object_agg(state,n),'{}') FROM (SELECT extra->'okad_connection_test'->>'state' state,count(*) n FROM accounts WHERE deleted_at IS NULL AND extra ? 'okad_connection_test' GROUP BY 1) q"))
assert out['accounts']['mock'] == 0, 'Unexpected production mock dataset'
if phase == 'before':
    assert out['containers']['adobe-relay-app-1']['image'] == 'adobe-relay:265e6badcca7c8071787045711e0a84128dfb799', 'Production has a different Sub2 release'
    subprocess.run(['docker','tag',out['containers']['adobe-relay-app-1']['image_id'],'adobe-relay:before-account-status-cookie-fix-20261014'],check=True)
else:
    before = json.loads((ROOT/'before.json').read_text())
    expected = expected_sha
    assert len(expected)==40
    (ROOT/'expected-sha').write_text(expected)
    assert out['containers']['adobe-relay-app-1']['image'] == 'adobe-relay:'+expected
    assert out['containers']['adobe-relay-app-1']['health'] == 'healthy'
    out['preserved'] = {
        'other_containers':all(out['containers'][name]['id']==value['id'] for name,value in before['containers'].items() if name!='adobe-relay-app-1'),
        'files':all(out['config_hashes'][path]==value for path,value in before['config_hashes'].items() if path!='/opt/adobe-relay/shared/.env'),
        'env_except_image':out['sub2_env_without_image']==before['sub2_env_without_image'],
        'mail':out['mail_hash']==before['mail_hash'],
        'okad_link':out['okad_link_hash']==before['okad_link_hash'],
        'alert_rule':all(out['rule'][k]==v for k,v in before['rule'].items() if k not in ('updated_at','last_triggered_at')),
        'accounts_and_proxies':out['accounts']==before['accounts']
    }
    out['backup'] = str(sorted(pathlib.Path('/opt/adobe-relay/backups').glob('*-'+expected))[-1])
if phase == 'after':
    assert all(out['preserved'][k] for k in ['other_containers','files','env_except_image','mail','alert_rule','okad_link']), 'Protected production state changed'
output.write_text(json.dumps(out,ensure_ascii=False,indent=2))
output.chmod(0o600)
print(json.dumps({k:v for k,v in out.items() if k not in ('rule','account_state')},ensure_ascii=False))
