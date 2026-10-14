"""Audit and roll back both applications without restoring business data."""
import argparse
import fcntl
import importlib.util
import json
from pathlib import Path
import re
import subprocess

spec = importlib.util.spec_from_file_location('release_audit', Path(__file__).with_name('image-response-release.py'))
base = importlib.util.module_from_spec(spec)
spec.loader.exec_module(base)
RID = 'okad-account-errors-20261014'
ROOT = Path('/opt/adobe-relay/releases') / RID
OLD_IMAGE = 'adobe-relay:020430320d0fbc31258ea21fcdfdd08d10101697'
OKAD = 'adobe-team-cookie-app-1'
COMPOSE = '/opt/adobe-team-cookie/compose.yml'
ROLLBACK = 'adobe-relay:before-' + RID


def snapshot():
    out = base.snapshot()
    out['okad_compose_without_image'] = base.digest(re.sub(
        r'(?m)^(    image:)\s*adobe-team-cookie:[^\s]+[ \t]*$', r'\1 <image>', Path(COMPOSE).read_text()).encode())
    override = Path('/opt/adobe-team-cookie/compose.override.yml')
    if override.exists():
        out['config_hashes'][str(override)] = base.digest(override.read_bytes())
    return out


def preserved(before, current):
    def configs(value):
        return {k: v for k, v in value['config_hashes'].items() if k != COMPOSE}
    checks = {
        'other_containers': all(current['containers'].get(k, {}).get('id') == v['id'] and
            current['containers'][k]['image_id'] == v['image_id'] for k, v in before['containers'].items()
            if k not in (base.APP, OKAD)),
        'configuration': configs(before) == configs(current),
        'sub2_environment': before['env_without_image_hash'] == current['env_without_image_hash'],
        'okad_compose': before['okad_compose_without_image'] == current['okad_compose_without_image'],
        'okad_settings': before['okad_settings_hash'] == current['okad_settings_hash'],
        'sub2_settings_including_smtp': before['settings_hashes'] == current['settings_hashes'],
        'alert_rules': before['rules_hash'] == current['rules_hash'],
    }
    assert all(checks.values()), 'Review concurrent changes; never restore old data: ' + json.dumps(checks)
    return checks


def validate_rollback(before, current, final, sha, rollback_id):
    assert re.fullmatch('[0-9a-f]{40}', sha)
    assert current['containers'][base.APP]['image'] == 'adobe-relay:' + sha, 'Later Sub2 version detected'
    assert current['containers'][base.APP]['image_id'] == final['containers'][base.APP]['image_id'], 'Sub2 image changed'
    assert current['containers'][OKAD]['image_id'] == final['containers'][OKAD]['image_id'], 'Later Okad version detected'
    assert rollback_id == before['containers'][base.APP]['image_id'], 'Rollback tag changed'
    return preserved(final, current)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['before', 'after-sub2', 'after', 'verify', 'check-rollback', 'rollback'])
    action = parser.parse_args().action
    ROOT.mkdir(parents=True, exist_ok=True, mode=0o700)
    base.ROOT = ROOT
    locks = []
    for path in ['/opt/adobe-relay/.release.lock', '/opt/adobe-team-cookie/.release.lock',
                 '/opt/adobe-team-cookie/.incremental-release.lock', '/opt/adobe-team-cookie/.local/deploy-login-batches.lock']:
        lock = open(path, 'a')
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        locks.append(lock)
    current = snapshot()
    if action == 'before':
        assert not (ROOT / 'before.json').exists()
        assert current['containers'][base.APP]['image'] == OLD_IMAGE, 'Production changed'
        assert current['containers'][base.APP]['health'] == 'healthy'
        assert not subprocess.check_output(['docker', 'image', 'ls', '-q', ROLLBACK]).strip(), 'Rollback tag exists'
        subprocess.run(['docker', 'tag', current['containers'][base.APP]['image_id'], ROLLBACK], check=True)
        base.save('before.json', current)
        print(json.dumps({'before_saved': True, 'accounts': len(current['account_state']), 'rollback_image': ROLLBACK}))
        return
    before = json.loads((ROOT / 'before.json').read_text())
    sha = (ROOT / 'expected-sha').read_text().strip()
    assert re.fullmatch('[0-9a-f]{40}', sha)
    assert current['containers'][base.APP]['image'] == 'adobe-relay:' + sha
    checks = preserved(before, current)
    if action in ('after-sub2', 'after'):
        assert current['containers'][base.APP]['health'] == 'healthy'
        assert current['containers'][OKAD]['health'] == 'healthy'
        if action == 'after-sub2':
            assert current['containers'][OKAD]['id'] == before['containers'][OKAD]['id']
        else:
            okad_sha = (ROOT / 'expected-okad-sha').read_text().strip()
            assert current['containers'][OKAD]['image'] == 'adobe-team-cookie:' + okad_sha
        backups = sorted(Path('/opt/adobe-relay/backups').glob('*-' + sha))
        assert backups and (backups[-1] / 'previous-image').read_text().strip() == OLD_IMAGE
        current.update(backup=str(backups[-1]), preserved=checks)
        base.save(action + '.json', current)
        print(json.dumps({'phase': action, 'preserved': checks, 'backup': current['backup']}))
        return
    if action == 'verify':
        value = base.runtime_verification(sha)
        base.save('runtime-verification.json', value)
        print(json.dumps(value))
        return
    final = json.loads((ROOT / 'after.json').read_text())
    validate_rollback(before, current, final, sha, base.inspect(ROLLBACK)['Id'])
    for lock in reversed(locks):
        lock.close()
    helper = str(ROOT / 'account-errors-okad-release.py')
    subprocess.run(['python3', helper, 'check-rollback'], check=True)
    if action == 'check-rollback':
        print(json.dumps({'both_rollback_guards_passed': True, 'mutated': False}))
        return
    # Each switch reacquires its locks and verifies its exact expected version.
    subprocess.run(['python3', helper, 'rollback'], check=True)
    subprocess.run(['bash', '/opt/adobe-relay/deploy/primary/release.sh', ROLLBACK, 'adobe-relay:' + sha], check=True)
    print('Both application images restored; current data, credentials, proxies, SMTP and rules retained.')


if __name__ == '__main__':
    main()
