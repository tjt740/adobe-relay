import subprocess,json,datetime
code='''import json,pathlib,importlib.metadata
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
 print(json.dumps({'playwright_version':importlib.metadata.version('playwright'),'chromium_path':p.chromium.executable_path,'chromium_exists':pathlib.Path(p.chromium.executable_path).exists()}))
'''
r=subprocess.run(['docker','exec','adobe-team-cookie-app-1','python','-c',code],capture_output=True,text=True,timeout=25)
mem={line.split(':')[0]:line.split(':')[1].strip() for line in open('/proc/meminfo') if line.startswith(('MemTotal:','MemAvailable:'))}
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'runtime':json.loads(r.stdout) if r.returncode==0 else {'error':r.stderr[-300:]},'memory':mem}))
