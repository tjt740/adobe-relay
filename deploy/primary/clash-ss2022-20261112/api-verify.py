"""Preview a synthetic Base64 feed without importing nodes or testing exits."""
import datetime, importlib.util, json, pathlib, urllib.request
ROOT=pathlib.Path('/opt/adobe-relay/releases/clash-ss2022-20261112')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
sha=(ROOT/'expected-sha').read_text().strip()
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
before=a.snapshot()
url='https://raw.githubusercontent.com/tjt740/adobe-relay/'+sha+'/backend/internal/clash/testdata/shadowsocks-base64.txt'
req=urllib.request.Request('https://creative.cheap/api/v1/admin/proxies/clash/preview',data=json.dumps({'url':url}).encode(),headers={'x-api-key':key,'Content-Type':'application/json'})
with urllib.request.urlopen(req,timeout=50) as res:
 assert res.status==200
 data=json.load(res)['data']
nodes=data['nodes']
assert len(nodes)==3 and {n['name'] for n in nodes}=={'SS2022-AES128','SS2022-AES256','SS2022-CHACHA20'}
assert all(n['type']=='ss' and n['server']=='example.invalid' and n['server_port']==443 and n['proxy_id']==0 for n in nodes)
assert all(not s.get('fetch_error') for s in data['sources'])
after=a.snapshot();a.check_preserved(before,after)
assert before['clash_contract_hash']==after['clash_contract_hash']
m=after['containers']['adobe-relay-mihomo-1'];original=json.loads((ROOT/'before.json').read_text())['containers']['adobe-relay-mihomo-1']
assert m['resources']['Memory']==1073741824 and m['id']==original['id'] and m['restarts']==original['restarts'] and m['started']==original['started']
proof={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'passed':True,'read_only':True,'public_https_preview':True,'base64_ss2022_nodes':3,'existing_subscription_unchanged':True,'mihomo_same_process_and_1g_limit':True,'active_generation':False,'node_imports':0,'outbound_probes':0}
a.save('api-verification.json',proof);print(json.dumps(proof))
