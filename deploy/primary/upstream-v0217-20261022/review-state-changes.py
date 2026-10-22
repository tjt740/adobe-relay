"""Review exact live error transitions; never restore old scheduling or credentials."""
import argparse,datetime,hashlib,importlib.util,json,pathlib
ROOT=pathlib.Path('/opt/adobe-relay/releases/upstream-v0217-20261022')
EXPECTED_IDS={1150,1151,1154,1156}
SOURCE_HASHES={'backend/internal/repository/account_repo_okad_connection_check.go': {'before': '973720c1267be3c9641fec66e1793f83035b711d13ffa99ab799c21edf99928f', 'candidate': '973720c1267be3c9641fec66e1793f83035b711d13ffa99ab799c21edf99928f'}, 'backend/internal/service/okad_connection_check.go': {'before': '7990dfa3a2c7baa894ad1bdbd503384c629c172f9ed83514c085db39ffc31fc0', 'candidate': '7990dfa3a2c7baa894ad1bdbd503384c629c172f9ed83514c085db39ffc31fc0'}, 'backend/internal/repository/account_repo_adobe_current_error.go': {'before': '6bdc581a53a365346a49b2172ddefd9c6d373aa1ee3936c0778054865437d135', 'candidate': '6bdc581a53a365346a49b2172ddefd9c6d373aa1ee3936c0778054865437d135'}, 'backend/internal/service/adobe_gateway_token.go': {'before': '527e31bd4bc9f0c3274333a2d269fcc5b54104f19f7456cfb96b43a00b166a2e', 'candidate': '527e31bd4bc9f0c3274333a2d269fcc5b54104f19f7456cfb96b43a00b166a2e'}, 'backend/internal/service/adobe_token_refresher.go': {'before': '3c5f318fb8930f75dcfe8e1191f6c4784b187c3fbbd416cea8cd4ad7738457d9', 'candidate': '3c5f318fb8930f75dcfe8e1191f6c4784b187c3fbbd416cea8cd4ad7738457d9'}, 'backend/internal/service/token_refresh_service.go': {'before': 'fe6b1a404bdc9378749ad9d996a9dc16e68fd7132ce44a7e7d73b3ac2c6dc130', 'candidate': 'fe6b1a404bdc9378749ad9d996a9dc16e68fd7132ce44a7e7d73b3ac2c6dc130'}}
def validate(initial,diagnostic,credentials,terminal_flags):
 rows={r['id']:r for r in initial['accounts']};current={r['id']:r for r in diagnostic['accounts']}
 assert set(rows)==set(current)==EXPECTED_IDS, 'Only the four independently observed IDs may be reviewed'
 assert all(v['before']==v['candidate'] for v in SOURCE_HASHES.values()), 'Original error/gating implementation changed'
 cookie={r['id']:r for r in credentials['cookie_changed_accounts']}
 token_only=set(credentials['token_fields_only_accounts'])
 approved=[]
 for i,row in rows.items():
  latest=current[i]
  assert row['before_status']=='active' and row['before_schedulable'] is True
  assert row['current_status']==latest['status']=='error' and row['current_schedulable'] is False and latest['schedulable'] is False
  assert row['manual_pause_unchanged'] and row['manual_pause'] is False and latest['manual_pause'] is False
  assert row['key_matches_cookie'] and row['verification']==latest['okad']['verification']=='verified'
  failures=[h for h in diagnostic['health'] if h['account_id']==i and h['source']=='business' and h['stage']=='token' and h['reason']=='auth' and h['status_code']==401 and datetime.datetime.fromisoformat(h['failed_at'])>=datetime.datetime.fromisoformat('2026-10-22T08:22:29+00:00')]
  assert failures, 'A live business authentication failure must be recorded after cutover'
  if i==1156:
   rotation=cookie[i];assert rotation['version_advanced'] and rotation['issuer_compatible'] and rotation['source_key_compatible']
   assert latest['gate']['state']=='failed' and row['gated'] is True and row['resume'] is True and row['last_check_status_code']==408 and row['completed_after_cutover']
   assert any(h['account_id']==i and h['source']=='okad' and h['status_code']==408 and datetime.datetime.fromisoformat(h['failed_at'])>=datetime.datetime.fromisoformat('2026-10-22T08:24:05+00:00') for h in diagnostic['health'])
   kind='original_cookie_sync_and_failed_408_gate'
  else:
   assert i in token_only and i not in cookie and terminal_flags[str(i)]
   assert latest['error_hash']=='b4e31fad13a7e21849c8d668f69016dc'
   kind='original_conditional_cookie_auth_error'
  approved.append({'id':i,'classification':kind,'before_status':'active','current_status':'error','before_schedulable':True,'current_schedulable':False,'manual_pause_preserved':True,'error_hash':latest['error_hash']})
 return sorted(approved,key=lambda r:r['id'])
def main():
 p=argparse.ArgumentParser();p.add_argument('action',choices=['approve']);p.parse_args()
 spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
 def read(name):return json.loads((ROOT/name).read_text())
 initial=read('live-state-change-evidence.json');diagnostic=read('state-change-diagnostic.json');credentials=read('credential-change-review.json')
 business_hash=hashlib.sha256((ROOT/'business-verification.json').read_bytes()).hexdigest()
 assert initial['business_verification_sha256']==business_hash
 flags=json.loads(a.sql("SELECT json_object_agg(id::text,(error_message LIKE 'Token refresh failed (non-retryable):%' OR error_message LIKE 'Authentication failed (401):%')) FROM accounts WHERE id IN(1150,1151,1154)"))
 approved=validate(initial,diagnostic,credentials,flags)
 out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'reviewed_ids':sorted(EXPECTED_IDS),'accounts':approved,'evidence_sha256':hashlib.sha256((ROOT/'live-state-change-evidence.json').read_bytes()).hexdigest(),'diagnostic_sha256':hashlib.sha256((ROOT/'state-change-diagnostic.json').read_bytes()).hexdigest(),'business_verification_sha256':business_hash,'original_source_sha256':SOURCE_HASHES,'all_original_failure_gates_verified':True,'manual_pauses_preserved':True,'restored_old_data':False}
 a.save('reviewed-live-state-changes.json',out);print(json.dumps(out))
if __name__=='__main__':main()
