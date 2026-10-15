"""Read-only natural-traffic observation; no Adobe calls, signals or forced GC."""
import argparse
import collections
import datetime
import json
from pathlib import Path
import re
import subprocess
import time

APP = 'adobe-relay-app-1'
UTC = datetime.timezone.utc

def inspect():
    return json.loads(subprocess.check_output(['docker', 'inspect', APP]))[0]

def sample(expected_id):
    app = inspect()
    assert app['Id'] == expected_id, 'Application changed during observation'
    pid = app['State']['Pid']
    cgroup = Path('/proc/%s/cgroup' % pid).read_text().strip().split(':')[-1]
    root = Path('/sys/fs/cgroup') / cgroup.lstrip('/')
    events = dict(line.split() for line in (root / 'memory.events').read_text().splitlines())
    meminfo = dict(re.findall(r'^(\w+):\s+(\d+)', Path('/proc/meminfo').read_text(), re.M))
    return {'at': datetime.datetime.now(UTC).isoformat(), 'current': int((root/'memory.current').read_text()),
            'peak': int((root/'memory.peak').read_text()), 'oom': int(events['oom']), 'oom_kill': int(events['oom_kill']),
            'restarts': app['RestartCount'], 'health': app['State'].get('Health', {}).get('Status'),
            'host_available': int(meminfo['MemAvailable'])*1024}

def traffic(start, end):
    text = subprocess.check_output(['tail', '-n', '200000', '/var/log/nginx/access.log'], text=True)
    counts = collections.Counter()
    for line in text.splitlines():
        m = re.search(r'\[([^\]]+)\] "(\w+) ([^ ]+) [^"]+" (\d{3}) ', line)
        if not m or ('/images/' not in m[3] and 'generateContent' not in m[3]):
            continue
        at = datetime.datetime.strptime(m[1], '%d/%b/%Y:%H:%M:%S %z')
        if start <= at < end:
            counts[m[4]] += 1
    output = subprocess.run(['docker', 'logs', '--since', start.isoformat(), '--until', end.isoformat(), APP],
                            text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, check=True).stdout
    logs = collections.Counter()
    samples = []
    for line in output.splitlines():
        parts = line.split('\t')
        if len(parts) < 5:
            continue
        message = parts[-2]
        if message.startswith(('image_memory.', 'adobe_images.')):
            logs[message] += 1
        if message == 'image_memory.capacity_rejected':
            try:
                fields = json.loads(parts[-1])
                samples.append({k: fields.get(k) for k in ('budget_bytes','reserved_bytes','requested_bytes','active_requests','runtime_bytes','phase')})
            except ValueError:
                pass
    return {'start': start.isoformat(), 'end': end.isoformat(), 'http_image_status': dict(counts),
            'application_messages': dict(logs), 'memory_rejection_samples': samples[-10:],
            'note': 'Memory rejection logs are rate-limited to once per 5 seconds; use HTTP counts for request totals.'}

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output')
    parser.add_argument('--seconds', type=int, default=600)
    parser.add_argument('--expected-sha')
    parser.add_argument('--baseline', action='store_true')
    args = parser.parse_args()
    target = Path(args.output)
    assert not target.exists(), 'Observation output already exists'
    app = inspect()
    if args.expected_sha:
        assert app['Config']['Image'] == 'adobe-relay:' + args.expected_sha
    start = datetime.datetime.now(UTC)
    if args.baseline:
        result = {'traffic': traffic(start-datetime.timedelta(seconds=args.seconds), start), 'sample': sample(app['Id'])}
    else:
        samples = []
        deadline = time.monotonic() + args.seconds
        while True:
            item = sample(app['Id'])
            samples.append(item)
            print(json.dumps(item), flush=True)
            if time.monotonic() >= deadline:
                break
            time.sleep(min(20, deadline-time.monotonic()))
        end = datetime.datetime.now(UTC)
        result = {'app_id': app['Id'], 'image': app['Config']['Image'], 'samples': samples,
                  'traffic': traffic(start, end), 'max_current': max(s['current'] for s in samples),
                  'min_host_available': min(s['host_available'] for s in samples),
                  'stable': all(s['health']=='healthy' and s['oom']==samples[0]['oom'] and
                                s['oom_kill']==samples[0]['oom_kill'] and s['restarts']==samples[0]['restarts'] for s in samples)}
    target.write_text(json.dumps(result, ensure_ascii=False, indent=2))
    target.chmod(0o600)
    print(json.dumps({k:v for k,v in result.items() if k != 'samples'}), flush=True)

if __name__ == '__main__':
    main()
