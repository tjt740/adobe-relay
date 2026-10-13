"""Read-only production verification; send-report is an explicit opt-in action."""
import datetime
import json
import pathlib
import subprocess
import sys
import urllib.request

root = pathlib.Path('/opt/adobe-relay/releases/account-health-20261013')
sha, phase = sys.argv[1:3]
assert phase in ('after', 'send-report') and len(sha) == 40
app = json.loads(subprocess.check_output(['docker', 'inspect', 'adobe-relay-app-1']))[0]
assert app['Config']['Image'] == 'adobe-relay:' + sha, 'Production changed since the requested release'


def sql(query):
    return subprocess.run(['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'psql', '-U', 'sub2api', '-d', 'sub2api', '-Atq', '-v', 'ON_ERROR_STOP=1'], input=query, text=True, capture_output=True, check=True).stdout.strip()


key = sql("SELECT value FROM settings WHERE key='admin_api_key'")
assert key


def request(path, method='GET'):
    req = urllib.request.Request('http://127.0.0.1:6666' + path, headers={'x-api-key': key}, method=method)
    with urllib.request.urlopen(req, timeout=40) as response:
        return json.load(response)


report = request('/api/v1/admin/ops/adobe-health')['data']
assert report['threshold'] == 20 and report['minimum'] == 5
assert 0 <= report['pending'] <= report['total']
assert request('/health')['status'] == 'ok'
with urllib.request.urlopen('https://creative.cheap/health', timeout=30) as response:
    assert response.status == 200
req = urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/runtime-events', headers={'x-api-key': key})
with urllib.request.urlopen(req, timeout=10) as response:
    assert response.headers['Content-Type'].startswith('text/event-stream')
    assert b'"change"' in response.readline()
out = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'sha': sha, 'health': 'ok', 'sse': 'ok', 'report': {k: v for k, v in report.items() if k != 'accounts'}}
assert int(sql("SELECT count(*) FROM accounts WHERE deleted_at IS NULL AND extra->>'mock_dataset'='account-filter-mocks-20261013'")) == 0
out['telemetry_errors'] = subprocess.run(['docker', 'logs', '--since', '5m', 'adobe-relay-app-1'], text=True, capture_output=True).stderr.count('adobe_health_save_failed')
assert out['telemetry_errors'] == 0, 'New health evidence failed to persist'
(root / 'verification.json').write_text(json.dumps(out, ensure_ascii=False, indent=2))
if phase == 'send-report':
    marker = root / 'mail-sent.json'
    assert not marker.exists(), 'A report was already sent for this release'
    result = request('/api/v1/admin/ops/adobe-health/email', 'POST')['data']
    assert result.get('sent') is True
    marker.write_text(json.dumps({'at': out['at'], 'sha': sha, 'smtp_accepted': True}, indent=2))
    out['smtp_accepted'] = True
print(json.dumps(out, ensure_ascii=False))
