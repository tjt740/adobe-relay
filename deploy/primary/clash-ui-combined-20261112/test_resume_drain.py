import importlib.util,pathlib,unittest
s=importlib.util.spec_from_file_location('resume',pathlib.Path(__file__).with_name('resume-drain.py'));r=importlib.util.module_from_spec(s);s.loader.exec_module(r)
class ResumeDrain(unittest.TestCase):
 def test_only_the_existing_failed_drain_can_continue(self):
  args=[{'step':'failed','error':"Command ['rolling.py', 'drain-canary'] failed"},{'candidate_sha':r.SHA,'canary_id':'same'},'adobe-relay:'+r.SHA,'same',True,False,False]
  r.validate(*args)
  for i,value in [(0,{'step':'failed','error':'prepare failed'}),(1,{'candidate_sha':'other','canary_id':'same'}),(2,'other-image'),(3,'other-container'),(4,False),(5,True),(6,True)]:
   with self.subTest(i=i),self.assertRaises(AssertionError):
    modified=args.copy();modified[i]=value;r.validate(*modified)
