"""Attribute live credential rotations without emitting any credential values."""
import datetime,hashlib,importlib.util,json,re,subprocess
from pathlib import Path
ROOT=Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
review=json.loads((ROOT/'business-verification.json').read_text());ids=[c['id'] for c in review['changes'] if 'cookie' in c['credential_keys_changed']]
dump=Path(review['cutover_backup'])/'database.dump'
raw=subprocess.check_output(['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--data-only','--table=accounts','-f','-'],input=dump.read_bytes(),timeout=90).decode()
escapes={'b':'\b','f':'\f','n':'\n','r':'\r','t':'\t','v':'\v','\\':'\\'}
def value(s):return None if s=='\\N' else re.sub(r'\\(.)',lambda m:escapes.get(m[1],m[1]),s)
old={};active=False
for line in raw.splitlines():
 if line.startswith('COPY public.accounts ('):
  cols=[c.strip().strip('"') for c in line.split('(',1)[1].split(')',1)[0].split(',')];active=True;continue
 if active and line=='\\.':active=False;continue
 if active:
  r=dict(zip(cols,map(value,line.split('\t'))))
  if int(r['id']) in ids:old[int(r['id'])]={'okad':json.loads(r['extra'] or '{}').get('okad',{}),'cookie_hash':hashlib.md5(json.loads(r['credentials']).get('cookie','').encode()).hexdigest()}
rows=json.loads(a.sql("SELECT coalesce(json_agg(json_build_object('id',id,'okad',extra->'okad','cookie_hash',md5(coalesce(credentials->>'cookie','')))),'[]') FROM accounts WHERE id IN ("+','.join(map(str,ids or [0]))+")"))
out=[]
for r in rows:
 previous=old[r['id']];before=previous['okad'];after=r['okad'] or {}
 out.append({'id':r['id'],'previous_version':before.get('version'),'current_version':after.get('version'),'version_advanced':int(after.get('version') or 0)>int(before.get('version') or 0),'issuer_compatible':not before.get('issuer') or before.get('issuer')==after.get('issuer'),'source_key_compatible':not before.get('source_key') or before.get('source_key')==after.get('source_key'),'received_at':after.get('received_at'),'verification':after.get('verification'),'cookie_differs_from_backup':previous['cookie_hash']!=r['cookie_hash']})
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'cookie_changed_accounts':out,'token_fields_only_accounts':[c['id'] for c in review['changes'] if c['credential_keys_changed'] and 'cookie' not in c['credential_keys_changed']],'credentials_in_output':False,'restored_old_data':False}
a.save('credential-change-review.json',result)
print(json.dumps(result,ensure_ascii=False))
assert len(out)==len(ids) and all(r['version_advanced'] and r['issuer_compatible'] and r['source_key_compatible'] for r in out),'Review live credential changes without restoring old data'
