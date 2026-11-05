"""One authorized real generation per phase; never replay an uncertain result."""
import base64, datetime, hashlib, importlib.util, json, os, pathlib, struct, sys, time
import urllib.error, urllib.request

phase, expected_sha = sys.argv[1:]
assert phase in ('before', 'after')
assert len(expected_sha) == 40 and all(c in '0123456789abcdef' for c in expected_sha)
os.umask(0o077)
root = pathlib.Path('/opt/adobe-relay/releases/adobe-auto-medium-20261105')
root.mkdir(mode=0o700, exist_ok=True)
spec = importlib.util.spec_from_file_location('audit', str(root / 'audit.py'))
a = importlib.util.module_from_spec(spec); spec.loader.exec_module(a)
assert a.inspect(a.APP)['Config']['Image'] == 'adobe-relay:' + expected_sha
quality = 'medium' if phase == 'before' else 'auto'
key = a.sql("SELECT key FROM api_keys WHERE id=12 AND status='active' AND deleted_at IS NULL AND group_id=7")
assert key and len(key) > 10
payload = {'model':'gpt-image-2.5-flare', 'prompt':'A single red ceramic coffee mug on a plain light gray studio background, no text.', 'quality':quality, 'size':'1024x1024', 'n':1, 'response_format':'b64_json'}
marker = root / (phase + '-live-test-started.json')
started = datetime.datetime.now(datetime.timezone.utc).isoformat()
with os.fdopen(os.open(marker, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'w') as f:
    json.dump({'at':started,'application_sha':expected_sha,'api_key_id':12,'quality':quality,'request_count':1,'never_replay_if_interrupted':True}, f)
request = urllib.request.Request('https://creative.cheap/v1/images/generations', data=json.dumps(payload).encode(), headers={'Authorization':'Bearer '+key,'Content-Type':'application/json'}, method='POST')
out = {'at':started,'application_sha':expected_sha,'phase':phase,'requested_quality':quality,'api_key_id':12,'outer_posts':1,'success':False}
begin = time.monotonic()
try:
    with urllib.request.urlopen(request, timeout=240) as response:
        out['status'] = response.status
        body = json.load(response)
    out['response_quality'] = body.get('quality')
    items = body.get('data', [])
    assert len(items) == 1 and out['response_quality'] == 'medium'
    data = base64.b64decode(items[0]['b64_json'], validate=True)
    assert data.startswith(b'\x89PNG\r\n\x1a\n')
    width, height = struct.unpack('>II', data[16:24])
    assert width > 0 and height > 0 and len(data) > 1024
    (root / (phase + '-live-test.png')).write_bytes(data)
    out.update(success=True, image={'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest(),'width':width,'height':height}, request_id=body.get('request_id'))
except urllib.error.HTTPError as error:
    out['status'] = error.code
    try:
        value = json.loads(error.read(32768)).get('error', {})
        out['error'] = {k:value[k] for k in ['type','code','param','stage','source','upstream_status','request_id'] if k in value}
    except Exception:
        out['error'] = {'type':'unparsed_http_error'}
except Exception as error:
    out['exception'] = type(error).__name__
finally:
    out['duration_seconds'] = round(time.monotonic()-begin, 3)
    (root / (phase+'-live-test.json')).write_text(json.dumps(out, ensure_ascii=False, indent=2))
    print(json.dumps(out), flush=True)
assert out['success'], 'Live generation did not pass; do not push or publish; inspect the recorded single attempt'
