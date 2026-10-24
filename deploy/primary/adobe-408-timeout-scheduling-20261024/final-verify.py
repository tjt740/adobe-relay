"""Verify current accounts, routing and preserved contracts after cleanup."""
import datetime,hashlib,importlib.util,json,re,subprocess,urllib.request
from pathlib import Path
ROOT=Path('/opt/adobe-relay/releases/adobe-408-timeout-scheduling-20261024')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
sha=(ROOT/'expected-sha').read_text().strip();cleanup=json.loads((ROOT/'rolling-finished.json').read_text())
assert all(cleanup[k] for k in ['temporary_container_removed','temporary_nat_rule_removed','rolling_lease_removed','nginx_configuration_unchanged'])
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
assert subprocess.run(['docker','inspect','adobe-relay-memory-canary'],capture_output=True).returncode!=0
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
assert 'adobe-408-timeout-scheduling-20261024' not in subprocess.check_output(['iptables-save','-t','nat'],text=True)
version=subprocess.check_output(['docker','exec',a.APP,'/app/sub2api','-version'],text=True).strip();assert sha in version
checks=a.check_preserved(json.loads((ROOT/'before.json').read_text()),a.snapshot())
heartbeat=json.loads(a.sql("SELECT json_build_object('at',last_success_at,'healthy',last_success_at>now()-interval '5 minutes' AND (last_error_at IS NULL OR last_success_at>last_error_at)) FROM ops_job_heartbeats WHERE job_name='ops_alert_evaluator'"))
assert heartbeat['healthy'] and datetime.datetime.fromisoformat(heartbeat['at'])>=datetime.datetime.fromisoformat(cleanup['at'])
def read(path):
 with urllib.request.urlopen('https://creative.cheap'+path,timeout=25) as response:
  assert response.status==200
  return response.read().decode()
html=read('/admin/proxies');initial=re.findall(r'<script[^>]+src="([^"]+\.js)"',html);assert initial
assets={p:read(p) for p in initial};names=set()
for content in list(assets.values()):names.update(re.findall(r'(?:ProxiesView|ChangelogView|AccountsView)-[A-Za-z0-9_-]+\.js',content))
assert any(n.startswith('ChangelogView-') for n in names)
for name in names:assets['/assets/'+name]=read('/assets/'+name)
combined='\n'.join(assets.values())
assert all(marker in combined for marker in ['adobe408TimeoutScheduling','clashBackgroundTests','manual_proxy_enabled','clash-background-test-status','adobeRouteReview'])
assert 'account-408-quarantine' not in combined
installed=json.loads(a.sql('SELECT json_object_agg(filename,checksum) FROM schema_migrations'))
old=json.loads((ROOT/'installed-migrations-before.json').read_text());new=json.loads((ROOT/'migration-manifest.json').read_text());assert installed==dict(old,**new)
recheck=json.loads((ROOT/'migration-final-recheck.json').read_text());assert recheck['legacy_writers_stopped'] and recheck['holds']==recheck['blocks']==0
subprocess.run(['/usr/bin/python3',str(ROOT/'account-state.py'),'after'],check=True,timeout=90)
accounts=json.loads((ROOT/'account-state-verification.json').read_text())
business=json.loads((ROOT/'business-verification.json').read_text());credentials=json.loads((ROOT/'credential-change-review.json').read_text())
assert not business['missing_ids'] and business['manual_pause_preserved'] and business['manual_proxy_modes_preserved']
assert all(c['proxy_chain_matches'] for c in business['changes'])
assert all(c['version_advanced'] and c['issuer_compatible'] and c['source_key_compatible'] for c in credentials['cookie_changed_accounts'])
observation=json.loads((ROOT/'observation.json').read_text());assert len(observation['samples'])==13
app=a.inspect(a.APP);assert app['RestartCount']==0 and not app['State']['OOMKilled']
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'application_version':version,'preserved':checks,'accounts':accounts,
 'migration_checksums_preserved':True,'retired_hold_tables_empty':True,'temporary_cleanup':cleanup,'heartbeat_after_cleanup':heartbeat,'passive_samples':13,
 'assets':{k:hashlib.sha256(v.encode()).hexdigest() for k,v in assets.items()},'browser_cases':0,'browser_limitation':'Static resources and read-only API verification; live browser not yet validated',
 'cookie_rotations_with_evidence':len(credentials['cookie_changed_accounts']),'active_adobe_tests':False,'sent_test_email':False}
a.save('final-verification.json',result);print(json.dumps(result,ensure_ascii=False))
