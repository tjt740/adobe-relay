import fcntl, hashlib, json, pathlib, sqlite3, subprocess, sys, time, yaml
phase=sys.argv[1]
assert phase in ['before','after']
root=pathlib.Path('/opt/adobe-relay/releases/okad-details-display-20261013')
root.mkdir(mode=0o700,exist_ok=True)
locks=[]
for p in ['/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.local/deploy-login-batches.lock']:
    f=open(p,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);locks.append(f)
def out(args):return subprocess.check_output(args,text=True,timeout=60)
def sha(value):return hashlib.sha256(value).hexdigest()
report={'at':time.time(),'containers':{},'files':{}}
for name in ['adobe-relay-app-1','adobe-team-cookie-app-1','adobe-relay-postgres-1','adobe-relay-redis-1','adobe-relay-mihomo-1']:
    obj=json.loads(out(['docker','inspect',name]))[0]
    report['containers'][name]={'id':obj['Id'],'image':obj['Image'],'tag':obj['Config']['Image'],'health':obj['State'].get('Health',{}).get('Status')}
for name in ['/opt/adobe-relay/shared/data/config.yaml','/opt/adobe-relay/shared/compose.yml','/opt/adobe-relay/shared/resources.yml','/opt/adobe-relay/shared/clash.yml','/opt/adobe-team-cookie/.env','/opt/adobe-team-cookie/compose.yml','/opt/adobe-team-cookie/compose.override.yml']:
    p=pathlib.Path(name)
    if p.exists():report['files'][name]=sha(p.read_bytes())
env=pathlib.Path('/opt/adobe-relay/shared/.env').read_text().splitlines()
report['sub2_env_without_image']=sha('\n'.join(x for x in env if not x.startswith('SUB2API_IMAGE=')).encode())
db=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True)
report['okad_settings_digest']=sha(json.dumps(db.execute('select key,value from settings order by key').fetchall()).encode())
settings=dict(db.execute('select key,value from settings'))
report['integration']={'enabled':settings.get('sub2_reliable_enabled'),'site_id':settings.get('sub2_site_id')}
for table in ['external_members','adobe_members']:
    rows=db.execute('select id,email,cookie from '+table+' order by id').fetchall()
    report[table]={'count':len(rows),'digest':sha(json.dumps(rows).encode())}
db.close()
sql="select json_build_object('count',count(*),'proxy_digest',md5(coalesce(string_agg(id::text||':'||coalesce(proxy_id::text,''),',' order by id),''))) from accounts where deleted_at is null;"
report['sub2_accounts']=json.loads(out(['docker','exec','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atc',sql]))
report['sub2_settings_digest']=out(['docker','exec','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atc',"select md5(coalesce(string_agg(key||':'||value,',' order by key),'')) from settings;"]).strip()
report['app_diff']=out(['docker','diff','adobe-relay-app-1']).splitlines()
target=root/(phase+'.json')
if phase=='before':
    assert not target.exists(),'Preflight already recorded'
    assert report['integration']['enabled']=='true' and report['integration']['site_id']=='primary'
    out(['docker','tag',report['containers']['adobe-relay-app-1']['image'],'adobe-relay:before-okad-details-display-20261013'])
else:
    before=json.loads((root/'before.json').read_text())
    report['preserved']={k:report[k]==before[k] for k in ['files','sub2_env_without_image','okad_settings_digest','integration','external_members','adobe_members','sub2_accounts','sub2_settings_digest']}
    report['preserved']['other_containers']=all(v==before['containers'][k] for k,v in report['containers'].items() if k!='adobe-relay-app-1')
target.write_text(json.dumps(report,indent=2)+'\n');target.chmod(0o600)
print(json.dumps(report))
