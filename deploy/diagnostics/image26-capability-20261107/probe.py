"""Small authorized production capability check. No deployments; no retries."""
import base64, datetime, hashlib, json, os, pathlib, subprocess, sys, time, urllib.error, urllib.request
os.umask(0o077)
ROOT = pathlib.Path('/opt/adobe-relay/diagnostics/image26-capability-20261107')
ROOT.mkdir(parents=True, exist_ok=True, mode=0o700)
SHA = 'da24f5634a0fd332f95a14d666fa2e9cea19db53'
def sql(q):
    p = subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY; SET LOCAL statement_timeout='15s'; "+q+'; COMMIT;',text=True,capture_output=True,timeout=25)
    if p.returncode: raise RuntimeError('read-only query failed: '+p.stderr[:400])
    return p.stdout.strip()
def save(name,data):
    (ROOT/name).write_text(json.dumps(data,ensure_ascii=False,indent=2))
def runtime():
    c=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
    return {'image':c['Config']['Image'],'container_id':c['Id'],'health':c['State'].get('Health',{}).get('Status'),'restarts':c['RestartCount'],'oom_killed':c['State']['OOMKilled']}
phase = sys.argv[1]
state=runtime()
assert state['image']=='adobe-relay:'+SHA and state['health']=='healthy', 'Review changed production version first'
if phase=='preflight':
    out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'runtime':state}
    out['admin_keys']=json.loads(sql("SELECT coalesce(json_agg(x),'[]') FROM (SELECT k.id,k.group_id,k.status,k.quota,k.quota_used,k.expires_at,g.platform,g.allow_image_generation FROM api_keys k JOIN users u ON u.id=k.user_id LEFT JOIN groups g ON g.id=k.group_id WHERE u.id=1 AND u.role='admin' AND k.status='active' AND k.deleted_at IS NULL) x"))
    out['recent_26']=json.loads(sql("SELECT coalesce(json_agg(x),'[]') FROM (SELECT group_id,model,upstream_model,count(*) successes,max(created_at) latest FROM usage_logs WHERE created_at>now()-interval '60 minutes' AND model LIKE 'gpt-image-2.6%' GROUP BY 1,2,3 ORDER BY successes DESC LIMIT 30) x"))
    out['group7_accounts']=json.loads(sql("SELECT coalesce(json_agg(x),'[]') FROM (SELECT a.id,a.type,a.status,a.schedulable,coalesce(a.extra->>'okad_manual_pause','') manual_pause FROM accounts a JOIN account_groups ag ON ag.account_id=a.id WHERE ag.group_id=7 AND a.deleted_at IS NULL ORDER BY a.id) x"))
    save('preflight.json',out);print(json.dumps(out));sys.exit()
assert phase=='run'
key_id=int(sys.argv[2]);assert key_id>0
key=sql(f"SELECT k.key FROM api_keys k JOIN users u ON u.id=k.user_id JOIN groups g ON g.id=k.group_id WHERE k.id={key_id} AND u.id=1 AND u.role='admin' AND k.status='active' AND k.deleted_at IS NULL AND (k.expires_at IS NULL OR k.expires_at>now()) AND g.platform='adobe' AND g.allow_image_generation")
assert key and '\n' not in key
start=datetime.datetime.now(datetime.timezone.utc).isoformat()
save('runtime-before.json',state)
cases=[]
for model in ['gpt-image-2.6-star-swirl','gpt-image-2.6-aurora']:
    stem=model.removeprefix('gpt-image-2.6-')
    base={'model':model,'prompt':'A single orange ceramic teapot, isolated, no text.','n':1,'quality':'low','size':'1024x1024','response_format':'b64_json','output_format':'png'}
    def add(name,extra):
        p=dict(base,**extra);p['prompt'] += ' Test reference '+stem+' '+name+' November seven.'
        cases.append((stem+'-'+name,p))
    for name,extra in [
        ('transparent-low',{'background':'transparent','complexity':'low'}),
        ('opaque-medium-partials',{'background':'opaque','complexity':'medium','partial_images':2,'stream':False}),
        ('jpeg25-high',{'background':'auto','complexity':'high','output_format':'jpeg','output_compression':25}),
        ('jpeg90-low',{'background':'opaque','complexity':'low','output_format':'jpeg','output_compression':90}),
        ('gif-default',{'size':'auto','output_format':'gif','complexity':'high','prompt':'A tiny orange ball bouncing in place on white, looping animation, no text.'}),
        ('gif16-round25',{'size':'auto','output_format':'gif','complexity':'medium','frame_count':16,'frame_duration_ms':25,'prompt':'A small green leaf gently rotating in place on white, looping animation, no text.'}),
        ('gif2-min20',{'size':'896x896','output_format':'gif','complexity':'low','frame_count':2,'frame_duration_ms':20,'prompt':'A small yellow star blinking on white, looping animation, no text.'}),
        ('reject-webp',{'output_format':'webp'}),('reject-svg',{'output_format':'svg'}),
        ('reject-svg-model',{'output_format':'svg','svg':{'model':'gpt-5'}}),
        ('reject-none',{'complexity':'none'}),('reject-xhigh',{'complexity':'xhigh'}),('reject-max',{'complexity':'max'}),
        ('reject-stream',{'stream':True,'partial_images':2}),
        ('reject-transparent-jpeg',{'background':'transparent','output_format':'jpeg'}),
        ('reject-frame1',{'size':'auto','output_format':'gif','frame_count':1}),
        ('reject-frame17',{'size':'auto','output_format':'gif','frame_count':17}),
        ('reject-duration19',{'size':'auto','output_format':'gif','frame_duration_ms':19}),
        ('reject-gif-compression',{'size':'auto','output_format':'gif','output_compression':50}),
        ('reject-gif-partials',{'size':'auto','output_format':'gif','partial_images':1}),
    ]:add(name,extra)
