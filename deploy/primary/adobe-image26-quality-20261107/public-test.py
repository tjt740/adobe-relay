"""Four sequential public API generations; preserve every result and never replay."""
import base64, datetime, hashlib, json, os, pathlib, struct, subprocess, sys, time, urllib.error, urllib.request, zlib
from public_contract_guard import MODELS, LEVELS, validate
os.umask(0o077)
ROOT = pathlib.Path('/opt/adobe-relay/releases/adobe-image26-quality-20261107')
PUBLIC = ROOT / 'public'
PUBLIC.mkdir(mode=0o700, exist_ok=True)
SHA = (ROOT / 'expected-sha').read_text().strip()
KEY_ID = int(sys.argv[1])

def sql(q):
    return subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'], input="BEGIN READ ONLY; SET LOCAL statement_timeout='15s'; "+q+'; COMMIT;', capture_output=True,text=True,check=True,timeout=25).stdout.strip()

def save(path, value):
    with path.open('x') as f: json.dump(value, f, ensure_ascii=False, indent=2)

def runtime():
    c=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
    assert c['Config']['Image']=='adobe-relay:'+SHA and c['State']['Health']['Status']=='healthy'
    return {'id':c['Id'],'image':c['Config']['Image'],'restarts':c['RestartCount'],'oom':c['State']['OOMKilled']}

def validate_png(data):
    assert data[:8]==b'\x89PNG\r\n\x1a\n'
    at=8;compressed=bytearray();ihdr=None;ended=False
    while at<len(data):
        size=struct.unpack('>I',data[at:at+4])[0];kind=data[at+4:at+8];body=data[at+8:at+8+size]
        assert at+12+size<=len(data)
        assert zlib.crc32(kind+body)&0xffffffff==struct.unpack('>I',data[at+8+size:at+12+size])[0]
        if kind==b'IHDR':
            assert ihdr is None and at==8
            ihdr=struct.unpack('>IIBBBBB',body)
        if kind==b'IDAT':compressed.extend(body)
        at+=12+size
        if kind==b'IEND':ended=True;break
    assert ended and at==len(data) and ihdr and ihdr[:2]==(3840,2160)
    w,h,depth,color,compression,filtering,interlace=ihdr
    assert compression==filtering==interlace==0 and depth in (8,16) and color in (0,2,4,6)
    channels={0:1,2:3,4:2,6:4}[color];stride=(w*channels*depth+7)//8+1
    dec=zlib.decompressobj();raw=dec.decompress(compressed,128*1024*1024)
    assert dec.eof and not dec.unconsumed_tail and not dec.unused_data and len(raw)==h*stride
    assert all(raw[row*stride]<=4 for row in range(h))
    return {'format':'png','width':w,'height':h,'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest(),'png_fully_validated':True}

def pipeline(usage):
    for _ in range(10):
        chunks=[]
        for command in [ ['docker','exec','adobe-relay-app-1','tail','-c','16777216','/app/data/logs/sub2api.log'], ['docker','logs','--since','15m','--tail','12000','adobe-relay-app-1'] ]:
            p=subprocess.run(command,capture_output=True,text=True,timeout=30)
            if p.returncode==0:chunks.extend([p.stdout,p.stderr])
        for chunk in chunks:
            for line in chunk.splitlines():
                try:entry=json.loads(line)
                except (ValueError,TypeError):continue
                if entry.get('msg')=='adobe_images.pipeline_completed' and entry.get('generation_request_id')==usage['request_id']:
                    assert entry['account_id']==usage['account_id']
                    return [{key:row[key] for key in ('submitted_model_version','submitted_detail_level','submit_attempts')} for row in entry['timings']['images']]
        time.sleep(1)
    raise AssertionError('No correlated applied-quality pipeline record; do not repeat generation')

before=runtime();save(ROOT/'public-runtime-before.json',before)
key=sql(f"SELECT k.key FROM api_keys k JOIN users u ON u.id=k.user_id WHERE k.id={KEY_ID} AND u.id=1 AND u.role='admin' AND k.status='active' AND k.deleted_at IS NULL AND k.expires_at>now()")
assert key and '\n' not in key
headers={'Authorization':'Bearer '+key,'Content-Type':'application/json'}
with urllib.request.urlopen(urllib.request.Request('https://creative.cheap/v1/models',headers=headers),timeout=30) as r:
    catalog=json.load(r)
ids={m['id'] for m in catalog['data']}
assert set(MODELS).issubset(ids) and not set(MODELS.values()).intersection(ids)
save(ROOT/'public-model-catalog.json',{'new_names_present':True,'old_names_hidden':True,'models':sorted(set(MODELS).intersection(ids))})
cases=[]
for model in MODELS:
    for quality in LEVELS:
        name=model.removeprefix('gpt-image-2.6-')+'-'+quality
        started=datetime.datetime.now(datetime.timezone.utc).isoformat()
        payload={'model':model,'quality':quality,'prompt':'A blue ceramic mug on a light wooden table beside an orange, natural window light, product photograph, no text. Release verification '+name+'.','size':'3840x2160','n':1,'response_format':'b64_json','output_format':'png'}
        save(PUBLIC/(name+'-started.json'),{'at':started,'application_sha':SHA,'api_key_id':KEY_ID,'payload':payload,'outer_posts':1,'never_replay':True})
        row={'at':started,'application_sha':SHA,'model':model,'quality':quality,'outer_posts':1,'success':False};began=time.monotonic()
        try:
            runtime()
            request=urllib.request.Request('https://creative.cheap/v1/images/generations',data=json.dumps(payload).encode(),headers=headers,method='POST')
            with urllib.request.urlopen(request,timeout=420) as response:
                row['http_status']=response.status;row['http_request_id']=response.headers.get('X-Request-ID');body=json.load(response)
            row['response_quality']=body.get('quality');assert row['response_quality']==quality
            items=body['data'];assert len(items)==1
            data=base64.b64decode(items[0]['b64_json'],validate=True)
            row['images']=[validate_png(data)];(PUBLIC/(name+'.png')).write_bytes(data)
            usage=[]
            for _ in range(10):
                usage=json.loads(sql("SELECT coalesce(json_agg(x),'[]') FROM (SELECT request_id,account_id,model,upstream_model,image_count,total_cost,actual_cost,created_at FROM usage_logs WHERE api_key_id="+str(KEY_ID)+" AND created_at>='"+started+"'::timestamptz ORDER BY created_at) x"))
                if usage:break
                time.sleep(1)
            assert len(usage)==1;row['usage']=usage[0]
            row['applied_submissions']=pipeline(usage[0]);row['pipeline_correlated']=True
            applied,=row['applied_submissions']
            assert applied['submitted_model_version']==MODELS[model] and applied['submitted_detail_level']==LEVELS[quality] and applied['submit_attempts']==1
            row['success']=True
        except urllib.error.HTTPError as error:
            row['http_status']=error.code
            try:
                value=json.loads(error.read(65536)).get('error',{})
                row['error']={k:value[k] for k in ['type','code','param','stage','source','upstream_status','request_id'] if k in value}
            except Exception:row['error']={'type':'unparsed_http_error'}
        except Exception as error:row['exception_type']=type(error).__name__
        finally:
            row['duration_seconds']=round(time.monotonic()-began,3)
            save(PUBLIC/(name+'-result.json'),row);cases.append(row);print(json.dumps(row),flush=True)
        assert row['success'],'Public case failed or outcome unknown; preserve evidence and never replay'
assert runtime()==before
contract=validate(cases,SHA)
save(ROOT/'post-release-tests.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':SHA,'api_key_id':KEY_ID,'cases':cases,'contract':contract})
