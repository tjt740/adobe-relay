"""Verify SS2022 in the newer shared release; never repeat its deployment."""
import datetime,hashlib,json,pathlib,subprocess,urllib.request
ROOT=pathlib.Path('/opt/adobe-relay/releases/clash-ss2022-20261112')
SHARED=pathlib.Path('/opt/adobe-relay/releases/proxy-reliability-20261112')
SHA='47c5c46ead01a03cbfa7c547e4da971a561f86c7'
def read(name):return json.loads((SHARED/name).read_text())
def inspect(name):return json.loads(subprocess.check_output(['docker','inspect',name]))[0]
assert not (ROOT/'progress.json').exists(),'Do not join a release after starting a separate handoff'
core=read('deployment-verification.json');loaded=read('image-loaded.json');api=read('api-verification.json')
assert core['application_sha']==SHA and core['status']=='deployed_api_verified'
assert core['official_workflow_completed'] and core['validation_mode']=='successful_github_workflow'
assert loaded['sha']==SHA and loaded['workflow_run']==43104288465
app=inspect('adobe-relay-app-1')
assert app['Config']['Image']=='adobe-relay:'+SHA and app['Image']==loaded['image_id']
assert app['State']['Health']['Status']=='healthy' and app['RestartCount']==0 and not app['State']['OOMKilled']
assert api['application_sha']==SHA and api['passed'] and api['public_https_preview'] and api['base64_ss2022_nodes']==3
assert api['existing_subscription_unchanged'] and api['node_imports']==0 and api['outbound_probes']==0
cleanup=read('rolling-finished.json')
assert all(cleanup[k] for k in ['temporary_container_removed','temporary_nat_rule_removed','rolling_lease_removed','nginx_configuration_unchanged'])
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
for side in ['old','canary']:
 drain=read('rolling-'+side+'-drained.json')
 assert drain['drained']==side and drain['stable_zero_seconds']>=45
 assert all(drain['samples'][-1][k]==0 for k in ['remaining_tracked_slots','pending_input_sockets','running_automatic_checks'])
m=inspect('adobe-relay-mihomo-1');before=read('before.json')['containers']['adobe-relay-mihomo-1']
assert m['Id']==before['id'] and m['State']['StartedAt']==before['started'] and m['RestartCount']==before['restarts']==51
assert m['HostConfig']['Memory']==1610612736 and m['HostConfig']['MemorySwap']==3221225472
locale={}
for path in core['canonical_assets']:
 if not path.rsplit('/',1)[-1].startswith('index-'):continue
 with urllib.request.urlopen('https://creative.cheap'+path,timeout=25) as r:
  assert r.status==200;body=r.read()
 if all(x.encode() in body for x in ['clashShadowsocks','mihomoMemory','1.5 GiB']):locale[path]=hashlib.sha256(body).hexdigest()
assert len(locale)==2 and all(core['canonical_assets'][p]==digest for p,digest in locale.items())
assert core['passive_sample_count']==13 and core['business_preservation']['manual_pauses_preserved'] and core['business_preservation']['manual_proxy_modes_preserved']
docs=subprocess.check_output(['systemctl','show','creative-docs.service','--property=MainPID,ExecMainStartTimestampMonotonic,ActiveState,ActiveEnterTimestampMonotonic'],text=True).strip()
assert 'MainPID=3681084' in docs and 'ExecMainStartTimestampMonotonic=2221275857118' in docs and 'ActiveState=active' in docs
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'status':'deployed_ss2022_preview_verified_in_shared_release','application_sha':SHA,'tested_ss_source_sha':'e944c5b5c7107cf4f0e79849adccc3503705acd1','shared_release':str(SHARED),'workflow_run':loaded['workflow_run'],'official_workflow_completed':True,'image_id':app['Image'],'shared_verification_at':core['at'],'api_verification':api,'locale_assets':locale,'mihomo_memory':m['HostConfig']['Memory'],'mihomo_swap_limit':m['HostConfig']['MemorySwap'],'mihomo_original_process_preserved':True,'docs_original_process_preserved':True,'natural_drain_confirmed':True,'temporary_cleanup':cleanup,'business_preservation':core['business_preservation'],'passive_sample_count':13,'separate_deployment_started':False,'node_imports':0,'generation_requests_by_this_task':0,'mail_sent_by_this_task':False,'cutover_backup':read('after.json')['backup'],'route_canary_at':read('rolling-route-canary.json')['at'],'route_canonical_at':read('rolling-route-canonical.json')['at']}
p=ROOT/'included-release-verification.json';p.write_text(json.dumps(out,indent=2));p.chmod(0o600)
print(json.dumps(out))
