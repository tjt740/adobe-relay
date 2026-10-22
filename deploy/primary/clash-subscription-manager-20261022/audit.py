"""Snapshot and guard this code-only release. Never restore business data."""
import argparse
import datetime
import fcntl
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import sqlite3
import subprocess
from urllib.parse import quote
import urllib.request

RID = 'clash-subscription-manager-20261022'
ROOT = Path('/opt/adobe-relay/releases') / RID
OLD_IMAGE = 'adobe-relay:41adb6a7622f2e4f53396eaf0916b4bc6d5d3272'
ROLLBACK_IMAGE = 'adobe-relay:before-' + RID
APP = 'adobe-relay-app-1'
CONTAINERS = [APP, 'adobe-team-cookie-app-1', 'adobe-relay-postgres-1',
              'adobe-relay-redis-1', 'adobe-relay-mihomo-1', 'image-api-lab-image-lab-1']


def sql(query):
    return subprocess.run(
        ['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'psql', '-U', 'sub2api', '-d', 'sub2api', '-Atq', '-v', 'ON_ERROR_STOP=1'],
        input="BEGIN READ ONLY; SET LOCAL statement_timeout='15s'; " + query + '; COMMIT;',
        text=True, capture_output=True, check=True, timeout=25).stdout.strip()


def digest(value):
    return hashlib.sha256(value).hexdigest()


def inspect(name):
    return json.loads(subprocess.check_output(['docker', 'inspect', name]))[0]


def save(name, value):
    target = ROOT / name
    assert not target.exists(), 'Audit already exists: ' + name
    target.write_text(json.dumps(value, ensure_ascii=False, indent=2))
    target.chmod(0o600)


def snapshot():
    out = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'containers': {}, 'config_hashes': {}}
    for name in CONTAINERS:
        d = inspect(name)
        out['containers'][name] = {'id': d['Id'], 'image': d['Config']['Image'], 'image_id': d['Image'],
                                   'started': d['State']['StartedAt'], 'health': d['State'].get('Health', {}).get('Status'), 'restarts': d['RestartCount'], 'resources': {k: d['HostConfig'].get(k) for k in ['Memory', 'MemorySwap', 'CpuShares', 'CpuQuota', 'CpuPeriod', 'NanoCpus', 'PidsLimit']}, 'go_env': {k: v for k, v in (item.split('=', 1) for item in d['Config']['Env']) if k in ['GOMEMLIMIT', 'GOMAXPROCS']}}
    paths = [Path(p) for p in ['/opt/adobe-relay/shared/compose.yml', '/opt/adobe-relay/shared/data/config.yaml',
             '/opt/adobe-relay/shared/clash.yml', '/opt/adobe-relay/shared/resources.yml',
             '/opt/adobe-team-cookie/.env', '/opt/adobe-team-cookie/compose.yml']]
    paths += [p for root in ('/etc/nginx/sites-enabled', '/etc/nginx/conf.d') for p in Path(root).glob('*') if p.is_file()]
    if Path('/etc/nginx/nginx.conf').exists():
        paths.append(Path('/etc/nginx/nginx.conf'))
    for path in paths:
        out['config_hashes'][str(path)] = digest(path.read_bytes())
    env = Path('/opt/adobe-relay/shared/.env').read_text()
    out['env_without_image_hash'] = digest('\n'.join(line for line in env.splitlines() if not line.startswith('SUB2API_IMAGE=')).encode())
    okad = inspect('adobe-team-cookie-app-1')
    url = next(v.split('=', 1)[1] for v in okad['Config']['Env'] if v.startswith('DATABASE_URL='))
    assert url.startswith('sqlite:///')
    db = PurePosixPath(url[len('sqlite:///'):])
    mount = next(m for m in sorted(okad['Mounts'], key=lambda m: len(m['Destination']), reverse=True) if db.is_relative_to(m['Destination']))
    host = Path(mount['Source']) / str(db.relative_to(mount['Destination']))
    with sqlite3.connect('file:' + quote(str(host)) + '?mode=ro', uri=True, timeout=5) as connection:
        rows = connection.execute('SELECT key,value FROM settings ORDER BY key').fetchall()
    out['okad_settings_hash'] = digest(json.dumps(rows, ensure_ascii=False).encode())
    out['settings_hashes'] = json.loads(sql("SELECT coalesce(json_object_agg(key,md5(value)),'{}') FROM settings"))
    out['clash_contract_hash'] = sql("SELECT md5(jsonb_build_object('url',document->'url','revision',document->'revision','nodes',(SELECT coalesce(jsonb_agg(n-'source_id'-'original_name' ORDER BY ord),'[]') FROM jsonb_array_elements(document->'nodes') WITH ORDINALITY AS e(n,ord)))::text) FROM clash_subscription WHERE id=1")
    out['rules_hash'] = sql("SELECT md5(coalesce(jsonb_agg(to_jsonb(r)-'updated_at'-'last_triggered_at' ORDER BY id)::text,'')) FROM ops_alert_rules r")
    out['account_state'] = json.loads(sql("SELECT coalesce(json_agg(json_build_object('id',id,'credentials_hash',md5(credentials::text),'proxy_id',proxy_id,'status',status,'schedulable',schedulable,'manual_pause',extra->>'okad_manual_pause') ORDER BY id),'[]') FROM accounts WHERE deleted_at IS NULL"))
    return out