results=[]
for name,payload in cases:
    marker=ROOT/(name+'-started.json'); target=ROOT/(name+'-result.json')
    if target.exists():
        results.append(json.loads(target.read_text()));continue
    with marker.open('x') as f:json.dump({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'case':name,'outer_posts':1,'never_replay':True,'api_key_id':key_id,'payload':payload},f)
    out={'case':name,'model':payload['model'],'requested':payload,'outer_posts':1,'api_key_id':key_id}; begin=time.monotonic()
    request=urllib.request.Request('https://creative.cheap/v1/images/generations',data=json.dumps(payload).encode(),headers={'Authorization':'Bearer '+key,'Content-Type':'application/json'},method='POST')
    try:
        with urllib.request.urlopen(request,timeout=360) as response:
            out['http_status']=response.status;out['content_type']=response.headers.get('Content-Type');out['request_id']=response.headers.get('X-Request-ID');body=json.load(response)
        if 'error' in body:
            out['error']={k:v for k,v in body['error'].items() if k in ['type','code','param','stage','source','upstream_status','request_id','status_code']}
        else:
            out['response']={k:v for k,v in body.items() if k not in ['data','usage']};out['image_count']=len(body.get('data',[]))
            out['images']=[]
            for i,item in enumerate(body.get('data',[])):
                data=base64.b64decode(item['b64_json'],validate=True)
                ext='png' if data[:8]==b'\x89PNG\r\n\x1a\n' else 'jpeg' if data[:2]==b'\xff\xd8' else 'gif' if data[:3]==b'GIF' else 'bin'
                filename=name+'-'+str(i)+'.'+ext;(ROOT/filename).write_bytes(data)
                out['images'].append({'file':filename,'format':ext,'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest()})
    except urllib.error.HTTPError as e:
        out['http_status']=e.code;out['request_id']=e.headers.get('X-Request-ID')
        try:
            error=json.loads(e.read(65536)).get('error',{})
            out['error']={k:v for k,v in error.items() if k in ['type','code','param','stage','source','upstream_status','request_id']}
        except Exception:out['error']={'type':'unparsed_http_error'}
    except Exception as e:out['exception_type']=type(e).__name__
    out['duration_seconds']=round(time.monotonic()-begin,3);save(name+'-result.json',out);results.append(out);save('results.json',results)
    print(json.dumps({k:v for k,v in out.items() if k!='requested'}),flush=True)
    if out.get('exception_type'):raise RuntimeError('Uncertain result; stopped without retrying')
    # An operational failure is not evidence for lack of model capability; stop costly probes.
    if out.get('http_status',0)>=500 or out.get('http_status') in (401,403,429):raise RuntimeError('Operational failure; inspect before further generation')
save('runtime-after.json',runtime())
q="SELECT coalesce(json_agg(x),'[]') FROM (SELECT request_id,account_id,group_id,model,upstream_model,image_count,total_cost,actual_cost,created_at FROM usage_logs WHERE api_key_id="+str(key_id)+" AND created_at>='"+start+"' ORDER BY created_at) x"
save('usage.json',json.loads(sql(q)))
