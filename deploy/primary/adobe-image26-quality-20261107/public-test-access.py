"""Temporary administrator-owned API key for a healthy current group, always cleaned."""
import datetime,json,os,pathlib,subprocess,sys,urllib.request,urllib.error
os.umask(0o077)
ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-image26-quality-20261107')
GROUP=2;NAME='release-quality-26-20261107-temporary';USER=1
REMAINING=len(sys.argv)==2 and sys.argv[1]=='remaining'
assert len(sys.argv)==1 or REMAINING
if REMAINING:NAME+='-remaining'
SHA=(ROOT/'expected-sha').read_text().strip()
def sql(q):
    return subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input='BEGIN READ ONLY; '+q+'; COMMIT;',capture_output=True,text=True,check=True,timeout=25).stdout.strip()
def groups():return json.loads(sql("SELECT coalesce(json_agg(group_id ORDER BY group_id),'[]') FROM user_allowed_groups WHERE user_id=1"))
def save(name,data):
    if REMAINING:name=name.replace('business-','business-remaining-',1)
    with (ROOT/name).open('x') as f:json.dump(data,f,indent=2)
def request(method,path,payload=None,token=None):
    headers={'Content-Type':'application/json'}
    if token:headers['Authorization']='Bearer '+token
    req=urllib.request.Request('http://127.0.0.1:6666/api/v1'+path,data=json.dumps(payload).encode() if payload is not None else None,headers=headers,method=method)
    try:
        with urllib.request.urlopen(req,timeout=35) as h:return json.load(h).get('data')
    except urllib.error.HTTPError as e:raise RuntimeError('Normal admin API denied or failed with HTTP '+str(e.code)) from None
c=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
assert c['Config']['Image']=='adobe-relay:'+SHA and c['State']['Health']['Status']=='healthy'
group=json.loads(sql("SELECT json_build_object('status',status,'type',subscription_type,'platform',platform,'images',allow_image_generation) FROM groups WHERE id=2"))
assert group=={'status':'active','type':'standard','platform':'adobe','images':True}
assert sql("SELECT count(*) FROM api_keys WHERE user_id=1 AND name='"+NAME+"'")=='0'
original=groups();added=GROUP not in original
env=dict(v.split('=',1) for v in c['Config']['Env'])
email=sql("SELECT email FROM users WHERE id=1 AND role='admin' AND status='active' AND deleted_at IS NULL")
assert email==env['ADMIN_EMAIL']
data=request('POST','/auth/login',{'email':email,'password':env['ADMIN_PASSWORD']})
assert not data.get('requires_2fa') and data['user']['id']==USER and data['user']['role']=='admin'
token=data['access_token'];env.clear()
save('business-access-started.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'group_id':GROUP,'original_allowed_groups':original,'added_grant':added,'quota':2,'cleanup_required':True})
try:
    if added:
        request('PUT','/admin/users/1',{'allowed_groups':original+[GROUP]},token)
        assert groups()==sorted(original+[GROUP])
    key=request('POST','/keys',{'name':NAME,'group_id':GROUP,'quota':2,'expires_in_days':1},token)
    assert key['user_id']==USER and key['group_id']==GROUP and key['name']==NAME and key['quota']==2 and key['expires_at']
    save('business-key-created.json',{k:key[k] for k in ['id','user_id','group_id','name','quota','expires_at']})
    key_id=key['id'];key.clear()
    p=subprocess.run(['python3',str(ROOT/'public-test.py'),str(key_id)]+(['remaining'] if REMAINING else []))
finally:
    rows=json.loads(sql("SELECT coalesce(json_agg(json_build_object('id',id,'user_id',user_id,'group_id',group_id,'name',name)),'[]') FROM api_keys WHERE user_id=1 AND name='"+NAME+"' AND deleted_at IS NULL"))
    assert len(rows)<=1
    for k in rows:
        assert k['user_id']==USER and k['group_id']==GROUP and k['name']==NAME
        request('DELETE','/keys/'+str(k['id']),token=token)
    assert sql("SELECT count(*) FROM api_keys WHERE user_id=1 AND name='"+NAME+"' AND deleted_at IS NULL")=='0'
    current=groups()
    if added:
        assert set(original).issubset(current)
        assert sql('SELECT count(*) FROM api_keys WHERE user_id=1 AND group_id=2 AND deleted_at IS NULL')=='0', 'Concurrent dependency; preserve grant for review'
        remaining=[g for g in current if g!=GROUP]
        if remaining!=current:request('PUT','/admin/users/1',{'allowed_groups':remaining},token)
        assert groups()==remaining
    save('business-access-cleanup.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'temporary_key_deleted':True,'temporary_grant_removed':added,'original_allowed_groups':original,'current_allowed_groups':groups(),'credentials_persisted':False})
assert p.returncode==0, 'One or more operational failures; no replay'
