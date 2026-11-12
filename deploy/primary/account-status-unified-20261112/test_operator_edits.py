import copy,importlib.util,json,pathlib,unittest
spec=importlib.util.spec_from_file_location('review',pathlib.Path(__file__).with_name('review-operator-edits.py'));r=importlib.util.module_from_spec(spec);spec.loader.exec_module(r)
def fixture():
 ids=[*r.PAUSE_IDS,*r.MAPPING_IDS,999999]
 old={i:{'manual_pause':None,'manual_proxy_enabled':False,'mapping':{'old':'old'}} for i in ids}
 current=copy.deepcopy(old)
 for i in r.PAUSE_IDS:current[i].update(manual_pause=True,schedulable=False)
 for i in r.MAPPING_IDS:current[i].update(mapping={'new':'new'},concurrency=1,group_ids=[2,3])
 review={'at':'2026-11-12T06:29:40+00:00','cutover_backup':'/backup/20261112T062115Z-'+r.APP_SHA,'missing_ids':[],'manual_proxy_modes_preserved':True,'changes':[{'id':i,'fields':['manual_pause'] if i in r.PAUSE_IDS else ['credentials'],'credential_keys_changed':['model_mapping'] if i in r.MAPPING_IDS else [],'proxy_chain_matches':True} for i in ids if i!=999999]}
 events=[]
 for eid in [*r.PAUSE_EVENTS,*r.BULK_EVENTS]:
  event={'id':eid,'created_at':'2026-11-12T06:22:00+00:00','actor_user_id':1,'actor_role':'admin','auth_method':'jwt','status_code':200,'method':'POST'}
  if eid in r.PAUSE_EVENTS:event.update(path='/api/v1/admin/accounts/:id/schedulable',action='admin.accounts.schedulable.create',extra={'params':{'id':str(r.PAUSE_EVENTS[eid])}},request_body=json.dumps({'schedulable':False}))
  else:event.update(path='/api/v1/admin/accounts/bulk-update',action='admin.accounts.bulk_update.create',request_body=json.dumps({'account_ids':r.BULK_EVENTS[eid],'credentials':{'model_mapping':{'new':'new'}},'concurrency':1,'group_ids':[2,3]}))
  events.append(event)
 return review,old,current,events
class OperatorReview(unittest.TestCase):
 def test_exact_authenticated_current_edits(self):r.validate(*fixture())
 def test_rejects_wrong_actor_failed_action_or_unrelated_evidence(self):
  for key,value in [('actor_user_id',2),('actor_role','user'),('auth_method','admin_api_key'),('status_code',403),('method','GET'),('path','/other'),('action','other'),('created_at','2026-11-12T06:20:00+00:00'),('created_at','2026-11-12T06:30:00+00:00'),('request_body','{"schedulable":true}'),('extra',{'params':{'id':'99'}})]:
   with self.subTest(key=key,value=value):
    args=fixture();args[3][0][key]=value
    with self.assertRaises(AssertionError):r.validate(*args)
 def test_rejects_missing_or_additional_live_changes(self):
  def change_pause(args):args[2][999999]['manual_pause']=True
  def change_mapping(args):args[2][999999]['mapping']={'unreviewed':'change'}
  def change_groups(args):args[2][r.MAPPING_IDS[0]]['group_ids']=[7]
  def change_concurrency(args):args[2][r.MAPPING_IDS[0]]['concurrency']=9
  def change_mapping_value(args):args[2][r.MAPPING_IDS[0]]['mapping']={'another':'model'}
  def change_manual_proxy(args):args[2][999999]['manual_proxy_enabled']=True
  def missing_account(args):del args[2][999999]
  def missing_event(args):args[3].pop()
  def unmatched_proxy(args):args[0]['changes'][0]['proxy_chain_matches']=False
  for mutate in [change_pause,change_mapping,change_groups,change_concurrency,change_mapping_value,change_manual_proxy,missing_account,missing_event,unmatched_proxy]:
   with self.subTest(mutate=mutate.__name__):
    args=fixture();mutate(args)
    with self.assertRaises(AssertionError):r.validate(*args)
