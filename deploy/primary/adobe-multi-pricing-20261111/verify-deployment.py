"""Verify deployment and API persistence; do not claim real upstream billing."""
import datetime,hashlib,importlib.util,json,re,subprocess,urllib.request
from pathlib import Path
from route_guard import validate_excluded_route, validate_subscription_exclusion
ROOT=Path('/opt/adobe-relay/releases/adobe-multi-pricing-20261111')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
sha=(ROOT/'expected-sha').read_text().strip()
cleanup=json.loads((ROOT/'rolling-finished.json').read_text())
assert all(cleanup[k] for k in ['temporary_container_removed','temporary_nat_rule_removed','rolling_lease_removed','nginx_configuration_unchanged'])
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
assert subprocess.run(['docker','inspect','adobe-relay-memory-canary'],capture_output=True).returncode!=0
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
assert 'adobe-multi-pricing-20261111' not in subprocess.check_output(['iptables-save','-t','nat'],text=True)
version=subprocess.check_output(['docker','exec',a.APP,'/app/sub2api','-version'],text=True).strip()
assert 'Sub2API 0.2.19' in version and sha in version, 'Image version and commit must match the checked build'
api=json.loads((ROOT/'api-verification.json').read_text());assert api['passed'] and api['temporary_group_deleted']
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
for content in list(assets.values()):names.update(re.findall(r'(?:ProxiesView|ChangelogView|AccountsView|GroupsView|index)-[A-Za-z0-9_-]+\.js',content))
assert any(n.startswith('ProxiesView-') for n in names) and any(n.startswith('ChangelogView-') for n in names)
for name in names:assets['/assets/'+name]=read('/assets/'+name)
locale_assets={p:content for p,content in assets.items() if p.rsplit('/',1)[-1].startswith('index-') and all(marker in content for marker in ['adobeGroupMultiPricing','basePreview','gpt-image-2.6-flare'])}
assert len(locale_assets)==2, 'Verify both lazily loaded Chinese and English locale bundles'
combined='\n'.join(assets.values())
assert 'tablist' in combined and 'tabpanel' in combined and 'image-quality-pricing' in combined and 'image_quality_pricing_enabled' in combined and 'image_quality_prices' in combined and 'image_quality' in combined, 'New pricing and usage controls must be in deployed assets'
assert all(marker in combined for marker in ['subscription-row','clash-bulk-urls','admin.proxies.clash.checkSource','clashSubscriptions','upstreamV0217','proxyVerificationPriority','adobeRouteReview','proxyHealth.nodeBalance','blocked_proxy_ids','manual-proxy-toggle','manual_proxy_enabled','clash-background-test-status','admin.proxies.clash.runInBackground','clashBackgroundTests','adobe408TimeoutScheduling','bulk-test-start','bulk-test-model','bulk-test-background-model','bulkAccountTestModels','adobeAutomaticTestFlare','gpt-image-2.5-flare','cookieRecoveryRevert','adobeImageErrorDetails','bulk-test-concurrency','bulk-test-background-concurrency','bulkTestConcurrency','clashNodeCount','2026-11-04','adobeAutoMedium','2026-11-05','adobeGptImage26Gif','complexity','frame_duration_ms','gpt-image-2.6-flare','gpt-image-2.6-sunburst','adobeImage26Quality','2026-11-07'])
browser_file=ROOT/'live-browser-verification.json'
browser=json.loads(browser_file.read_text()) if browser_file.exists() else {'case_count':0,'attempted_writes':[],'runtime_errors':[],'limitation':'Live authenticated browser verification unavailable; asset and API checks only'}
assert not browser['attempted_writes'] and not browser['runtime_errors']
observation=json.loads((ROOT/'observation.json').read_text());assert len(observation['samples'])==13
business=json.loads((ROOT/'business-verification.json').read_text())
credentials=json.loads((ROOT/'credential-change-review.json').read_text())
assert not business['missing_ids'], 'Review concurrent deletions before final verification'
assert business['manual_pause_preserved'] and business['manual_proxy_modes_preserved'] and all(c['proxy_chain_matches'] for c in business['changes'])
assert all(c['version_advanced'] and c['issuer_compatible'] and c['source_key_compatible'] for c in credentials['cookie_changed_accounts'])
summary={'backup_accounts':business['before_count'],'checked_accounts':business['current_count'],'manual_pauses_preserved':True,'manual_proxy_modes_preserved':True,'proxy_bindings_changed_with_event_chain':sum('proxy_id' in c['fields'] for c in business['changes']),'cookie_rotations_with_version_advance':len(credentials['cookie_changed_accounts']),'non_source_cookie_credential_changes':len(credentials['token_fields_only_accounts']), 'credential_field_names_changed':sorted({key for change in business['changes'] for key in change['credential_keys_changed']})}
installed=json.loads(a.sql("SELECT json_object_agg(filename,checksum) FROM schema_migrations"))
expected=json.loads((ROOT/'installed-migrations-before.json').read_text())
new_migrations=json.loads((ROOT/'migration-manifest.json').read_text())
assert installed=={**expected,**new_migrations}, 'Unexpected migration set or checksum changed'
assert '255_adobe_cookie_recovery_three_attempts.sql' in installed,'Applied migration must remain'
assert any(name.startswith('254_') for name in installed),'Missing deployed 408 migration'
retired=json.loads(a.sql("SELECT json_build_object('holds',(SELECT count(*) FROM adobe_proxy_408_holds),'blocks',(SELECT count(*) FROM adobe_proxy_408_blocks),'flags',(SELECT count(*) FROM accounts WHERE extra ? 'adobe_proxy_408'))"))
assert all(value==0 for value in retired.values()),'Retired 408 scheduling gates reappeared'

