import datetime, fcntl, hashlib, json, pathlib, subprocess, sys

ROOT = pathlib.Path('/opt/adobe-relay/releases/account-health-20261013')
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
    result = subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'], input=query, text=True, capture_output=True, check=True)
    return result.stdout.strip()

def digest(value):
    return hashlib.sha256(value).hexdigest()

out = {'at':datetime.datetime.now(datetime.timezone.utc).isoformat(), 'containers':{}, 'config_hashes':{}}
for name in ['adobe-relay-app-1','adobe-team-cookie-app-1','adobe-relay-postgres-1','adobe-relay-redis-1','adobe-relay-mihomo-1']:
    data = json.loads(subprocess.check_output(['docker','inspect',name]))[0]
    out['containers'][name] = {'id':data['Id'],'image':data['Config']['Image'],'image_id':data['Image'],'started':data['State']['StartedAt'],'health':data['State'].get('Health',{}).get('Status'),'restarts':data['RestartCount']}
for path in ['/opt/adobe-relay/shared/.env','/opt/adobe-relay/shared/compose.yml','/opt/adobe-relay/shared/data/config.yaml','/opt/adobe-relay/shared/clash.yml','/opt/adobe-relay/shared/resources.yml','/opt/adobe-team-cookie/.env','/opt/adobe-team-cookie/compose.yml']:
    out['config_hashes'][path] = digest(pathlib.Path(path).read_bytes())
probe = "from app.db.session import engine; from sqlalchemy import text; import hashlib,json; c=engine.connect(); rows=[tuple(r) for r in c.execute(text(\"SELECT key,value FROM settings WHERE key LIKE 'sub2_%' OR key LIKE '%integration%' ORDER BY key\"))]; print(hashlib.sha256(json.dumps(rows,ensure_ascii=False).encode()).hexdigest()); c.close()"
out['okad_link_hash'] = subprocess.check_output(['docker','exec','adobe-team-cookie-app-1','python','-c',probe],text=True).strip()
out['sub2_env_without_image'] = digest('\n'.join(line for line in pathlib.Path('/opt/adobe-relay/shared/.env').read_text().splitlines() if not line.startswith('SUB2API_IMAGE=')).encode())
out['pagination'] = json.loads(sql("SELECT COALESCE(json_object_agg(key,value),'{}') FROM settings WHERE key IN ('table_default_page_size','table_page_size_options')"))
out['mail_hash'] = sql("SELECT md5(COALESCE(string_agg(key||':'||value,',' ORDER BY key),'')) FROM settings WHERE key LIKE 'smtp_%' OR key IN ('ops_adobe_alert_mail_private','ops_email_notification_config','email_verify_enabled','ops_alert_runtime_settings')")
out['policy'] = json.loads(sql("SELECT json_build_object('ratio',filters->'adobe_error_ratio_percent','minimum',threshold,'window',window_minutes,'cooldown',cooldown_minutes,'enabled',enabled) FROM ops_alert_rules WHERE id=9"))
assert out['policy']['ratio'] == 20 and out['policy']['minimum'] == 5 and out['policy']['enabled']
out['rule_hash'] = sql("SELECT md5(row_to_json(r)::text) FROM ops_alert_rules r WHERE id=9")
out['accounts'] = json.loads(sql("SELECT json_build_object('count',count(*),'adobe',count(*) FILTER(WHERE platform='adobe'),'mock',count(*) FILTER(WHERE extra->>'mock_dataset'='account-filter-mocks-20261013'),'id_proxy_hash',md5(string_agg(id::text||':'||COALESCE(proxy_id::text,'null'),',' ORDER BY id))) FROM accounts WHERE deleted_at IS NULL"))
out['automatic_tests'] = json.loads(sql("SELECT COALESCE(json_object_agg(state,n),'{}') FROM (SELECT extra->'okad_connection_test'->>'state' state,count(*) n FROM accounts WHERE deleted_at IS NULL AND extra ? 'okad_connection_test' GROUP BY 1) q"))
assert out['accounts']['mock'] == 0, 'Unexpected production mock dataset'
if phase == 'before':
    assert out['containers']['adobe-relay-app-1']['image'] == 'adobe-relay:035bcef4311d606fee461321cb4a0c2eeaa3b5a2', 'Production has a different Sub2 release'
    subprocess.run(['docker','tag',out['containers']['adobe-relay-app-1']['image_id'],'adobe-relay:before-account-health-20261013'],check=True)
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
        'alert_rule':out['rule_hash']==before['rule_hash'],
        'accounts_and_proxies':out['accounts']==before['accounts']
    }
    out['backup'] = str(sorted(pathlib.Path('/opt/adobe-relay/backups').glob('*-'+expected))[-1])
if phase == 'after':
    assert all(out['preserved'][k] for k in ['other_containers','files','env_except_image','mail','alert_rule','okad_link']), 'Protected production state changed'
output.write_text(json.dumps(out,ensure_ascii=False,indent=2))
output.chmod(0o600)
print(json.dumps(out,ensure_ascii=False))
