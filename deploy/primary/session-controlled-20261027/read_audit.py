import sqlite3,json,datetime
c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);c.row_factory=sqlite3.Row
ids=[75,69,62,63,65,1138]
rows=[]
for r in c.execute('SELECT id,email,profile_preference,last_login_at FROM external_members WHERE id IN ('+','.join('?' for _ in ids)+')',ids):rows.append(dict(r))
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'selected_profile_preferences','data':rows},ensure_ascii=False))
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'profile_preference_counts','data':[dict(r) for r in c.execute('SELECT profile_preference,count(*) count FROM external_members GROUP BY profile_preference')]},ensure_ascii=False));c.close()
