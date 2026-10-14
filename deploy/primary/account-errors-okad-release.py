"""Guarded account-status Okad overlay; retain all existing runtime code and business data."""
import datetime
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import sqlite3
import subprocess
import sys
import time
import urllib.request

RID = 'okad-account-errors-20261014'
ROOT = Path('/opt/adobe-relay/releases') / RID
APP = Path('/opt/adobe-team-cookie')
STAGE = ROOT / 'okad-stage'
BACKUP = ROOT / 'okad-pre-release'
mode = sys.argv[1]
assert mode in ('prepare', 'deploy', 'check-rollback', 'rollback')
locks = []
for name in ('/opt/adobe-relay/.release.lock', '.release.lock', '.incremental-release.lock', '.local/deploy-login-batches.lock'):
    path = APP / name
    path.parent.mkdir(parents=True, exist_ok=True)
    handle = path.open('a')
    fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
    locks.append(handle)


def run(args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)


def inspect(name):
    return json.loads(subprocess.check_output(['docker', 'inspect', name]))[0]


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None


def runtime_hashes():
    code = """import pathlib,hashlib,json
root=pathlib.Path('/app')
print(json.dumps({str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in root.rglob('*') if p.is_file() and '__pycache__' not in str(p) and '.venv' not in str(p) and p.suffix in ('.py','.js','.css','.html')}))
"""
    return json.loads(subprocess.run(['docker', 'exec', '-i', 'adobe-team-cookie-app-1', 'python', '-'], input=code, text=True, capture_output=True, check=True).stdout)


def config_hashes():
    return {name: digest(APP / name) for name in ('.env', 'compose.yml', 'compose.override.yml')}


def delivery_is_busy(state, lease, due, now):
    # Future durable retries survive restart and are not active HTTP work.
    # Drain current leases, due retries, and imminent first deliveries.
    return lease > now or (state == 'pending' and due <= now + 60) or (state == 'retry' and due <= now)


def require_idle():
    with sqlite3.connect('file:' + str(APP / 'data/app.db') + '?mode=ro', uri=True) as db:
        active = db.execute("SELECT id FROM job_records WHERE deleted=0 AND status IN ('running','pausing','paused','cancelling')").fetchall()
        active += db.execute("SELECT id FROM integration_recovery_jobs WHERE state IN ('queued','running')").fetchall()
        now = time.time()
        deliveries = db.execute('SELECT source_key,target,state,lease_until,next_at FROM integration_deliveries').fetchall()
        busy = [(source,state) for source,target,state,lease,due in deliveries if delivery_is_busy(state,lease,due,now)]
        pending = []
        for mid,raw in db.execute('SELECT id,sub2_links FROM external_members'):
            for target,link in json.loads(raw or '{}').items():
                if link.get('status') not in ('pending','syncing'): continue
                durable = [d for d in deliveries if d[0]=='external:'+str(mid) and d[1]==target]
                # A stopped/delayed durable outbox survives restart. Untracked legacy
                # work and live leases must finish; do not infer idleness from UI text.
                if not durable: pending.append(mid)
        assert not active and not busy and not pending, 'Wait for current login/delivery tasks: ' + repr((active,busy,pending))


def save(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2))
    path.chmod(0o600)


def set_image(image):
    path = APP / 'compose.yml'
    content, count = re.subn(r'(?m)^(    image:)\s*adobe-team-cookie:[^\s]+[ \t]*$', lambda m: m[1] + ' ' + image, path.read_text())
    assert count == 1
    path.write_text(content)


def switch(image):
    set_image(image)
    compose = ['docker', 'compose', '--project-directory', str(APP)]
    for path in record['compose_files']:
        compose += ['-f', path]
    # Graceful shutdown stops the outbox loop and lets any just-started work drain.
    run(compose + ['up', '-d', '--no-build', '--no-deps', '--timeout', '300', '--wait', '--wait-timeout', '180', 'app'])
    assert inspect('adobe-team-cookie-app-1')['Config']['Image'] == image
    with urllib.request.urlopen('http://127.0.0.1:9500/api/health', timeout=15) as response:
        assert json.load(response)['status'] == 'ok'


def restore_host(record):
    for name, hashes in record['files'].items():
        destination = APP / 'payload' / name
        if hashes['host_before'] is None:
            destination.unlink(missing_ok=True)
        else:
            shutil.copy2(BACKUP / 'host' / name, destination)


