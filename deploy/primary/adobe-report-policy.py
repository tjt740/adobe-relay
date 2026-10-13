"""Apply the user-approved quiet policy after deployment, without sending mail."""
import datetime, fcntl, json, pathlib, subprocess, sys, urllib.request
root = pathlib.Path('/opt/adobe-relay/releases/adobe-report-notifications-20261014')
sha = sys.argv[1]
assert len(sha) == 40
locks=[]
for path in ['/opt/adobe-relay/.release.lock', '/opt/adobe-team-cookie/.release.lock']:
    lock=open(path,'a'); fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB); locks.append(lock)
app=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
assert app['Config']['Image']=='adobe-relay:'+sha and app['State']['Health']['Status']=='healthy'
def sql(query):
    return subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input=query,text=True,capture_output=True,check=True).stdout.strip()
before=json.loads((root/'before.json').read_text())
rule=json.loads(sql("SELECT row_to_json(r) FROM ops_alert_rules r WHERE id=9"))
expected=dict(before['rule']['filters'],adobe_confirmed_notifications=True)
assert all(rule[k]==v for k,v in before['rule'].items() if k not in ('filters','sustained_minutes','cooldown_minutes','updated_at','last_triggered_at'))
assert rule['filters'] in (before['rule']['filters'],expected)
assert rule['sustained_minutes'] in (before['rule']['sustained_minutes'],4)
assert rule['cooldown_minutes'] in (before['rule']['cooldown_minutes'],60)
# Take a DB advisory lock also used by email delivery to serialize policy writes.
sql("BEGIN; SELECT pg_advisory_xact_lock(731946,9); UPDATE ops_alert_rules SET filters=jsonb_set(COALESCE(filters,'{}'::jsonb),'{adobe_confirmed_notifications}','true'::jsonb),sustained_minutes=4,cooldown_minutes=60,updated_at=NOW() WHERE id=9; COMMIT;")
key=sql("SELECT value FROM settings WHERE key='admin_api_key'")
req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/ops/adobe-health',headers={'x-api-key':key})
with urllib.request.urlopen(req,timeout=40) as response: report=json.load(response)['data']
assert report['threshold']==20 and report['minimum']==5 and report['quiet_notifications']
assert report['recovery_threshold']==10 and report['confirmation_minutes']==4 and report['notification_cooldown_minutes']==60
assert len(report['window_accounts'])==report['historical']
for a in report['window_accounts']:
    assert a['count_401']+a['count_408']==a['business_errors']+a['test_errors']
assert 0<=report['recent_pending']<=report['pending']<=report['total']
assert report['unknown']>=0
with urllib.request.urlopen('https://creative.cheap/health',timeout=30) as response: assert response.status==200
with urllib.request.urlopen('https://creative.cheap/okad/api/health',timeout=30) as response: assert response.status==200
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'report':{k:v for k,v in report.items() if k not in ('accounts','window_accounts')},'window_rows':len(report['window_accounts']),'evidence_rows':len(report['accounts']),'mail_sent_by_verification':False}
(root/'policy-verification.json').write_text(json.dumps(out,ensure_ascii=False,indent=2))
print(json.dumps(out,ensure_ascii=False))
