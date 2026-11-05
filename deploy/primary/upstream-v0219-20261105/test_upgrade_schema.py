import copy,hashlib,importlib.util,json,unittest
from pathlib import Path
HERE=Path(__file__).parent
spec=importlib.util.spec_from_file_location('resume_guard',HERE/'resume_guard.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
class UpgradeSchemaTests(unittest.TestCase):
 def test_manifest_matches_migration_runner_trimspace(self):
  root=HERE.parents[2]
  manifest=json.loads((HERE/'migration-manifest.json').read_text())
  self.assertEqual(len(manifest),3)
  for name,value in manifest.items():
   raw=(root/'backend/migrations'/name).read_bytes()
   self.assertEqual(value,hashlib.sha256(raw.decode().strip().encode()).hexdigest())
 def records(self):
  healthy={'Health':{'Status':'healthy'}}
  return [dict(step='failed',state='attention_required',error='schema-verify.py returned non-zero'), 'AssertionError: Unexpected new migration set',dict(candidate_sha='abc',old_id='old',canary_id='new'),dict(Id='old',Config={'Image':'adobe-relay:4569e1bd2e1e8957d1a6bbba677f67d76d56b47e'},State=healthy),dict(Id='new',Config={'Image':'adobe-relay:abc'},Image='image',State=healthy),dict(image_id='image'),'abc',False,'owner','owner']
 def test_accepts_only_known_preroute_failure(self):m.validate_records(*self.records())
 def test_rejects_changed_container_image_route_or_lease(self):
  for index,key,value in [(3,'Id','later'),(4,'Image','other')]:
   args=self.records();args[index][key]=value
   with self.assertRaises(AssertionError):m.validate_records(*args)
  for index,value in [(7,True),(8,''),(8,'another')]:
   args=self.records();args[index]=value
   with self.assertRaises(AssertionError):m.validate_records(*args)
 def test_rejects_other_failure(self):
  args=self.records();args[1]='Something else'
  with self.assertRaises(AssertionError):m.validate_records(*args)
if __name__=='__main__':unittest.main()
