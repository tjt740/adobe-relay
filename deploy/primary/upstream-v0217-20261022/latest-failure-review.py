"""Attribute a point-in-time snapshot to unchanged conditional Adobe error gates."""
import datetime,hashlib,json,pathlib,re,subprocess
ROOT=pathlib.Path('/opt/adobe-relay/releases/upstream-v0217-20261022')
TOKEN_KEYS={'access_token','expires_at','_token_version','adobe_session_cookie','adobe_session_cookie_source_hash'}
def background_evidence(raw,row):
 evidence=[]
 for line in raw.splitlines():
  if 'token_refresh.account_refresh_failed' not in line:continue
  try:meta=json.loads(line[line.index('{'):])
  except (ValueError,json.JSONDecodeError):continue
  if meta.get('account_id')!=row['id'] or meta.get('platform')!='adobe':continue
  error=meta.get('error','')
  if not isinstance(error,str) or 'adobe cookie is no longer valid' not in error.lower():continue
  if not row['error_message'].endswith(error):continue
  evidence.append({'event':'token_refresh.account_refresh_failed','line_sha256':hashlib.sha256(line.encode()).hexdigest(),'error_sha256':hashlib.sha256(error.encode()).hexdigest(),'same_terminal_error':True})
 return evidence

def classify(before,current,health,cutover):
 assert before['status']=='active' and before['schedulable'] is True and current['status']=='error' and current['schedulable'] is False
 old_extra=before['extra'];new_extra=current['extra']
 old_pause=old_extra.get('okad_manual_pause');new_pause=new_extra.get('okad_manual_pause')
 assert old_pause==new_pause and (old_pause is None or old_pause is False)
 business_auth=any(h['source']=='business' and h['stage']=='token' and h['reason']=='auth' and h['status_code']==401 and datetime.datetime.fromisoformat(h['failed_at'])>=cutover for h in health)
 assert business_auth or current.get('_background_refresh_evidence'), 'No post-cutover business or original background terminal auth evidence'
 old_creds=before['credentials'];new_creds=current['credentials']
 changed={k for k in set(old_creds)|set(new_creds) if old_creds.get(k)!=new_creds.get(k)}
 assert changed <= TOKEN_KEYS|{'cookie'}, 'Operator model mapping or unrelated credentials changed'
 if old_creds.get('cookie')==new_creds.get('cookie'):
  assert current['error_message'].startswith(('Token refresh failed (non-retryable):','Authentication failed (401):'))
  return 'original_business_auth_error_same_cookie' if business_auth else 'original_background_auth_error_same_cookie'
 old_okad=old_extra.get('okad',{});okad=new_extra.get('okad',{});gate=new_extra.get('okad_connection_test',{})
 assert int(okad.get('version') or 0)>int(old_okad.get('version') or 0) and okad.get('verification')=='verified'
 assert not old_okad.get('issuer') or old_okad['issuer']==okad.get('issuer')
 assert not old_okad.get('source_key') or old_okad['source_key']==okad.get('source_key')
 assert gate.get('key')==hashlib.sha256(new_creds.get('cookie','').encode()).hexdigest()
 assert gate.get('gated') is True and gate.get('resume') is True and gate.get('state')=='failed'
 assert new_extra.get('last_check_success') is False and new_extra.get('last_check_status_code')==408
 assert datetime.datetime.fromisoformat(gate['finished_at'].replace('Z','+00:00'))>=cutover
 assert any(h['source']=='okad' and h['status_code']==408 and datetime.datetime.fromisoformat(h['failed_at'])>=cutover for h in health)
 return 'original_verified_cookie_sync_failed_408_gate'
