import pathlib,json,datetime,subprocess,collections
ROOT=pathlib.Path('/opt/adobe-relay/releases/expanded-20261027')
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
for name in ['started.json','isolated.json','finished.json','restored.json']+[f'auth-{i}.json' for i in [1843,1815,1813,1833,1807,1817]]:
 f=ROOT/name
 if f.exists():print(json.dumps({'at':now(),'file':name,'document':json.loads(f.read_text())},ensure_ascii=False))
for f in sorted(ROOT.glob('*.jsonl')):
 rows=[]
 for l in f.read_text().splitlines():
  try:rows.append(json.loads(l))
  except:pass
 statuses=collections.Counter(str(x.get('status')) for x in rows if x.get('event')=='http_finish' and x.get('host')=='firefly-3p.ff.adobe.io')
 keep=[r for r in rows if r.get('event') in ['fatal','stop','account_done','account_start','login_ready','login_failed','guard_stop','worker_failed','resolved_model','web_ready','job_finish']]
 bad=[r for r in rows if r.get('status')==401 or r.get('valid') is False]
 print(json.dumps({'at':now(),'file':f.name,'record_count':len(rows),'statuses':statuses,'events':keep[-12:],'bad':bad,'tail':rows[-3:]},ensure_ascii=False))
for f in sorted(ROOT.glob('*stderr.log'))+[ROOT/'controller-process.log']:
 if f.exists() and f.stat().st_size:print(json.dumps({'at':now(),'file':f.name,'log_tail':f.read_text()[-1800:]},ensure_ascii=False))
print(json.dumps({'at':now(),'containers':[n for n in subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines() if n.startswith('adobe-expanded-')]}))
