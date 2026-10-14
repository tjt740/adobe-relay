"""Restore this release's two code images, never an old business database."""
import argparse
import fcntl
import hashlib
import json
from pathlib import Path
import re
import sqlite3
import subprocess
import sys

ROOT = Path('/opt/adobe-relay/releases/okad-company-mobile-mail-20261014')
RID = ROOT.name
SHA = '58638a1bb1d5b95e3ecb578be323ad556dfc932b'
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--check', action='store_true', help='Validate guards without changing code or data')
check_only = parser.parse_args().check
expected = json.loads((ROOT/'after.json').read_text())
handle = open('/opt/adobe-relay/.release.lock','a')
fcntl.flock(handle,fcntl.LOCK_EX|fcntl.LOCK_NB)

def inspect(name):
    return json.loads(subprocess.check_output(['docker','inspect',name]))[0]

def digest(raw):
    return hashlib.sha256(raw).hexdigest()

current = inspect('adobe-relay-app-1')
assert current['Config']['Image']=='adobe-relay:'+SHA, 'Later Sub2 version detected'
assert current['Image']==expected['containers']['adobe-relay-app-1']['image_id'], 'Sub2 image changed'
assert inspect('adobe-relay:before-'+RID)['Id']==json.loads((ROOT/'before.json').read_text())['containers']['adobe-relay-app-1']['image_id']
for name, value in expected['config_hashes'].items():
    assert digest(Path(name).read_bytes())==value, 'Later config changes detected: '+name
with sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True) as db:
    settings=db.execute('SELECT key,value FROM settings ORDER BY key').fetchall()
    assert digest(json.dumps(settings,ensure_ascii=False).encode())==expected['okad_settings_hash'], 'Okad settings changed'
query="""BEGIN READ ONLY;
SELECT json_build_object('mail_hash',(SELECT md5(COALESCE(string_agg(key||':'||value,',' ORDER BY key),'')) FROM settings WHERE key LIKE 'smtp_%' OR key IN ('ops_adobe_alert_mail_private','ops_email_notification_config','email_verify_enabled','ops_alert_runtime_settings')),'rule',(SELECT row_to_json(r) FROM ops_alert_rules r WHERE id=9));
COMMIT;"""
state=json.loads(subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input=query,text=True,capture_output=True,check=True).stdout.strip())
assert state['mail_hash']==expected['mail_hash'], 'Email configuration changed'
assert all(state['rule'][k]==v for k,v in expected['rule'].items() if k not in ('updated_at','last_triggered_at')), 'Alert policy changed'
subprocess.run(['python3',str(ROOT/'company-okad-release.py'),'check-rollback'],check=True)
if check_only:
    print('Both rollback guards passed; no files, containers, settings or data changed')
else:
    subprocess.run(['python3',str(ROOT/'company-okad-release.py'),'rollback'],check=True)
    fcntl.flock(handle,fcntl.LOCK_UN)
    # Formal script takes its own lock and checks the current image atomically.
    subprocess.run(['bash','/opt/adobe-relay/deploy/primary/release.sh','adobe-relay:before-'+RID,'adobe-relay:'+SHA],check=True)
    print('Rolled back both code images; current accounts, metadata, credentials, proxies, SMTP and policy retained')
