import json, pathlib, re, urllib.request, urllib.parse
base='http://127.0.0.1:6666'
def get(url):
    with urllib.request.urlopen(url,timeout=20) as response:
        assert response.status==200
        return response.read().decode()
index=get(base+'/')
scripts=re.findall(r'<script[^>]+src="([^"]+\.js)"',index)
assert scripts,'Missing frontend entrypoint'
entry=get(urllib.parse.urljoin(base,scripts[0]))
chunks=re.findall(r'(?:assets/)?AccountsView-[a-zA-Z0-9_-]+\.js',entry)
assert chunks,'Missing account page chunk'
chunk=chunks[0]
url=base+'/assets/'+chunk.removeprefix('assets/')
body=get(url)
assert 'account-show-okad-details' in body,'New display preference missing from deployed frontend'
assert 'showOkadDetailsHint' in body and 'showOkadDetails' in body,'New display control missing'
status_chunks=re.findall(r'(?:assets/)?AccountStatusIndicator-[a-zA-Z0-9_-]+\.js',entry+body)
if status_chunks:status_body=get(base+'/assets/'+status_chunks[0].removeprefix('assets/'))
else:status_body=body
assert 'showOkadDetails' in status_body,'Status display gate missing'
assert re.search(r'showOkadDetails:\{type:Boolean,(?:required:!1,)?default:!1\}',status_body),'Default-hidden prop not found'
for url in ['https://creative.cheap/health','https://creative.cheap/okad/api/health']:
    json.loads(get(url))
report={'frontend_chunk':chunk,'display_preference_present':True,'details_hidden_by_default':True,'public_health':True}
path=pathlib.Path('/opt/adobe-relay/releases/okad-details-display-20261013/frontend-verification.json')
path.write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report))
