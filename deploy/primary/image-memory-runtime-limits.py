"""Read only non-secret production memory/parallelism limits for isolated replay."""
import json
from pathlib import Path
import subprocess

app = json.loads(subprocess.check_output(['docker', 'inspect', 'adobe-relay-app-1']))[0]
pid = app['State']['Pid']
cgroup = Path('/sys/fs/cgroup') / (Path('/proc/%s/cgroup' % pid).read_text().strip().split(':')[-1].lstrip('/'))
print(json.dumps({
    'image': app['Config']['Image'],
    'memory_bytes': app['HostConfig']['Memory'],
    'pids_limit': app['HostConfig']['PidsLimit'],
    'pids_max': (cgroup / 'pids.max').read_text().strip(),
    'environment': {key: value for key, value in (s.split('=', 1) for s in app['Config']['Env'])
                    if key in ('GOMEMLIMIT', 'GOMAXPROCS')},
}))
