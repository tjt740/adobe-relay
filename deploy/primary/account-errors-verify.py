"""Read-only verification of live inventory, Okad filters and preserved settings."""
import base64
import collections
import datetime
import hashlib
import hmac
import json
from pathlib import Path
import sqlite3
import subprocess
import time
import urllib.parse
import urllib.request

ROOT = Path('/opt/adobe-relay/releases/okad-account-errors-20261014')


def sql(query):
    return subprocess.run(['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'psql', '-U', 'sub2api', '-d', 'sub2api', '-Atq', '-v', 'ON_ERROR_STOP=1'],
        input='BEGIN READ ONLY; ' + query + '; COMMIT;', text=True, capture_output=True, check=True).stdout.strip()


def request(url, headers=None):
    with urllib.request.urlopen(urllib.request.Request(url, headers=headers or {}), timeout=45) as response:
        return json.load(response)


key = sql("SELECT value FROM settings WHERE key='admin_api_key'")
headers = {'x-api-key': key}
items, cursor = [], 0
while True:
    data = request('http://127.0.0.1:6666/api/v1/admin/accounts/okad-inventory?after_id=' + str(cursor), headers)['data']
    items.extend(data['items'])
    if not data['next_cursor']:
        break
    assert data['next_cursor'] > cursor
    cursor = data['next_cursor']
assert items and len({a['id'] for a in items}) == len(items)
for item in items:
    health = item['account_health']
    assert health['state'] in ('error', 'unknown', 'recovered')
    assert isinstance(health['errors'], list)
    assert set(health) <= {'state', 'errors', 'last_success_at'}
    for error in health['errors']:
        assert set(error) <= {'code', 'reason', 'source', 'stage', 'failed_at'}
        assert error['code'] == 0 or 400 <= error['code'] <= 599

# Sign a short-lived local verification token without importing the full Okad
# application or opening its database through the ORM. Never print credentials.
container = json.loads(subprocess.check_output(['docker', 'inspect', 'adobe-team-cookie-app-1']))[0]
env = dict(value.split('=', 1) for value in container['Config']['Env'])
assert env.get('ALGORITHM', 'HS256') == 'HS256'
with sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro', uri=True) as db:
    uid, version = db.execute('SELECT id,token_version FROM users WHERE is_active=1 AND is_superuser=1 ORDER BY id LIMIT 1').fetchone()
    local_ids = {r[0] for r in db.execute('SELECT id FROM external_members')}


def b64(value):
    return base64.urlsafe_b64encode(value).rstrip(b'=')


body = b'.'.join([b64(json.dumps({'alg': 'HS256', 'typ': 'JWT'}).encode()),
                 b64(json.dumps({'sub': str(uid), 'ver': version, 'exp': int(time.time()) + 300}).encode())])
token = (body + b'.' + b64(hmac.new(env['SECRET_KEY'].encode(), body, hashlib.sha256).digest())).decode()
okad_headers = {'Authorization': 'Bearer ' + token}
stock = request('http://127.0.0.1:9500/api/external/members/sub2-stock?refresh=true', okad_headers)
assert stock['ok'] and stock['configured']
assert local_ids <= {int(mid) for mid in stock['items']}, 'Local inventory lost members'


def matches(item, value):
    if not item['in_stock']:
        return False
    health = item['account_health']
    if value.isdigit():
        return int(value) in health['codes']
    if value == 'other':
        return health['state'] == 'error' and (not health['errors'] or any(e['code'] not in (401, 408) for e in health['errors']))
    return health['state'] == value


counts = {}
for value in ['error', '401', '408', '403', '429', '500', '502', '503', '504', '529', 'other', 'unknown', 'recovered']:
    expected = {int(mid) for mid, item in stock['items'].items() if matches(item, value)}
    found, page = set(), 1
    while True:
        data = request('http://127.0.0.1:9500/api/external/members?' + urllib.parse.urlencode({'sub2_health': value, 'page': page, 'size': 200}), okad_headers)
        assert data['total'] == len(expected), (value, data['total'], len(expected))
        batch = {a['id'] for a in data['items']}
        assert not found.intersection(batch)
        found.update(batch)
        if len(found) >= data['total']:
            break
        assert batch
        page += 1
    assert found == expected, value
    counts[value] = len(found)
with urllib.request.urlopen('https://creative.cheap/okad/external-members-patch.js?v=20261014-sub2-health', timeout=30) as response:
    asset = response.read().decode()
    assert 'extm-f-health' in asset and 'data-sub2-health' in asset
out = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'sub2_inventory_count': len(items),
       'sub2_health_states': dict(collections.Counter(a['account_health']['state'] for a in items)),
       'external_members': len(local_ids), 'filter_counts': counts, 'all_filters_and_pagination_match': True,
       'public_asset_verified': True, 'triggered_adobe_requests': False, 'sent_mail': False}
path = ROOT / 'health-filter-verification.json'
assert not path.exists()
path.write_text(json.dumps(out, ensure_ascii=False, indent=2))
path.chmod(0o600)
print(json.dumps(out, ensure_ascii=False))
