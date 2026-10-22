"""Read-only release verification after temporary routing and container cleanup."""
import datetime,hashlib,importlib.util,json,re,subprocess,urllib.request
from pathlib import Path
ROOT=Path('/opt/adobe-relay/releases/upstream-v0217-20261022')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
sha=(ROOT/'expected-sha').read_text().strip()
cleanup=json.loads((ROOT/'rolling-finished.json').read_text())
assert all(cleanup[k] for k in ['temporary_container_removed','temporary_nat_rule_removed','rolling_lease_removed','nginx_configuration_unchanged'])
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
assert subprocess.run(['docker','inspect','adobe-relay-memory-canary'],capture_output=True).returncode!=0
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
assert 'upstream-v0217-20261022' not in subprocess.check_output(['iptables-save','-t','nat'],text=True)
version=subprocess.check_output(['docker','exec',a.APP,'/app/sub2api','-version'],text=True).strip()
assert 'Sub2API 0.2.17' in version and sha in version, 'Image version and commit must match the checked build'
before_snapshot=json.loads((ROOT/'before.json').read_text())
latest_snapshot=a.snapshot()
checks=a.check_preserved(before_snapshot,latest_snapshot)
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
for content in list(assets.values()):names.update(re.findall(r'(?:ProxiesView|ChangelogView)-[A-Za-z0-9_-]+\.js',content))
assert any(n.startswith('ProxiesView-') for n in names) and any(n.startswith('ChangelogView-') for n in names)
for name in names:assets['/assets/'+name]=read('/assets/'+name)
combined='\n'.join(assets.values())
assert all(marker in combined for marker in ['subscription-row','clash-bulk-urls','admin.proxies.clash.checkSource','clashSubscriptions','upstreamV0217'])
browser_file=ROOT/'live-browser-verification.json'
browser=json.loads(browser_file.read_text()) if browser_file.exists() else {'case_count':0,'attempted_writes':[],'runtime_errors':[],'limitation':'Live authenticated browser verification unavailable; asset and API checks only'}
assert not browser['attempted_writes'] and not browser['runtime_errors']
observation=json.loads((ROOT/'observation.json').read_text());assert len(observation['samples'])==13
business=json.loads((ROOT/'business-verification.json').read_text())
credentials=json.loads((ROOT/'credential-change-review.json').read_text())
assert not business['missing_ids'] and business['manual_pause_preserved'] and all(c['proxy_chain_matches'] for c in business['changes'])
state_review=json.loads((ROOT/'reviewed-live-state-changes.json').read_text())
changed_ids=sorted(c['id'] for c in business['changes'] if set(c['fields'])&{'status','schedulable'})
assert changed_ids==state_review['reviewed_ids']==[1150,1151,1154,1156]
assert state_review['business_verification_sha256']==hashlib.sha256((ROOT/'business-verification.json').read_bytes()).hexdigest()
assert state_review['all_original_failure_gates_verified'] and state_review['manual_pauses_preserved']
initial_accounts={r['id']:r for r in json.loads((ROOT/'cutover-account-state.json').read_text())}
latest_accounts={r['id']:r for r in latest_snapshot['account_state']}
assert set(initial_accounts)==set(latest_accounts), 'Review any live account additions or removals'
assert all(initial_accounts[i]['manual_pause']==latest_accounts[i]['manual_pause'] for i in initial_accounts), 'Preserve and review current operator actions'
latest_changed_ids={i for i in initial_accounts if any(initial_accounts[i][k]!=latest_accounts[i][k] for k in ['status','schedulable'])}
latest_state_differences=[{'id':i,'before_status':initial_accounts[i]['status'],'before_schedulable':initial_accounts[i]['schedulable'],'current_status':latest_accounts[i]['status'],'current_schedulable':latest_accounts[i]['schedulable']} for i in sorted(latest_changed_ids)]
assert latest_changed_ids <= set(state_review['reviewed_ids']), 'Review additional live state transitions: '+json.dumps(latest_state_differences)
assert all(latest_accounts[r['id']]['status']==r['current_status'] and latest_accounts[r['id']]['schedulable']==r['current_schedulable'] for r in state_review['accounts'])
assert all(set(c['fields']) <= {'credentials','proxy_id','status','schedulable'} for c in business['changes'])
assert all(c['version_advanced'] and c['issuer_compatible'] and c['source_key_compatible'] for c in credentials['cookie_changed_accounts'])
schema=json.loads((ROOT/'schema-business-verification.json').read_text())
assert all(schema['business_contracts'].values()) and schema['history_checksums_preserved']
summary={'backup_accounts':business['before_count'],'checked_accounts':business['current_count'],'manual_pauses_preserved':True,'proxy_bindings_changed_with_event_chain':sum('proxy_id' in c['fields'] for c in business['changes']),'cookie_rotations_with_version_advance':len(credentials['cookie_changed_accounts']),'token_fields_only_changes':len(credentials['token_fields_only_accounts']),'reviewed_live_failure_state_changes':len(changed_ids)}
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'application_version':version,'preserved':checks,'business_preservation':summary,'schema_and_bindings':schema,'canonical_assets':{p:hashlib.sha256(v.encode()).hexdigest() for p,v in assets.items()},'live_browser_cases':browser['case_count'],'browser_limitation':browser.get('limitation'),'heartbeat_after_temporary_process_cleanup':heartbeat,'passive_sample_count':13,'temporary_cleanup':cleanup,'latest_account_ids_and_manual_states_preserved':True,'latest_account_states_compared_with_actual_cutover':True,'adobe_custom_business_preserved':True,'active_adobe_probes':False,'test_mail_sent':False}
a.save('final-verification.json',out)
print(json.dumps(out,ensure_ascii=False))
