"""Persist the explicitly authorized Adobe exclusion; leave proxy config intact."""
import datetime,fcntl,importlib.util,json,pathlib,subprocess
ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-route-health-20261023')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
held=[]
for p in ['/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(p,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
sha=(ROOT/'expected-sha').read_text().strip()
assert a.inspect(a.APP)['Config']['Image']==a.OLD_IMAGE
candidate=a.inspect('adobe-relay-memory-canary')
assert candidate['Config']['Image']=='adobe-relay:'+sha and candidate['State']['Health']['Status']=='healthy'
query="""BEGIN; SET LOCAL lock_timeout='5s';SET LOCAL statement_timeout='15s';
 SELECT document FROM clash_subscription WHERE id=1 FOR SHARE;
 LOCK TABLE accounts IN SHARE ROW EXCLUSIVE MODE;
 SELECT id FROM proxies WHERE id=1290 FOR UPDATE;
 DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM proxies WHERE id=1290 AND host='mihomo' AND port=20522 AND status='inactive' AND deleted_at IS NULL)
 OR NOT EXISTS(SELECT 1 FROM clash_subscription c, jsonb_array_elements(c.document->'nodes') n WHERE c.id=1 AND (n->>'proxy_id')::bigint=1290 AND n->>'source_id'='6dcfb89fdc6d6eef04362357')
 OR EXISTS(SELECT 1 FROM accounts WHERE deleted_at IS NULL AND proxy_id=1290)
 OR EXISTS(SELECT 1 FROM adobe_proxy_test_leases WHERE proxy_id=1290)
 THEN RAISE EXCEPTION 'Containment changed; inspect before proceeding';END IF;
 IF EXISTS(SELECT 1 FROM adobe_route_exclusions WHERE proxy_id=1290 AND reason<>'operator_confirmed_generation_failure')
 THEN RAISE EXCEPTION 'Another operator exclusion exists';END IF;
 END $$;
 INSERT INTO adobe_route_exclusions(proxy_id,proxy_updated_at,reason)
 SELECT id,updated_at,'operator_confirmed_generation_failure' FROM proxies WHERE id=1290
 ON CONFLICT(proxy_id) DO NOTHING;
 COMMIT;
"""
# SELECT lock results can include the private Clash document; never print them.
p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input=query,text=True,capture_output=True,timeout=25)
assert p.returncode==0,'Exclusion transaction failed; review privately without printing the subscription document'
result=json.loads(a.sql("SELECT json_build_object('proxy_id',p.id,'status',p.status,'reason',e.reason,'source_id','6dcfb89fdc6d6eef04362357','bound_accounts',(SELECT count(*) FROM accounts WHERE deleted_at IS NULL AND proxy_id=p.id),'temporary_tests',(SELECT count(*) FROM adobe_proxy_test_leases WHERE proxy_id=p.id)) FROM proxies p JOIN adobe_route_exclusions e ON e.proxy_id=p.id WHERE p.id=1290"))
assert result['status']=='inactive' and result['bound_accounts']==result['temporary_tests']==0
result.update(at=datetime.datetime.now(datetime.timezone.utc).isoformat(),changed_proxy_configuration=False,changed_account_state=False)
a.save('exclusion-1290.json',result)
print(json.dumps(result))
