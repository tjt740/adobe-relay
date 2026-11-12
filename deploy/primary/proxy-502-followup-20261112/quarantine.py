import datetime, fcntl, hashlib, json, pathlib, subprocess

ROOT=pathlib.Path('/opt/adobe-relay/releases/proxy-502-followup-20261112')
ROOT.mkdir(mode=0o700,exist_ok=True)
held=[]
for path in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock']:
    f=open(path,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def sql(q,write=False):
    p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input=("BEGIN;" if write else "BEGIN READ ONLY;")+" SET LOCAL lock_timeout='500ms'; SET LOCAL statement_timeout='10s'; "+q+'; COMMIT;',text=True,capture_output=True,check=True,timeout=20)
    return json.loads(p.stdout.strip())
def save(name,data):
    p=ROOT/name
    with p.open('x') as f:json.dump(data,f,ensure_ascii=False,indent=2)
    p.chmod(0o600)
def containers():
    ds=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1','adobe-relay-mihomo-1']))
    return {d['Name']:{'id':d['Id'],'image':d['Config']['Image'],'pid':d['State']['Pid'],'started':d['State']['StartedAt'],'restarts':d['RestartCount']} for d in ds}
query="""SELECT json_agg(t) FROM (SELECT p.id,p.status,p.updated_at,
 md5(jsonb_build_array(protocol,host,port,username,password)::text) identity_hash,
 (SELECT count(*) FROM accounts a WHERE a.proxy_id=p.id AND a.deleted_at IS NULL AND a.status='active' AND a.schedulable) active_bindings,
 (SELECT count(*) FROM adobe_proxy_test_leases l WHERE l.proxy_id=p.id AND l.expires_at>NOW()) leases,
 EXISTS(SELECT 1 FROM adobe_route_exclusions e WHERE e.proxy_id=p.id) excluded
 FROM proxies p WHERE p.id IN (1120,1149) AND deleted_at IS NULL ORDER BY p.id)t"""
assert not (ROOT/'quarantine-applied.json').exists(),'Already applied; inspect receipt'
before=sql(query)
assert len(before)==2 and all(not n['active_bindings'] and not n['leases'] and not n['excluded'] for n in before)
accounts=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,status,schedulable,proxy_id,md5(credentials::text) credentials_hash,extra->>'okad_manual_pause' manual_pause,extra->>'manual_proxy_enabled' manual_proxy FROM accounts WHERE deleted_at IS NULL AND proxy_id IN (1120,1149) ORDER BY id)t")
for a in accounts:
    for prefix in ['concurrency:account:','concurrency:live:account:']:
        count=int(subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','ZCARD',prefix+str(a['id'])],text=True))
        assert count==0, 'Wait for bound account requests to drain'
refs=containers()
configs={str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in pathlib.Path('/opt/adobe-relay/shared').glob('*.yml')}
save('quarantine-before.json',{'at':now(),'nodes':before,'accounts':accounts,'containers':refs,'configuration':configs})
# No listener, credential, subscription, account or process mutation. Keeping the
# proxy revision preserves in-flight observations; persistent exclusions are read
# immediately before future submissions and by ordinary allocation.
write="""DO $$ BEGIN
PERFORM id FROM proxies WHERE id IN (1120,1149) AND deleted_at IS NULL ORDER BY id FOR UPDATE;
IF (SELECT count(*) FROM proxies WHERE id IN (1120,1149) AND deleted_at IS NULL)<>2
 OR EXISTS(SELECT 1 FROM accounts WHERE proxy_id IN (1120,1149) AND deleted_at IS NULL AND status='active' AND schedulable)
 OR EXISTS(SELECT 1 FROM adobe_proxy_test_leases WHERE proxy_id IN (1120,1149) AND expires_at>NOW())
THEN RAISE EXCEPTION 'Eligibility changed; inspect before retry'; END IF;
INSERT INTO adobe_route_exclusions(proxy_id,proxy_updated_at,reason)
 SELECT id,updated_at,'manual_quarantine_repeated_business_eof' FROM proxies WHERE id IN (1120,1149);
END $$;
SELECT json_agg(t) FROM (SELECT proxy_id,reason,created_at FROM adobe_route_exclusions WHERE proxy_id IN (1120,1149) ORDER BY proxy_id)t"""
applied=sql(write,True)
unchanged=refs==containers() and all(hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()==v for p,v in configs.items())
out={'at':now(),'exclusions':applied,'containers_and_configuration_unchanged':unchanged,'direct_account_writes':False,'listeners_removed':False,'connections_closed':False,'nodes':sql(query)}
save('quarantine-applied.json',out)
assert unchanged
print(json.dumps(out,ensure_ascii=False))
