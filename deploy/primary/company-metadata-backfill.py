"""One-off company metadata sync while a live Okad login batch prevents restart.
Uses only existing account records and the dedicated metadata endpoint; no cookies.
"""
import collections
import datetime
import json
from pathlib import Path
import re
import sqlite3
import urllib.request

root=Path('/opt/adobe-relay/releases/okad-company-mobile-mail-20261014')
text=Path('/opt/adobe-relay/shared/data/config.yaml').read_text()
values=re.findall(r'^\s+okad_integration_api_key:\s*(\S.*?)\s*$',text,re.M)
assert len(values)==1
value=values[0]
if value.startswith('"'):
    key=json.loads(value)
elif value.startswith("'") and value.endswith("'"):
    key=value[1:-1].replace("''", "'")
else:
    assert re.fullmatch(r'[A-Za-z0-9_\-]+',value), 'Unsupported key quoting; do not guess'
    key=value
assert key

def request(path,body=None):
    raw=None if body is None else json.dumps(body).encode()
    req=urllib.request.Request('http://127.0.0.1:6666/api/v1/integrations/okad'+path,data=raw,headers={'X-Integration-Key':key,'Content-Type':'application/json'})
    with urllib.request.urlopen(req,timeout=30) as response:
        return json.load(response)['data']

caps=request('/capabilities')
assert caps['site_id']=='primary' and caps['account_metadata']
with sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True) as db:
    issuer=db.execute("SELECT value FROM settings WHERE key='okad_instance_id'").fetchone()[0]
    assert db.execute("SELECT value FROM settings WHERE key='sub2_reliable_enabled'").fetchone()[0]=='true'
    sources={}
    for mid,email,mode,mail,refresh,client in db.execute('SELECT id,email,otp_mode,mail_url,refresh_token,client_id FROM external_members'):
        source='external:'+str(mid)
        sources[source]={'issuer':issuer,'source_key':source,'email':email.strip().lower(),'is_company':bool(mode!='manual' and not mail and not(refresh and client))}
    for mid,email in db.execute('SELECT id,email FROM adobe_members WHERE is_admin=0'):
        source='pool:'+str(mid)
        sources[source]={'issuer':issuer,'source_key':source,'email':email.strip().lower(),'is_company':False}
by_email=collections.defaultdict(list)
for meta in sources.values():by_email[meta['email']].append(meta)
items=[];cursor=0
while True:
    page=request('/accounts?after_id='+str(cursor));items+=page['items']
    if not page['next_cursor']:break
    assert page['next_cursor']>cursor
    cursor=page['next_cursor']
updates=[]
for account in items:
    email=(account.get('email') or '').strip().lower()
    owner=account.get('integration') or {};old=account.get('account_metadata') or {}
    if owner.get('issuer'):
        if owner['issuer']!=issuer:continue
        meta=sources.get(owner.get('source_key'))
    else:
        candidates=by_email.get(email,[])
        if (account.get('imported_email') or '').strip().lower()!=email or len(candidates)!=1:continue
        meta=candidates[0]
    if not meta or meta['email']!=email:continue
    if not owner.get('issuer') and old.get('issuer') and (old['issuer']!=issuer or old.get('source_key')!=meta['source_key']):continue
    if old!=meta:updates.append({'id':account['id'],**meta})
updated=0
for offset in range(0,len(updates),200):
    updated+=request('/account-metadata',{'items':updates[offset:offset+200]})['updated']
record={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'inventory':len(items),'eligible_updates':len(updates),'updated':updated,'requested_cookie_operations':False,'requested_adobe_operations':False,'reason':'User approved waiting for the active login batch; classify existing imports without restarting that batch.'}
(root/'metadata-backfill.json').write_text(json.dumps(record,indent=2))
print(json.dumps(record))