key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
expected_labels={'gpt-image-2.6-flare':'GPT Image 2.6 Flare','gpt-image-2.6-sunburst':'GPT Image 2.6 Sunburst'}
admin_labels={}
account_ids=json.loads(a.sql("SELECT json_agg(id) FROM (SELECT id FROM accounts WHERE platform='adobe' AND status='active' AND schedulable AND deleted_at IS NULL ORDER BY id LIMIT 1) x"))
assert account_ids
for account_id in account_ids:
 req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/'+str(account_id)+'/models',headers={'x-api-key':key})
 with urllib.request.urlopen(req,timeout=20) as response:models=json.load(response)['data']
 labels={row['id']:row['display_name'] for row in models}
 assert not {'gpt-image-2.6-star-swirl','gpt-image-2.6-aurora'}.intersection(labels)
 for model,label in expected_labels.items():
  assert labels.get(model)==label
  admin_labels[model]=label
assert admin_labels==expected_labels
req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/proxies/clash/health',headers={'x-api-key':key})
with urllib.request.urlopen(req,timeout=20) as response:health=json.load(response)['data']
assert len(health['sources'])>=3
subscription_exclusion=validate_subscription_exclusion(health['sources'])
for source in health['sources']:
 assert all(k in source for k in ['traffic_known','remaining_bytes','total_bytes','traffic_observed_at','expires_at','generation_blocked','proxy_ids','blocked_proxy_ids'])
 if source['traffic_known']:assert 0<=source['remaining_bytes']<=source['total_bytes']
 else:assert source['remaining_bytes'] is None and source['total_bytes'] is None
exclusion=json.loads(a.sql("SELECT json_build_object('excluded',EXISTS(SELECT 1 FROM adobe_route_exclusions WHERE proxy_id=1290),'status',(SELECT status FROM proxies WHERE id=1290),'bindings',(SELECT count(*) FROM accounts WHERE deleted_at IS NULL AND proxy_id=1290),'leases',(SELECT count(*) FROM adobe_proxy_test_leases WHERE proxy_id=1290))"))
review=json.loads((ROOT/'route-guard-review.json').read_text())
assert review['cutover_backup']==json.loads((ROOT/'after.json').read_text())['backup']
route_checks=validate_excluded_route(review,exclusion)
assert a.inspect(a.APP)['RestartCount']==0 and not a.inspect(a.APP)['State']['OOMKilled']
health_proof={'sources':health['sources'],'subscription_exclusion':subscription_exclusion,'excluded_1290':exclusion,'cutover_route_checks':route_checks,'active_adobe_requests':False,'http_writes':False}
a.save('route-api-verification.json',health_proof)

route_at=datetime.datetime.fromisoformat(json.loads((ROOT/'rolling-route-canary.json').read_text())['at']);assert route_at.astimezone(datetime.timezone(datetime.timedelta(hours=8))).date().isoformat()=='2026-11-11', 'Release crossed its Beijing date; rebuild notes before claiming success'
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'application_version':version,'preserved':checks,'business_preservation':summary,'migration_checksums_preserved':True,'migration_set_retained':True,'retired_408_gates_empty':True,'canonical_assets':{p:hashlib.sha256(v.encode()).hexdigest() for p,v in assets.items()},'live_browser_cases':browser['case_count'],'browser_limitation':browser.get('limitation'),'heartbeat_after_temporary_process_cleanup':heartbeat,'passive_sample_count':13,'temporary_cleanup':cleanup,'api_verification':api,'route_api_verified':True,'excluded_1290':exclusion,'cutover_route_checks':route_checks,'live_billing_verified':False,'live_generation_tested':False,'test_mail_sent':False,'verified_locale_bundle_count':len(locale_assets),'verified_admin_model_labels':admin_labels,'status':'deployed_api_verified'}
a.save('deployment-verification.json',out)
print(json.dumps(out,ensure_ascii=False))
