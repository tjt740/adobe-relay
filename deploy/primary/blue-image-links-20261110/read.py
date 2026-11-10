import datetime,gzip,json,pathlib,re,subprocess
def sql(q):
 p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY; SET LOCAL statement_timeout='15s'; "+q+'; COMMIT;',capture_output=True,text=True,timeout=22)
 if p.returncode:return {'error':p.stderr[:800]}
 return json.loads(p.stdout)
def scrub(v):
 if isinstance(v,dict):return {k:scrub(x) for k,x in v.items() if k.lower() not in ('authorization','cookie','access_token','refresh_token','prompt','request_body','response_body','request_headers','response_headers','b64_json')}
 if isinstance(v,list):return [scrub(x) for x in v]
 if isinstance(v,str):
  v=re.sub(r'https?://[^\s"<>]+','[URL]',v)
  v=re.sub(r'(?i)Bearer\s+[^\s"<>]+','Bearer [REDACTED]',v)
  v=re.sub(r'sk-[A-Za-z0-9_-]{8,}','[KEY]',v)
  return v[:3500]
 return v

ids=('14a3ccc8-e8d7-4a0e-b721-62062bd73ee8','c4a2dfa3-c7b4-428b-aafb-3b1ca1adfc32','7792ed46-03df-412f-9c90-f3a2a3eb0616')
prefixes=('20261110071951','20261110071329','20261110071401')
pattern=re.compile('|'.join(re.escape(x) for x in ids+prefixes))
files=[p for p in sorted(pathlib.Path('/opt/adobe-relay/shared/data/logs').glob('sub2api*')) if p.is_file() and p.stat().st_mtime>=1794240000]
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'read_only':True,'exact_external_id_hits':0,'candidate_events':[],'near_start_events':[],'files_scanned':[],'scanned_lines':0}
for p in files:
 out['files_scanned'].append(p.name)
 op=gzip.open if p.suffix=='.gz' else open
 with op(p,'rt',errors='replace') as f:
  for line in f:
   out['scanned_lines']+=1
   exact=any(x in line for x in prefixes)
   if exact:out['exact_external_id_hits']+=1
   near=any(line.startswith('2026-11-10T'+t) for t in ('15:13:29','15:13:30','15:14:01','15:14:02','15:19:51','15:19:52','15:19:53','07:13:29','07:14:01','07:19:51'))
   if not pattern.search(line) and not near:continue
   try:
    j=line.index('{');entry={'prefix':line[:j],'fields':json.loads(line[j:])}
   except:continue
   fields=entry['fields']
   if 'timings' in fields:
    timing=fields['timings']
    for im in timing.get('images',[]):
     polls=im.pop('polls',[])
     im['last_poll']=polls[-1] if polls else None
   if exact or fields.get('client_request_id') in ids:out['candidate_events'].append(scrub(entry))
   elif near and fields.get('group_id') in (28,51) and 'gateway_check_start' in entry['prefix']:out['near_start_events'].append(scrub(entry))
q="SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,created_at,request_id,group_id,model,image_count,image_size,image_quality,actual_cost FROM usage_logs WHERE api_key_id=107 AND request_id IN ("+','.join("'client:"+x+"'" for x in ids)+")) t"
out['usage']=sql(q)
print(json.dumps(out,ensure_ascii=False,indent=2))
