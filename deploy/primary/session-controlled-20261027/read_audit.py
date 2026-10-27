"""Read-only incident collection. No Adobe calls, imports or production writes."""
import base64
import collections
import datetime
import hashlib
import json
import re
import sqlite3
import subprocess

TARGETS = ['AbascalChoute99@outlook.com', 'AgrestaNeuzil7786@outlook.com', 'AimeeGreenlaw434@outlook.com', 'AliahSamra500@outlook.com', 'AmiotLoyal123@outlook.com', 'AshleyChambers2212@outlook.com', 'AstryCocklin113@outlook.com', 'BabaTijing8893@outlook.com', 'BarbourAryana2308@outlook.com', 'BazanDeonte646@outlook.com', 'BehrmannConatser45@outlook.com', 'BergesenVillafranca062@outlook.com', 'BirosDorinda1309@hotmail.com', 'BorkinOstasiewicz66@outlook.com', 'BostelPortell94@outlook.com', 'BrazingtonCallais4181@outlook.com', 'BreeseGrandusky89@outlook.com', 'BrianSchmidtfY445921@outlook.com', 'ChristopherHicksZE47079@outlook.com', 'CichockiCaho107@outlook.com', 'CirelliFrech1163@outlook.com', 'CommingsJayde855@outlook.com', 'CongroveSettler473@outlook.com', 'CoureyBookout1583@outlook.com', 'CowingBryanne86@outlook.com', 'CrawfordHeather6275@hotmail.com', 'CustardHurban3299@outlook.com', 'CzubakowskiMcmurtry055@outlook.com', 'DameronMcneeley3698@outlook.com', 'DeblancTorda696@outlook.com', 'DechellisHuseman7093@outlook.com', 'DelaportillaSchneiter096@outlook.com', 'DeveauUsrey5943@hotmail.com', 'DieffenbacherFencl096@outlook.com', 'DimaioDowney7664@outlook.com', 'DonaldSimril48@outlook.com', 'DoraisWaltermire72@outlook.com', 'DoyleDushane552@outlook.com', 'DubieWrightsel775@outlook.com', 'DulayCaspari423@outlook.com', 'EakesPigler0257@outlook.com', 'FairyDillie7427@outlook.com', 'FranceseSiroka95@outlook.com', 'FyockNassie22@outlook.com', 'GanglBritteny806@outlook.com', 'GenslerPalesano721@outlook.com', 'GilsonKirkner5232@outlook.com', 'GilstadRaina7223@outlook.com', 'GittlerGaito663@outlook.com', 'GobeilRangnow9316@outlook.com', 'HaskinsRisler116@outlook.com', 'HeatherOckman24@outlook.com', 'HelfinstineSulit6602@outlook.com', 'HolcombBraz15@outlook.com', 'HorseCoretta3549@outlook.com', 'IsidoraRichelle731@outlook.com', 'JaminetKnezevich5244@outlook.com', 'JinMirisola476@outlook.com', 'KadelHeaslet468@outlook.com', 'KienleOliver9614@outlook.com', 'KischBest160@outlook.com', 'KishParrinello6127@outlook.com', 'LiptakFabiszewski2669@outlook.com', 'LitteralReash66@outlook.com', 'LopeyBrasby28@outlook.com', 'MaclinGrantland2461@outlook.com', 'MalbroughMastrianni32@outlook.com', 'MaltbaKinsolving20@outlook.com', 'MapstoneSteinert058@outlook.com', 'MarinoPavish2264@outlook.com', 'MccuinConsalvo924@outlook.com', 'McnicholasBergquist89@hotmail.com', 'MeinerdingRobson895@outlook.com', 'MilardoColeman418@outlook.com', 'MoffetRini481@outlook.com', 'MohsinUnger934@outlook.com', 'MontezumaFalke41@outlook.com', 'MoriskyRabinovich1519@hotmail.com', 'NicksRainey1890@outlook.com', 'NouriMorrisey70@outlook.com', 'OrrellHuettman5786@outlook.com', 'OstheimerDalya1868@outlook.com', 'ParkhurstYarnell548@outlook.com', 'PaschallMerales6367@outlook.com', 'PatrickCaseyUT43130@outlook.com', 'PennixJaynie3783@outlook.com', 'PerfectKichline869@outlook.com', 'PeterHoeke6642@outlook.com', 'PoleraRode077@outlook.com', 'PorteusPhil304@outlook.com', 'PrayKinds027@outlook.com', 'RosendorfIglesia9381@outlook.com', 'SchifoTurbiner70@outlook.com', 'SchofillWadusky1495@outlook.com', 'SelkeyVanderbeek1606@outlook.com', 'SharleyHinish96@outlook.com', 'SillmanPulis408@outlook.com', 'SpivackLapeyrolerie14@outlook.com', 'SpringsteenNuckoles68@outlook.com', 'TangaroTamsyn54@outlook.com', 'TieshaLindsay4418@outlook.com', 'ToutonChapnick935@outlook.com', 'UyematsuTaubman51@outlook.com', 'WhyelHemple227@outlook.com', 'YoungbergSchoonmaker63@outlook.com', 'ZeminskiMerlyn30@outlook.com', 'jtdoug32246@outlook.com', 'mphbdcxbprsxbp@outlook.com']
OUT = {'started_at': datetime.datetime.now(datetime.timezone.utc).isoformat()}

