"""Compare with the actual deployment backup; do not restore live business data."""
import datetime,json,hashlib,re,pathlib,subprocess
root=pathlib.Path('/opt/adobe-relay/releases/okad-company-mobile-mail-20261014')
sha='58638a1bb1d5b95e3ecb578be323ad556dfc932b'
backups=sorted(pathlib.Path('/opt/adobe-relay/backups').glob('*-'+sha))
assert backups
backup=backups[-1]
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'backup':str(backup)}
def sql(query):
 return subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'], input="BEGIN READ ONLY; SET LOCAL statement_timeout='10s'; "+query+'; COMMIT;',text=True,capture_output=True,check=True,timeout=20).stdout.strip()
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

prior = {}; prior_state = {}; prior_credentials = {}; fields = None
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
        prior_credentials[account_id] = json.loads(copy_value(row['credentials']) or '{}')
        prior_state[account_id] = {'credentials_hash':credentials_digest(json.loads(copy_value(row['credentials']) or '{}')),
            'status':row['status'],'schedulable':row['schedulable']=='t','manual_pause':extra.get('okad_manual_pause')}

assert prior
current_rows = json.loads(sql("SELECT COALESCE(json_agg(json_build_object('id',id,'credentials',credentials,'status',status,'schedulable',schedulable,'manual_pause',extra->'okad_manual_pause')),'[]') FROM accounts WHERE deleted_at IS NULL"))
current_state = {a['id']:{'credentials_hash':credentials_digest(a['credentials'] or {}),'status':a['status'],'schedulable':a['schedulable'],'manual_pause':a['manual_pause']} for a in current_rows}
state_changes = {str(account):[field for field in old if old[field]!=current_state.get(account,{}).get(field)] for account,old in prior_state.items()}
state_changes = {account:fields for account,fields in state_changes.items() if fields}
out['account_state_changes_since_cutover'] = state_changes
current_credentials = {a['id']:a['credentials'] or {} for a in current_rows}
out['changed_credential_fields'] = {str(a):[k for k in sorted(set(old)|set(current_credentials.get(a,{}))) if old.get(k)!=current_credentials.get(a,{}).get(k)] for a,old in prior_credentials.items() if old!=current_credentials.get(a,{})}
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
        if event['account_id'] == account and (event['from_proxy_id'] or 0) == (value or 0): value = event['to_proxy_id']
    if value == current[account]: explained.append(account)
out['account_preservation'] = {'backup_count':len(prior),'current_count':len(current),'same_ids':set(prior)==set(current),'changed_proxies':changed,'explained_by_existing_failover':explained}
(root/'proxy-audit.json').write_text(json.dumps({'summary':out['account_preservation'],'events':events},indent=2))
assert set(prior)<=set(current) and set(changed)==set(explained), 'Account changes require investigation'
out['added_account_ids'] = sorted(set(current)-set(prior))

out['connection_evidence_for_changed_accounts'] = json.loads(sql("SELECT COALESCE(json_agg(json_build_object('id',id,'state',extra->'okad_connection_test'->>'state','finished_at',extra->'okad_connection_test'->>'finished_at')),'[]') FROM accounts WHERE id IN ("+','.join(state_changes or ['0'])+")"))
(root/'business-verification.json').write_text(json.dumps(out,ensure_ascii=False,indent=2))
print(json.dumps(out,ensure_ascii=False))
