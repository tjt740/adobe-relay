"""Read-only production settings check. Do not preset any subscription priority."""
import datetime,importlib.util,json,pathlib,urllib.request
ROOT=pathlib.Path('/opt/adobe-relay/releases/account-status-filter-20261112')
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
blocked=[node for source in health['sources'] for node in (source.get('blocked_nodes') or [])]
for source in health['sources']:
 assert source['generation_blocked']==len(source.get('blocked_nodes') or [])
 assert set(source.get('blocked_proxy_ids') or [])=={node['proxy_id'] for node in (source.get('blocked_nodes') or [])}
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
from current_result import result_status
filter_checks=[]
for status in ['test_passed','test_failed','request_succeeded','request_failed']:
 page=1;returned=0;totals=[]
 while True:
  view=read('/api/v1/admin/accounts?platform=adobe&status='+status+'&page='+str(page)+'&page_size=100')
  rows=view['items'];totals.append(view['total']);returned+=len(rows)
  assert all(result_status(row)==status for row in rows), 'Filter differs from visible result: '+status
  if page*100>=view['total']:break
  page+=1;assert page<=100
 filter_checks.append({'status':status,'pages':page,'returned':returned,'total_observed':totals,'every_returned_result_matches':True})
proof={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'passed':True,'read_only':True,'account_filters':filter_checks,'sources':len(urls),'nodes':len(data['nodes']),'settings_fields_verified':True,'health_details_verified':True,'related_account_queries':related_proofs,'priorities_not_preset':True,'active_generation':False,'production_writes':False}
a.save('api-verification.json',proof);print(json.dumps(proof))
