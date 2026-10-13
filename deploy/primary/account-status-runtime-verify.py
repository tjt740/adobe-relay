"""Read-only report, stream, account preservation, and notification verification."""
import datetime
import json
import hashlib
import re
import pathlib
import subprocess
import sys
import urllib.request

root = pathlib.Path('/opt/adobe-relay/releases/account-status-cookie-fix-20261014')
sha = sys.argv[1]
assert len(sha) == 40
app = json.loads(subprocess.check_output(['docker', 'inspect', 'adobe-relay-app-1']))[0]
assert app['Config']['Image'] == 'adobe-relay:' + sha, 'Production changed since the requested release'


def sql(query):
    return subprocess.run(['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'psql', '-U', 'sub2api', '-d', 'sub2api', '-Atq', '-v', 'ON_ERROR_STOP=1'], input="BEGIN READ ONLY; SET LOCAL statement_timeout='10s'; "+query+"; COMMIT;", text=True, capture_output=True, check=True, timeout=20).stdout.strip()


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
out = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'sha': sha, 'health': 'ok', 'sse': 'ok', 'report': {k: v for k, v in report.items() if k not in ('accounts', 'window_accounts')}}
assert int(sql("SELECT count(*) FROM accounts WHERE deleted_at IS NULL AND extra->>'mock_dataset'='account-filter-mocks-20261013'")) == 0
logs = subprocess.run(['docker', 'logs', '--since', '5m', 'adobe-relay-app-1'], text=True, capture_output=True)
out['telemetry_errors'] = (logs.stdout + logs.stderr).count('adobe_health_save_failed')
assert out['telemetry_errors'] == 0, 'New health evidence failed to persist'
# Compare with the actual cutover backup, not only the earlier preflight.
backup = pathlib.Path(json.loads((root/'after.json').read_text())['backup'])
with (backup/'database.dump').open('rb') as source:
    dump = subprocess.check_output(['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--data-only','--table=accounts','-f','-'],stdin=source,text=True)
def copy_value(value):
    if value == r'\N': return None
    escapes = {'b':'\b','f':'\f','n':'\n','r':'\r','t':'\t','v':'\v'}
    def decode(match):
        token = match.group(1)
        if token.startswith('x') and len(token)>1: return chr(int(token[1:],16))
        if token[0] in '01234567': return chr(int(token,8))
        return escapes.get(token,token)
    return re.sub(r'\\([0-7]{1,3}|x[0-9a-fA-F]{1,2}|.)',decode,value)

def credentials_digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()

prior = {}; prior_state = {}; fields = None
for line in dump.splitlines():
    if line.startswith('COPY public.accounts ('):
        fields = line.split('(',1)[1].split(')',1)[0].split(', '); continue
    if fields is None: continue
    if line == r'\.': fields = None; continue
    row = dict(zip(fields,line.split('\t')))
    if row.get('deleted_at') == r'\N':
        account_id = int(row['id'])
        prior[account_id] = None if row['proxy_id'] == r'\N' else int(row['proxy_id'])
        extra = json.loads(copy_value(row['extra']) or '{}')
        prior_state[account_id] = {'credentials_hash':credentials_digest(json.loads(copy_value(row['credentials']) or '{}')),
            'status':row['status'],'schedulable':row['schedulable']=='t','manual_pause':extra.get('okad_manual_pause')}

assert prior
current_rows = json.loads(sql("SELECT COALESCE(json_agg(json_build_object('id',id,'credentials',credentials,'status',status,'schedulable',schedulable,'manual_pause',extra->'okad_manual_pause')),'[]') FROM accounts WHERE deleted_at IS NULL"))
current_state = {a['id']:{'credentials_hash':credentials_digest(a['credentials'] or {}),'status':a['status'],'schedulable':a['schedulable'],'manual_pause':a['manual_pause']} for a in current_rows}
state_changes = {str(account):[field for field in old if old[field]!=current_state.get(account,{}).get(field)] for account,old in prior_state.items()}
state_changes = {account:fields for account,fields in state_changes.items() if fields}
out['account_state_changes_since_cutover'] = state_changes
# Runtime credential refresh or genuine request results can legitimately update a row.
# Persist only field names and hashes; investigate changes without restoring old data.
(root/'account-state-audit.json').write_text(json.dumps({'changes':state_changes,'before':prior_state,'after':current_state},indent=2))
(root/'account-state-audit.json').chmod(0o600)
current = {a['id']:a['proxy_id'] for a in json.loads(sql("SELECT COALESCE(json_agg(json_build_object('id',id,'proxy_id',proxy_id)),'[]') FROM accounts WHERE deleted_at IS NULL"))}
changed = [account for account in prior if account in current and prior[account] != current[account]]
cutover = datetime.datetime.strptime(backup.name.split('-')[0],'%Y%m%dT%H%M%SZ').replace(tzinfo=datetime.timezone.utc)
events = json.loads(sql("SELECT COALESCE(json_agg(row_to_json(e)),'[]') FROM (SELECT account_id,from_proxy_id,to_proxy_id,created_at FROM proxy_failover_events WHERE created_at>='"+(cutover-datetime.timedelta(seconds=3)).isoformat()+"' ORDER BY created_at,id) e"))
explained = []
for account in changed:
    value = prior[account]
    for event in events:
        if event['account_id'] == account and event['from_proxy_id'] == value: value = event['to_proxy_id']
    if value == current[account]: explained.append(account)
out['account_preservation'] = {'backup_count':len(prior),'current_count':len(current),'same_ids':set(prior)==set(current),'changed_proxies':changed,'explained_by_existing_failover':explained}
(root/'proxy-audit.json').write_text(json.dumps({'summary':out['account_preservation'],'events':events},indent=2))
assert set(prior)==set(current) and set(changed)==set(explained), 'Account changes require investigation'
before = json.loads((root/'before.json').read_text())['containers']['adobe-team-cookie-app-1']
okad = json.loads(subprocess.check_output(['docker','inspect','adobe-team-cookie-app-1']))[0]
oom = subprocess.run(['docker','events','--since',before['started'],'--until',datetime.datetime.now(datetime.timezone.utc).isoformat(),'--filter','container=adobe-team-cookie-app-1','--filter','event=oom','--format','{{json .}}'],capture_output=True,text=True,check=True)
oom_times = [json.loads(line)['time'] for line in oom.stdout.splitlines() if line.strip()]
out['okad_runtime'] = {'health':okad['State']['Health']['Status'],'restart_delta':okad['RestartCount']-before['restarts'],'oom_times':oom_times,'same_image':okad['Image']==before['image_id']}
assert out['okad_runtime']['health']=='healthy' and out['okad_runtime']['same_image']
with urllib.request.urlopen('https://creative.cheap/okad/api/health',timeout=30) as response:
    assert response.status==200
out['report_recipient'] = sql("SELECT value::jsonb->>'recipient' FROM settings WHERE key='ops_adobe_alert_mail_private'")
assert out['report_recipient']=='tjt740@gmail.com'

assert report['quiet_notifications'] and report['recovery_threshold'] == 10
assert report['confirmation_minutes'] == 4 and report['notification_cooldown_minutes'] == 60
assert len({a['account_id'] for a in report['window_accounts']}) == report['historical']
assert len({(a['id'], a['scope']) for a in report['accounts']}) == len(report['accounts'])
for a in report['window_accounts']:
    assert a['count_401'] == a['auth_401'] + a['quota_401'] + a['entitled_401'] + a['unknown_401']
    assert a['count_401'] + a['count_408'] == a['business_errors'] + a['test_errors']
heartbeat = json.loads(sql("SELECT json_build_object('last_success_at',last_success_at,'last_result',last_result,'healthy',last_success_at > now()-interval '5 minutes' AND (last_error_at IS NULL OR last_success_at>last_error_at)) FROM ops_job_heartbeats WHERE job_name='ops_alert_evaluator'"))
assert heartbeat['healthy'], 'Alert evaluator has no recent successful heartbeat'
out['alert_evaluator'] = heartbeat
out['notification_states'] = json.loads(sql("SELECT COALESCE(json_agg(json_build_object('id',id,'status',status,'email_sent',email_sent,'fired_at',fired_at,'resolved_at',resolved_at,'delivery_state',dimensions->>'adobe_email_state','last_attempt_at',dimensions->>'adobe_email_attempt_at') ORDER BY id),'[]') FROM ops_alert_events WHERE rule_id=9"))
out['verification_sent_mail'] = False
(root/'runtime-verification.json').write_text(json.dumps(out,ensure_ascii=False,indent=2))
print(json.dumps(out, ensure_ascii=False))
