import base64, datetime, hashlib, json, os, pathlib, subprocess, sys, urllib.parse
os.umask(0o077)
case_file = pathlib.Path(sys.argv[1]); spec=json.loads(case_file.read_text()); case=spec['case']; model=spec['payload']['model']; variant='service'; complexity=spec['payload'].get('complexity','')
assert case.replace('-','').isalnum()
assert model in ['gpt-image-2','gpt-image-2.5-flare','gpt-image-2.5-sunburst','gpt-image-2.6-star-swirl','gpt-image-2.6-aurora']
assert variant=='service'
assert complexity in ['', 'low', 'medium', 'high']
root=pathlib.Path('/opt/adobe-relay/releases/adobe-gpt-image-26-gif-20261106/preflight')
root.mkdir(parents=True,exist_ok=True,mode=0o700)
container=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
assert container['Config']['Image']=='adobe-relay:b97759edd35a80cc85dc868acce5e670a1f6efa0'
assert container['State'].get('Health',{}).get('Status')=='healthy'
query="""BEGIN READ ONLY; SET LOCAL statement_timeout='15s'; SELECT json_build_object('id',a.id,'status',a.status,'schedulable',a.schedulable,'credentials',a.credentials,'proxy_id',p.id,'proxy',json_build_object('protocol',p.protocol,'host',p.host,'port',p.port,'username',p.username,'password',p.password,'status',p.status,'expires_at',p.expires_at)) FROM accounts a JOIN proxies p ON p.id=a.proxy_id WHERE a.id=2933 AND a.deleted_at IS NULL AND a.platform='adobe'; COMMIT;"""
r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input=query,text=True,capture_output=True,timeout=25)
assert r.returncode==0,'read-only account query failed'
a=json.loads(r.stdout)
assert a['status']=='active' and a['schedulable'] and a['proxy']['status']=='active'
p=a['proxy']; c=a['credentials']
if p['expires_at']:assert datetime.datetime.fromisoformat(p['expires_at'])>datetime.datetime.now(datetime.timezone.utc)
assert c.get('access_token')
assert datetime.datetime.fromisoformat(c['expires_at'].replace('Z','+00:00'))>datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(minutes=10)
auth=''
if p['username'] and p['password']:auth=urllib.parse.quote(p['username'],safe='')+':'+urllib.parse.quote(p['password'],safe='')+'@'
proxy=p['protocol']+'://'+auth+p['host']+':'+str(p['port'])
request=spec['payload'].copy()
if spec.get('reference_case'):
 data=base64.b64encode((root/(spec['reference_case']+'.png')).read_bytes()).decode()
 request['images']=[{'image_url':'data:image/png;base64,'+data} for _ in range(spec.get('reference_count',1))]
payload={'AccountID':a['id'],'ProxyID':a['proxy_id'],'Token':c['access_token'],'ProxyURL':proxy,'ARPSessionID':c.get('arp_session_id',''),'Case':case,'Model':model,'Size':request.get('size',''),'OutputFormat':request.get('output_format','png'),'Mode':'service','ExpectedError':spec.get('expected_error',''),'Multipart':bool(spec.get('multipart')),'Payload':request,'Endpoint':spec.get('endpoint','/v1/images/generations'),'Credentials':{'model_mapping':c.get('model_mapping',{})}}
network=list(container['NetworkSettings']['Networks'])[0]
binary='/opt/adobe-relay/releases/adobe-gpt-image-26-gif-20261106/adobe-gif-service-probe-v9'
print(json.dumps({'starting':case,'account_id':a['id'],'proxy_id':a['proxy_id'],'variant':variant,'complexity':complexity,'binary_sha256':hashlib.sha256(pathlib.Path(binary).read_bytes()).hexdigest()}),flush=True)
result=subprocess.run(['docker','run','--rm','-i','--name','adobe-gif-probe-'+case,'--network',('none' if spec.get('expected_error') else network),'--memory','768m','--cpus','1','--pids-limit','64','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--tmpfs','/tmp:rw,noexec,nosuid,size=512m','-e','GOMEMLIMIT=512MiB','-v',binary+':/probe:ro','-v',str(root)+':'+str(root)+':rw','--entrypoint','/probe',container['Config']['Image']],input=json.dumps(payload),text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=540)
print(result.stdout,flush=True)
if result.stderr:print(json.dumps({'diagnostic_stderr_present':True}),flush=True)
sys.exit(result.returncode)
