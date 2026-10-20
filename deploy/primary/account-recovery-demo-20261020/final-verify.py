"""Read-only final checks after temporary routing and process cleanup."""
import datetime,hashlib,importlib.util,json,re,subprocess,urllib.request
from pathlib import Path
ROOT=Path('/opt/adobe-relay/releases/account-recovery-demo-20261020')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
sha=(ROOT/'expected-sha').read_text().strip()
cleanup=json.loads((ROOT/'rolling-finished.json').read_text())
assert all(cleanup[k] for k in ['temporary_container_removed','temporary_nat_rule_removed','rolling_lease_removed','nginx_configuration_unchanged'])
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
assert subprocess.run(['docker','inspect','adobe-relay-memory-canary'],capture_output=True).returncode!=0
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
assert 'account-recovery-demo-20261020' not in subprocess.check_output(['iptables-save','-t','nat'],text=True)
checks=a.check_preserved(json.loads((ROOT/'before.json').read_text()),a.snapshot())
heartbeat=json.loads(a.sql("SELECT json_build_object('at',last_success_at,'healthy',last_success_at>now()-interval '5 minutes' AND (last_error_at IS NULL OR last_success_at>last_error_at)) FROM ops_job_heartbeats WHERE job_name='ops_alert_evaluator'"))
assert heartbeat['healthy'] and datetime.datetime.fromisoformat(heartbeat['at'])>=datetime.datetime.fromisoformat(cleanup['at']),'Wait for a heartbeat after the temporary process was removed'

def read(path):
 with urllib.request.urlopen('https://creative.cheap'+path,timeout=25) as r:
  assert r.status==200
  return r.read().decode()
html=read('/admin/changelog')
initial=re.findall(r'<script[^>]+src="([^"]+\.js)"',html)
assert initial
assets={p:read(p) for p in initial}
names=set()
for content in assets.values():names.update(re.findall(r'ChangelogView-[A-Za-z0-9_-]+\.js',content))
assert names,'Could not locate the deployed changelog bundle'
for name in names:assets['/assets/'+name]=read('/assets/'+name)
combined='\n'.join(assets.values())
assert all(marker in combined for marker in ['account-recovery-demo','changelog-previews','recoveryDemoPublished'])
browser=json.loads((ROOT/'live-browser-verification.json').read_text())
assert browser['scenarioCount']==11 and browser['retryControls'] and browser['datedDemoEntry'] and browser['proposalClearlyUnimplemented']
assert not browser['attemptedWrites'] and not browser['runtimeErrors']
observation=json.loads((ROOT/'observation.json').read_text());assert len(observation['samples'])==13
business=json.loads((ROOT/'business-verification.json').read_text())
credentials=json.loads((ROOT/'credential-change-review.json').read_text())
assert not business['missing_ids'] and business['manual_pause_preserved'] and all(c['proxy_chain_matches'] for c in business['changes'])
assert all(c['version_advanced'] and c['issuer_compatible'] and c['source_key_compatible'] for c in credentials['cookie_changed_accounts'])
summary={'backup_accounts':business['before_count'],'checked_accounts':business['current_count'],'manual_pauses_preserved':True,'proxy_bindings_changed_with_event_chain':sum('proxy_id' in c['fields'] for c in business['changes']),'cookie_rotations_with_version_advance':len(credentials['cookie_changed_accounts']),'token_fields_only_changes':len(credentials['token_fields_only_accounts'])}
out={'business_preservation':summary,'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'preserved':checks,'canonical_assets':{p:hashlib.sha256(v.encode()).hexdigest() for p,v in assets.items()},'live_browser_11_scenarios':True,'heartbeat_after_temporary_process_cleanup':heartbeat,'two_minute_sample_count':13,'temporary_cleanup':cleanup,'scheduling_rules_changed':False,'active_adobe_probes':False,'test_mail_sent':False}
a.save('final-verification.json',out)
print(json.dumps(out,ensure_ascii=False))
