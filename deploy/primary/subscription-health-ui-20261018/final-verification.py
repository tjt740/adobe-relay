"""Read-only final checks after the temporary instance is gone."""
import datetime
import importlib.util
import json
from pathlib import Path
import subprocess

ROOT = Path('/opt/adobe-relay/releases/subscription-health-ui-20261018')
spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)
before = json.loads((ROOT / 'before.json').read_text())
current = a.snapshot()
sha = (ROOT / 'expected-sha').read_text().strip()
assert current['containers'][a.APP]['image'] == 'adobe-relay:' + sha
assert current['containers'][a.APP]['image_id'] == json.loads((ROOT / 'release-metadata.json').read_text())['image_id']
checks = a.check_preserved(before, current)
assert current['containers'][a.APP]['restarts'] == 0
observation = json.loads((ROOT / 'observation.json').read_text())
assert len(observation['samples']) == 13
finished = json.loads((ROOT / 'rolling-finished.json').read_text())
assert all(finished[key] for key in ['temporary_container_removed', 'temporary_nat_rule_removed', 'rolling_lease_removed', 'nginx_configuration_unchanged'])
result = subprocess.run(['docker', 'inspect', 'adobe-relay-memory-canary'], capture_output=True)
assert result.returncode != 0
owner = subprocess.check_output(['docker', 'exec', 'adobe-relay-redis-1', 'redis-cli', '--raw', 'GET', 'concurrency:rolling_deployment'], text=True).strip()
assert not owner
nat = subprocess.check_output(['iptables', '-t', 'nat', '-S', 'OUTPUT'], text=True)
assert 'subscription-health-ui-20261018' not in nat
heartbeat = json.loads(a.sql("SELECT json_build_object('at',last_success_at,'healthy',last_success_at>now()-interval '5 minutes' AND (last_error_at IS NULL OR last_success_at>last_error_at)) FROM ops_job_heartbeats WHERE job_name='ops_alert_evaluator'"))
assert heartbeat['healthy']
assert datetime.datetime.fromisoformat(heartbeat['at']) > datetime.datetime.fromisoformat(finished['at'])
runtime = a.runtime_verification(sha)
out = {
    'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'application_sha': sha,
    'image_digest_verified': True, 'preserved': checks, 'cleanup_verified': True,
    'observation_samples': len(observation['samples']),
    'peak_sample_bytes': max(row['memory_current'] for row in observation['samples']),
    'canonical_process_heartbeat_after_canary_cleanup': heartbeat,
    'runtime': runtime,
}
a.save('final-verification.json', out)
print(json.dumps(out, ensure_ascii=False))
