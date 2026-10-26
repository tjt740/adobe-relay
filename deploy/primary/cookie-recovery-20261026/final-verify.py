"""Read-only release verification after temporary routing and container cleanup."""
import datetime,hashlib,importlib.util,json,re,subprocess,urllib.request
from pathlib import Path
from route_guard import validate_excluded_route
ROOT=Path('/opt/adobe-relay/releases/cookie-recovery-20261026')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
sha=(ROOT/'expected-sha').read_text().strip()
cleanup=json.loads((ROOT/'rolling-finished.json').read_text())
assert all(cleanup[k] for k in ['temporary_container_removed','temporary_nat_rule_removed','rolling_lease_removed','nginx_configuration_unchanged'])
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
assert subprocess.run(['docker','inspect','adobe-relay-memory-canary'],capture_output=True).returncode!=0
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
assert 'cookie-recovery-20261026' not in subprocess.check_output(['iptables-save','-t','nat'],text=True)
version=subprocess.check_output(['docker','exec',a.APP,'/app/sub2api','-version'],text=True).strip()
assert 'Sub2API 0.2.17' in version and sha in version, 'Image version and commit must match the checked build'
checks=a.check_preserved(json.loads((ROOT/'before.json').read_text()),a.snapshot())
heartbeat=json.loads(a.sql("SELECT json_build_object('at',last_success_at,'healthy',last_success_at>now()-interval '5 minutes' AND (last_error_at IS NULL OR last_success_at>last_error_at)) FROM ops_job_heartbeats WHERE job_name='ops_alert_evaluator'"))
assert heartbeat['healthy'] and datetime.datetime.fromisoformat(heartbeat['at'])>=datetime.datetime.fromisoformat(cleanup['at']),'Wait for the canonical heartbeat after cleanup'
def read(path):
 with urllib.request.urlopen('https://creative.cheap'+path,timeout=25) as r:
  assert r.status==200
  return r.read().decode()
html=read('/admin/proxies')
initial=re.findall(r'<script[^>]+src="([^"]+\.js)"',html)
assert initial
assets={p:read(p) for p in initial}
names=set()
for content in list(assets.values()):names.update(re.findall(r'(?:ProxiesView|ChangelogView|AccountsView)-[A-Za-z0-9_-]+\.js',content))
assert any(n.startswith('ProxiesView-') for n in names) and any(n.startswith('ChangelogView-') for n in names)
for name in names:assets['/assets/'+name]=read('/assets/'+name)
combined='\n'.join(assets.values())
assert all(marker in combined for marker in ['subscription-row','clash-bulk-urls','admin.proxies.clash.checkSource','clashSubscriptions','upstreamV0217','proxyVerificationPriority','adobeRouteReview','proxyHealth.nodeBalance','blocked_proxy_ids','manual-proxy-toggle','manual_proxy_enabled','clash-background-test-status','admin.proxies.clash.runInBackground','clashBackgroundTests','adobe408TimeoutScheduling','bulk-test-start','bulk-test-model','bulk-test-background-model','bulkAccountTestModels','adobeAutomaticTestFlare','gpt-image-2.5-flare','cookieRecoveryRetries'])
browser_file=ROOT/'live-browser-verification.json'
browser=json.loads(browser_file.read_text()) if browser_file.exists() else {'case_count':0,'attempted_writes':[],'runtime_errors':[],'limitation':'Live authenticated browser verification unavailable; asset and API checks only'}
assert not browser['attempted_writes'] and not browser['runtime_errors']
observation=json.loads((ROOT/'observation.json').read_text());assert len(observation['samples'])==13
business=json.loads((ROOT/'business-verification.json').read_text())
credentials=json.loads((ROOT/'credential-change-review.json').read_text())
assert not business['missing_ids'], 'Review concurrent deletions before final verification'
assert business['manual_pause_preserved'] and business['manual_proxy_modes_preserved'] and all(c['proxy_chain_matches'] for c in business['changes'])
assert all(c['version_advanced'] and c['issuer_compatible'] and c['source_key_compatible'] for c in credentials['cookie_changed_accounts'])
summary={'backup_accounts':business['before_count'],'checked_accounts':business['current_count'],'manual_pauses_preserved':True,'manual_proxy_modes_preserved':True,'proxy_bindings_changed_with_event_chain':sum('proxy_id' in c['fields'] for c in business['changes']),'cookie_rotations_with_version_advance':len(credentials['cookie_changed_accounts']),'token_fields_only_changes':len(credentials['token_fields_only_accounts'])}
installed=json.loads(a.sql("SELECT json_object_agg(filename,checksum) FROM schema_migrations"))
expected=json.loads((ROOT/'installed-migrations-before.json').read_text())
expected['255_adobe_cookie_recovery_three_attempts.sql']='7a29ad94c1738a4463f9ce3723fcc393b54f516527441ee1dbe5396271277158'
assert installed==expected,'Migration set or old checksum changed'
assert any(name.startswith('254_') for name in installed),'Missing deployed 408 migration'
retired=json.loads(a.sql("SELECT json_build_object('holds',(SELECT count(*) FROM adobe_proxy_408_holds),'blocks',(SELECT count(*) FROM adobe_proxy_408_blocks),'flags',(SELECT count(*) FROM accounts WHERE extra ? 'adobe_proxy_408'))"))
assert all(value==0 for value in retired.values()),'Retired 408 scheduling gates reappeared'

