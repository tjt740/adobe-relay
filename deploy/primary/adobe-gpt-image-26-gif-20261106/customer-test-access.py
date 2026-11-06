"""Bounded administrator-owned test access through normal authenticated APIs."""
import datetime
import json
import os
from pathlib import Path
import subprocess
import urllib.error
import urllib.request

ROOT = Path('/opt/adobe-relay/releases/adobe-gpt-image-26-gif-20261106')
SHA = 'da24f5634a0fd332f95a14d666fa2e9cea19db53'
NAME = 'release-gif-20261106-current-group-temporary'
USER, GROUP = 1, 33


def sql(query):
    return subprocess.run(['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'psql', '-U', 'sub2api', '-d', 'sub2api', '-Atq', '-v', 'ON_ERROR_STOP=1'], input='BEGIN READ ONLY; ' + query + '; COMMIT;', text=True, capture_output=True, check=True, timeout=25).stdout.strip()


def save(name, value):
    with os.fdopen(os.open(ROOT / name, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), 'w') as f:
        json.dump(value, f, indent=2)


def load(name):
    return json.loads((ROOT / name).read_text())


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def groups():
    return json.loads(sql('SELECT coalesce(json_agg(group_id ORDER BY group_id),\'[]\') FROM user_allowed_groups WHERE user_id=1'))


def request(method, path, payload=None, token=None):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request('http://127.0.0.1:6666/api/v1' + path, data=json.dumps(payload).encode() if payload is not None else None, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=35) as h:
            return json.load(h).get('data')
    except urllib.error.HTTPError as error:
        raise RuntimeError(f'Authenticated test-access API returned HTTP {error.code}; do not bypass the denial') from None


def validate_owned_key(key):
    assert isinstance(key['id'], int) and key['id'] > 0 and key['id'] != 12
    assert key['user_id'] == USER and key['group_id'] == GROUP and key['name'] == NAME


def cleanup_groups(original, current, other_group_keys):
    assert GROUP not in original, 'Never revoke pre-existing access'
    assert not other_group_keys, 'Another key now depends on this grant; review concurrent changes'
    assert set(original).issubset(current), 'Pre-existing access changed independently'
    return [group for group in current if group != GROUP]


def prepare():
    os.umask(0o077)
    c = json.loads(subprocess.check_output(['docker', 'inspect', 'adobe-relay-app-1']))[0]
    assert c['Config']['Image'] == 'adobe-relay:' + SHA and c['State']['Health']['Status'] == 'healthy'
    assert (ROOT / 'rolling-finished.json').exists()
    original = groups()
    assert GROUP not in original
    assert sql("SELECT count(*) FROM api_keys WHERE user_id=1 AND name='" + NAME + "'") == '0'
    group = json.loads(sql("SELECT json_build_object('status',status,'exclusive',is_exclusive,'type',subscription_type) FROM groups WHERE id=33"))
    assert group == {'status': 'active', 'exclusive': True, 'type': 'standard'}
    save('customer-test-access-started.json', {'at': now(), 'user_id': USER, 'group_id': GROUP, 'original_allowed_groups': original, 'quota': 5, 'expires_in_days': 1, 'cleanup_required': True, 'existing_customer_keys_changed': False})
    env = dict(x.split('=', 1) for x in c['Config']['Env'])
    email = sql("SELECT email FROM users WHERE id=1 AND role='admin' AND status='active' AND deleted_at IS NULL")
    assert email == env['ADMIN_EMAIL'] and env['ADMIN_PASSWORD']
    data = request('POST', '/auth/login', {'email': email, 'password': env['ADMIN_PASSWORD']})
    assert not data.get('requires_2fa') and data['user']['id'] == USER and data['user']['role'] == 'admin'
    token = data['access_token']
    save('private-customer-test-admin-session.json', {'access_token': token})
    save('customer-test-grant-started.json', {'at': now(), 'user_id': USER, 'added_group': GROUP})
    request('PUT', '/admin/users/1', {'allowed_groups': original + [GROUP]}, token)
    assert groups() == sorted(original + [GROUP])
    save('customer-test-grant-result.json', {'at': now(), 'authenticated_admin_grant': True, 'user_id': USER, 'added_group': GROUP})
    save('customer-test-key-started.json', {'at': now(), 'owner_user_id': USER, 'group_id': GROUP, 'name': NAME})
    key = request('POST', '/keys', {'name': NAME, 'group_id': GROUP, 'quota': 5, 'expires_in_days': 1}, token)
    validate_owned_key(key)
    assert key['quota'] == 5 and key['expires_at'] and key['key']
    save('private-customer-test-key.json', {k: key[k] for k in ['id', 'user_id', 'group_id', 'name', 'key']})
    save('customer-test-key-result.json', {'at': now(), **{k: key[k] for k in ['id', 'user_id', 'group_id', 'name', 'quota', 'expires_at', 'status']}})
    print(json.dumps({'temporary_test_key_created': True, 'key_id': key['id'], 'group_id': GROUP, 'quota': 5, 'expires_at': key['expires_at']}), flush=True)


def cleanup():
    before = load('customer-test-access-started.json')
    private = ROOT / 'private-customer-test-admin-session.json'
    if not private.exists():
        assert groups() == before['original_allowed_groups'], 'Test access changed without an authenticated session'
        save('customer-test-access-cleanup.json', {'at': now(), 'key_deleted': True, 'temporary_grant_removed': True, 'private_files_removed': True, 'no_key_created': True})
        return
    token = load(private.name)['access_token']
    rows = json.loads(sql("SELECT coalesce(json_agg(json_build_object('id',id,'user_id',user_id,'group_id',group_id,'name',name)),'[]') FROM api_keys WHERE user_id=1 AND name='" + NAME + "' AND deleted_at IS NULL"))
    assert len(rows) <= 1
    for key in rows:
        validate_owned_key(key)
        request('DELETE', '/keys/' + str(key['id']), token=token)
    assert sql("SELECT count(*) FROM api_keys WHERE user_id=1 AND name='" + NAME + "' AND deleted_at IS NULL") == '0'
    other = int(sql('SELECT count(*) FROM api_keys WHERE user_id=1 AND group_id=33 AND deleted_at IS NULL'))
    current = groups()
    remaining = cleanup_groups(before['original_allowed_groups'], current, other)
    if current != remaining:
        request('PUT', '/admin/users/1', {'allowed_groups': remaining}, token)
    assert groups() == remaining
    (ROOT / 'private-customer-test-key.json').unlink(missing_ok=True)
    private.unlink()
    save('customer-test-access-cleanup.json', {'at': now(), 'key_deleted': True, 'temporary_grant_removed': True, 'private_files_removed': True, 'other_allowed_groups_preserved': True, 'original_allowed_groups': before['original_allowed_groups'], 'current_allowed_groups': remaining, 'existing_customer_keys_changed': False})
    print(json.dumps({'temporary_test_access_cleaned': True}), flush=True)
