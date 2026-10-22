"""Attribute only the four observed recoveries to original successful test gates."""
import argparse,datetime,hashlib,importlib.util,json,pathlib,re,subprocess
ROOT=pathlib.Path('/opt/adobe-relay/releases/upstream-v0217-20261022')
EXPECTED_IDS={1150,1151,1154,1156}
SOURCE_HASHES={'backend/internal/repository/account_repo_okad_connection_check.go': {'before': '973720c1267be3c9641fec66e1793f83035b711d13ffa99ab799c21edf99928f', 'candidate': '973720c1267be3c9641fec66e1793f83035b711d13ffa99ab799c21edf99928f'}, 'backend/internal/service/okad_connection_check.go': {'before': '7990dfa3a2c7baa894ad1bdbd503384c629c172f9ed83514c085db39ffc31fc0', 'candidate': '7990dfa3a2c7baa894ad1bdbd503384c629c172f9ed83514c085db39ffc31fc0'}, 'backend/internal/repository/account_repo_connection_result.go': {'before': 'cd25e5dbf9eea753a6de3dd12b0b78a05babd9af303fb93dab89045b450df295', 'candidate': 'cd25e5dbf9eea753a6de3dd12b0b78a05babd9af303fb93dab89045b450df295'}, 'backend/internal/repository/account_repo_adobe_test_recovery.go': {'before': '04b398b08dcea1b9cfe7ea8da678c5a3af1dd60f1fde17b62ff79ecb741b2c26', 'candidate': '04b398b08dcea1b9cfe7ea8da678c5a3af1dd60f1fde17b62ff79ecb741b2c26'}}
def validate(row):
 return (row['before_status']=='error' and not row['before_schedulable'] and row['current_status']=='active' and row['current_schedulable']
  and row['manual_pause_unchanged'] and not row['manual_pause'] and row['gate_state']=='success' and row['gated'] is True and row['resume'] is True
  and row['key_matches_cookie'] and row['last_check_success'] is True and row['last_check_status_code']==200 and row['completed_after_cutover']
  and row['has_post_cutover_full_test_success'] and row['verification']=='verified')
def main():
 p=argparse.ArgumentParser();p.add_argument('action',choices=['capture','approve']);args=p.parse_args()
 spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
 review_path=ROOT/'business-verification.json';review=json.loads(review_path.read_text())
 changes=[c for c in review['changes'] if set(c['fields'])&{'status','schedulable'}]
 ids={c['id'] for c in changes};assert ids==EXPECTED_IDS
 evidence_path=ROOT/'live-state-change-evidence.json'
 if args.action=='capture':
  raw=subprocess.check_output(['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--data-only','--table=accounts','-f','-'],input=(pathlib.Path(review['cutover_backup'])/'database.dump').read_bytes(),timeout=90).decode()
  escapes={'b':'\b','f':'\f','n':'\n','r':'\r','t':'\t','v':'\v','\\':'\\'}
  def value(s):return None if s=='\\N' else re.sub(r'\\(.)',lambda m:escapes.get(m[1],m[1]),s)
  old={};active=False
  for line in raw.splitlines():
   if line.startswith('COPY public.accounts ('):
    cols=[c.strip().strip('"') for c in line.split('(',1)[1].split(')',1)[0].split(',')];active=True;continue
   if active and line=='\\.':active=False;continue
   if active:
    row=dict(zip(cols,map(value,line.split('\t'))))
    if int(row['id']) in ids:old[int(row['id'])]=row
  rows=json.loads(a.sql("SELECT json_agg(json_build_object('id',id,'status',status,'schedulable',schedulable,'manual_pause',extra->'okad_manual_pause','verification',extra->'okad'->>'verification','gate',extra->'okad_connection_test','last_check_success',extra->'last_check_success','last_check_status_code',extra->'last_check_status_code','last_check_at',extra->>'last_check_at','key_matches_cookie',extra->'okad_connection_test'->>'key'=encode(sha256(convert_to(coalesce(credentials->>'cookie',''),'UTF8')),'hex'))) FROM accounts WHERE id IN ("+','.join(map(str,sorted(ids)))+")"))
  cutover=datetime.datetime.strptime(pathlib.Path(review['cutover_backup']).name.split('-')[0],'%Y%m%dT%H%M%SZ').replace(tzinfo=datetime.timezone.utc)
  health=json.loads(a.sql("SELECT coalesce(json_agg(json_build_object('id',account_id,'model',scope,'success_at',success_at,'success_started_at',success_started_at,'source',success_source)),'[]') FROM adobe_account_health WHERE account_id IN ("+','.join(map(str,sorted(ids)))+") AND success_started_at >= '"+cutover.isoformat()+"'::timestamptz"))
  evidence=[]
  for row in rows:
   previous=old[row['id']];prior=json.loads(previous['extra'] or '{}');gate=row['gate'] or {};finished=gate.get('finished_at')
   evidence.append({'id':row['id'],'before_status':previous['status'],'before_schedulable':previous['schedulable']=='t','current_status':row['status'],'current_schedulable':row['schedulable'],'manual_pause':row['manual_pause'],'manual_pause_unchanged':prior.get('okad_manual_pause')==row['manual_pause'],'gate_state':gate.get('state'),'gated':gate.get('gated'),'resume':gate.get('resume'),'verification':row['verification'],'key_matches_cookie':row['key_matches_cookie'],'last_check_success':row['last_check_success'],'last_check_status_code':row['last_check_status_code'],'last_check_at':row['last_check_at'],'queued_at':gate.get('queued_at'),'started_at':gate.get('started_at'),'finished_at':finished,'completed_after_cutover':bool(finished) and datetime.datetime.fromisoformat(finished.replace('Z','+00:00'))>=cutover,'has_post_cutover_full_test_success':any(h['id']==row['id'] and h['source']=='test' for h in health)})
  out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'business_verification_sha256':hashlib.sha256(review_path.read_bytes()).hexdigest(),'accounts':evidence,'health_success_evidence':health,'original_source_sha256':SOURCE_HASHES,'mutated_business_data':False}
  a.save(evidence_path.name,out);print(json.dumps(out,ensure_ascii=False));return
 evidence=json.loads(evidence_path.read_text());assert evidence['business_verification_sha256']==hashlib.sha256(review_path.read_bytes()).hexdigest()
 assert {r['id'] for r in evidence['accounts']}==EXPECTED_IDS and all(validate(r) for r in evidence['accounts']), 'Recovery evidence does not satisfy the original gate'
 assert all(v['before']==v['candidate'] for v in evidence['original_source_sha256'].values())
 out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'reviewed_ids':sorted(ids),'evidence_sha256':hashlib.sha256(evidence_path.read_bytes()).hexdigest(),'business_verification_sha256':evidence['business_verification_sha256'],'all_original_recovery_gates_verified':True,'manual_pauses_preserved':True,'restored_old_data':False}
 a.save('reviewed-live-state-changes.json',out);print(json.dumps(out))
if __name__=='__main__':main()
