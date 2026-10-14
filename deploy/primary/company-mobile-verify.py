"""Read-only production checks: no emails, Adobe requests, or account mutations."""
import collections
import datetime
import hashlib
import json
from pathlib import Path
import re
import sqlite3
import subprocess
import urllib.parse
import urllib.request

ROOT = Path('/opt/adobe-relay/releases/okad-company-mobile-mail-20261014')
SHA = '58638a1bb1d5b95e3ecb578be323ad556dfc932b'

def sql(query):
    return subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'], input="BEGIN READ ONLY; SET LOCAL statement_timeout='10s'; " + query + '; COMMIT;', text=True, capture_output=True, check=True, timeout=25).stdout.strip()

key = sql("SELECT value FROM settings WHERE key='admin_api_key'")
assert key

def request(path):
    req = urllib.request.Request('http://127.0.0.1:6666'+path, headers={'x-api-key':key})
    with urllib.request.urlopen(req, timeout=40) as response:
        return json.load(response)

app = json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
assert app['Config']['Image'] == 'adobe-relay:'+SHA
out = {'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sha':SHA}
for url in ('https://creative.cheap/health','https://creative.cheap/okad/api/health'):
    with urllib.request.urlopen(url, timeout=30) as response:
        assert response.status == 200
out['public_health'] = 'ok'
report = request('/api/v1/admin/ops/adobe-health')['data']
for field, value in {'threshold':20,'minimum':5,'quiet_notifications':True,'recovery_threshold':10,'confirmation_minutes':4,'notification_cooldown_minutes':60}.items():
    assert report[field] == value, field
out['report'] = {key:report[key] for key in ('total','pending','recent_pending','threshold','recovery_threshold') if key in report}
req = urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/runtime-events',headers={'x-api-key':key})
with urllib.request.urlopen(req,timeout=10) as response:
    assert response.headers['Content-Type'].startswith('text/event-stream')
    assert b'"change"' in response.readline()
out['runtime_events'] = 'ok'
heartbeat = json.loads(sql("SELECT json_build_object('last_success_at',last_success_at,'healthy',last_success_at>now()-interval '5 minutes' AND (last_error_at IS NULL OR last_success_at>last_error_at)) FROM ops_job_heartbeats WHERE job_name='ops_alert_evaluator'"))
assert heartbeat['healthy']
out['alert_heartbeat'] = heartbeat
rows = json.loads(sql("SELECT COALESCE(json_agg(json_build_object('id',id,'platform',platform,'type',type,'email',credentials->>'email','extra',extra)),'[]') FROM accounts WHERE deleted_at IS NULL"))
by_id = {a['id']:a for a in rows}

def kind(a):
    extra = a['extra'] or {}
    owner = extra.get('okad') or {}
    marker = (extra.get('adobeteam_external_email') or '').strip().lower()
    if a['platform'] != 'adobe' or a['type'] != 'oauth' or (not owner.get('issuer') and not marker): return ''
    meta = extra.get('okad_account') or {}
    email = (a['email'] or '').strip().lower()
    if not meta.get('issuer') or not re.fullmatch(r'(external|pool):[1-9][0-9]*',meta.get('source_key','')) or type(meta.get('is_company')) is not bool or meta.get('email','').lower() != (email or marker) or (email and marker and email != marker) or (meta['is_company'] and not meta['source_key'].startswith('external:')) or (owner.get('issuer') and (owner['issuer'] != meta['issuer'] or owner.get('source_key') != meta['source_key'])): return 'unknown'
    return 'company' if meta['is_company'] else 'non_company'

expected = {k:{a['id'] for a in rows if kind(a)==k} for k in ('company','non_company','unknown')}
with sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True) as db:
    issuer = db.execute("SELECT value FROM settings WHERE key='okad_instance_id'").fetchone()[0]
    sources = {}
    for mid,email,mode,url,refresh,client in db.execute('SELECT id,email,otp_mode,mail_url,refresh_token,client_id FROM external_members'):
        sources['external:'+str(mid)] = (email.strip().lower(), mode!='manual' and not url and not(refresh and client))
    for mid,email in db.execute('SELECT id,email FROM adobe_members WHERE is_admin=0'):
        sources['pool:'+str(mid)] = (email.strip().lower(), False)
    duplicates = collections.Counter(email for email,_ in sources.values())
for k in ('company','non_company'):
    for aid in expected[k]:
        meta = by_id[aid]['extra']['okad_account']
        assert meta['issuer']==issuer and sources[meta['source_key']]==(meta['email'],meta['is_company']), 'Classification differs from Okad'
# The unknown bucket is retained for genuinely missing or ambiguous source identities.
unknown_reasons = collections.Counter()
for aid in expected['unknown']:
    account = by_id[aid]
    email = (account['email'] or account['extra'].get('adobeteam_external_email') or '').strip().lower()
    unknown_reasons['missing_source' if duplicates[email]==0 else 'ambiguous_source' if duplicates[email]>1 else 'pending_or_mismatched_identity'] += 1
out['classification_counts'] = {k:len(v) for k,v in expected.items()}
out['unknown_reasons'] = dict(unknown_reasons)
for k, ids in expected.items():
    found = set(); page=1
    while True:
        data = request('/api/v1/admin/accounts?'+urllib.parse.urlencode({'okad_account_type':k,'page':page,'page_size':50,'lite':'true'}))['data']
        assert data['total']==len(ids), (k,data['total'],len(ids))
        batch = {item['id'] for item in data['items']}
        assert not found.intersection(batch)
        found.update(batch)
        if len(found)>=data['total']: break
        assert batch
        page += 1
        assert page<100
    assert found==ids
out['filter_pagination'] = 'ok'
# Verify the deployed binary contains the exact mobile email template, without sending mail.
marker = '@media screen and (min-width:760px)'
subprocess.run(['docker','exec','adobe-relay-app-1','grep','-aFq',marker,'/app/sub2api'],check=True)
subprocess.run(['docker','exec','adobe-relay-app-1','grep','-aFq','Stacked records are the fallback','/app/sub2api'],check=True)
out['deployed_mobile_email_template'] = True
out['verification_sent_mail'] = False
out['verification_called_adobe'] = False
path = ROOT / 'runtime-verification.json'
path.write_text(json.dumps(out,ensure_ascii=False,indent=2));path.chmod(0o600)
print(json.dumps(out,ensure_ascii=False))
