import importlib.util,pathlib,unittest
s=importlib.util.spec_from_file_location('review',pathlib.Path(__file__).with_name('review-operator-edits.py'));r=importlib.util.module_from_spec(s);s.loader.exec_module(r)
class OperatorReview(unittest.TestCase):
 def test_exact_confirmed_edits_only(self):
  changes=[{'id':i,'fields':['manual_pause'] if i in r.APPROVED_PAUSE_IDS else [],'credential_keys_changed':['model_mapping'] if i in r.APPROVED_MAPPING_IDS else [],'proxy_chain_matches':True} for i in sorted(set(r.APPROVED_PAUSE_IDS+r.APPROVED_MAPPING_IDS))]
  review={'changes':changes,'manual_proxy_modes_preserved':True,'missing_ids':[]}
  r.validate(review,r.APPROVED_PAUSE_IDS,r.APPROVED_MAPPING_IDS,[])
  for pause,mapping,missing in [(r.APPROVED_PAUSE_IDS+[999999],r.APPROVED_MAPPING_IDS,[]),(r.APPROVED_PAUSE_IDS,r.APPROVED_MAPPING_IDS+[999999],[]),(r.APPROVED_PAUSE_IDS,r.APPROVED_MAPPING_IDS,[1]),([],r.APPROVED_MAPPING_IDS,[])]:
   with self.assertRaises(AssertionError):r.validate(review,pause,mapping,missing)