def digest(value):
    return hashlib.sha256(str(value).encode()).hexdigest()[:24] if value else None

def sql(query):
    p = subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-X','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'], input="BEGIN READ ONLY;SET LOCAL statement_timeout='10s';SET LOCAL TIME ZONE 'Asia/Shanghai';"+query+';COMMIT;', text=True,capture_output=True,timeout=16)
    if p.returncode:
        raise RuntimeError(p.stderr[:500])
    return [json.loads(line) for line in p.stdout.splitlines() if line.startswith('{')]

def query(label, query):
    try:
        OUT[label] = sql(query)
    except Exception as e:
        OUT[label+'_error'] = str(e)[:600]

def token_metadata(token):
    if not token:
        return {'present': False}
    try:
        part=token.split('.')[1]
        claims=json.loads(base64.urlsafe_b64decode(part+'='*(-len(part)%4)))
    except Exception:
        return {'present': True,'sha256':digest(token),'decode_failed':True}
    safe={'present':True,'sha256':digest(token),'keys':list(claims)}
    for k in ['client_id','type','expires_in','created_at','exp','iat','scope','account_type','as']:
        if k in claims:safe[k]=claims[k] if isinstance(claims[k],(str,int,float,bool)) else type(claims[k]).__name__
    for k in ['user_id','aa_id','sub','ownerOrg','owner_org','sid','session_id','jti','authId','id']:
        if k in claims:safe[k+'_hash']=digest(claims[k])
    return safe

def cookie_metadata(cookie):
    pairs=[]
    for item in (cookie or '').split(';'):
        name,sep,value=item.strip().partition('=')
        if sep:pairs.append((name,value))
    count=collections.Counter(n for n,v in pairs)
    return {'hash':digest(cookie),'names':list(count),'duplicates':{n:c for n,c in count.items() if c>1},'sessions':{n:digest(v) for n,v in pairs if re.search('sid|session|auth',n,re.I)}}

