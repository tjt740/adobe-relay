"""Read-only API verification; real WebP requests have a separate one-shot record."""
import datetime,importlib.util,json,pathlib,urllib.request
ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-webp-20261111')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
sha=(ROOT/'expected-sha').read_text().strip()
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/groups/adobe-models',headers={'x-api-key':key})
with urllib.request.urlopen(req,timeout=35) as res:
 assert res.status==200
 data=json.load(res)['data'];assert data['models']
proof={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'passed':True,'read_only':True,'model_directory_source':data['source'],'active_generation':False,'existing_group_changes':False}
a.save('api-verification.json',proof);print(json.dumps(proof))
