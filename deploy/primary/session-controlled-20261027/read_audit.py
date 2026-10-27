import subprocess,json,datetime
code=r'''
import ast,pathlib,json
p=pathlib.Path('/app/backend/app/api/routes/external_member.py');s=p.read_text();tree=ast.parse(s);out={}
for n in tree.body:
 if isinstance(n,(ast.FunctionDef,ast.AsyncFunctionDef)) and any(x in n.name for x in ['mail','message','otp']):out[n.name]=ast.get_source_segment(s,n)
for name in ['external_mail_reader.py','external_mail.py','mail_reader.py']:
 p=pathlib.Path('/app/backend/app/services')/name
 if p.exists():out[name]=p.read_text()[:16000]
print(json.dumps(out,ensure_ascii=False))
'''
r=subprocess.run(['docker','exec','adobe-team-cookie-app-1','python','-c',code],text=True,capture_output=True,timeout=20)
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sources':json.loads(r.stdout) if r.returncode==0 else None}))