if mode == 'prepare':
    assert not (ROOT / 'okad-prepared.json').exists(), 'Preparation already exists; inspect before replacing'
    record = json.loads((STAGE / 'manifest.json').read_text())
    current = inspect('adobe-team-cookie-app-1')
    assert current['State']['Health']['Status'] == 'healthy'
    hashes = runtime_hashes()
    for name, item in record['files'].items():
        assert digest(STAGE / 'payload' / name) == item['after']
        assert hashes.get(name) == item['before'], 'Runtime changed: ' + name
        assert digest(APP / 'payload' / name) == item['host_before'], 'Host source changed: ' + name
    assert hashes == json.loads((ROOT / 'okad-cutover-before.json').read_text())['okad_source_hashes']
    rollback = 'adobe-team-cookie:before-' + RID
    run(['docker', 'commit', '--pause=false', 'adobe-team-cookie-app-1', rollback], stdout=subprocess.DEVNULL)
    BACKUP.mkdir(mode=0o700, exist_ok=False)
    for name in ('.env', 'compose.yml', 'compose.override.yml'):
        if (APP / name).is_file():
            shutil.copy2(APP / name, BACKUP / name)
    for name, item in record['files'].items():
        if item['host_before'] is not None:
            target = BACKUP / 'host' / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(APP / 'payload' / name, target)
    with sqlite3.connect('file:' + str(APP / 'data/app.db') + '?mode=ro', uri=True) as src, sqlite3.connect(str(BACKUP / 'app.db')) as dst:
        src.backup(dst)
        assert dst.execute('PRAGMA integrity_check').fetchone()[0] == 'ok'
    image = 'adobe-team-cookie:' + record['sha']
    (STAGE / 'Dockerfile').write_text('FROM ' + rollback + '\nLABEL org.opencontainers.image.revision="' + record['sha'] + '"\n' + ''.join('COPY payload/' + name + ' /app/' + name + '\n' for name in record['files']))
    run(['docker', 'build', '--network', 'none', '--pull=false', '-t', image, str(STAGE)])
    run(['docker', 'run', '--rm', '--network', 'none', '--entrypoint', 'python', image, '-m', 'py_compile'] + ['/app/' + name for name in record['files'] if name.endswith('.py')])
    compose_files = current['Config']['Labels']['com.docker.compose.project.config_files'].split(',')
    assert compose_files and all(Path(p).parent == APP and Path(p).name in ('compose.yml','compose.override.yml') for p in compose_files)
    record.update(compose_files=compose_files, container=current['Id'], source_image_id=current['Image'], image=image, image_id=inspect(image)['Id'], rollback=rollback, rollback_image_id=inspect(rollback)['Id'], source_hashes=hashes, config_hashes=config_hashes(), prepared_at=datetime.datetime.now(datetime.timezone.utc).isoformat())
    save(ROOT / 'okad-prepared.json', record)
    print(json.dumps({'prepared': image, 'rollback': rollback}))
    raise SystemExit(0)

record = json.loads((ROOT / 'okad-prepared.json').read_text())
current = inspect('adobe-team-cookie-app-1')
expected_after = record['source_hashes'] | {name: item['after'] for name, item in record['files'].items()}
if mode == 'deploy':
    assert current['Id'] == record['container'] and current['Image'] == record['source_image_id'], 'Another deployment occurred'
    assert runtime_hashes() == record['source_hashes'], 'Runtime code changed'
    assert config_hashes() == record['config_hashes'], 'Configuration changed'
    assert inspect(record['image'])['Id'] == record['image_id']
    for name, item in record['files'].items():
        assert digest(APP / 'payload' / name) == item['host_before']
    assert inspect('adobe-relay-app-1')['Config']['Image'] == 'adobe-relay:' + (ROOT / 'expected-sha').read_text().strip(), 'Deploy Sub2 protocol first'
    require_idle()
    # Capture a consistent SQLite backup immediately before the switch.
    with sqlite3.connect('file:' + str(APP / 'data/app.db') + '?mode=ro', uri=True) as src, sqlite3.connect(str(BACKUP / 'cutover.db')) as dst:
        src.backup(dst)
        assert dst.execute('PRAGMA integrity_check').fetchone()[0] == 'ok'
    try:
        switch(record['image'])
        assert runtime_hashes() == expected_after, 'Unexpected source changes'
        for name in record['files']:
            target = APP / 'payload' / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(STAGE / 'payload' / name, target)
        after = inspect('adobe-team-cookie-app-1')
        save(ROOT / 'okad-deployed.json', {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'container': after['Id'], 'image_id': after['Image'], 'config_hashes': config_hashes()})
        print(json.dumps({'deployed': record['image'], 'health': after['State']['Health']['Status']}))
    except BaseException:
        restore_host(record)
        switch(record['rollback'])
        raise
else:
    deployed = json.loads((ROOT / 'okad-deployed.json').read_text())
    assert current['Config']['Image'] == record['image'] and current['Image'] == record['image_id'], 'Later release detected; inspect before rollback'
    assert runtime_hashes() == expected_after, 'Later runtime changes detected'
    assert config_hashes() == deployed['config_hashes'], 'Later configuration changes detected'
    assert inspect(record['rollback'])['Id'] == record['rollback_image_id']
    for name, item in record['files'].items():
        assert digest(APP / 'payload' / name) == item['after'], 'Later host source changes detected'
    if mode == 'check-rollback':
        print('Okad rollback guard passed; no mutation performed')
    else:
        require_idle()
        try:
            switch(record['rollback'])
            restore_host(record)
            assert runtime_hashes() == record['source_hashes']
        except BaseException:
            switch(record['image'])
            for name in record['files']:
                shutil.copy2(STAGE / 'payload' / name, APP / 'payload' / name)
            raise
        print('Okad code restored; current database and configuration retained')
