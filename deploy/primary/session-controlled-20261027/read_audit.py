import pathlib,json,os,signal,datetime
R=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028-r2')
assert not (R/'cleaned.json').exists() and not (R/'handoff.json').exists()
a=[json.loads(x) for x in (R/'controller.jsonl').read_text().splitlines()];done=[x for x in a if x.get('event')=='business_finish'];assert len(done)==16 and all(x['status']==400 for x in done),'do not interrupt live or accepted requests'
starts=[x for x in a if x.get('event')=='business_start'];cut=min(x['at'] for x in starts)
e=[json.loads(x) for x in (R/'events.jsonl').read_text().splitlines()]
assert not any(x.get('event')=='http_finish' and x.get('stage')=='generate-async' and x.get('status') in (200,201,202) and x.get('at','')>=cut for x in e)
ps=[]
for d in pathlib.Path('/proc').iterdir():
 if not d.name.isdigit():continue
 try:c=(d/'cmdline').read_bytes().split(b'\0')
 except OSError:continue
 if len(c)>1 and c[0]==b'python3' and c[1]==str(R/'run_trial.py').encode():ps.append(int(d.name))
assert len(ps)==1
os.kill(ps[0],signal.SIGSTOP)
d={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'pid':ps[0],'reason':'all sixteen requests rejected with 400, zero accepted business generations; retain same diagnostic session while correcting request dimensions','watchdog_retained':True};(R/'handoff.json').write_text(json.dumps(d));print(json.dumps({'at':d['at'],'kind':'diagnostic_session_held','data':d}))
