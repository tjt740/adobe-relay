import pathlib,subprocess,json,datetime
root=pathlib.Path('/opt/adobe-relay/releases/three-arm-20261027')
script="import json,sys,runpy,pathlib; src=pathlib.Path('/experiment/login_isolated.py').read_text().split('try:\n # Python module')[0]; exec(src)"
cmd=['docker','run','--rm','-i','--network','none','--memory','512m','--read-only','--tmpfs','/tmp:rw,size=32m','--mount','type=bind,src=/opt/adobe-team-cookie/data,dst=/app/data,readonly','--mount',f'type=bind,src={root},dst=/experiment,readonly','-e','DATABASE_URL=sqlite:////app/data/app.db','--entrypoint','python','adobe-team-cookie:79382b0f79d559575956e023dc9f52fbf07dbfde','-c',script]
r=subprocess.run(cmd,input=json.dumps({'account_id':1739,'member_id':1259}),text=True,capture_output=True,timeout=45)
# Prefix only sets hooks and imports; no login, no outbound networking.
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'rc':r.returncode,'bootstrap_error':r.stderr[-2500:],'bootstrap_output':r.stdout[-500:]}))
