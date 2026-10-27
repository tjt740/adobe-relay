import json,subprocess,datetime
code='''import json,time
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright
with sync_playwright() as pw:
 b=pw.chromium.connect_over_cdp('http://127.0.0.1:9222');ctx=b.contexts[0];p=ctx.pages[0]
 if 'account.adobe.com' in p.url:
  p.goto('https://firefly.adobe.com/generate/image',wait_until='domcontentloaded',timeout=45000);p.wait_for_timeout(22000)
 body=p.locator('body').aria_snapshot()
 if 'dialog "Sign in to Adobe Firefly"' not in body:
  btn=p.get_by_role('button',name='Sign in',exact=True)
  if btn.count()==1 and btn.is_visible():btn.click(timeout=12000);p.wait_for_timeout(8000)
 out=[]
 for pi,page in enumerate(ctx.pages):
  for fi,f in enumerate(page.frames):
   u=urlparse(f.url);item={'page_index':pi,'frame_index':fi,'url':u.scheme+'://'+u.netloc+u.path,'name':f.name}
   try:item['text']=f.locator('body').inner_text(timeout=5000)[:6500];item['accessibility']=f.locator('body').aria_snapshot(timeout=5000)[:9000]
   except Exception as e:item['error_class']=type(e).__name__
   out.append(item)
 print(json.dumps({'frames':out},ensure_ascii=False))
'''
r=subprocess.run(['docker','exec','adobe-expanded-native-1843','python','-c',code],capture_output=True,text=True,timeout=110)
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'returncode':r.returncode,'browser':[json.loads(l) for l in r.stdout.splitlines() if l.startswith('{')],'error_tail':r.stderr[-700:] if r.returncode else None},ensure_ascii=False))
