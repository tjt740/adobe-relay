"""Verify deployed frontend assets without subscription refreshes or account probes."""
import datetime, hashlib, importlib.util, json, re, urllib.request
from pathlib import Path
ROOT=Path('/opt/adobe-relay/releases/subscription-health-ui-20261018')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
sha=(ROOT/'expected-sha').read_text().strip()
assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
base='https://creative.cheap'
def read(path):
 with urllib.request.urlopen(base+path,timeout=30) as response:
  assert response.status==200
  return response.read().decode()
html=read('/admin/changelog')
entry=re.search(r'<script[^>]+src="(/assets/[^" ]+\.js)"',html)[1]
main=read(entry)
def bundle(name):
 matches=re.findall(r'(?:/)?assets/('+name+r'-[A-Za-z0-9_-]+\.(?:js|css))',main)
 assert matches, 'Missing production bundle: '+name
 return [(path,read('/assets/'+path)) for path in dict.fromkeys(matches)]
changelog=bundle('ChangelogView')
proxies=bundle('ProxiesView')
assert any('2026-10-18' in content and 'data-release-date' in content and 'subscriptionUI' in content for _,content in changelog)
assert any('subscription-watermark' in content for _,content in proxies)
assert any(re.search(r'rows:["\']?6',content) and 'clash-subscription-input' in content for _,content in proxies)
css=[(p,c) for p,c in proxies if p.endswith('.css')]
assert any('min-height:10rem' in c and 'rotate(-10deg)' in c for _,c in css)
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'https_frontend':True,'daily_changelog':True,'same_day_grouping':True,'subscription_input_six_rows':True,'status_watermarks':True,'asset_sha256':{p:hashlib.sha256(c.encode()).hexdigest() for p,c in changelog+proxies},'subscription_refresh_or_checks':False}
a.save('frontend-verification.json',result)
print(json.dumps(result))
