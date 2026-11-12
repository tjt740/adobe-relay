import copy,unittest
from subscription_review_guard import validate_addition
class SubscriptionReview(unittest.TestCase):
 def test_only_confirmed_incremental_addition(self):
  before={'url':' '.join(map(str,range(8))),'nodes':[{'proxy_id':i,'name':str(i),'config':{'x':i}} for i in range(623)],'revision':39}
  current=copy.deepcopy(before);current['url']+=' 8';current['nodes'] += [{'proxy_id':i} for i in range(623,666)];current['revision']=40
  validate_addition(before,current)
  for key,changed in [('url',current['url']+' 9'),('revision',41),('priorities',{'source':100}),('nodes',current['nodes'][1:])]:
   with self.subTest(key=key),self.assertRaises(AssertionError):
    other=copy.deepcopy(current);other[key]=changed;validate_addition(before,other)
  other=copy.deepcopy(current);other['nodes'][0]['config']={'x':99}
  with self.assertRaises(AssertionError):validate_addition(before,other)