# Shared utilities above are read-only. Token validation below does not renew sessions.
OUT={'started_at':datetime.datetime.now(datetime.timezone.utc).isoformat()}
names=','.join("'"+e.lower()+"'" for e in TARGETS)
query('first_auth_request_timing',"WITH logins(account_id,login_at) AS (VALUES (1738,'2026-10-27T17:43:01.151531+08:00'::timestamptz),(1739,'2026-10-27T16:35:10.597509+08:00'::timestamptz),(1740,'2026-10-27T16:41:01.122019+08:00'::timestamptz),(1741,'2026-10-27T16:42:25.771211+08:00'::timestamptz),(1742,'2026-10-27T16:41:05.460397+08:00'::timestamptz),(1743,'2026-10-27T16:40:09.971991+08:00'::timestamptz),(1744,'2026-10-27T16:41:01.292391+08:00'::timestamptz),(1745,'2026-10-27T16:05:11.285908+08:00'::timestamptz),(1746,'2026-10-27T16:40:10.208002+08:00'::timestamptz),(1747,'2026-10-27T16:49:52.972237+08:00'::timestamptz),(1748,'2026-10-27T16:41:25.081154+08:00'::timestamptz),(1749,'2026-10-27T16:40:04.69523+08:00'::timestamptz),(1750,'2026-10-27T16:06:33.511438+08:00'::timestamptz),(1751,'2026-10-27T16:40:29.384567+08:00'::timestamptz),(1752,'2026-10-27T16:07:13.358824+08:00'::timestamptz),(1753,'2026-10-27T16:49:54.049867+08:00'::timestamptz),(1754,'2026-10-27T16:39:36.43415+08:00'::timestamptz),(1755,'2026-10-27T16:08:09.127607+08:00'::timestamptz),(1756,'2026-10-27T16:45:59.930447+08:00'::timestamptz),(1757,'2026-10-27T16:49:53.529126+08:00'::timestamptz),(1758,'2026-10-27T16:45:03.154566+08:00'::timestamptz),(1759,'2026-10-27T16:51:00.055069+08:00'::timestamptz),(1760,'2026-10-27T16:50:12.198496+08:00'::timestamptz),(1761,'2026-10-27T16:50:12.712772+08:00'::timestamptz),(1762,'2026-10-27T16:51:05.592492+08:00'::timestamptz),(1763,'2026-10-27T16:50:11.91657+08:00'::timestamptz),(1764,'2026-10-27T16:10:47.270066+08:00'::timestamptz),(1765,'2026-10-27T16:47:29.545962+08:00'::timestamptz),(1766,'2026-10-27T16:11:24.044112+08:00'::timestamptz),(1767,'2026-10-27T16:50:30.351892+08:00'::timestamptz),(1768,'2026-10-27T16:59:31.07924+08:00'::timestamptz),(1769,'2026-10-27T16:52:03.370428+08:00'::timestamptz),(1770,'2026-10-27T16:48:29.136885+08:00'::timestamptz),(1771,'2026-10-27T16:52:08.217229+08:00'::timestamptz),(1772,'2026-10-27T16:51:00.289289+08:00'::timestamptz),(1773,'2026-10-27T16:49:06.343964+08:00'::timestamptz),(1774,'2026-10-27T17:31:17.129364+08:00'::timestamptz),(1775,'2026-10-27T16:52:18.413158+08:00'::timestamptz),(1776,'2026-10-27T16:53:38.730909+08:00'::timestamptz),(1777,'2026-10-27T16:49:08.715069+08:00'::timestamptz),(1778,'2026-10-27T16:51:24.070661+08:00'::timestamptz),(1779,'2026-10-27T16:50:40.93891+08:00'::timestamptz),(1780,'2026-10-27T16:53:43.23276+08:00'::timestamptz),(1781,'2026-10-27T16:51:24.164163+08:00'::timestamptz),(1782,'2026-10-27T17:34:06.055618+08:00'::timestamptz),(1783,'2026-10-27T16:51:30.012634+08:00'::timestamptz),(1784,'2026-10-27T16:18:18.063046+08:00'::timestamptz),(1785,'2026-10-27T16:54:08.579074+08:00'::timestamptz),(1786,'2026-10-27T16:59:34.166805+08:00'::timestamptz),(1787,'2026-10-27T17:00:49.979209+08:00'::timestamptz),(1788,'2026-10-27T17:48:39.16899+08:00'::timestamptz),(1789,'2026-10-27T16:57:09.743717+08:00'::timestamptz),(1790,'2026-10-27T17:00:46.671937+08:00'::timestamptz),(1791,'2026-10-27T16:57:16.768734+08:00'::timestamptz),(1792,'2026-10-27T16:57:47.127859+08:00'::timestamptz),(1793,'2026-10-27T16:56:18.687193+08:00'::timestamptz),(1794,'2026-10-27T17:48:38.554789+08:00'::timestamptz),(1795,'2026-10-27T17:35:10.468207+08:00'::timestamptz),(1796,'2026-10-27T16:23:08.838413+08:00'::timestamptz),(1797,'2026-10-27T16:59:33.567899+08:00'::timestamptz),(1798,'2026-10-27T16:23:57.144685+08:00'::timestamptz),(1799,'2026-10-27T16:56:16.483075+08:00'::timestamptz),(1800,'2026-10-27T16:24:45.529139+08:00'::timestamptz),(1801,'2026-10-27T17:02:20.469454+08:00'::timestamptz),(1802,'2026-10-27T16:57:47.530827+08:00'::timestamptz),(1803,'2026-10-27T16:25:55.445268+08:00'::timestamptz),(1804,'2026-10-27T17:38:15.438217+08:00'::timestamptz),(1805,'2026-10-27T17:09:38.597898+08:00'::timestamptz),(1806,'2026-10-27T17:01:55.862263+08:00'::timestamptz),(1807,'2026-10-27T17:01:56.134917+08:00'::timestamptz),(1808,'2026-10-27T16:27:55.892165+08:00'::timestamptz),(1809,'2026-10-27T17:03:18.074531+08:00'::timestamptz),(1810,'2026-10-27T17:04:18.450526+08:00'::timestamptz),(1811,'2026-10-27T17:02:47.743841+08:00'::timestamptz),(1812,'2026-10-27T17:03:47.289281+08:00'::timestamptz),(1813,'2026-10-27T17:05:18.233715+08:00'::timestamptz),(1814,'2026-10-27T17:05:07.208294+08:00'::timestamptz),(1815,'2026-10-27T17:08:15.222973+08:00'::timestamptz),(1816,'2026-10-27T17:08:50.38691+08:00'::timestamptz),(1817,'2026-10-27T17:08:10.768352+08:00'::timestamptz),(1818,'2026-10-27T17:07:44.843121+08:00'::timestamptz),(1819,'2026-10-27T17:10:14.868071+08:00'::timestamptz),(1820,'2026-10-27T17:13:37.968591+08:00'::timestamptz),(1821,'2026-10-27T17:15:39.739117+08:00'::timestamptz),(1822,'2026-10-27T16:34:03.333987+08:00'::timestamptz),(1823,'2026-10-27T17:14:12.857203+08:00'::timestamptz),(1824,'2026-10-27T17:10:09.86572+08:00'::timestamptz),(1825,'2026-10-27T16:35:23.095132+08:00'::timestamptz),(1826,'2026-10-27T16:35:51.803713+08:00'::timestamptz),(1827,'2026-10-27T17:10:48.083713+08:00'::timestamptz),(1828,'2026-10-27T16:36:39.599488+08:00'::timestamptz),(1829,'2026-10-27T17:13:37.550497+08:00'::timestamptz),(1830,'2026-10-27T17:14:04.774692+08:00'::timestamptz),(1831,'2026-10-27T16:37:42.678186+08:00'::timestamptz),(1832,'2026-10-27T17:14:06.2207+08:00'::timestamptz),(1833,'2026-10-27T17:16:15.613039+08:00'::timestamptz),(1834,'2026-10-27T16:38:40.897789+08:00'::timestamptz),(1835,'2026-10-27T16:39:03.148021+08:00'::timestamptz),(1836,'2026-10-27T17:14:26.038681+08:00'::timestamptz),(1837,'2026-10-27T16:39:41.63721+08:00'::timestamptz),(1838,'2026-10-27T17:14:24.7362+08:00'::timestamptz),(1839,'2026-10-27T16:40:13.693771+08:00'::timestamptz),(1840,'2026-10-27T16:40:30.049652+08:00'::timestamptz),(1841,'2026-10-27T17:16:46.896981+08:00'::timestamptz),(1842,'2026-10-27T17:15:37.343904+08:00'::timestamptz),(1843,'2026-10-27T17:19:01.975742+08:00'::timestamptz),(1844,'2026-10-27T14:47:41.044098+08:00'::timestamptz)), firsts AS (SELECT s.*, (SELECT min(e.created_at) FROM ops_error_logs e WHERE e.account_id=s.account_id AND e.created_at>=s.login_at AND (e.status_code=401 OR e.upstream_status_code=401)) AS first_401 FROM logins s) SELECT row_to_json(t) FROM (SELECT b.account_id,b.login_at,e.created_at,e.request_id,e.model,e.duration_ms,e.created_at-(COALESCE(e.duration_ms,0)*interval '1 millisecond') AS estimated_request_started_at,e.error_phase,e.upstream_status_code FROM firsts b JOIN ops_error_logs e ON e.account_id=b.account_id AND e.created_at=b.first_401) t")
db=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);db.execute('PRAGMA query_only=ON');db.row_factory=sqlite3.Row
OUT['cleanup_statuses']=[]
for row in db.execute('SELECT id,email,account_profile FROM external_members'):
    if row['email'].lower() not in {x.lower() for x in TARGETS}:continue
    try:profile=json.loads(row['account_profile'] or '{}')
    except Exception:continue
    clean=profile.get('organization_cleanup') or {}
    safe={'id':row['id'],'email':row['email'],'status':clean.get('status'),'recorded_at':clean.get('recorded_at'),'item_count':len(clean.get('items',[]))}
    safe['items']=[{k:v for k,v in item.items() if k in ['status','action','attempted','success','code','result','method']} for item in clean.get('items',[]) if isinstance(item,dict)]
    OUT['cleanup_statuses'].append(safe)
