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

names=','.join("'"+e.lower().replace("'","''")+"'" for e in TARGETS)
for n in ['adobe-relay-app-1','adobe-team-cookie-app-1']:
    d=json.loads(subprocess.check_output(['docker','inspect',n],text=True))[0]
    OUT.setdefault('containers',[]).append({'name':n,'image':d['Config']['Image'],'started_at':d['State']['StartedAt'],'restarts':d['RestartCount'],'workdir':d['Config']['WorkingDir'],'log':d['HostConfig']['LogConfig']})
assert OUT['containers'][0]['image']=='adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c'
rows=sql("SELECT json_build_object('id',id,'email',name,'created_at',created_at,'updated_at',updated_at,'status',status,'schedulable',schedulable,'concurrency',concurrency,'proxy_id',proxy_id,'error',error_message,'credentials',credentials,'okad',extra->'okad','manual_pause',extra->'okad_manual_pause','test',extra->'okad_connection_test') FROM accounts WHERE deleted_at IS NULL AND lower(name) IN ("+names+") ORDER BY id")
OUT['accounts']=[]
for r in rows:
    raw=r.pop('credentials') or {}
    r['credential_keys']=list(raw)
    r['token']=token_metadata(raw.get('access_token'))
    r['source_cookie']=cookie_metadata(raw.get('cookie'))
    r['runtime_cookie']=cookie_metadata(raw.get('adobe_session_cookie'))
    r['runtime_source_hash']=raw.get('adobe_session_cookie_source_hash')
    r['expires_at']=raw.get('expires_at')
    OUT['accounts'].append(r)
