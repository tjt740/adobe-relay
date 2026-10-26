"""Two-minute passive observation; no Adobe requests or test mail."""
import datetime
import importlib.util
import json
from pathlib import Path
import time
import urllib.request

ROOT = Path('/opt/adobe-relay/releases/adobe-default-test-flare-20261026')
spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)
samples = []
for index in range(13):
    app = a.inspect(a.APP)
    pid = app['State']['Pid']
    cgroup = next(line.split('::', 1)[1] for line in Path(f'/proc/{pid}/cgroup').read_text().splitlines() if line.startswith('0::'))
    base = Path('/sys/fs/cgroup') / cgroup.lstrip('/')
    events = dict(line.split() for line in (base / 'memory.events').read_text().splitlines())
    with urllib.request.urlopen('http://127.0.0.1:6666/health', timeout=10) as response:
        assert response.status == 200
    row = {
        'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'healthy': app['State']['Health']['Status'] == 'healthy',
        'restarts': app['RestartCount'], 'oom_killed': app['State']['OOMKilled'],
        'memory_current': int((base / 'memory.current').read_text()), 'memory_events': events,
    }
    samples.append(row)
    print(json.dumps(row), flush=True)
    assert row['healthy'] and not row['oom_killed'] and row['restarts'] == 0
    assert int(events.get('oom', '0')) == 0 and int(events.get('oom_kill', '0')) == 0
    if index < 12:
        time.sleep(10)
heartbeat = json.loads(a.sql("SELECT json_build_object('at',last_success_at,'healthy',last_success_at>now()-interval '5 minutes' AND (last_error_at IS NULL OR last_success_at>last_error_at)) FROM ops_job_heartbeats WHERE job_name='ops_alert_evaluator'"))
started = datetime.datetime.fromisoformat(a.inspect(a.APP)['State']['StartedAt'].replace('Z', '+00:00'))
assert heartbeat['healthy'] and datetime.datetime.fromisoformat(heartbeat['at']) >= started
out = {'samples': samples, 'alert_heartbeat_from_new_process': heartbeat, 'adobe_test_requests': False, 'sent_test_email': False}
a.save('observation.json', out)
print(json.dumps({'observation_passed': True, 'sample_count': len(samples), 'peak_sample_bytes': max(row['memory_current'] for row in samples), 'heartbeat': heartbeat}), flush=True)
