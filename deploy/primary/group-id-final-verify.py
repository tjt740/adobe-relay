"""Read-only postflight for the group ID frontend release."""
import datetime
import fcntl
import importlib.util
import json
from pathlib import Path

root = Path('/opt/adobe-relay/releases/group-id-visible-20261016')
spec = importlib.util.spec_from_file_location('group_release', root / 'group-id-visible-release.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
release = module.release
locks = []
for name in ['/opt/adobe-relay/.release.lock', '/opt/adobe-team-cookie/.release.lock', '/opt/adobe-team-cookie/.incremental-release.lock']:
    lock = open(name, 'a')
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    locks.append(lock)
sha = (root / 'expected-sha').read_text().strip()
before = json.loads((root / 'before.json').read_text())
current = release.snapshot()
preserved = release.validate_rollback(before, current, sha, release.inspect(release.ROLLBACK_IMAGE)['Id'])
app = release.inspect(release.APP)
env = dict(value.split('=', 1) for value in app['Config']['Env'])
assert app['HostConfig']['Memory'] == 12 * 1024 ** 3
assert env['GOMEMLIMIT'] == '9216MiB' and env['GOMAXPROCS'] == '6'
groups = json.loads(release.sql("SELECT coalesce(json_agg(row_to_json(s)),'[]') FROM (SELECT g.id,g.name,count(a.id) AS accounts,count(a.id) FILTER (WHERE a.schedulable) AS schedulable FROM groups g LEFT JOIN account_groups ag ON ag.group_id=g.id LEFT JOIN accounts a ON a.id=ag.account_id AND a.deleted_at IS NULL WHERE g.id IN (2,7) GROUP BY g.id,g.name ORDER BY g.id) s"))
assert len(groups) == 2 and all(row['schedulable'] == 0 for row in groups)
started = current['containers'][release.APP]['started']
assert release.sql("SELECT (last_success_at>'" + started + "'::timestamptz)::int FROM ops_job_heartbeats WHERE job_name='ops_alert_evaluator'") == '1', 'Await post-start alert heartbeat'
result = {
    'at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'application_sha': sha,
    'app': current['containers'][release.APP],
    'preservation': preserved,
    'runtime': release.runtime_verification(sha),
    'memory': {'container_bytes': app['HostConfig']['Memory'], 'GOMEMLIMIT': env['GOMEMLIMIT'], 'GOMAXPROCS': env['GOMAXPROCS']},
    'paused_groups': groups,
}
release.save('final-verification.json', result)
print(json.dumps(result, ensure_ascii=False))
