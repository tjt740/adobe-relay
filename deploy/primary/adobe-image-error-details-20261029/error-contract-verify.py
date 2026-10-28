"""Exercise only pre-admission invalid input: never submit an Adobe generation."""
import datetime
import importlib.util
import json
from pathlib import Path
import time
import urllib.error
import urllib.request
import uuid

ROOT = Path('/opt/adobe-relay/releases/adobe-image-error-details-20261029')
spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)
assert (ROOT / 'rolling-finished.json').exists()
sha = (ROOT / 'expected-sha').read_text().strip()
assert a.inspect(a.APP)['Config']['Image'] == 'adobe-relay:' + sha
keys = json.loads(a.sql("SELECT coalesce(json_agg(json_build_object('id',k.id,'key',k.key,'group_id',k.group_id)),'[]') FROM api_keys k JOIN groups g ON g.id=k.group_id JOIN users u ON u.id=k.user_id WHERE k.deleted_at IS NULL AND k.status='active' AND g.deleted_at IS NULL AND g.status='active' AND g.platform='adobe' AND u.deleted_at IS NULL AND u.status='active' ORDER BY k.id"))
assert keys, 'No existing Adobe key for pre-admission error verification; do not create one'
proofs = []
for key in keys[:5]:
    request_id = 'image-error-release-' + uuid.uuid4().hex
    request = urllib.request.Request('https://creative.cheap/v1/images/generations',
        data=json.dumps({'model': 'gpt-image-2.5-sunburst', 'n': 0, 'prompt': 'invalid n release verification; must fail before admission'}).encode(),
        headers={'Authorization': 'Bearer ' + key['key'], 'Content-Type': 'application/json', 'X-Request-ID': request_id})
    try:
        response = urllib.request.urlopen(request, timeout=30)
    except urllib.error.HTTPError as error:
        response = error
    status = response.code
    payload = json.loads(response.read())
    if status in (401, 402, 403, 429):
        proofs.append({'api_key_id': key['id'], 'entry_denied_status': status})
        continue
    assert status == 400, 'Invalid n must be rejected before account selection'
    error = payload['error']
    assert error['code'] == 'invalid_n' and error['param'] == 'n'
    assert error['stage'] == 'validation' and error['source'] == 'request'
    assert error['request_id'] == response.headers['X-Request-ID'] == request_id
    assert 'upstream_status' not in error
    time.sleep(2)
    counts = json.loads(a.sql("SELECT json_build_object('usage_rows',(SELECT count(*) FROM usage_logs WHERE request_id='" + request_id + "'),'selected_accounts',(SELECT count(*) FROM ops_error_logs WHERE request_id='" + request_id + "' AND account_id IS NOT NULL))"))
    assert counts['usage_rows'] == counts['selected_accounts'] == 0
    out = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'application_sha': sha,
        'passed': True, 'api_key_id': key['id'], 'group_id': key['group_id'], 'http_status': status,
        'error': error, 'generation_requests': 0, 'usage_rows': 0, 'selected_accounts': 0,
        'entry_denials': proofs, 'credentials_in_output': False}
    a.save('error-contract-verification.json', out)
    print(json.dumps(out, ensure_ascii=False))
    break
else:
    raise AssertionError('Existing keys were denied before parsing; no account or configuration was changed')