def reviewed_setting_hashes(original, review=None):
    expected = dict(original)
    if review is None:
        return expected
    key = review['key']
    assert key == 'claude_code_client_version_synced', 'Only the independently synced client version can be reviewed'
    assert review['service_tree_before'] == review['service_tree_candidate'] == '839be5cf5eb4e1e0d16ebf349530550052c75766'
    assert review['event'] == 'claude_code_version_synced'
    assert re.fullmatch('[0-9a-f]{64}', review['evidence_log_sha256'])
    old, new = review['old_value'], review['new_value']
    assert re.fullmatch(r'\d+\.\d+\.\d+', old) and re.fullmatch(r'\d+\.\d+\.\d+', new)
    assert tuple(map(int, new.split('.'))) > tuple(map(int, old.split('.'))), 'Automatic version must advance'
    assert hashlib.md5(old.encode()).hexdigest() == expected[key] == review['old_hash']
    assert hashlib.md5(new.encode()).hexdigest() == review['new_hash']
    expected[key] = review['new_hash']
    return expected


def check_preserved(before, current):
    review_file = ROOT / 'reviewed-automatic-settings.json'
    review = json.loads(review_file.read_text()) if review_file.exists() else None
    expected_settings = reviewed_setting_hashes(before['settings_hashes'], review)
    checks = {
        'other_containers': all(current['containers'].get(name, {}).get('id') == value['id'] and
                                current['containers'][name]['image_id'] == value['image_id']
                                for name, value in before['containers'].items() if name != APP),
        'configuration': current['config_hashes'] == before['config_hashes'],
        'resources_and_go': all(current['containers'][name]['resources'] == item['resources'] and current['containers'][name]['go_env'] == item['go_env'] for name, item in before['containers'].items()),
        'environment': current['env_without_image_hash'] == before['env_without_image_hash'],
        'okad_settings': current['okad_settings_hash'] == before['okad_settings_hash'],
        'sub2_settings_including_smtp': current['settings_hashes'] == expected_settings,
        'clash_nodes_bindings_and_configuration': current['clash_contract_hash'] == before['clash_contract_hash'],
        'alert_rules': current['rules_hash'] == before['rules_hash'],
    }
    assert all(checks.values()), 'Protected state changed; review without restoring old data: ' + json.dumps(checks)
    return checks


def validate_rollback(before, current, expected_sha, saved_image_id):
    assert re.fullmatch('[0-9a-f]{40}', expected_sha), 'Invalid application revision'
    assert current['containers'][APP]['image'] == 'adobe-relay:' + expected_sha, 'A later release exists; review before rollback'
    assert saved_image_id == before['containers'][APP]['image_id'], 'Rollback image changed'
    return check_preserved(before, current)


