import datetime,json,pathlib,subprocess,urllib.parse
root=pathlib.Path('/opt/adobe-relay/releases/proxy-502-followup-20261112')
receipt=root/'transport-probe-container-network.json'
if receipt.exists():print(receipt.read_text());raise SystemExit(0)
q="BEGIN READ ONLY; SET LOCAL statement_timeout='5s'; SELECT json_agg(t) FROM (SELECT id,protocol,host,port,username,password FROM proxies WHERE id IN (1944,1946,1969,1970,1951) ORDER BY id)t; COMMIT;"
p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input=q,text=True,capture_output=True,check=True,timeout=10)
nodes=json.loads(p.stdout.strip());assert len(nodes)==5
ds=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1','adobe-relay-mihomo-1']))
networks=set(ds[0]['NetworkSettings']['Networks']) & set(ds[1]['NetworkSettings']['Networks'])
assert len(networks)==1
net=ds[1]['NetworkSettings']['Networks'][networks.pop()]
aliases=set(net.get('Aliases') or []) | {'mihomo','adobe-relay-mihomo-1'}
assert net['IPAddress']
out={'started_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'adobe_credentials_sent':False,'generation_submissions':0,'samples':[]}
for n in nodes:
 host=n['host']
 if host in aliases:host=net['IPAddress']
 host='['+host+']' if ':' in host else host
 auth=(urllib.parse.quote(n['username'],safe='')+':'+urllib.parse.quote(n['password'] or '',safe='')+'@') if n['username'] else ''
 proxy=n['protocol']+'://'+auth+host+':'+str(n['port'])
 for label,url in [('adobe','https://firefly.adobe.com/'),('independent_https','https://www.cloudflare.com/')]:
  r=subprocess.run(['curl','--silent','--show-error','--head','--output','/dev/null','--proxy',proxy,'--noproxy','','--proto','=https','--connect-timeout','3','--max-time','5','--retry','0','--write-out','%{http_code} %{http_connect} %{time_total}',url],text=True,capture_output=True,timeout=7)
  parts=r.stdout.split()
  out['samples'].append({'proxy_id':n['id'],'target':label,'curl_exit':r.returncode,'http':int(parts[0]) if parts else 0,'connect':int(parts[1]) if len(parts)>1 else 0,'seconds':float(parts[2]) if len(parts)>2 else None,'at':datetime.datetime.now(datetime.timezone.utc).isoformat()})
out['at']=datetime.datetime.now(datetime.timezone.utc).isoformat()
with receipt.open('x') as f:json.dump(out,f,ensure_ascii=False,indent=2)
receipt.chmod(0o600)
print(json.dumps(out,ensure_ascii=False))