OUT['job_events']=[]
patterns=['DEL-','退出组织','组织清理','会话失效','重新认证','重新登录','登录成功','修复路线','原路线','403','401','会话冷却','proxy','sub2']
for row in db.execute('SELECT id,type,created_at,logs_json,meta_json FROM job_records WHERE created_at>=? ORDER BY created_at',(1793082600,)):
    try:logs=json.loads(row['logs_json'] or '[]')
    except Exception:logs=[]
    for line in logs:
        if not isinstance(line,str):continue
        hits=[word for word in patterns if word.lower() in line.lower()]
        if not hits:continue
        emails=[email for email in TARGETS if email.lower() in line.lower()]
        OUT['job_events'].append({'job_id':row['id'],'job_created_at':row['created_at'],'time_prefix':line[:8] if re.match(r'\d\d:\d\d:\d\d',line) else None,'emails':emails,'markers':hits})
selected=['IsidoraRichelle731@outlook.com','BostelPortell94@outlook.com','CrawfordHeather6275@hotmail.com','jtdoug32246@outlook.com']
probe=[]
for email in selected:
    member=db.execute('SELECT access_token,last_login_at FROM external_members WHERE lower(email)=?',(email.lower(),)).fetchone()
    if not member or not member['access_token']:continue
    rs=sql("SELECT json_build_object('id',a.id,'proxy_id',a.proxy_id,'proxy',json_build_object('protocol',p.protocol,'host',p.host,'port',p.port,'username',p.username,'password',p.password)) FROM accounts a LEFT JOIN proxies p ON p.id=a.proxy_id WHERE a.deleted_at IS NULL AND lower(a.name)='"+email.lower()+"'")
    if rs:probe.append({**rs[0],'email':email,'token':member['access_token'],'last_login_at':member['last_login_at']})
