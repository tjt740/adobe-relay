"""Read-only production settings check. Do not preset any subscription priority."""
import datetime,importlib.util,json,pathlib,urllib.request
ROOT=pathlib.Path('/opt/adobe-relay/releases/clash-ui-combined-20261112')
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
def read(path):
 req=urllib.request.Request('http://127.0.0.1:6666'+path,headers={'x-api-key':key})
 with urllib.request.urlopen(req,timeout=35) as res:
  assert res.status==200
  return json.load(res)['data']
health=read('/api/v1/admin/proxies/clash/health')
blocked=[node for source in health['sources'] for node in source['blocked_nodes']]
for source in health['sources']:
 assert source['generation_blocked']==len(source['blocked_nodes'])
 assert set(source['blocked_proxy_ids'])=={node['proxy_id'] for node in source['blocked_nodes']}
for node in blocked:
 assert all(k in node for k in ['proxy_id','name','bound_accounts','manual','enabled'])
ids=[node['proxy_id'] for node in blocked[:2]]
if not ids:ids=[node['proxy_id'] for node in data['nodes'][:2]]
assert ids
related_proofs=[]
for pid in ids:
 view=read('/api/v1/admin/proxies/'+str(pid)+'/related-accounts?page=1&page_size=10&relation=all')
 assert view['proxy']['id']==pid and isinstance(view['items'],list)
 assert view['total']>=len(view['items']) and len(view['items'])<=10
 assert all(k in view for k in ['checked_at','history_since','current_count','departed_count'])
 for item in view['items']:
  assert all(k in item for k in ['using_this_node','mode','account_state','route_state','impact'])
  assert not {'credentials','password','token','cookie'}.intersection(item)
 related_proofs.append({'proxy_id':pid,'total':view['total'],'returned':len(view['items'])})
before=json.loads((ROOT/'before.json').read_text())
assert a.snapshot()['clash_settings_hash']==before['clash_settings_hash']
proof={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'passed':True,'read_only':True,'sources':len(urls),'nodes':len(data['nodes']),'settings_fields_verified':True,'health_details_verified':True,'related_account_queries':related_proofs,'priorities_not_preset':True,'active_generation':False,'production_writes':False}
a.save('api-verification.json',proof);print(json.dumps(proof))
