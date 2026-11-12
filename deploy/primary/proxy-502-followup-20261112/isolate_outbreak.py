"""Stop future Adobe selections on four evidenced routes; never close sockets."""
import datetime, fcntl, hashlib, json, pathlib, subprocess

root=pathlib.Path('/opt/adobe-relay/releases/proxy-502-followup-20261112')
ids='1944,1946,1969,1970'
reason='manual_quarantine_repeated_business_eof'
held=[]
# This is an ordinary future-route DB exclusion, not a deployment. Serialize with
# the actual application switch, while allowing a concurrent natural drain to
# continue. Both reviewed applications have byte-identical selection guards.
for path in ['/opt/adobe-relay/.release.lock']:
    f=open(path,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
def sql(q,write=False):
    p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input=('BEGIN;' if write else 'BEGIN READ ONLY;')+"SET LOCAL lock_timeout='500ms';SET LOCAL statement_timeout='10s';"+q+';COMMIT;',text=True,capture_output=True,check=True,timeout=20)
    return json.loads(p.stdout.strip())
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def containers():
    ds=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1','adobe-relay-mihomo-1']))
    return {d['Name']:{'id':d['Id'],'image':d['Config']['Image'],'pid':d['State']['Pid'],'started':d['State']['StartedAt'],'restarts':d['RestartCount']}for d in ds}
def save(name,value):
    with (root/name).open('x') as f:json.dump(value,f,ensure_ascii=False,indent=2)
    (root/name).chmod(0o600)
receipt=root/'outbreak-applied.json'
if receipt.exists():
    print(receipt.read_text());raise SystemExit(0)
query=f"""SELECT json_agg(t) FROM (SELECT p.id,p.name,p.status,p.updated_at,
 md5(jsonb_build_array(protocol,host,port,username,password)::text) identity_hash,
 (SELECT count(*) FROM ops_error_logs l WHERE l.created_at>NOW()-INTERVAL '15 minutes' AND l.status_code=502
 AND l.upstream_errors->-1->>'proxy_id'=p.id::text
 AND (l.upstream_errors->-1->>'detail')::jsonb->>'code'='upstream_connection_failed'
 AND NULLIF((l.upstream_errors->-1->>'detail')::jsonb->>'proxy_updated_at','')::timestamptz=p.updated_at) recent_transport_errors,
 (SELECT count(*) FROM accounts a WHERE a.proxy_id=p.id AND a.deleted_at IS NULL) bindings,
 (SELECT count(*) FROM adobe_proxy_test_leases l WHERE l.proxy_id=p.id AND l.expires_at>NOW()) leases
 FROM proxies p WHERE p.id IN ({ids}) AND deleted_at IS NULL ORDER BY p.id)t"""
nodes=sql(query)
assert len(nodes)==4 and all(n['recent_transport_errors']>=10 for n in nodes),'Fresh matching transport evidence required'
accounts=sql(f"SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,status,schedulable,proxy_id,md5(credentials::text) credentials_hash,extra->>'okad_manual_pause' manual_pause,extra->>'manual_proxy_enabled' manual_proxy FROM accounts WHERE deleted_at IS NULL AND proxy_id IN ({ids}) ORDER BY id)t")
refs=containers()
assert refs['/adobe-relay-app-1']['image'] in ['adobe-relay:e1c9b83ace0472ad7e97b35d4bf2c4dd5dc1574b','adobe-relay:871822afc3d6b30d51a2abbbbbcddf5644cd5f89'],'Later application: review first'
configs={str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in pathlib.Path('/opt/adobe-relay/shared').glob('*.yml')}
before_path=root/'outbreak-before.json'
if not before_path.exists():save(before_path.name,{'at':now(),'nodes':nodes,'accounts':accounts,'containers':refs,'configuration':configs})
original=json.loads(before_path.read_text())
assert original['containers']==refs and original['configuration']==configs,'Baseline changed; review first'
# Exclusions are future-selection guards. Keeping proxy revisions, listener
# definitions and existing slots intact allows accepted work to finish normally.
expected=' OR '.join("(id="+str(n['id'])+" AND updated_at='"+n['updated_at']+"'::timestamptz)" for n in nodes)
applied=sql(f"""DO $$ BEGIN
 PERFORM id FROM proxies WHERE id IN ({ids}) ORDER BY id FOR UPDATE;
 IF (SELECT count(*) FROM proxies WHERE deleted_at IS NULL AND ({expected}))<>4
 THEN RAISE EXCEPTION 'Route revision changed; inspect'; END IF;
 END $$;
 INSERT INTO adobe_route_exclusions(proxy_id,proxy_updated_at,reason)
 SELECT id,updated_at,'{reason}' FROM proxies WHERE id IN ({ids}) AND deleted_at IS NULL
 ON CONFLICT(proxy_id) DO NOTHING;
 SELECT json_agg(t) FROM (SELECT proxy_id,proxy_updated_at,reason,created_at FROM adobe_route_exclusions WHERE proxy_id IN ({ids}) ORDER BY proxy_id)t""",True)
assert len(applied)==4 and all(n['reason']==reason for n in applied),'Conflicting operator exclusion preserved; inspect'
unchanged=refs==containers() and all(hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()==v for p,v in configs.items())
out={'at':now(),'exclusions':applied,'containers_and_configuration_unchanged':unchanged,'direct_account_writes':False,'listeners_removed':False,'connections_closed':False,'replayed_requests':False,'nodes':nodes}
save(receipt.name,out)
assert unchanged
print(json.dumps(out,ensure_ascii=False))