db.close()
code=r'''
import sys,json,hashlib,urllib.parse,datetime,concurrent.futures
from curl_cffi import requests
rows=json.load(sys.stdin)
def check(a):
 out={'account_id':a['id'],'email':a['email'],'proxy_id':a['proxy_id'],'token_source':'okad_original','last_login_at':a['last_login_at'],'token_hash':hashlib.sha256(a['token'].encode()).hexdigest()[:24],'started_at':datetime.datetime.now(datetime.timezone.utc).isoformat()}
 p=a['proxy'];url=''
 if p['host']:
  auth=urllib.parse.quote(p['username'] or '',safe='')+':'+urllib.parse.quote(p['password'] or '',safe='')+'@' if p['username'] else ''
  url=p['protocol']+'://'+auth+p['host']+':'+str(p['port'])
 s=requests.Session(impersonate='chrome',proxies={'http':url,'https':url} if url else None,verify=True,timeout=8)
 try:
  h={'Content-Type':'application/x-www-form-urlencoded;charset=UTF-8','Origin':'https://firefly.adobe.com','Referer':'https://firefly.adobe.com/'}
  r=s.post('https://ims-na1.adobelogin.com/ims/validate_token/v1',headers=h,data=urllib.parse.urlencode({'type':'access_token','client_id':'clio-playground-web','token':a['token']}),timeout=8)
  try:d=r.json()
  except Exception:d={}
  out['validate_http_status']=r.status_code;out['valid']=d.get('valid');out['reason']=d.get('reason');out['expires_at']=d.get('expires_at');out['expires_at_str']=d.get('expires_at_str')
  r=s.get('https://ims-na1.adobelogin.com/ims/profile/v1',headers={'Authorization':'Bearer '+a['token'],'Accept':'application/json','Origin':'https://firefly.adobe.com','Referer':'https://firefly.adobe.com/'},timeout=8)
  try:d=r.json()
  except Exception:d={}
  out['profile_http_status']=r.status_code;out['identity_matches']=str(d.get('email','')).lower()==a['email'].lower();out['profile_error']=d.get('error');out['profile_error_description']=d.get('error_description')
 except Exception as e:out['exception']=type(e).__name__;out['curl_code']=getattr(e,'code',None)
 finally:s.close()
 out['finished_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();return out
with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
 print(json.dumps(list(pool.map(check,rows))))
'''
proc=subprocess.run(['docker','run','--rm','--network','adobe-relay_default','--memory','256m','--cpus','0.5','--read-only','--tmpfs','/tmp:rw,size=16m','--entrypoint','python','adobe-team-cookie:79382b0f79d559575956e023dc9f52fbf07dbfde','-c',code],input=json.dumps(probe),text=True,capture_output=True,timeout=25)
if proc.returncode==0:OUT['token_validation']=json.loads(proc.stdout)
else:OUT['token_validation_error']='validation process failed'
source=r'''
import pathlib,json,ast,hashlib
base=pathlib.Path('/app/backend/app/services');result={}
for filename in ['firefly.py','organization_cleanup.py','external_login.py','firefly_arp.py','login_coordinator.py']:
 p=base/filename
 if not p.exists():continue
 text=p.read_text();tree=ast.parse(text)
 for n in ast.walk(tree):
  if isinstance(n,(ast.FunctionDef,ast.AsyncFunctionDef)) and (n.name in ['_finish_firefly_token','_switch_firefly_profile','_session_cookie_str','_login_and_store','run'] or filename in ['organization_cleanup.py','firefly_arp.py']):
   result[filename+':'+n.name]='\n'.join(text.splitlines()[n.lineno-1:n.end_lineno])
print(json.dumps(result))
'''
proc=subprocess.run(['docker','exec','adobe-team-cookie-app-1','python','-c',source],text=True,capture_output=True,timeout=10)
if proc.returncode==0:OUT['source_review']=json.loads(proc.stdout)
query('accounts',"SELECT json_build_object('id',id,'email',name,'status',status,'error',error_message,'schedulable',schedulable,'proxy_id',proxy_id) FROM accounts WHERE deleted_at IS NULL AND lower(name) IN ("+names+") ORDER BY id")
OUT['finished_at']=datetime.datetime.now(datetime.timezone.utc).isoformat()
print(json.dumps(OUT,ensure_ascii=False))
