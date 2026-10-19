"""Exercise group APIs with explicitly inert synthetic records; never touch business accounts."""
import argparse
import datetime
import importlib.util
import json
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parent
STATE = ROOT / 'group-test-fixture.json'
REPORT = ROOT / 'group-verification.json'
EMAIL = 'group-release-20261020@verification.invalid'
PREFIX = '发布验证-20261020-'
spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
audit = importlib.util.module_from_spec(spec); spec.loader.exec_module(audit)


def sql(query):
    result = subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'], input=query, text=True, capture_output=True, check=True, timeout=30)
    return result.stdout.strip()


def quoted(value):
    return "'" + value.replace("'", "''") + "'"


def okad_code(code):
    return json.loads(subprocess.check_output(['docker','exec','-i','adobe-team-cookie-app-1','python','-c',code], text=True, timeout=30))


def credentials():
    # Tokens and keys stay in memory and never enter the audit output.
    context = okad_code('''from app.db.session import SessionLocal
from app.models.user import User
from app.models.setting import Setting
from app.core.security import create_access_token
from sqlalchemy import select
import json
with SessionLocal() as db:
 u=db.scalar(select(User).where(User.is_active.is_(True),User.is_superuser.is_(True)))
 issuer=db.scalar(select(Setting.value).where(Setting.key=='okad_instance_id'))
 print(json.dumps({'token':create_access_token(u.id,expires_minutes=10,token_version=u.token_version),'issuer':issuer}))''')
    key = audit.sql("SELECT value FROM settings WHERE key='admin_api_key'")
    assert key and context['issuer']
    return context, key


def request(base, headers, path, method='GET', body=None, expected=200):
    payload = None if body is None else json.dumps(body, ensure_ascii=False).encode()
    req = urllib.request.Request(base+path, data=payload, method=method, headers={**headers,'Content-Type':'application/json'})
    try:
        with urllib.request.urlopen(req,timeout=45) as response:
            assert response.status == expected, (path,response.status)
            return json.load(response)
    except urllib.error.HTTPError as error:
        if error.code == expected:
            return json.load(error)
        # Do not emit a response that could contain credentials.
        raise RuntimeError(f'{method} {path.split("?")[0]} returned {error.code}') from None


def save_state(state):
    STATE.write_text(json.dumps(state, ensure_ascii=False, indent=2)); STATE.chmod(0o600)


def cleanup():
    if not STATE.exists():
        return {'cleaned':False,'reason':'no fixture'}
    state = json.loads(STATE.read_text())
    if state.get('cleaned'):
        return {'cleaned':True,'already_cleaned':True}
    # Only delete the exact synthetic account created by this run; no bulk predicates.
    if state.get('remote_id'):
        rid = int(state['remote_id'])
        row = json.loads(sql(f"SELECT json_build_object('name',name,'email',credentials->>'email','schedulable',schedulable,'status',status,'proxy_id',proxy_id) FROM accounts WHERE id={rid}"))
        assert row['name'] == PREFIX+'账号' and row['email'] == EMAIL and not row['schedulable'] and row['status'] == 'inactive' and row['proxy_id'] is None
        # Delete through the API so scheduler caches receive the normal invalidation.
        _, key = credentials()
        request('http://127.0.0.1:6666/api/v1', {'x-api-key':key},f'/admin/accounts/{rid}','DELETE')
        assert sql(f'SELECT count(*) FROM accounts WHERE id={rid} AND deleted_at IS NULL') == '0'
    code = '''from app.db.session import SessionLocal
from app.models.external_member import ExternalMember,ExternalMemberGroup
from sqlalchemy import select
import json
state='''+repr(state)+'''
with SessionLocal() as db:
 row=db.get(ExternalMember,state.get('member_id')) if state.get('member_id') else None
 if row:
  assert row.email=='''+repr(EMAIL)+''' and not row.cookie and not row.adobe_password
  db.delete(row);db.flush()
 for gid in state.get('groups',[]):
  group=db.get(ExternalMemberGroup,gid)
  if group:
   assert group.name.startswith('''+repr(PREFIX)+''')
   assert not db.scalar(select(ExternalMember.id).where(ExternalMember.group_id==gid).limit(1))
   db.delete(group)
 db.commit()
 print(json.dumps({'cleaned':True}))'''
    result = okad_code(code)
    state['cleaned'] = True; save_state(state)
    return result


