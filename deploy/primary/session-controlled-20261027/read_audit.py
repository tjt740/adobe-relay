import pathlib,json,datetime,hashlib
root=pathlib.Path('/opt/adobe-relay/releases/crawford-20261027');root.mkdir(mode=0o700,exist_ok=True)
assert not (root/'started.json').exists()
src=pathlib.Path('/opt/adobe-relay/releases/three-arm-20261027/crawfordprobe');dest=root/'probe';dest.write_bytes(src.read_bytes());dest.chmod(0o700)
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'prepared':True,'probe_sha256':hashlib.sha256(dest.read_bytes()).hexdigest(),'production_changes':0}))