def runtime_verification(expected_sha):
    key = sql("SELECT value FROM settings WHERE key='admin_api_key'")
    def request(path):
        req = urllib.request.Request('http://127.0.0.1:6666' + path, headers={'x-api-key': key})
        with urllib.request.urlopen(req, timeout=30) as response:
            return json.load(response)
    for url in ['https://creative.cheap/health', 'https://creative.cheap/okad/api/health']:
        with urllib.request.urlopen(url, timeout=30) as response:
            assert response.status == 200
    report = request('/api/v1/admin/ops/adobe-health')['data']
    assert report['quiet_notifications'] and report['threshold'] == 20 and report['minimum'] == 5
    assert report['recovery_threshold'] == 10 and report['confirmation_minutes'] == 4 and report['notification_cooldown_minutes'] == 60
    req = urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/runtime-events', headers={'x-api-key': key})
    with urllib.request.urlopen(req, timeout=10) as response:
        assert response.headers['Content-Type'].startswith('text/event-stream')
        assert b'"change"' in response.readline()
    heartbeat = json.loads(sql("SELECT json_build_object('last_success_at',last_success_at,'healthy',last_success_at>now()-interval '5 minutes' AND (last_error_at IS NULL OR last_success_at>last_error_at)) FROM ops_job_heartbeats WHERE job_name='ops_alert_evaluator'"))
    assert heartbeat['healthy'], 'No recent successful alert evaluation'
    return {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'sha': expected_sha,
            'https_both_sites': True, 'runtime_events': True, 'report_policy_preserved': True,
            'report_total': report['total'], 'report_pending': report['pending'], 'heartbeat': heartbeat,
            'sent_mail': False, 'triggered_adobe_generation': False}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['before', 'after', 'verify', 'check-rollback', 'rollback'])
    args = parser.parse_args()
    ROOT.mkdir(parents=True, exist_ok=True)
    ROOT.chmod(0o700)
    locks = []
    for path in ['/opt/adobe-relay/.release.lock', '/opt/adobe-team-cookie/.release.lock', '/opt/adobe-team-cookie/.incremental-release.lock']:
        lock = open(path, 'a')
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        locks.append(lock)
    if args.action == 'verify':
        sha = (ROOT / 'expected-sha').read_text().strip()
        assert inspect(APP)['Config']['Image'] == 'adobe-relay:' + sha
        out = runtime_verification(sha)
        save('runtime-verification.json', out)
        print(json.dumps(out))
        return
    current = snapshot()
    if args.action == 'before':
        assert not (ROOT / 'before.json').exists(), 'Preflight already recorded'
        assert current['containers'][APP]['image'] == OLD_IMAGE, 'Production changed; review it first'
        assert current['containers'][APP]['health'] == 'healthy'
        tags = json.loads(subprocess.check_output(['docker', 'image', 'ls', '--format', '{{json .}}', ROLLBACK_IMAGE]).strip() or b'null')
        assert tags is None, 'Rollback tag already exists'
        subprocess.run(['docker', 'tag', current['containers'][APP]['image_id'], ROLLBACK_IMAGE], check=True)
        save('before.json', current)
        print(json.dumps({'preflight': 'saved', 'rollback_image': ROLLBACK_IMAGE, 'accounts': len(current['account_state']), 'containers': current['containers']}))
        return
    before = json.loads((ROOT / 'before.json').read_text())
    sha = (ROOT / 'expected-sha').read_text().strip()
    if args.action == 'after':
        assert re.fullmatch('[0-9a-f]{40}', sha)
        assert current['containers'][APP]['image'] == 'adobe-relay:' + sha
        assert current['containers'][APP]['health'] == 'healthy'
        current['preserved'] = check_preserved(before, current)
        backups = sorted(Path('/opt/adobe-relay/backups').glob('*-' + sha))
        assert backups, 'Missing cutover backup'
        current['backup'] = str(backups[-1])
        assert (backups[-1] / 'previous-image').read_text().strip() == before['containers'][APP]['image'], 'Cutover replaced an unreviewed version'
        save('after.json', current)
        print(json.dumps({'preserved': current['preserved'], 'backup': current['backup'], 'accounts': len(current['account_state'])}))
        return
    checks = validate_rollback(before, current, sha, inspect(ROLLBACK_IMAGE)['Id'])
    if args.action == 'check-rollback':
        print(json.dumps({'rollback_guards_passed': True, 'mutated': False, 'preserved': checks}))
        return
    # release.sh reacquires its lock and atomically checks the current revision.
    for lock in reversed(locks):
        lock.close()
    subprocess.run(['bash', '/opt/adobe-relay/deploy/primary/release.sh', ROLLBACK_IMAGE, 'adobe-relay:' + sha], check=True)
    print('Previous application restored; current data, credentials, proxies, SMTP, rules and Okad retained.')


if __name__ == '__main__':
    main()
