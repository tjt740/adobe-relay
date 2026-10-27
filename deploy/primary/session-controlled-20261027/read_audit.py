import json,pathlib,time,datetime
r=pathlib.Path('/opt/adobe-relay/releases/three-arm-20261027');p=r/'control'
assert (r/'isolated.json').exists(),'selected accounts not isolated'
a=json.loads((r/'isolated.json').read_text())['accounts'];assert all(x['manual_pause'] is True and x['manual_proxy'] is True and not x['schedulable'] for x in a)
f=p/'cmd-0001.json'
if not f.exists():
 with f.open('x') as h:json.dump({'action':'click_role','role':'button','name':'Sign in'},h)
end=time.monotonic()+16;receipt=p/'cmd-0001.receipt.json'
while time.monotonic()<end:
 if receipt.exists() and json.loads(receipt.read_text()).get('state')!='started':break
 time.sleep(.5)
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'receipt':json.loads(receipt.read_text()) if receipt.exists() else None,'state':json.loads((p/'web-state.json').read_text()) if (p/'web-state.json').exists() else None},ensure_ascii=False))
