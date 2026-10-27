import pathlib,sys,json,re,datetime
ROOT=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028-r2');sys.path.insert(0,str(ROOT));from replica import *
def safe(s):
 s=re.sub(r'https?://[^\s<>"\x27]+','[url]',str(s));s=re.sub(r'eyJ[\w-]+\.[\w-]+\.[\w-]+','[token]',s);s=re.sub(r'(?i)(bearer\s+|(?:token|cookie|password|authorization|secret)[\"\x27]?\s*[:=]\s*)[^\s,;}]+',r'\1[redacted]',s)
 return s[:900]
a=crows("SELECT json_build_object('at',created_at,'request_id',request_id,'model',model,'status',status_code,'upstream_status',upstream_status_code,'phase',error_phase,'message',error_message,'upstream_message',upstream_error_message) FROM ops_error_logs WHERE created_at>NOW()-INTERVAL '15 minutes' ORDER BY created_at DESC LIMIT 18")
for x in a:
 for k in ['message','upstream_message']:x[k]=safe(x[k])
print(json.dumps({'at':now(),'kind':'request_errors','data':a},ensure_ascii=False));save('request-diagnosis.json',a)
e=[]
for l in (ROOT/'events.jsonl').read_text().splitlines():
 try:
  x=json.loads(l)
  if x.get('event') in ['http_start','http_finish']:e.append(x)
 except:pass
print(json.dumps({'at':now(),'kind':'recent_http','data':e[-20:]},ensure_ascii=False))
print(json.dumps({'at':now(),'kind':'current','data':selected()},ensure_ascii=False))