ids=','.join(str(r['id']) for r in rows)
db=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True)
db.execute('PRAGMA query_only=ON');db.row_factory=sqlite3.Row
tables=[r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table'")]
OUT['sqlite_schema']={t:[r[1] for r in db.execute('PRAGMA table_info('+t+')')] for t in tables}
members=[]
for e in TARGETS:
    m=db.execute('SELECT * FROM external_members WHERE lower(email)=?',(e.lower(),)).fetchone()
    if not m:continue
    m=dict(m)
    safe={k:m[k] for k in ['id','email','last_login_at','created_at','updated_at','expires_at','login_status','subscription_ok','subscription_status','credits_available','credits_total','profile_preference','force_code_login','otp_mode','group_id'] if k in m}
    safe['token']=token_metadata(m.get('access_token'))
    safe['cookie']=cookie_metadata(m.get('cookie'))
    profile=m.get('account_profile')
    try:profile=json.loads(profile) if isinstance(profile,str) else profile
    except Exception:profile={}
    if isinstance(profile,dict):
        safe['profile_keys']=list(profile)
        safe['profile_hashes']={k:digest(profile[k]) for k in profile if re.search('org|id|profile|type',k,re.I)}
    members.append(safe)
OUT['okad_members']=members
if 'integration_recovery_jobs' in tables:
    cols=OUT['sqlite_schema']['integration_recovery_jobs']
    fields=[k for k in ['id','email','source_key','state','code','created_at','updated_at','automatic'] if k in cols]
    jobs=[]
    for e in TARGETS:
        jobs += [dict(r) for r in db.execute('SELECT '+','.join(fields)+' FROM integration_recovery_jobs WHERE lower(email)=? AND created_at>=? ORDER BY created_at',(e.lower(),1793030400))]
    OUT['recovery_jobs']=jobs
db.close()

query('postgres_schema',"SELECT json_build_object('table',table_name,'columns',json_agg(column_name ORDER BY ordinal_position)) FROM information_schema.columns WHERE table_schema='public' AND (table_name IN ('proxy_failover_events','audit_logs','scheduler_outbox') OR table_name LIKE '%okad%') GROUP BY table_name")
query('proxy_events',"SELECT row_to_json(t) FROM (SELECT account_id,from_proxy_id,to_proxy_id,created_at,reason FROM proxy_failover_events WHERE account_id IN ("+ids+") AND created_at>='2026-10-27T00:00:00+08:00' ORDER BY created_at) t")
query('current_usage',"SELECT row_to_json(t) FROM (SELECT account_id,count(*) AS count,min(created_at) AS first_success,max(created_at) AS last_success FROM usage_logs WHERE account_id IN ("+ids+") AND created_at>='2026-10-27T00:00:00+08:00' GROUP BY account_id) t")
query('error_detail_shape',"SELECT row_to_json(t) FROM (SELECT account_id,created_at,status_code,upstream_status_code,jsonb_typeof(upstream_errors) AS upstream_errors_type,CASE WHEN jsonb_typeof(upstream_errors)='array' THEN jsonb_array_length(upstream_errors) END AS upstream_error_count,CASE WHEN jsonb_typeof(upstream_errors)='array' AND jsonb_array_length(upstream_errors)>0 THEN (SELECT json_agg(k) FROM jsonb_object_keys(upstream_errors->0) k) END AS first_error_keys FROM ops_error_logs WHERE account_id IN ("+ids+") AND created_at>='2026-10-27T14:30:00+08:00' ORDER BY created_at DESC LIMIT 8) t")

sessions=[]
byemail={m['email'].lower():m for m in members}
for a in rows:
    m=byemail.get(a['email'].lower())
    if m and m.get('last_login_at'):
        stamp=m['last_login_at'].replace(' ','T')+'Z'
        sessions.append('('+str(a['id'])+",'"+stamp+"'::timestamptz)")
if sessions:
    cte='WITH sessions(account_id,login_at) AS (VALUES '+','.join(sessions)+'), boundary AS (SELECT s.*, (SELECT min(e.created_at) FROM ops_error_logs e WHERE e.account_id=s.account_id AND e.created_at>=s.login_at AND (e.status_code=401 OR e.upstream_status_code=401)) AS first_401 FROM sessions s) '
    query('latest_login_windows',cte+"SELECT row_to_json(t) FROM (SELECT b.*, (SELECT max(u.created_at) FROM usage_logs u WHERE u.account_id=b.account_id AND u.created_at>=b.login_at AND u.created_at<COALESCE(b.first_401,NOW())) AS last_success_before_401,(SELECT count(*) FROM usage_logs u WHERE u.account_id=b.account_id AND u.created_at>=b.login_at AND u.created_at<COALESCE(b.first_401,NOW())) AS successes_before_401,(SELECT count(*) FROM ops_error_logs e WHERE e.account_id=b.account_id AND e.created_at>=b.login_at AND e.created_at<COALESCE(b.first_401,NOW()) AND (e.status_code IN (408,429) OR e.upstream_status_code IN (408,429))) AS failed_408_429_before_401,(SELECT count(*) FROM proxy_failover_events e WHERE e.account_id=b.account_id AND e.created_at>=b.login_at AND e.created_at<COALESCE(b.first_401,NOW())) AS proxy_changes_before_401 FROM boundary b) t")
    query('recent_session_errors',cte+"SELECT row_to_json(t) FROM (SELECT e.account_id,e.created_at,e.status_code,e.upstream_status_code,e.model,left(e.upstream_error_message,500) AS upstream_message,jsonb_path_query_array(e.upstream_errors,'$[*].status_code') AS attempt_status_codes FROM boundary b JOIN ops_error_logs e ON e.account_id=b.account_id WHERE b.first_401 IS NOT NULL AND e.created_at>=GREATEST(b.login_at,b.first_401-interval '2 minutes') AND e.created_at<=b.first_401 ORDER BY e.account_id,e.created_at) t")

# Only structured log fields; never emit message bodies or credentials.
for name,label in [('adobe-relay-app-1','sub2_logs'),('adobe-team-cookie-app-1','okad_logs')]:
    proc=subprocess.run(['docker','logs','--timestamps','--since','2026-10-27T06:30:00Z','--tail','120000',name],text=True,capture_output=True,timeout=20)
    lines=(proc.stdout+proc.stderr).splitlines()
    out=[]
    for line in lines:
        at=line.split()[0] if line.split() else ''
        pos=line.find('{')
        if pos<0:continue
        try:d=json.loads(line[pos:])
        except ValueError:continue
        match=re.search(r'/accounts/(\d+)(?:/|$)',d.get('path',''))
        aid=d.get('account_id',int(match[1]) if match else None)
        if aid not in {r['id'] for r in rows}:continue
        safe={k:d[k] for k in ['account_id','request_id','path','method','status_code','stage','attempt','version','proxy_id','upstream_http_status','stage_attempts','stage_elapsed_ms','processing_ms','latency_ms','model','result'] if k in d}
        safe['at']=at
        safe['event_names']=re.findall(r'(?:adobe_|okad_)[a-z_]+',line[:pos])
        out.append(safe)
    OUT[label]={'scanned':len(lines),'first_at':lines[0].split()[0] if lines else None,'last_at':lines[-1].split()[0] if lines else None,'events':out,'returncode':proc.returncode}
OUT['finished_at']=datetime.datetime.now(datetime.timezone.utc).isoformat()
print(json.dumps(OUT,ensure_ascii=False))
