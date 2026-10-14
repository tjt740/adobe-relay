"""Finish the approved release using the same guarded, version-pinned helpers."""
import hashlib
import json
from pathlib import Path
import subprocess
import time

root = Path('/opt/adobe-relay/releases/okad-account-errors-20261014')
assert (root / 'expected-sha').read_text().strip() == 'a2b46f5eac2bd04ddf6dc2d08843ec4843a086ad'
assert (root / 'expected-okad-sha').read_text().strip() == 'c6e00d968d521d2b157eb0d3470a78faabd0e64d'


def run(name, *args):
    subprocess.run(['python3', str(root / name), *args], check=True)


if not (root / 'okad-deployed.json').exists():
    run('account-errors-release.py', 'after-sub2')
    if not (root / 'okad-prepared.json').exists():
        manifest = root / 'okad-stage/manifest.json'
        data = json.loads(manifest.read_text())
        for name, item in data['files'].items():
            host = Path('/opt/adobe-team-cookie/payload') / name
            item['host_before'] = hashlib.sha256(host.read_bytes()).hexdigest() if host.is_file() else None
        manifest.write_text(json.dumps(data, indent=2))
        manifest.chmod(0o600)
        run('account-errors-okad-release.py', 'prepare')
    deadline = time.monotonic() + 2700
    while True:
        result = subprocess.run(['python3', str(root / 'account-errors-okad-release.py'), 'deploy'], text=True, capture_output=True)
        if result.returncode == 0:
            print(result.stdout, flush=True)
            break
        # Retry only the explicit busy-job rejection, which occurs before mutation.
        if 'AssertionError: Wait for current login/delivery tasks:' not in result.stderr:
            print(result.stdout, flush=True)
            raise RuntimeError(result.stderr)
        assert time.monotonic() < deadline, 'Safe release window did not open; current app retained'
        print('Waiting for existing login and delivery work; no task interrupted.', flush=True)
        time.sleep(30)

run('account-errors-release.py', 'after')
run('account-errors-release.py', 'verify')
if not (root / 'health-filter-verification.json').exists():
    run('account-errors-verify.py')
run('account-errors-business-verify.py')
run('account-errors-release.py', 'check-rollback')
