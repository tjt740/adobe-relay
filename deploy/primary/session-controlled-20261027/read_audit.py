import subprocess,json,datetime
code="""import inspect,json
from app.services import firefly,external_login
s=inspect.getsource(firefly._finish_firefly_token).splitlines()
start=inspect.getsourcelines(firefly._finish_firefly_token)[1]
print(json.dumps({'finish_excerpt':[{'line':start+i,'text':x} for i,x in enumerate(s) if 1000<=start+i<=1040],'cleanup_contract':inspect.getsource(firefly._cleanup_profile_organizations)}))
"""
r=subprocess.run(['docker','exec','-i','adobe-team-cookie-app-1','python','-'],input=code,capture_output=True,text=True,timeout=30)
assert r.returncode==0,'source_contract_read_failed'
for x in r.stdout.splitlines():
 if x.startswith('{'):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'source_contract','data':json.loads(x)},ensure_ascii=False))