key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/proxies/clash/health',headers={'x-api-key':key})
with urllib.request.urlopen(req,timeout=20) as response:health=json.load(response)['data']
assert len(health['sources'])>=3
source1290=next(s for s in health['sources'] if 1290 in s['proxy_ids'])
assert source1290['id']=='6dcfb89fdc6d6eef04362357' and 1290 in source1290['blocked_proxy_ids']
for source in health['sources']:
 assert all(k in source for k in ['traffic_known','remaining_bytes','total_bytes','traffic_observed_at','expires_at','generation_blocked','proxy_ids','blocked_proxy_ids'])
 if source['traffic_known']:assert 0<=source['remaining_bytes']<=source['total_bytes']
 else:assert source['remaining_bytes'] is None and source['total_bytes'] is None
exclusion=json.loads(a.sql("SELECT json_build_object('excluded',EXISTS(SELECT 1 FROM adobe_route_exclusions WHERE proxy_id=1290),'status',(SELECT status FROM proxies WHERE id=1290),'bindings',(SELECT count(*) FROM accounts WHERE deleted_at IS NULL AND proxy_id=1290),'leases',(SELECT count(*) FROM adobe_proxy_test_leases WHERE proxy_id=1290))"))
review=json.loads((ROOT/'route-guard-review.json').read_text())
assert review['cutover_backup']==json.loads((ROOT/'after.json').read_text())['backup']
route_checks=validate_excluded_route(review,exclusion)
assert a.inspect(a.APP)['RestartCount']==0 and not a.inspect(a.APP)['State']['OOMKilled']
health_proof={'sources':health['sources'],'excluded_1290':exclusion,'cutover_route_checks':route_checks,'active_adobe_requests':False,'http_writes':False}
a.save('route-api-verification.json',health_proof)

out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'application_version':version,'preserved':checks,'business_preservation':summary,'migration_checksums_preserved':True,'new_retry_migration_verified':True,'retired_408_gates_empty':True,'canonical_assets':{p:hashlib.sha256(v.encode()).hexdigest() for p,v in assets.items()},'live_browser_cases':browser['case_count'],'browser_limitation':browser.get('limitation'),'heartbeat_after_temporary_process_cleanup':heartbeat,'passive_sample_count':13,'temporary_cleanup':cleanup,'adobe_custom_business_preserved':True,'route_api_verified':True,'excluded_1290':exclusion,'cutover_route_checks':route_checks,'active_adobe_probes':False,'test_mail_sent':False}
a.save('final-verification.json',out)
print(json.dumps(out,ensure_ascii=False))
