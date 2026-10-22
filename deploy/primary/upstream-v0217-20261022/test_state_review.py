import copy,importlib.util,json
from pathlib import Path
from unittest import TestCase
spec=importlib.util.spec_from_file_location('review',Path(__file__).with_name('review-state-changes.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
class StateReviewTests(TestCase):
 def setUp(self):
  self.initial={'accounts':[]};self.diagnostic={'accounts':[],'health':[]}
  for i in sorted(m.EXPECTED_IDS):
   self.initial['accounts'].append({'id':i,'before_status':'active','before_schedulable':True,'current_status':'error','current_schedulable':False,'manual_pause_unchanged':True,'manual_pause':False,'key_matches_cookie':True,'verification':'verified','gated':True,'resume':True,'last_check_status_code':408,'completed_after_cutover':True})
   self.diagnostic['accounts'].append({'id':i,'status':'error','schedulable':False,'manual_pause':False,'okad':{'verification':'verified'},'gate':{'state':'failed'},'error_hash':'b4e31fad13a7e21849c8d668f69016dc'})
   self.diagnostic['health'].append({'account_id':i,'source':'business','stage':'token','reason':'auth','status_code':401,'failed_at':'2026-10-22T16:23:42+08:00'})
  self.diagnostic['health'].append({'account_id':1156,'source':'okad','status_code':408,'failed_at':'2026-10-22T16:24:14+08:00'})
  self.credentials={'cookie_changed_accounts':[{'id':1156,'version_advanced':True,'issuer_compatible':True,'source_key_compatible':True}],'token_fields_only_accounts':[1150,1151,1154]};self.flags={str(i):True for i in [1150,1151,1154]}
 def test_exact_evidence_is_accepted_without_mutating_it(self):
  original=copy.deepcopy((self.initial,self.diagnostic,self.credentials))
  self.assertEqual([r['id'] for r in m.validate(self.initial,self.diagnostic,self.credentials,self.flags)],sorted(m.EXPECTED_IDS))
  self.assertEqual(original,(self.initial,self.diagnostic,self.credentials))
 def test_manual_pause_unknown_ids_and_missing_auth_evidence_are_rejected(self):
  for variant in ['pause','unknown','missing']:
   initial=copy.deepcopy(self.initial);diagnostic=copy.deepcopy(self.diagnostic)
   if variant=='pause':initial['accounts'][0]['manual_pause']=True
   elif variant=='unknown':initial['accounts'][0]['id']=999
   else:diagnostic['health']=[h for h in diagnostic['health'] if h['account_id']!=1150]
   with self.subTest(variant=variant),self.assertRaises(AssertionError):m.validate(initial,diagnostic,self.credentials,self.flags)
 def test_nonterminal_error_and_unverified_rotation_are_rejected(self):
  flags={**self.flags,'1150':False}
  with self.assertRaises(AssertionError):m.validate(self.initial,self.diagnostic,self.credentials,flags)
  credentials=copy.deepcopy(self.credentials);credentials['cookie_changed_accounts'][0]['version_advanced']=False
  with self.assertRaises(AssertionError):m.validate(self.initial,self.diagnostic,credentials,self.flags)
