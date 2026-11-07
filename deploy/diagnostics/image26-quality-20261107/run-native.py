"""Run exactly four sequential native probes, using an existing active account.
Only diagnostic files and temporary containers are created; no account or DB writes.
"""
import datetime, hashlib, json, os, pathlib, subprocess, urllib.parse
os.umask(0o077)
ROOT=pathlib.Path('/opt/adobe-relay/diagnostics/image26-quality-20261107')
SHA='da24f5634a0fd332f95a14d666fa2e9cea19db53'
def sql(q):
    p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY; SET LOCAL statement_timeout='10s'; "+q+'; COMMIT;',text=True,capture_output=True,timeout=20)
    assert p.returncode==0,'read-only query failed'
    return p.stdout.strip()
def runtime():
    c=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
    return c,{'id':c['Id'],'image':c['Config']['Image'],'restarts':c['RestartCount'],'oom':c['State']['OOMKilled'],'health':c['State']['Health']['Status']}
c,before=runtime();assert before['image']=='adobe-relay:'+SHA and before['health']=='healthy'
native=ROOT/'native';native.mkdir(mode=0o700,exist_ok=True)
with (native/'run-started.json').open('x') as f:json.dump({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'runtime':before,'cases':4,'no_replay':True,'binary_sha256':hashlib.sha256((ROOT/'quality-probe').read_bytes()).hexdigest()},f)
network=list(c['NetworkSettings']['Networks'])[0]
for model in ['gpt-image-2.6-star-swirl','gpt-image-2.6-aurora']:
    for quality in ['xhigh','max']:
        a=json.loads(sql("SELECT json_build_object('id',a.id,'status',a.status,'schedulable',a.schedulable,'manual_pause',coalesce(a.extra->>'okad_manual_pause',''),'credentials',a.credentials,'proxy_id',p.id,'proxy',json_build_object('protocol',p.protocol,'host',p.host,'port',p.port,'username',p.username,'password',p.password,'status',p.status,'expires_at',p.expires_at)) FROM accounts a JOIN proxies p ON p.id=a.proxy_id WHERE a.id=2470 AND a.deleted_at IS NULL AND a.platform='adobe'"))
        assert a['status']=='active' and a['schedulable'] and a['manual_pause']!='true' and a['proxy']['status']=='active'
        cred=a['credentials'];proxy=a['proxy'];now=datetime.datetime.now(datetime.timezone.utc)
        assert cred.get('access_token') and datetime.datetime.fromisoformat(cred['expires_at'].replace('Z','+00:00'))>now+datetime.timedelta(minutes=5),'token expiration; stop without login'
        if proxy['expires_at']:assert datetime.datetime.fromisoformat(proxy['expires_at'])>now
        auth=''
        if proxy['username'] and proxy['password']:auth=urllib.parse.quote(proxy['username'],safe='')+':'+urllib.parse.quote(proxy['password'],safe='')+'@'
        payload={'AccountID':a['id'],'ProxyID':a['proxy_id'],'Token':cred['access_token'],'ProxyURL':proxy['protocol']+'://'+auth+proxy['host']+':'+str(proxy['port']),'ARPSessionID':cred.get('arp_session_id',''),'Model':model,'Quality':quality}
        print(json.dumps({'starting':model,'quality':quality,'detail_level':6 if quality=='xhigh' else 7,'account_id':a['id']}),flush=True)
        result=subprocess.run(['docker','run','--rm','-i','--name','image26-quality-'+model.removeprefix('gpt-image-2.6-')+'-'+quality,'--network',network,'--memory','512m','--cpus','1','--pids-limit','64','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--tmpfs','/tmp:rw,noexec,nosuid,size=128m','-e','GOMEMLIMIT=384MiB','-v',str(ROOT/'quality-probe')+':/probe:ro','-v',str(native)+':'+str(native)+':rw','--entrypoint','/probe',c['Config']['Image']],input=json.dumps(payload),text=True,capture_output=True,timeout=520)
        print(result.stdout,flush=True)
        if result.stderr:print(json.dumps({'stderr_present':True}),flush=True)
        assert result.returncode==0,'probe failed or outcome unknown; do not replay'
        assert json.loads((native/(model+'-'+quality+'-result.json')).read_text())['success']
_,after=runtime();assert before==after
(native/'run-complete.json').write_text(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'runtime_unchanged':True,'cases_succeeded':4,'no_account_or_config_writes':True},indent=2))