def verify(keep):
    assert not STATE.exists(), 'Prior test state exists; inspect and clean it first'
    context,key = credentials()
    oh={'Authorization':'Bearer '+context['token']};sh={'x-api-key':key}
    ob='http://127.0.0.1:9500/api/external';sb='http://127.0.0.1:6666/api/v1'
    state={'email':EMAIL,'groups':[],'started':datetime.datetime.now(datetime.timezone.utc).isoformat()};save_state(state)
    checks={}
    try:
        for suffix in ('A','B'):
            data=request(ob,oh,'/groups','POST',{'name':PREFIX+suffix},201)
            state['groups'].append(data['id']);save_state(state)
        request(ob,oh,'/groups','POST',{'name':PREFIX+'A'},409);checks['create_and_duplicate']=True
        local=okad_code('''from app.db.session import SessionLocal
from app.models.external_member import ExternalMember
from sqlalchemy import select
import json
with SessionLocal() as db:
 assert not db.scalar(select(ExternalMember.id).where(ExternalMember.email=='''+repr(EMAIL)+'''))
 row=ExternalMember(email='''+repr(EMAIL)+''',otp_mode='vietnam',note='''+repr(PREFIX+'独立临时测试')+''')
 db.add(row);db.commit();print(json.dumps({'id':row.id}))''')
        state['member_id']=local['id'];save_state(state)
        creds=json.dumps({'email':EMAIL});extra=json.dumps({'adobeteam_external_email':EMAIL,'okad_manual_pause':True})
        state['remote_id']=int(sql("INSERT INTO accounts(name,platform,type,credentials,extra,status,schedulable,concurrency,priority) VALUES ("+quoted(PREFIX+'账号')+",'adobe','oauth',"+quoted(creds)+"::jsonb,"+quoted(extra)+"::jsonb,'inactive',false,1,99999) RETURNING id" ).splitlines()[0]);save_state(state)
        before=sql(f"SELECT md5(credentials::text)||':'||status||':'||schedulable::text||':'||COALESCE(proxy_id::text,'') FROM accounts WHERE id={state['remote_id']}")
        def result(base,headers,params):
            value=request(base,headers,'/members?'+urllib.parse.urlencode(params)) if base==ob else request(base,headers,'/admin/accounts?'+urllib.parse.urlencode(params))['data']
            return value
        def move(gid):
            request(ob,oh,'/members/group','PATCH',{'ids':[state['member_id']],'group_id':gid})
            own=result(ob,oh,{'keyword':EMAIL,'group_id':gid or 0,'account_type':'vietnam','size':1})
            assert own['total']==1 and own['items'][0]['id']==state['member_id']
            group='ungrouped' if gid is None else context['issuer']+':'+str(gid)
            deadline=time.monotonic()+60
            while True:
                remote=result(sb,sh,{'search':EMAIL,'okad_group':group,'okad_account_type':'vietnam','page_size':1})
                if remote['total']==1 and remote['items'][0]['id']==state['remote_id']:break
                assert time.monotonic()<deadline,'Metadata did not converge'
                time.sleep(2)
            return group
        first,second=state['groups']
        ga=move(first);checks['assign_and_cross_app_type_group_filter']=True
        options=request(sb,sh,'/admin/accounts/okad-groups')['data'];assert any(g['value']==ga and g['name']==PREFIX+'A' for g in options)
        assert result(sb,sh,{'search':EMAIL,'okad_group':ga,'okad_account_type':'company'})['total']==0
        gb=move(second)
        assert result(ob,oh,{'keyword':EMAIL,'group_id':first})['total']==0
        assert result(sb,sh,{'search':EMAIL,'okad_group':ga})['total']==0
        assert result(sb,sh,{'search':EMAIL,'okad_group':gb,'okad_account_type':'non_company'})['total']==0
        checks['regroup_removes_old_membership']=True
        move(None);checks['ungrouped_filter']=True
        move(second)
        after=sql(f"SELECT md5(credentials::text)||':'||status||':'||schedulable::text||':'||COALESCE(proxy_id::text,'') FROM accounts WHERE id={state['remote_id']}")
        assert before==after;checks['credentials_status_proxy_and_schedule_unchanged']=True
        jobs=okad_code('''from app.db.session import engine
from sqlalchemy import text
import json
with engine.connect() as c:
 print(json.dumps({'count':c.execute(text("SELECT count(*) FROM job_records WHERE deleted=0 AND status IN ('running','pausing','paused','cancelling')")).scalar()}))''')
        checks['no_active_okad_jobs']=jobs['count']==0
        output={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'checks':checks,'fixture_member_id':state['member_id'],'fixture_remote_id':state['remote_id'],'kept_for_browser':keep,'triggered_adobe_login_or_generation':False,'sent_mail':False}
        REPORT.write_text(json.dumps(output,ensure_ascii=False,indent=2));REPORT.chmod(0o600)
        print(json.dumps(output,ensure_ascii=False))
    finally:
        if not keep:
            print(json.dumps(cleanup(),ensure_ascii=False))


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('action',choices=['verify','cleanup']);parser.add_argument('--keep-fixture',action='store_true');args=parser.parse_args()
    if args.action=='cleanup':print(json.dumps(cleanup(),ensure_ascii=False))
    else:verify(args.keep_fixture)
