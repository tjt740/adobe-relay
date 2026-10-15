"""Guard this application repair and the explicitly approved 8 GiB memory change.

Code rollback keeps the expanded memory limit and current business data.
"""
import copy
import fcntl
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

path = Path(__file__).with_name('image-response-release.py')
spec = importlib.util.spec_from_file_location('image_response_release', path)
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)
release.RID = 'adobe-oom-20261015'
release.ROOT = Path('/opt/adobe-relay/releases') / release.RID
release.OLD_IMAGE = 'adobe-relay:c811477d654651142d15a3721e6c2a45ab6506ed'
release.ROLLBACK_IMAGE = 'adobe-relay:before-' + release.RID
RESOURCE = Path('/opt/adobe-relay/shared/resources.yml')
RESOURCE_BACKUP = release.ROOT / 'resources-before.yml'
_original_snapshot = release.snapshot
_original_check = release.check_preserved


def expanded_resources(raw):
    old = b'  app:\n    mem_limit: 2g\n    environment:\n      GOMEMLIMIT: 1536MiB\n'
    new = b'  app:\n    mem_limit: 8g\n    environment:\n      GOMEMLIMIT: 6144MiB\n'
    assert raw.count(old) == 1, 'Unexpected resource configuration; review instead of overwriting'
    return raw.replace(old, new, 1)


def runtime_memory():
    d = release.inspect(release.APP)
    return {'memory': d['HostConfig']['Memory'], 'memory_swap': d['HostConfig']['MemorySwap'],
            'go_memory_limit': next(v.split('=', 1)[1] for v in d['Config']['Env'] if v.startswith('GOMEMLIMIT=')),
            'other_environment_hash': release.digest(json.dumps(sorted(v for v in d['Config']['Env'] if not v.startswith('GOMEMLIMIT='))).encode())}


def snapshot():
    out = _original_snapshot()
    out['app_memory'] = runtime_memory()
    return out


def check_preserved(before, current):
    original = RESOURCE_BACKUP.read_bytes()
    assert release.digest(original) == before['config_hashes'][str(RESOURCE)], 'Resource backup changed'
    assert current['config_hashes'][str(RESOURCE)] == release.digest(expanded_resources(original)), 'Unreviewed resource change'
    approved = copy.deepcopy(current)
    approved['config_hashes'][str(RESOURCE)] = before['config_hashes'][str(RESOURCE)]
    review_path = release.ROOT / 'reviewed-concurrent-setting.json'
    if review_path.exists():
        # A separate successful settings PUT at 17:00:46 changed this one UI
        # preference after cutover. Preserve it; do not normalize any other key
        # or accept another later value when checking rollback.
        review = json.loads(review_path.read_text())
        key = review['key']
        assert key == 'channel_monitor_hide_user_ranking'
        assert review['request']['method'] == 'PUT' and review['request']['path'] == '/api/v1/admin/settings' and review['request']['status'] == 200
        assert review['before_hash'] == '68934a3e9455fa72420237eb05902327'
        assert review['after_hash'] == 'b326b5062b2f0e69046810717534cb09'
        assert before['settings_hashes'].get(key) == review['before_hash']
        assert current['settings_hashes'].get(key) == review['after_hash'], 'Reviewed UI preference changed again'
        approved['settings_hashes'][key] = review['before_hash']
    checks = _original_check(before, approved)
    if review_path.exists():
        checks['reviewed_concurrent_ui_setting_preserved'] = True
    memory = current['app_memory']
    assert memory['memory'] == 8 * 1024**3 and memory['go_memory_limit'] == '6144MiB', 'Approved memory limits not active'
    assert memory['memory_swap'] == 16 * 1024**3, 'Derived memory/swap limit changed'
    assert memory['other_environment_hash'] == before['app_memory']['other_environment_hash'], 'Other runtime environment changed'
    checks['approved_memory_expansion_only'] = True
    return checks


release.snapshot = snapshot
release.check_preserved = check_preserved


def prepare_memory():
    locks = []
    for name in ['/opt/adobe-relay/.release.lock', '/opt/adobe-team-cookie/.release.lock', '/opt/adobe-team-cookie/.incremental-release.lock']:
        lock = open(name, 'a')
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        locks.append(lock)
    before = json.loads((release.ROOT / 'before.json').read_text())
    original = RESOURCE_BACKUP.read_bytes()
    assert release.digest(original) == before['config_hashes'][str(RESOURCE)]
    current = snapshot()
    assert current['containers'][release.APP]['image'] == release.OLD_IMAGE, 'Production changed before memory preparation'
    assert RESOURCE.read_bytes() == original, 'Resources changed since preflight'
    _original_check(before, current)
    # Compose validation is read-only. Compare the full resolved configuration,
    # allowing exactly the two fields approved by the user.
    resolved_before = json.loads(subprocess.check_output(['adobe-relay', 'config', '--format', 'json']))
    updated = expanded_resources(original)
    temporary = RESOURCE.with_name('resources.yml.oom-prepared')
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, RESOURCE.stat().st_mode & 0o777)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(updated)
    os.replace(temporary, RESOURCE)
    try:
        resolved_after = json.loads(subprocess.check_output(['adobe-relay', 'config', '--format', 'json']))
        expected = copy.deepcopy(resolved_before)
        # Compose serializes mem_limit as a decimal string on the primary;
        # retain the existing JSON type instead of weakening full comparison.
        memory_type = type(expected['services']['app']['mem_limit'])
        assert memory_type in (str, int)
        assert int(expected['services']['app']['mem_limit']) == 2 * 1024**3
        expected['services']['app']['mem_limit'] = memory_type(8 * 1024**3)
        expected['services']['app']['environment']['GOMEMLIMIT'] = '6144MiB'
        assert resolved_after == expected, 'Resolved Compose changed beyond the approved memory fields'
    except BaseException:
        RESOURCE.write_bytes(original)
        raise
    release.save('memory-prepared.json', {'before_sha256': release.digest(original), 'after_sha256': release.digest(updated),
                 'approved_changes': {'app.mem_limit': ['2g', '8g'], 'app.environment.GOMEMLIMIT': ['1536MiB', '6144MiB']},
                 'compose_validation': True, 'container_recreated': False})
    print('Memory configuration prepared and validated; the formal release will activate it. Other Compose settings unchanged.')


if __name__ == '__main__':
    if sys.argv[1:] == ['prepare-memory']:
        prepare_memory()
    else:
        if sys.argv[1:] == ['before']:
            release.ROOT.mkdir(parents=True, exist_ok=True, mode=0o700)
            original = RESOURCE.read_bytes()
            expanded_resources(original)
            if RESOURCE_BACKUP.exists():
                assert RESOURCE_BACKUP.read_bytes() == original, 'Existing resource backup differs'
            else:
                fd = os.open(RESOURCE_BACKUP, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
                with os.fdopen(fd, 'wb') as stream:
                    stream.write(original)
        release.main()
