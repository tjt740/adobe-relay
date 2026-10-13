"""Roll back this report/policy release without restoring business data or SMTP."""
import fcntl, json, pathlib, subprocess
root=pathlib.Path('/opt/adobe-relay/releases/adobe-report-notifications-20261014')
before=json.loads((root/'before.json').read_text())
expected=(root/'expected-sha').read_text().strip()
lock=open('/opt/adobe-relay/.release.lock','a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
app=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
assert app['Config']['Image']=='adobe-relay:'+expected, 'Later release exists; review first'
def sql(query):
    return subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input=query,text=True,capture_output=True,check=True).stdout.strip()
rule=json.loads(sql("SELECT row_to_json(r) FROM ops_alert_rules r WHERE id=9"))
assert rule['filters']==dict(before['rule']['filters'],adobe_confirmed_notifications=True)
assert rule['sustained_minutes']==4 and rule['cooldown_minutes']==60
assert all(rule[k]==v for k,v in before['rule'].items() if k not in ('filters','sustained_minutes','cooldown_minutes','updated_at','last_triggered_at'))
image='adobe-relay:before-adobe-report-notifications-20261014'
saved=json.loads(subprocess.check_output(['docker','image','inspect',image]))[0]
assert saved['Id']==before['containers']['adobe-relay-app-1']['image_id']
# Only the policy fields changed by this release are restored. Keep all events,
# delivery receipts, health evidence and live account/proxy data.
filters=json.dumps(before['rule']['filters']).replace("'","''")
sql("UPDATE ops_alert_rules SET filters='"+filters+"'::jsonb,sustained_minutes="+str(before['rule']['sustained_minutes'])+",cooldown_minutes="+str(before['rule']['cooldown_minutes'])+",updated_at=NOW() WHERE id=9")
lock.close()
subprocess.run(['bash','/opt/adobe-relay/deploy/primary/release.sh',image,app['Config']['Image']],check=True)
print('Restored prior report code and alert policy; current business data, SMTP and Okad preserved.')
