import copy
from pathlib import Path
from unittest import TestCase
from blocked_contract_guard import validate_blocked

class BlockedContractTests(TestCase):
 def rows(self):
  return [dict(name=name,model=model,application_sha='a'*40,success=False,http_status=408,outer_posts=1,error=dict(stage='submit',source='upstream',upstream_status=408)) for name,model in [('flare-xhigh','gpt-image-2.6-flare'),('sunburst-auto-inherit','gpt-image-2.6-sunburst')]]
 def test_blocked_status_explicitly_fails_live_billing(self):
  result=validate_blocked(self.rows(),'a'*40)
  self.assertFalse(result['all_passed']);self.assertFalse(result['live_billing_verified'])
 def test_unknown_or_generated_result_cannot_be_labelled_rejection(self):
  for field,value in [('http_status',502),('success',True),('images',[{}]),('usage',{'image_count':1}),('application_sha','b'*40)]:
   rows=self.rows();rows[0][field]=value
   with self.subTest(field=field),self.assertRaises(AssertionError):validate_blocked(rows,'a'*40)
 def test_deployment_proof_does_not_write_final_success(self):
  code=Path(__file__).with_name('verify-deployment.py').read_text()
  self.assertIn("a.save('deployment-verification.json',out)",code)
  self.assertNotIn("a.save('final-verification.json'",code)
  final=Path(__file__).with_name('final-verify.py').read_text()
  self.assertIn("validate_public_contract(live_after['cases'],sha)",final)
  self.assertNotIn('validate_blocked',final)
