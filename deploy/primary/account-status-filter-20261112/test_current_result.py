import json,pathlib,unittest
from current_result import result_status
class CurrentResultProof(unittest.TestCase):
 def test_shared_frontend_backend_examples(self):
  root=pathlib.Path(__file__).resolve().parents[3]
  cases=json.loads((root/'frontend/src/utils/__tests__/fixtures/account-current-status.json').read_text())
  for case in cases:
   with self.subTest(name=case['name']):
    expected=next((s for s in case['matches'] if s in ['test_passed','test_failed','request_succeeded','request_failed']),None)
    self.assertEqual(result_status(case['account']),expected)
