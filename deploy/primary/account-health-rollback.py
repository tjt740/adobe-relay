"""Restore only application code; retain all current data and protected settings."""
import fcntl
import json
import pathlib
import subprocess

root = pathlib.Path('/opt/adobe-relay/releases/account-health-20261013')
before = json.loads((root / 'before.json').read_text())
expected = (root / 'expected-sha').read_text().strip()
lock = open('/opt/adobe-relay/.release.lock', 'a')
fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
app = json.loads(subprocess.check_output(['docker', 'inspect', 'adobe-relay-app-1']))[0]
assert app['Config']['Image'] == 'adobe-relay:' + expected, 'Later version present; review before rollback'
image = 'adobe-relay:before-account-health-20261013'
saved = json.loads(subprocess.check_output(['docker', 'image', 'inspect', image]))[0]
assert saved['Id'] == before['containers']['adobe-relay-app-1']['image_id']
lock.close()
subprocess.run(['bash', '/opt/adobe-relay/deploy/primary/release.sh', image, app['Config']['Image']], check=True)
print('Previous Sub2 code restored. Current data, email, 20% rule and Okad retained.')
