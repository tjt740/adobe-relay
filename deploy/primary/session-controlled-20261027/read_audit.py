import pathlib,json,subprocess,datetime
ROOT=pathlib.Path('/opt/adobe-relay/releases/expanded-20261027');assert not (ROOT/'finished.json').exists();assert not (ROOT/'native-email-entered.json').exists()
code='''import json
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright
with sync_playwright() as pw:
 b=pw.chromium.connect_over_cdp('http://127.0.0.1:9222');ctx=b.contexts[0];p=ctx.pages[0];f=next(f for f in p.frames if 'auth-light.identity.adobe.com' in f.url)
 f.get_by_role('textbox',name='Email address',exact=True).fill('BrianSchmidtfY445921@outlook.com')
 f.get_by_role('button',name='Continue',exact=True).click();p.wait_for_timeout(5000)
 out=[]
 for pi,p in enumerate(ctx.pages):
  for fi,f in enumerate(p.frames):
   u=urlparse(f.url);x={'page_index':pi,'frame_index':fi,'url':u.scheme+'://'+u.netloc+u.path}
   try:x['accessibility']=f.locator('body').aria_snapshot(timeout=4000)[:11000];x['text']=f.locator('body').inner_text(timeout=4000)[:6500]
   except Exception as e:x['error_class']=type(e).__name__
   out.append(x)
 print(json.dumps({'frames':out},ensure_ascii=False))
'''
(ROOT/'native-email-entered.json').write_text(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat()}))
r=subprocess.run(['docker','exec','adobe-expanded-native-1843','python','-c',code],capture_output=True,text=True,timeout=70)
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'returncode':r.returncode,'browser':[json.loads(l) for l in r.stdout.splitlines() if l.startswith('{')],'error_tail':r.stderr[-600:] if r.returncode else None},ensure_ascii=False))
