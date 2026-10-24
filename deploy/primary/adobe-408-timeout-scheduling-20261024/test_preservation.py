import ast,copy,json
from pathlib import Path
import unittest
ROOT=Path(__file__).parent
ns={}
tree=ast.parse((ROOT/'account-state.py').read_text())
exec(compile(ast.Module(body=[n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=='compare'],type_ignores=[]),'<comparison>','exec'),ns)
compare=ns['compare']

class PreservationTests(unittest.TestCase):
 def fixture(self):
  row={'id':1,'status':'error','schedulable':False,'proxy_id':None,'credentials_hash':'same','manual_pause':False,'manual_proxy':False,'verification':'verified','code':408,'success':False,'gate':True,'test_state':'failed','groups':[7],'intent_hash':'same'}
  before={'at':'before','accounts':[row]}
  after={'at':'after','accounts':[dict(row,status='active',schedulable=True,proxy_id=8,gate=False)],'allocation':{},'groups':[]}
  return before,after
 def test_owned_pause_returns_without_fabricating_success(self):
  b,a=self.fixture();r=compare(b,a,[1]);self.assertEqual(r['active_schedulable'],1);self.assertEqual(r['target_accounts'][0]['code'],408)
 def test_removal_or_independent_changes_require_review(self):
  for mutate in [lambda b,a:a['accounts'].clear(),lambda b,a:a['accounts'][0].update(manual_pause=True),lambda b,a:a['accounts'][0].update(manual_proxy=True),lambda b,a:a['accounts'][0].update(intent_hash='changed'),lambda b,a:a['accounts'][0].update(groups=[]),lambda b,a:a['accounts'][0].update(gate=True),lambda b,a:a['accounts'][0].update(code=200),lambda b,a:a['accounts'][0].update(status='error')]:
   with self.subTest(mutate=mutate):
    b,a=self.fixture();mutate(b,a)
    with self.assertRaises(AssertionError):compare(b,a,[1])
 def test_manual_pause_and_quota_are_never_resumed(self):
  for field,value in [('manual_pause',True),('verification','quota_blocked')]:
   b,a=self.fixture();b['accounts'][0][field]=a['accounts'][0][field]=value
   with self.assertRaises(AssertionError):compare(b,a,[])
 def test_operator_route_cannot_move(self):
  b,a=self.fixture();b['accounts'][0]['manual_proxy']=a['accounts'][0]['manual_proxy']=True
  b['accounts'][0]['proxy_id']=7
  with self.assertRaises(AssertionError):compare(b,a,[])
 def test_release_orders_backup_and_old_writer_recheck(self):
  s=(ROOT/'driver.py').read_text()
  self.assertLess(s.index("run_step('pre-migration-backup'"),s.index("run_step('prepare-candidate'"))
  self.assertLess(s.index("run_step('canonical-recreate'"),s.index("run_step('retired-writer-migration'"))
  self.assertLess(s.index("run_step('retired-writer-migration'"),s.index("for step in ['route-canonical'"))
  self.assertIn('--keep-canary-on-failure',s)
 def test_recheck_proves_old_exit_and_exact_migrations(self):
  s=(ROOT/'migration-recheck.py').read_text()
  self.assertLess(s.index("assert row['Image']!=a.OLD_IMAGE"),s.index('p=subprocess.run(cmd'))
  self.assertIn("installed==dict(expected,**{name:digest})",s)
  self.assertIn("SET LOCAL lock_timeout='15s'",s)
  self.assertIn("result=={'holds':0,'blocks':0}",s)
 def test_workflow_gates_production_on_exact_successful_build(self):
  s=(ROOT.parents[2]/'.github/workflows/adobe-408-timeout-primary.yml').read_text()
  self.assertIn("r['head_sha']==sys.argv[1] and r['conclusion']=='success'",s)
  self.assertIn('git diff --exit-code "$APP_SHA" -- backend frontend Dockerfile',s)
  self.assertIn('group: adobe-relay-production',s)
  self.assertNotIn('  push:',s)
 def test_previous_image_is_not_an_automatic_rollback_target(self):
  s=(ROOT/'audit.py').read_text();self.assertNotIn("'check-rollback', 'rollback'",s)

if __name__=='__main__':unittest.main()
