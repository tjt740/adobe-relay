"""Explain concurrent business updates using existing records only."""
import datetime
import json
from pathlib import Path
import re
import sqlite3
import subprocess

root = Path('/opt/adobe-relay/releases/okad-account-errors-20261014')
business = json.loads((root/'business-verification.json').read_text())
snapshot = json.loads((root/'account-state-audit.json').read_text())
ids = ','.join(business['account_state_changes_since_cutover']) or '0'
query = "SELECT COALESCE(json_agg(json_build_object('id',id,'status',status,'schedulable',schedulable,'proxy_id',proxy_id,'proxy_auto_paused',proxy_auto_paused,'allocation_wait',error_message='自动代理分配：等待可用节点（每个节点最多 3 个账号）','updated_at',updated_at,'has_access_token',credentials ? 'access_token','has_token_expiry',credentials ? 'expires_at','verification',extra->'okad'->>'verification','verified_at',extra->'okad'->>'verified_at')),'[]') FROM accounts WHERE id IN ("+ids+")"
raw = subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input='BEGIN READ ONLY; '+query+'; COMMIT;',text=True,capture_output=True,check=True).stdout
out = {'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'current_account_states':json.loads(raw)}
out['state_transitions']={mid:{phase:{key:snapshot[phase][mid][key] for key in ('status','schedulable','manual_pause')} for phase in ('before','after')} for mid,fields in business['account_state_changes_since_cutover'].items() if 'status' in fields}
logs = subprocess.run(['docker','logs','--since','2026-10-14T13:52:35Z','adobe-relay-app-1'],capture_output=True,text=True,check=True)
refreshes = []
for line in (logs.stdout+'\n'+logs.stderr).splitlines():
    kind=next((v for v in ('token_refresh.account_refreshed','okad_credential_verification_failed') if v in line),None)
    if not kind:continue
    try:
        entry=json.loads(line[line.index('{'):])
        account=entry.get('account_id')
    except (ValueError,KeyError):
        match=re.search(r'account_id[=:][\s"]*(\d+)',line)
        account=int(match[1]) if match else None
        entry={}
    if account is None:
        match=re.search(r'account_id["\s]*[=:][\s"]*(\d+)',line)
        account=int(match[1]) if match else None
    if str(account) in business['changed_credential_fields']:
        refreshes.append({'account_id':account,'event':kind,'stage':entry.get('stage'),'result':entry.get('result'),'at':entry.get('time',entry.get('ts',entry.get('timestamp')))})
out['recorded_credential_events']=refreshes
changed = json.loads((root/'okad-data-verification.json').read_text())['changed_fields']
out['okad_link_updates']=[]
with sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True) as live, sqlite3.connect('file:'+str(root/'okad-pre-release/cutover.db')+'?mode=ro',uri=True) as old:
    for mid, fields in changed.items():
        assert set(fields)<= {'updated_at','sub2_links'}, 'Review other business changes'
        prior = json.loads(old.execute('SELECT sub2_links FROM external_members WHERE id=?',(mid,)).fetchone()[0] or '{}')
        current = json.loads(live.execute('SELECT sub2_links FROM external_members WHERE id=?',(mid,)).fetchone()[0] or '{}')
        deltas = {target:[key for key in set(prior.get(target,{}))|set(link) if prior.get(target,{}).get(key)!=link.get(key)] for target,link in current.items()}
        deliveries = live.execute('SELECT state,version,attempts,updated_at,next_at,lease_until FROM integration_deliveries WHERE source_key=?',('external:'+mid,)).fetchall()
        out['okad_link_updates'].append({'id':int(mid),'changed_link_fields':list(deltas.values()),'delivery_rows':deliveries})
path=root/'business-state-source-v2.json'
assert not path.exists()
path.write_text(json.dumps(out,ensure_ascii=False,indent=2));path.chmod(0o600)
print(json.dumps(out,ensure_ascii=False))
