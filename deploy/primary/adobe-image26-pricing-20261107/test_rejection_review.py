import copy
from pathlib import Path
import runpy
from unittest import TestCase

review=runpy.run_path(str(Path(__file__).with_name('review-rejection.py')))

class RejectionReviewTests(TestCase):
 def setUp(self):
  self.failure={'http_status':408,'success':False,'outer_posts':1,'error':{'request_id':review['RID'],'stage':'submit','source':'upstream','upstream_status':408}}
  self.events=[]
  for i in range(1,17):
   base={'request_id':review['RID'],'api_key_id':177,'account_id':100+i,'stage':'submit','job_id':'','upstream_http_status':408,'stage_attempts':3,'known_load_shedding':True}
   self.events.extend([dict(base,msg='adobe_images.upstream_failed'),dict(base,msg='adobe_images.account_failover',attempts=i)])
  self.events.append({'msg':'http request completed','request_id':review['RID'],'status_code':408})
 def test_only_proven_preacceptance_rejection_passes(self):
  result=review['validate'](self.events,self.failure,177,0)
  self.assertTrue(result['explicit_preacceptance_rejection'])
  self.assertEqual(result['rejected_submit_attempts'],48)
 def test_bill_or_wrong_key_cannot_be_rechecked(self):
  for key,count in [(178,0),(177,1)]:
   with self.assertRaises(AssertionError):review['validate'](self.events,self.failure,key,count)
 def test_missing_or_unknown_evidence_stops(self):
  for field,value in [('job_id','accepted-job'),('stage','poll'),('upstream_http_status',503),('known_load_shedding',False),('api_key_id',1),('request_id','wrong')]:
   events=copy.deepcopy(self.events);events[0][field]=value
   with self.subTest(field=field),self.assertRaises(AssertionError):review['validate'](events,self.failure,177,0)
  with self.assertRaises(AssertionError):review['validate'](self.events[1:],self.failure,177,0)
 def test_completed_generation_is_never_rechecked(self):
  events=self.events+[{'msg':'adobe_images.pipeline_completed','request_id':review['RID']}]
  with self.assertRaises(AssertionError):review['validate'](events,self.failure,177,0)
