"""Match concurrent operator edits to successful authenticated audit records; never restore data."""
import datetime,hashlib,json,pathlib,re,subprocess,sys
ROOT=pathlib.Path('/opt/adobe-relay/releases/account-status-unified-20261112')
APP_SHA='871822afc3d6b30d51a2abbbbbcddf5644cd5f89'
PAUSE_EVENTS={178459:1954,178461:2008,178462:2015,178463:2173}
BULK_EVENTS={178450:[2680,3064,3065,3066,3067],178488:[3089,3090,3091,3092,3093,3095,3096,3097,3098,3100]}
PAUSE_IDS=sorted(PAUSE_EVENTS.values())
MAPPING_IDS=sorted(i for ids in BULK_EVENTS.values() for i in ids)

def validate(review,old,current,events):
 assert not review['missing_ids'] and not set(old)-set(current), 'Original account missing'
 assert review['manual_proxy_modes_preserved'] and all(c['proxy_chain_matches'] for c in review['changes'])
 assert sorted(c['id'] for c in review['changes'] if 'manual_pause' in c['fields'])==PAUSE_IDS
 assert sorted(c['id'] for c in review['changes'] if 'model_mapping' in c['credential_keys_changed'])==MAPPING_IDS
 assert sorted(i for i in old if old[i]['manual_pause']!=current[i]['manual_pause'])==PAUSE_IDS, 'Unreviewed manual pause change'
 assert sorted(i for i in old if old[i]['mapping']!=current[i]['mapping'])==MAPPING_IDS, 'Unreviewed model mapping change'
 assert all(old[i]['manual_proxy_enabled']==current[i]['manual_proxy_enabled'] for i in old)
 assert sorted(e['id'] for e in events)==sorted([*PAUSE_EVENTS,*BULK_EVENTS])
 cutoff=datetime.datetime.strptime(pathlib.Path(review['cutover_backup']).name.split('-',1)[0],'%Y%m%dT%H%M%SZ').replace(tzinfo=datetime.timezone.utc)
 for event in events:
  assert event['actor_user_id']==1 and event['actor_role']=='admin' and event['auth_method']=='jwt'
  assert event['status_code']==200 and event['method']=='POST'
  assert cutoff<=datetime.datetime.fromisoformat(event['created_at'])<=datetime.datetime.fromisoformat(review['at'])
  body=json.loads(event['request_body'])
  if event['id'] in PAUSE_EVENTS:
   i=PAUSE_EVENTS[event['id']]
   assert event['path']=='/api/v1/admin/accounts/:id/schedulable'
   assert event['action']=='admin.accounts.schedulable.create' and event['extra']['params']['id']==str(i)
   assert body=={'schedulable':False} and current[i]['manual_pause'] is True and current[i]['schedulable'] is False
  else:
   assert event['path']=='/api/v1/admin/accounts/bulk-update' and event['action']=='admin.accounts.bulk_update.create'
   assert sorted(body['account_ids'])==BULK_EVENTS[event['id']]
   assert isinstance(body['credentials']['model_mapping'],dict)
   for i in body['account_ids']:
    assert current[i]['mapping']==body['credentials']['model_mapping'], 'Model mapping differs from audited update'
    assert current[i]['concurrency']==body['concurrency'] and sorted(current[i]['group_ids'])==sorted(body['group_ids'])

def main():
 sys.path.insert(0,str(ROOT));import audit as a
 from backup_guard import validate_accounts_copy
 assert (ROOT/'expected-sha').read_text().strip()==APP_SHA and a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+APP_SHA
 report_path=ROOT/'business-verification.json';review=json.loads(report_path.read_text())
 raw=subprocess.check_output(['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--data-only','--table=accounts','-f','-'],input=(pathlib.Path(review['cutover_backup'])/'database.dump').read_bytes(),timeout=90).decode()
 escapes={'b':'\b','f':'\f','n':'\n','r':'\r','t':'\t','v':'\v','\\':'\\'}
 def value(s):return None if s=='\\N' else re.sub(r'\\(.)',lambda m:escapes.get(m[1],m[1]),s)
 old={};active=False;seen=False;ended=False;count=0
 for line in raw.splitlines():
  if line.startswith('COPY public.accounts ('):
   cols=[c.strip().strip('"') for c in line.split('(',1)[1].split(')',1)[0].split(',')];active=True;seen=True;continue
  if active and line=='\\.':active=False;ended=True;continue
  if active:
   count+=1;row=dict(zip(cols,map(value,line.split('\t'))))
   if row['deleted_at'] is None:
    extra=json.loads(row['extra'] or '{}');old[int(row['id'])]={'manual_pause':extra.get('okad_manual_pause'),'manual_proxy_enabled':extra.get('manual_proxy_enabled'),'mapping':json.loads(row['credentials'] or '{}').get('model_mapping')}
 validate_accounts_copy(seen,ended,count,len(old))
 rows=json.loads(a.sql("SELECT coalesce(json_agg(json_build_object('id',a.id,'manual_pause',extra->'okad_manual_pause','manual_proxy_enabled',extra->'manual_proxy_enabled','schedulable',schedulable,'mapping',credentials->'model_mapping','concurrency',concurrency,'group_ids',(SELECT coalesce(json_agg(group_id ORDER BY group_id),'[]') FROM account_groups WHERE account_id=a.id))),'[]') FROM accounts a WHERE deleted_at IS NULL"));current={r['id']:r for r in rows}
 ids=','.join(map(str,[*PAUSE_EVENTS,*BULK_EVENTS]))
 events=json.loads(a.sql("SELECT json_agg(e ORDER BY id) FROM (SELECT id,created_at,actor_user_id,actor_role,auth_method,method,path,action,status_code,request_body,extra FROM audit_logs WHERE id IN ("+ids+")) e"))
 validate(review,old,current,events)
 out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':APP_SHA,'status':'audited_operator_changes_retained','authorization':'Preserve current operator changes as requested; exact successful administrator audit records independently verified. No new user confirmation inferred.','original_business_report_sha256':hashlib.sha256(report_path.read_bytes()).hexdigest(),'approved_pause_ids':PAUSE_IDS,'approved_mapping_ids':MAPPING_IDS,'audit_event_ids':sorted([*PAUSE_EVENTS,*BULK_EVENTS]),'manual_pause_changes':[{'id':i,'before':old[i]['manual_pause'],'current':current[i]['manual_pause'],'schedulable':current[i]['schedulable']} for i in PAUSE_IDS],'bulk_updates_match_current_mapping_concurrency_and_groups':True,'other_manual_pause_states_preserved':True,'manual_proxy_modes_preserved':True,'business_data_written':False,'restored_old_data':False}
 if '--validate-only' not in sys.argv:a.save('operator-changes-review.json',out)
 print(json.dumps(out))
if __name__=='__main__':main()
