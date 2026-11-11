import importlib.util, pathlib, tempfile, unittest, json
s=importlib.util.spec_from_file_location('observer',pathlib.Path(__file__).with_name('mihomo-observer.py'));m=importlib.util.module_from_spec(s);s.loader.exec_module(m)
class ObserverTests(unittest.TestCase):
 def test_counter_baseline_restart_oom_and_replacement(self):
  a={'container_id':'a','running':True,'started_at':'one','restart_count':51,'oom_events':4,'oom_kills':3,'memory_current_bytes':10,'memory_limit_bytes':100}
  first=m.next_snapshot(a,{},'2026-11-12T00:00:00Z');self.assertNotIn('last_oom_at',first);self.assertNotIn('last_restart_at',first)
  same=m.next_snapshot(a,first,'2026-11-12T00:01:00Z');self.assertNotIn('last_restart_at',same)
  b=dict(a,oom_kills=4,restart_count=52,started_at='two');changed=m.next_snapshot(b,same,'2026-11-12T00:02:00Z');self.assertEqual(changed['last_oom_at'],changed['observed_at']);self.assertEqual(changed['last_restart_at'],changed['observed_at'])
  replacement=m.next_snapshot(dict(a,container_id='b',oom_events=0,oom_kills=0,restart_count=0),changed,'2026-11-12T00:03:00Z');self.assertEqual(replacement['last_restart_at'],replacement['observed_at']);self.assertEqual(replacement['last_oom_at'],changed['last_oom_at'])
 def test_atomic_public_snapshot_has_requested_permissions(self):
  with tempfile.TemporaryDirectory()as d:
   p=pathlib.Path(d)/'sample.json';m.atomic_write(p,{'sample_ok':True},0o644);self.assertEqual(json.loads(p.read_text()),{'sample_ok':True});self.assertEqual(p.stat().st_mode&0o777,0o644);self.assertFalse(p.with_suffix('.tmp').exists())
if __name__=='__main__':unittest.main()
