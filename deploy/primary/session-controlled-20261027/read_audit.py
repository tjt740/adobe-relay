import json,sqlite3,datetime,hashlib
c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);c.row_factory=sqlite3.Row;c.execute('PRAGMA query_only=ON')
def safe(v):
 if isinstance(v,dict):
  return {k:safe(x) for k,x in v.items() if k in ['status','state','action','result','code','recorded_at','attempted_at','completed_at','items','attempts','verified','complete','count','success','changed','requested_preference','current_kind','account_type','profiles_complete','personal_organizations_complete']}
 if isinstance(v,list):return [safe(x) for x in v[:15]]
 if isinstance(v,(str,int,float,bool)) or v is None:return v
 return str(type(v))
for mid in [1138,75,90]:
 m=dict(c.execute('SELECT id,email,last_login_at,account_profile FROM external_members WHERE id=?',(mid,)).fetchone());p=json.loads(m['account_profile'] or '{}');cleanup=p.get('organization_cleanup')
 print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'member_id':mid,'last_login_at':m['last_login_at'],'profile':safe(p),'cleanup_keys':list(cleanup) if isinstance(cleanup,dict) else [],'cleanup':safe(cleanup),'personal_organizations_type':type(p.get('personal_organizations')).__name__,'personal_organizations_count':len(p.get('personal_organizations') or [])},ensure_ascii=False))
c.close()
