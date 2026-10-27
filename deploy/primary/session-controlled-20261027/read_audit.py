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

# Executed after shared read-only utility definitions and TARGETS.
OUT = {'started_at': datetime.datetime.now(datetime.timezone.utc).isoformat()}
names=','.join("'"+e.lower()+"'" for e in TARGETS)
query('proxy_events',"SELECT row_to_json(t) FROM (SELECT account_id,from_proxy_id,to_proxy_id,created_at FROM proxy_failover_events WHERE account_id IN (SELECT id FROM accounts WHERE deleted_at IS NULL AND lower(name) IN ("+names+")) AND created_at>='2026-10-27T14:30:00+08:00' ORDER BY created_at) t")
query('test_errors',"SELECT row_to_json(t) FROM (SELECT account_id,created_at,status_code,left(reason,600) AS reason FROM adobe_account_test_errors WHERE account_id IN (SELECT id FROM accounts WHERE deleted_at IS NULL AND lower(name) IN ("+names+")) AND created_at>='2026-10-27T14:30:00+08:00' ORDER BY created_at) t")
query('sub2_audit',"SELECT row_to_json(t) FROM (SELECT id,created_at,action,method,path,request_id,auth_method,status_code,jsonb_object_keys(CASE WHEN jsonb_typeof(request_body)='object' THEN request_body ELSE '{}'::jsonb END) AS body_key FROM audit_logs WHERE created_at>='2026-10-27T14:30:00+08:00' AND (path LIKE '%accounts%' OR path LIKE '%settings%') ORDER BY created_at DESC LIMIT 2000) t")
db=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);db.execute('PRAGMA query_only=ON');db.row_factory=sqlite3.Row
members=[]
for e in TARGETS:
    for table in ['external_members','adobe_members']:
        recs=db.execute('SELECT * FROM '+table+' WHERE lower(email)=?',(e.lower(),)).fetchall()
        for rec in recs:
            m=dict(rec);safe={k:m[k] for k in ['id','admin_id','email','status','registered','is_admin','is_imported','created_at','updated_at','last_login_at','login_status','subscription_status','subscription_ok','profile_preference','expires_at','sub2_pushed_at','exported_at','export_count','credits_available','credits_total'] if k in m}
            safe['table']=table;safe['token']=token_metadata(m.get('access_token'));safe['cookie']=cookie_metadata(m.get('cookie'))
            try:profile=json.loads(m.get('account_profile') or '{}')
            except Exception:profile={}
            if isinstance(profile,dict):
                safe['profile']={k:profile.get(k) for k in ['current_kind','account_type','requested_preference','fallback_reason','fetched_at','profiles_complete','personal_organizations_complete']}
                safe['profile']['current_org_hash']=digest(profile.get('current_org_id'))
                clean=profile.get('organization_cleanup')
                safe['cleanup_keys']=list(clean) if isinstance(clean,dict) else None
                if isinstance(clean,dict):safe['cleanup_scalar']={k:v for k,v in clean.items() if isinstance(v,(bool,int,float))}
                safe['profiles_summary']=[{k:p.get(k) for k in ['kind','type','account_type','is_default','is_current']} for p in profile.get('profiles',[]) if isinstance(p,dict)]
            members.append(safe)
OUT['member_profiles']=members
OUT['admin_accounts']=[{'id':r['id'],'email':r['email'],'is_valid':r['is_valid'],'has_org':r['has_org'],'org_hash':digest(r['org_id']),'last_checked_at':r['last_checked_at'],'last_login_at':r['last_login_at'],'updated_at':r['updated_at'],'member_count':r['member_count']} for r in db.execute('SELECT * FROM adobe_accounts')]
OUT['integration_deliveries']=[{k:r[k] for k in ['source_key','site_id','version','account_id','state','attempts','next_at','lease_until','updated_at']} for r in db.execute('SELECT * FROM integration_deliveries WHERE updated_at>=? ORDER BY updated_at',(1793082600,))]
job_rows=[dict(r) for r in db.execute('SELECT id,type,status,target,success,fail,created_at,finished_at,meta_json,extra_json,logs_json FROM job_records WHERE created_at>=? ORDER BY created_at',(1793082600,))]
OUT['okad_jobs']=[]
for job in job_rows:
    safe={k:job[k] for k in ['id','type','status','success','fail','created_at','finished_at']}
    safe['target_hash']=digest(job['target'])
    for col in ['meta_json','extra_json']:
        try:v=json.loads(job[col] or '{}')
        except Exception:v={}
        safe[col+'_keys']=list(v) if isinstance(v,dict) else []
        if isinstance(v,dict):safe[col+'_scalars']={k:z for k,z in v.items() if isinstance(z,(bool,int,float))}
    text=job['logs_json'] or ''
    safe['log_markers']={word:len(re.findall(word,text,re.I)) for word in ['invalid_credentials','invalid_token','401013','logout','revoke','profile','proxy','自动登录','冷却']}
    OUT['okad_jobs'].append(safe)
OUT['setting_keys']=[r['key'] for r in db.execute('SELECT key FROM settings')]
db.close()
source_code=r'''
import json,pathlib,hashlib,re,ast
base=pathlib.Path('/app/backend/app/services')
result={'paths':[],'matches':[],'snippets':{}}
for p in base.rglob('*.py'):
 text=p.read_text(errors='replace');relative=str(p.relative_to(base))
 if any(word in relative for word in ['firefly','integration','external','admin_auth','protocol/client','protocol/admin']):
  result['paths'].append({'path':relative,'sha256':hashlib.sha256(text.encode()).hexdigest()})
 for no,line in enumerate(text.splitlines(),1):
  words=re.findall(r'revoke|signout|logout|invalidate_token',line,re.I)
  if words and 'imap' not in relative and 'mail' not in relative:result['matches'].append({'path':relative,'line':no,'keywords':words})
 if relative in ['firefly.py','adobe_protocol/admin.py','adobe_protocol/client.py','reliable_integration.py']:
  try:tree=ast.parse(text)
  except Exception:continue
  for n in ast.walk(tree):
   if isinstance(n,(ast.FunctionDef,ast.AsyncFunctionDef)) and any(word in n.name for word in ['acquire_firefly','session_cookie','new_session','login_cooldown','claim_login','release_login','organization_cleanup']):
    result['snippets'][relative+':'+n.name]='\n'.join(text.splitlines()[n.lineno-1:n.end_lineno])
print(json.dumps(result))
'''
proc=subprocess.run(['docker','exec','-i','adobe-team-cookie-app-1','python','-c',source_code],text=True,capture_output=True,timeout=15)
if proc.returncode==0:OUT['actual_okad_source']=json.loads(proc.stdout)
else:OUT['actual_okad_source_error']='source inspection failed'
OUT['finished_at']=datetime.datetime.now(datetime.timezone.utc).isoformat()
print(json.dumps(OUT,ensure_ascii=False))