def review(a,initial_accounts,latest_accounts):
 changed=sorted(i for i in initial_accounts if any(initial_accounts[i][k]!=latest_accounts[i][k] for k in ['status','schedulable']))
 backup=pathlib.Path(json.loads((ROOT/'after.json').read_text())['backup'])
 cutover=datetime.datetime.strptime(backup.name.split('-')[0],'%Y%m%dT%H%M%SZ').replace(tzinfo=datetime.timezone.utc)
 raw=subprocess.check_output(['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--data-only','--table=accounts','-f','-'],input=(backup/'database.dump').read_bytes(),timeout=90).decode()
 escapes={'b':'\b','f':'\f','n':'\n','r':'\r','t':'\t','v':'\v','\\':'\\'}
 def value(s):return None if s=='\\N' else re.sub(r'\\(.)',lambda m:escapes.get(m[1],m[1]),s)
 old={};active=False
 for line in raw.splitlines():
  if line.startswith('COPY public.accounts ('):
   cols=[c.strip().strip('"') for c in line.split('(',1)[1].split(')',1)[0].split(',')];active=True;continue
  if active and line=='\\.':active=False;continue
  if active:
   r=dict(zip(cols,map(value,line.split('\t'))));i=int(r['id'])
   if i in changed:old[i]={'status':r['status'],'schedulable':r['schedulable']=='t','credentials':json.loads(r['credentials']),'extra':json.loads(r['extra'] or '{}')}
 ids=','.join(map(str,changed or [0]))
 current=json.loads(a.sql("SELECT json_agg(json_build_object('id',id,'status',status,'schedulable',schedulable,'error_message',error_message,'credentials',credentials,'extra',extra)) FROM accounts WHERE id IN ("+ids+")")) or []
 health=json.loads(a.sql("SELECT coalesce(json_agg(to_jsonb(h)),'[]') FROM adobe_account_health h WHERE account_id IN ("+ids+")"))
 source=json.loads((ROOT/'reviewed-live-state-changes.json').read_text())['original_source_sha256'];assert all(v['before']==v['candidate'] for v in source.values())
 logs=subprocess.run(['docker','logs','--since',cutover.isoformat(),a.APP],text=True,capture_output=True,check=True,timeout=45)
 log_text=logs.stdout+'\n'+logs.stderr
 proof=[]
 for row in current:
  row['_background_refresh_evidence']=background_evidence(log_text,row)
  proof.append({'id':row['id'],'cookie_unchanged':old[row['id']]['credentials'].get('cookie')==row['credentials'].get('cookie'),'background_terminal_evidence':row['_background_refresh_evidence'],'health':[h for h in health if h['account_id']==row['id']],'error_sha256':hashlib.sha256((row['error_message'] or '').encode()).hexdigest()})
 evidence_name='latest-failure-evidence-'+datetime.datetime.now(datetime.timezone.utc).strftime('%H%M%S')+'.json'
 a.save(evidence_name,{'accounts':proof,'credentials_in_output':False,'active_probes':False})
 result=[]
 for row in current:
  i=row['id'];assert row['status']==latest_accounts[i]['status'] and row['schedulable']==latest_accounts[i]['schedulable']
  records=[h for h in health if h['account_id']==i]
  try:kind=classify(old[i],row,records,cutover)
  except AssertionError as error:raise AssertionError('Account '+str(i)+': '+str(error)+'; inspect '+evidence_name) from error
  result.append({'id':i,'classification':kind,'manual_pause_preserved':True,'error_sha256':hashlib.sha256((row['error_message'] or '').encode()).hexdigest(),'post_cutover_auth_failure_times':[h['failed_at'] for h in records if h['source']=='business' and h['status_code']==401 and datetime.datetime.fromisoformat(h['failed_at'])>=cutover]})
 assert sorted(r['id'] for r in result)==changed
 out={'at':latest_accounts and datetime.datetime.now(datetime.timezone.utc).isoformat(),'snapshot_changed_ids':changed,'reviewed_count':len(result),'accounts':result,'evidence_file':evidence_name,'source_implementation_unchanged':True,'manual_pause_preserved':True,'restored_old_data':False,'active_upstream_probes':False}
 a.save('latest-live-failure-review.json',out);return out
