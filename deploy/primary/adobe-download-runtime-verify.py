"""Read-only evidence from natural image traffic after the download fallback release."""
import datetime
import json
from pathlib import Path
import subprocess

root = Path('/opt/adobe-relay/releases/adobe-download-fallback-20261015')
sha = (root / 'expected-sha').read_text().strip()
app = json.loads(subprocess.check_output(['docker', 'inspect', 'adobe-relay-app-1']))[0]
assert app['Config']['Image'] == 'adobe-relay:' + sha
created = app['Created']
logs = subprocess.run(['docker', 'logs', '--since', created, '--tail', '60000', 'adobe-relay-app-1'], capture_output=True, text=True, check=True, timeout=25)
successes = []
fallback_failures = []
for line in (logs.stdout + logs.stderr).splitlines():
    try:
        row = json.loads(line)
    except (ValueError, TypeError):
        # Production uses zap's console prefix followed by a JSON fields object.
        try:
            prefix, fields = line.split('\t{', 1)
            row = json.loads('{' + fields)
            row.setdefault('msg', prefix.rsplit('\t', 1)[-1])
            row.setdefault('time', prefix.split('\t', 1)[0])
        except (ValueError, TypeError):
            continue
    message = row.get('msg', row.get('message'))
    if message == 'adobe_images.pipeline_completed':
        timings = row.get('timings', {})
        images = timings.get('images', [])
        if any(image.get('download_fallback') for image in images):
            successes.append({key: row.get(key) for key in ['time', 'ts', 'request_id', 'client_request_id', 'generation_request_id', 'account_id', 'upstream_model', 'image_count', 'processing_ms']})
            successes[-1]['images'] = [{key: image.get(key) for key in ['submit_attempts', 'poll_count', 'download_attempts', 'download_fallback', 'download_ms']} for image in images]
    elif message == 'adobe_images.upstream_failed' and row.get('download_fallback'):
        fallback_failures.append({key: row.get(key) for key in ['request_id', 'account_id', 'stage', 'error_type', 'stage_attempts']})

def sql(query):
    r = subprocess.run(['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'psql', '-U', 'sub2api', '-d', 'sub2api', '-Atq', '-v', 'ON_ERROR_STOP=1'], input="BEGIN READ ONLY; SET LOCAL statement_timeout='15s'; " + query + '; COMMIT;', text=True, capture_output=True, check=True, timeout=25)
    return json.loads(r.stdout.strip())

# Usage prefers the client-supplied idempotency key; retain all observed IDs.
usage = sql("SELECT COALESCE(json_agg(json_build_object('request_id',request_id,'account_id',account_id,'image_count',image_count,'created_at',created_at)), '[]') FROM usage_logs WHERE created_at >= '" + created + "' AND image_count > 0")
by_request = {}
for record in usage:
    by_request.setdefault(record['request_id'], []).append(record)
for result in successes:
    keys = [result.get('generation_request_id'), result.get('request_id')]
    if result.get('client_request_id'):
        keys.insert(0, 'client:' + result['client_request_id'])
    if result.get('request_id'):
        keys.append('local:' + result['request_id'])
    result['usage_rows'] = next((by_request[key] for key in keys if key in by_request), [])

out = {
    'at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'application_sha': sha,
    'container_created': created,
    'healthy': app['State'].get('Health', {}).get('Status') == 'healthy',
    'restarts': app['RestartCount'],
    'memory_limit_bytes': app['HostConfig']['Memory'],
    'natural_fallback_success_count': len(successes),
    'fallback_successes_with_one_matching_usage': sum(len(x['usage_rows']) == 1 and x['usage_rows'][0]['image_count'] == x['image_count'] and x['usage_rows'][0]['account_id'] == x['account_id'] for x in successes),
    'natural_fallback_successes': successes[-20:],
    'natural_fallback_failure_count': len(fallback_failures),
    'natural_fallback_failures': fallback_failures[-10:],
    'adobe_requests_triggered_by_verification': 0,
}
name = 'download-runtime-' + datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '.json'
target = root / name
target.write_text(json.dumps(out, ensure_ascii=False, indent=2))
target.chmod(0o600)
print(json.dumps(out, ensure_ascii=False))
