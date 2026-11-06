"""Normal API test access for the user's selected account, with isolated cleanup."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess

ROOT = Path('/opt/adobe-relay/releases/adobe-gpt-image-26-gif-20261106')
SHA = 'da24f5634a0fd332f95a14d666fa2e9cea19db53'
NAME = 'release-gif-20261106-selected-2933-temporary'
ACCOUNT = 2933
spec = importlib.util.spec_from_file_location('base_access', Path(__file__).with_name('customer-test-access.py'))
base = importlib.util.module_from_spec(spec)
spec.loader.exec_module(base)
sql, save, load, now, request = base.sql, base.save, base.load, base.now, base.request


def account_groups():
    return json.loads(sql("SELECT coalesce(json_agg(group_id ORDER BY group_id),'[]') FROM account_groups WHERE account_id=2933"))


def validate_group(group):
    assert isinstance(group['id'], int) and group['id'] > 0 and group['id'] not in [7, 33]
    assert group['name'] == NAME and group['platform'] == 'adobe' and group['is_exclusive'] is True


def validate_key(key, gid):
    assert isinstance(key['id'], int) and key['id'] > 0 and key['id'] != 12
    assert key['user_id'] == 1 and key['group_id'] == gid and key['name'] == NAME


def prepare():
    os.umask(0o077)
    c = json.loads(subprocess.check_output(['docker', 'inspect', 'adobe-relay-app-1']))[0]
    assert c['Config']['Image'] == 'adobe-relay:' + SHA and c['State']['Health']['Status'] == 'healthy'
    prior_cleanup = load('customer-test-access-cleanup.json')
    assert prior_cleanup['key_deleted'] and prior_cleanup['temporary_grant_removed']
    assert sql("SELECT count(*) FROM groups WHERE name='" + NAME + "'") == '0'
    assert sql("SELECT count(*) FROM api_keys WHERE user_id=1 AND name='" + NAME + "'") == '0'
    assert sql("SELECT count(*) FROM accounts WHERE id=2933 AND deleted_at IS NULL AND platform='adobe' AND type='oauth' AND status='active' AND schedulable AND COALESCE(extra->>'okad_manual_pause','false')<>'true'") == '1'
    original = {'at': now(), 'selected_account_id': ACCOUNT, 'original_user_groups': base.groups(), 'original_account_groups': account_groups(), 'existing_groups_modified': False, 'cleanup_required': True}
    assert 33 in original['original_account_groups']
    save('selected-test-access-started.json', original)
    env = dict(x.split('=', 1) for x in c['Config']['Env'])
    email = sql("SELECT email FROM users WHERE id=1 AND role='admin' AND status='active' AND deleted_at IS NULL")
    assert email == env['ADMIN_EMAIL'] and env['ADMIN_PASSWORD']
    data = request('POST', '/auth/login', {'email': email, 'password': env['ADMIN_PASSWORD']})
    assert not data.get('requires_2fa') and data['user']['id'] == 1 and data['user']['role'] == 'admin'
    token = data['access_token']
    save('private-selected-test-admin-session.json', {'access_token': token})
    source = request('GET', '/admin/groups/33', token=token)
    assert source['id'] == 33 and source['platform'] == 'adobe'
    fields = ['rate_multiplier', 'model_pricing', 'image_count_billing', 'image_rate_independent', 'image_rate_multiplier', 'image_price_1k', 'image_price_2k', 'image_price_4k', 'require_oauth_only', 'model_allowlist']
    config = {k: source[k] for k in fields if k in source and source[k] is not None}
    config.update(name=NAME, description='Temporary release verification for selected Adobe account; removed after test', platform='adobe', is_exclusive=True, subscription_type='standard')
    save('selected-test-group-started.json', {'at': now(), 'name': NAME, 'source_group_id': 33, 'copied_fields': [k for k in fields if k in config], 'configuration_sha256': hashlib.sha256(json.dumps(config, sort_keys=True).encode()).hexdigest()})
    group = request('POST', '/admin/groups', config, token)
    validate_group(group)
    gid = group['id']
    save('selected-test-group-result.json', {'at': now(), 'id': gid, 'name': NAME, 'platform': 'adobe', 'is_exclusive': True, 'selected_account_id': ACCOUNT})
    current = account_groups()
    assert current == original['original_account_groups']
    request('PUT', '/admin/accounts/2933', {'group_ids': current + [gid]}, token)
    assert set(account_groups()) == set(current + [gid])
    groups = base.groups()
    assert groups == original['original_user_groups']
    request('PUT', '/admin/users/1', {'allowed_groups': groups + [gid]}, token)
    key = request('POST', '/keys', {'name': NAME, 'group_id': gid, 'quota': 5, 'expires_in_days': 1}, token)
    validate_key(key, gid)
    assert key['quota'] == 5 and key['expires_at'] and key['key']
    save('private-selected-test-key.json', {k: key[k] for k in ['id', 'user_id', 'group_id', 'name', 'key']})
    save('selected-test-key-result.json', {'at': now(), **{k: key[k] for k in ['id', 'user_id', 'group_id', 'name', 'quota', 'expires_at', 'status']}})
    print(json.dumps({'selected_account_id': ACCOUNT, 'temporary_group_id': gid, 'temporary_key_id': key['id'], 'quota': 5}), flush=True)


def cleanup():
    before = load('selected-test-access-started.json')
    session = ROOT / 'private-selected-test-admin-session.json'
    found = json.loads(sql("SELECT coalesce(json_agg(json_build_object('id',id,'name',name,'platform',platform,'is_exclusive',is_exclusive)),'[]') FROM groups WHERE name='" + NAME + "' AND deleted_at IS NULL"))
    assert len(found) <= 1
    gid = None
    if found:
        group = found[0]
        validate_group(group)
        gid = group['id']
        token = load(session.name)['access_token']
        keys = json.loads(sql("SELECT coalesce(json_agg(json_build_object('id',id,'user_id',user_id,'group_id',group_id,'name',name)),'[]') FROM api_keys WHERE group_id=" + str(gid) + " AND deleted_at IS NULL"))
        assert len(keys) <= 1, 'Unexpected key in the temporary group; inspect before deletion'
        for key in keys:
            validate_key(key, gid)
            request('DELETE', '/keys/' + str(key['id']), token=token)
        assert sql('SELECT count(*) FROM api_keys WHERE group_id=' + str(gid) + ' AND deleted_at IS NULL') == '0'
        current = base.groups()
        assert set(before['original_user_groups']).issubset(current)
        if gid in current:
            request('PUT', '/admin/users/1', {'allowed_groups': [x for x in current if x != gid]}, token)
        assert sql('SELECT count(*) FROM user_allowed_groups WHERE group_id=' + str(gid)) == '0', 'Another user depends on this temporary group'
        bindings = json.loads(sql("SELECT coalesce(json_agg(account_id),'[]') FROM account_groups WHERE group_id=" + str(gid)))
        assert set(bindings).issubset({ACCOUNT}), 'Another account was added; inspect before deletion'
        request('DELETE', '/admin/groups/' + str(gid), token=token)
        assert sql('SELECT count(*) FROM groups WHERE id=' + str(gid) + ' AND deleted_at IS NULL') == '0'
        assert sql('SELECT count(*) FROM account_groups WHERE group_id=' + str(gid)) == '0'
    original_groups_preserved = set(before['original_account_groups']).issubset(account_groups())
    original_access_preserved = set(before['original_user_groups']).issubset(base.groups())
    assert original_groups_preserved and original_access_preserved
    (ROOT / 'private-selected-test-key.json').unlink(missing_ok=True)
    session.unlink(missing_ok=True)
    save('selected-test-access-cleanup.json', {'at': now(), 'selected_account_id': ACCOUNT, 'temporary_group_id': gid, 'key_deleted': True, 'temporary_grant_removed': True, 'temporary_group_deleted': True, 'private_files_removed': True, 'other_allowed_groups_preserved': original_access_preserved, 'account_original_groups_preserved': original_groups_preserved, 'original_account_groups': before['original_account_groups'], 'current_account_groups': account_groups(), 'existing_customer_keys_changed': False})
    print(json.dumps({'selected_test_access_cleaned': True, 'temporary_group_id': gid}), flush=True)
