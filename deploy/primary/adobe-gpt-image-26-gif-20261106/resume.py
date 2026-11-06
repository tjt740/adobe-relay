"""Resume this exact interrupted old-side drain without rebuilding or replaying work."""
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

import rolling as r

ROOT = r.ROOT
SHA = 'da24f5634a0fd332f95a14d666fa2e9cea19db53'
OLD = 'adobe-relay:b97759edd35a80cc85dc868acce5e670a1f6efa0'


def validate_resume(progress, prepared, old, canary, owner, redirected):
    assert progress['step'] == 'failed' and progress['state'] == 'attention_required'
    assert "'drain-old'" in progress['error']
    assert old['Id'] == prepared['old_id'] and old['Config']['Image'] == OLD
    assert canary['Id'] == prepared['canary_id'] and canary['Config']['Image'] == 'adobe-relay:' + SHA
    assert prepared['candidate_sha'] == SHA and owner == r.RID and redirected
    for container in (old, canary):
        assert container['State']['Health']['Status'] == 'healthy'
        assert not container['State']['OOMKilled']


def main():
    os.umask(0o077)
    assert (ROOT / 'expected-sha').read_text().strip() == SHA
    orchestration = open('/opt/adobe-relay/shared/.adobe-deploy.lock', 'a')
    fcntl.flock(orchestration, fcntl.LOCK_EX | fcntl.LOCK_NB)
    lock = open(ROOT / 'driver.lock', 'a')
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    assert not (ROOT / 'after.json').exists() and not (ROOT / 'rolling-old-drained.json').exists()
    progress = r.load('progress.json')
    prepared = r.load('rolling-prepared.json')
    old, canary = r.inspect(r.APP), r.inspect(r.CANARY)
    validate_resume(progress, prepared, old, canary, r.redis('GET', r.LEASE), r.rule_present())
    error_log = (ROOT / 'driver.log').read_text()
    assert 'FATAL:  sorry, too many clients already' in error_log
    spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
    audit = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(audit)
    checks = audit.check_preserved(r.load('before.json'), audit.snapshot())
    for name in ['progress.json', 'rolling-old-drain-progress.json', 'driver.log']:
        original = ROOT / name
        archived = ROOT / ('initial-failed-' + name)
        assert not archived.exists()
        archived.write_bytes(original.read_bytes())
    r.save('resume-validated.json', {'at': r.now(), 'application_sha': SHA, 'same_canary_id': canary['Id'], 'same_old_id': old['Id'], 'original_log_sha256': hashlib.sha256(error_log.encode()).hexdigest(), 'preserved': checks, 'database_settings_changed': False, 'unknown_observations_never_count_as_zero': True})
    r.own_lease()
    heartbeat = subprocess.Popen([sys.executable, str(ROOT / 'rolling.py'), 'heartbeat'])
    spec = importlib.util.spec_from_file_location('driver', ROOT / 'driver.py')
    driver = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(driver)
    try:
        driver.run_step('drain-old', [sys.executable, str(ROOT / 'rolling.py'), 'drain-old'])
        driver.run_step('canonical-recreate', ['bash', '/opt/adobe-relay/deploy/primary/release.sh', 'adobe-relay:' + SHA, OLD, '--keep-canary-on-failure'], timeout=600)
        for step in ['route-canonical', 'drain-canary', 'cleanup']:
            driver.run_step(step, [sys.executable, str(ROOT / 'rolling.py'), step])
        driver.run_step('preservation-verification', [sys.executable, str(ROOT / 'audit.py'), 'after'], timeout=180)
        driver.run_step('runtime-verification', [sys.executable, str(ROOT / 'audit.py'), 'verify'], timeout=180)
        driver.progress('complete', 'done')
    except Exception as error:
        driver.progress('failed', 'attention_required', error=str(error)[:400])
        raise
    finally:
        if (ROOT / 'rolling-finished.json').exists():
            heartbeat.terminate()
            heartbeat.wait(timeout=10)


if __name__ == '__main__':
    main()
