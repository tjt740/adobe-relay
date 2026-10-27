import json,pathlib,sqlite3,hashlib,base64,datetime
ROOT=pathlib.Path('/opt/adobe-relay/releases/expanded-20261027');sel=json.loads((ROOT/'selected.json').read_text())
def digest(v):return hashlib.sha256(str(v).encode()).hexdigest()
def meta(token):
 try:
  p=token.split('.')[1];c=json.loads(base64.urlsafe_b64decode(p+'='*(-len(p)%4)))
 except:return {'present':bool(token)}
 return {'token_sha256':digest(token),**{k:c.get(k) for k in ['client_id','account_type','created_at','expires_in','exp','iat']},**{k+'_sha256':digest(c.get(k,'')) for k in ['sid','user_id','aa_id','ownerOrg','as']}}
c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);c.row_factory=sqlite3.Row
for a in sel:
 r=dict(c.execute('SELECT * FROM external_members WHERE id=?',(a['member_id'],)).fetchone());f=pathlib.Path('/dev/shm/adobe-expanded-20261027-'+str(a['id'])+'.json')
 private=json.loads(f.read_text()) if f.exists() else {};rec=private.get('record') or {}
 old=meta(r.get('access_token') or '');new=meta(rec.get('access_token') or '')
 out={'account_id':a['id'],'old':old,'new':new,'same_identity':old.get('user_id_sha256')==new.get('user_id_sha256'),'same_profile_type':old.get('account_type')==new.get('account_type'),'stored_profile':{k:r.get(k) for k in ['profile_preference','force_code_login','otp_mode','subscription_ok','subscription_status','credits_available','credits_total','last_login_at']},'new_record_keys':list(rec),'new_profile':{k:rec.get(k) for k in ['account_type','subscription_ok','subscription_status','credits_available','credits_total']}}
 print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'comparison':out},ensure_ascii=False))
