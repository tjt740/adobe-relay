#!/usr/bin/env python3
"""Read-only host sampling; no restarts, reloads, request interception or mail."""
import datetime, fcntl, json, os, pathlib, subprocess

OUTPUT = pathlib.Path('/opt/adobe-relay/shared/data/mihomo-runtime.json')
STATE = pathlib.Path('/var/lib/adobe-proxy-observer/state.json')
NAME = 'adobe-relay-mihomo-1'

def atomic_write(path, value, mode):
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix('.tmp')
    with tmp.open('w') as f:
        os.chmod(tmp, mode)
        json.dump(value, f, separators=(',', ':'))
        f.write('\n')
    os.replace(tmp, path)

def next_snapshot(current, previous, at):
    out = dict(current, schema_version=1, observed_at=at, sample_ok=True)
    for key in ('last_oom_at', 'last_restart_at'):
        if previous.get(key):
            out[key] = previous[key]
    if previous.get('sample_ok'):
        same = current['container_id'] == previous.get('container_id')
        if same and (current['oom_kills'] > previous.get('oom_kills', current['oom_kills']) or current['oom_events'] > previous.get('oom_events', current['oom_events'])):
            out['last_oom_at'] = at
        if not same or current['restart_count'] > previous.get('restart_count', current['restart_count']) or current['started_at'] != previous.get('started_at', current['started_at']):
            out['last_restart_at'] = at
    return out

def collect():
    cp = subprocess.run(['docker', 'inspect', NAME], capture_output=True, text=True, timeout=8, check=True)
    d = json.loads(cp.stdout)[0]
    assert d['Name'] == '/' + NAME
    pid = int(d['State']['Pid'])
    assert d['State']['Running'] and pid > 0
    rel = next(x.split('::', 1)[1] for x in pathlib.Path(f'/proc/{pid}/cgroup').read_text().splitlines() if x.startswith('0::'))
    cg = pathlib.Path('/sys/fs/cgroup') / rel.lstrip('/')
    vals = lambda name: {k: int(v) for k, v in (x.split() for x in (cg/name).read_text().splitlines())}
    events, stat = vals('memory.events'), vals('memory.stat')
    limit = int((cg/'memory.max').read_text()); assert limit > 0
    return {'container_id': d['Id'], 'running': True, 'started_at': d['State']['StartedAt'], 'restart_count': d['RestartCount'],
            'memory_current_bytes': int((cg/'memory.current').read_text()), 'memory_limit_bytes': limit,
            'memory_peak_bytes': int((cg/'memory.peak').read_text()), 'socket_bytes': stat.get('sock', 0), 'kernel_bytes': stat.get('kernel', 0),
            'oom_events': events.get('oom', 0), 'oom_kills': events.get('oom_kill', 0)}

def main():
    STATE.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    with (STATE.parent/'observer.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        at = datetime.datetime.now(datetime.timezone.utc).isoformat()
        try:
            try:
                previous = json.loads(STATE.read_text()) if STATE.exists() else {}
            except (ValueError, OSError):
                previous = {}
            out = next_snapshot(collect(), previous, at)
            atomic_write(STATE, out, 0o600)
        except Exception:
            # No raw subprocess errors: Docker inspect can include environment secrets.
            out = {'schema_version': 1, 'observed_at': at, 'sample_ok': False, 'running': False, 'sample_error': 'collection_failed'}
        atomic_write(OUTPUT, out, 0o644)

if __name__ == '__main__':
    main()
