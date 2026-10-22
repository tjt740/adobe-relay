import copy,datetime,hashlib,importlib.util
from pathlib import Path
from unittest import TestCase
spec=importlib.util.spec_from_file_location('m',Path(__file__).with_name('latest-failure-review.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
class LatestFailureReviewTests(TestCase):
 def setUp(self):
  self.cutover=datetime.datetime.fromisoformat('2026-10-22T08:22:29+00:00')
  self.old={'status':'active','schedulable':True,'credentials':{'cookie':'synthetic-cookie','access_token':'old','model_mapping':{'image':'model'}},'extra':{'okad_manual_pause':False,'okad':{'version':3,'issuer':'same','source_key':'same'}}}
  self.current=copy.deepcopy(self.old);self.current.update(status='error',schedulable=False,error_message='Token refresh failed (non-retryable): synthetic typed auth rejection');self.current['credentials']['access_token']='new'
  self.health=[{'source':'business','stage':'token','reason':'auth','status_code':401,'failed_at':'2026-10-22T16:23:42+08:00'}]
 def test_original_conditional_auth_error_retains_live_state(self):
  old=copy.deepcopy(self.old);current=copy.deepcopy(self.current)
  self.assertEqual(m.classify(self.old,self.current,self.health,self.cutover),'original_business_auth_error_same_cookie')
  self.assertEqual(self.old,old);self.assertEqual(self.current,current)
 def test_manual_pause_model_mapping_and_unattributed_failures_are_rejected(self):
  for variant in ['pause','mapping','no_auth','untyped']:
   row=copy.deepcopy(self.current);health=copy.deepcopy(self.health)
   if variant=='pause':row['extra']['okad_manual_pause']=True
   elif variant=='mapping':row['credentials']['model_mapping']={}
   elif variant=='no_auth':health[0]['status_code']=500
   else:row['error_message']='unrecognized reason'
   with self.subTest(variant=variant),self.assertRaises(AssertionError):m.classify(self.old,row,health,self.cutover)
 def test_verified_rotation_requires_version_and_failed_full_test(self):
  row=copy.deepcopy(self.current);row['credentials']['cookie']='new-synthetic-cookie';row['extra']['okad'].update(version=4,verification='verified');row['extra'].update(last_check_success=False,last_check_status_code=408,okad_connection_test={'gated':True,'resume':True,'state':'failed','key':hashlib.sha256(b'new-synthetic-cookie').hexdigest(),'finished_at':'2026-10-22T08:24:14Z'})
  health=self.health+[{'source':'okad','status_code':408,'failed_at':'2026-10-22T16:24:14+08:00'}]
  self.assertEqual(m.classify(self.old,row,health,self.cutover),'original_verified_cookie_sync_failed_408_gate')
  row['extra']['okad']['version']=3
  with self.assertRaises(AssertionError):m.classify(self.old,row,health,self.cutover)

 def test_background_refresh_matches_exact_terminal_error(self):
  import json
  row=copy.deepcopy(self.current);row['id']=7;error='adobe cookie is no longer valid: synthetic typed auth rejection';row['error_message']='Token refresh failed (non-retryable): '+error
  raw=json.dumps({'msg':'token_refresh.account_refresh_failed','account_id':7,'platform':'adobe','error':error})
  evidence=m.background_evidence(raw,row);self.assertEqual(len(evidence),1)
  row['_background_refresh_evidence']=evidence
  self.assertEqual(m.classify(self.old,row,[],self.cutover),'original_background_auth_error_same_cookie')
  self.assertFalse(m.background_evidence(raw.replace('no longer valid','temporarily unavailable'),row))
  self.assertFalse(m.background_evidence(raw.replace('"account_id": 7','"account_id": 8'),row))

 def test_existing_redacted_permanent_wrapper_requires_typed_current_error(self):
  import json
  row=copy.deepcopy(self.current);row['id']=7;row['error_message']='Token refresh failed (non-retryable): adobe cookie is no longer valid: synthetic auth rejection'
  raw=json.dumps({'msg':'token_refresh.account_refresh_failed','account_id':7,'platform':'adobe','error':'account OAuth credentials permanently rejected'})
  self.assertEqual(len(m.background_evidence(raw,row)),1)
  row['error_message']='temporary failure'
  self.assertFalse(m.background_evidence(raw,row))
