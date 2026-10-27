import pathlib,hashlib,json,subprocess,datetime
p=pathlib.Path('/opt/adobe-relay/releases/408-causal-20261028/sub2api')
x=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1'],text=True))[0]
assert x['Config']['Image']=='adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c'
assert p.is_file()
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'diagnostic_binary','data':{'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'bytes':p.stat().st_size,'production_image':x['Config']['Image'],'production_untouched':True}}))
