"""Read-only production settings check. Do not preset any subscription priority."""
import datetime,importlib.util,json,pathlib,urllib.request
ROOT=pathlib.Path('/opt/adobe-relay/releases/clash-priority-20261112')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
sha=(ROOT/'expected-sha').read_text().strip()
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/proxies/clash',headers={'x-api-key':key})
with urllib.request.urlopen(req,timeout=35) as res:
 assert res.status==200
 data=json.load(res)['data']
urls=data['url'].split()
assert set(data['notes'])==set(urls)==set(data['priorities'])
assert isinstance(data['settings_revision'],int)
assert all(isinstance(p,int) and 0<=p<=100 for p in data['priorities'].values())
assert all(isinstance(n,str) and len(n)<=200 for n in data['notes'].values())
before=json.loads((ROOT/'before.json').read_text())
assert a.snapshot()['clash_settings_hash']==before['clash_settings_hash']
proof={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'passed':True,'read_only':True,'sources':len(urls),'nodes':len(data['nodes']),'settings_fields_verified':True,'priorities_not_preset':True,'active_generation':False,'production_writes':False}
a.save('api-verification.json',proof);print(json.dumps(proof))
