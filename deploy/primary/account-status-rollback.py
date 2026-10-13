"""Restore the previous application only; retain all live data and settings."""
import fcntl, hashlib, json, pathlib, re, subprocess
root = pathlib.Path('/opt/adobe-relay/releases/account-status-cookie-fix-20261014')
before = json.loads((root/'before.json').read_text())
expected = (root/'expected-sha').read_text().strip()
assert re.fullmatch('[0-9a-f]{40}', expected)
lock = open('/opt/adobe-relay/.release.lock', 'a')
fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
app = json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
assert app['Config']['Image'] == 'adobe-relay:'+expected, 'A later release exists; review it before rollback'
for path, digest in before['config_hashes'].items():
    if path == '/opt/adobe-relay/shared/.env': continue
    assert hashlib.sha256(pathlib.Path(path).read_bytes()).hexdigest() == digest, 'Configuration changed after preflight: '+path
value = '\n'.join(line for line in pathlib.Path('/opt/adobe-relay/shared/.env').read_text().splitlines() if not line.startswith('SUB2API_IMAGE='))
assert hashlib.sha256(value.encode()).hexdigest() == before['sub2_env_without_image'], 'Environment changed; review first'
image = 'adobe-relay:before-account-status-cookie-fix-20261014'
saved = json.loads(subprocess.check_output(['docker','image','inspect',image]))[0]
assert saved['Id'] == before['containers']['adobe-relay-app-1']['image_id'], 'Rollback image mismatch'
lock.close()
# release.sh reacquires the lock and atomically checks the expected current image.
subprocess.run(['bash','/opt/adobe-relay/deploy/primary/release.sh',image,app['Config']['Image']],check=True)
print('Prior Sub2 application restored; live accounts, proxies, evidence, SMTP, rules and Okad preserved.')
