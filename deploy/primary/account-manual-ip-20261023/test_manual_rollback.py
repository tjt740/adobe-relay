import importlib.util
from pathlib import Path
from unittest import TestCase,mock
spec=importlib.util.spec_from_file_location('manual_audit',Path(__file__).with_name('audit.py'))
a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
class ManualRollbackGuardTests(TestCase):
 def test_old_image_cannot_silently_replace_manual_ownership(self):
  sha='b'*40
  before={'containers':{a.APP:{'image_id':'old'}}}
  current={'containers':{a.APP:{'image':'adobe-relay:'+sha}},'manual_proxy_account_ids':[7]}
  with mock.patch.object(a,'check_preserved',return_value={'preserved':True}) as check:
   with self.assertRaises(AssertionError):a.validate_rollback(before,current,sha,'old')
   check.assert_not_called()
   current['manual_proxy_account_ids']=[]
   self.assertEqual(a.validate_rollback(before,current,sha,'old'),{'preserved':True})
