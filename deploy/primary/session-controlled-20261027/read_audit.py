import pathlib,hashlib,json,datetime
p=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028/sub2api');p.chmod(0o755)
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'binary_ready','sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'bytes':p.stat().st_size,'production_changed':False}))
