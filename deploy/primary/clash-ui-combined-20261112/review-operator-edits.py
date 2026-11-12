"""Retain exactly the live operator edits explicitly confirmed in this chat."""
import datetime,hashlib,importlib.util,json,pathlib,re,subprocess,sys
ROOT=pathlib.Path('/opt/adobe-relay/releases/clash-ui-combined-20261112')
APP_SHA='e1c9b83ace0472ad7e97b35d4bf2c4dd5dc1574b'
APPROVED_PAUSE_IDS=[1912, 1924, 1930, 1931, 1939, 1942, 1989, 1990, 2029, 2046, 2093, 2095, 2096, 2097, 2098, 2099, 2100, 2101, 2102, 2103, 2104, 2105, 2106, 2108, 2116, 2117, 2120, 2121, 2122, 2128, 2135, 2142, 2157, 2159, 2196, 2199, 2201, 2202, 2205, 2210, 2227, 2233, 2236, 2239, 2242, 2243, 2245, 2255, 2256, 2297, 2301, 2311, 2335, 2339, 2419, 2481, 2488, 2522, 2530, 2531, 2582, 2662, 2710]
APPROVED_MAPPING_IDS=[2805, 2807, 2808, 2811, 2812, 2814, 2815, 2816, 2817, 2818, 2819, 2820, 2821, 2822, 2823, 2824, 2825, 2838, 2839, 2840, 2841, 2842, 2843, 2844, 2845, 2846, 2848, 2955, 2964, 2970, 3018, 3019, 3020, 3021, 3022, 3023, 3028, 3029, 3039, 3040, 3041, 3042, 3043, 3044, 3045, 3046, 3047, 3048, 3050]

def validate(review,pause_ids,mapping_ids,missing):
 assert not missing,'Original accounts are missing'
 assert sorted(c['id'] for c in review['changes'] if 'manual_pause' in c['fields'])==APPROVED_PAUSE_IDS
 assert sorted(c['id'] for c in review['changes'] if 'model_mapping' in c['credential_keys_changed'])==APPROVED_MAPPING_IDS
 assert sorted(pause_ids)==APPROVED_PAUSE_IDS,'Additional manual pause changes require their own review'
 assert sorted(mapping_ids)==APPROVED_MAPPING_IDS,'Additional model mapping changes require their own review'
 assert review['manual_proxy_modes_preserved'] and not review['missing_ids']
 assert all(c['proxy_chain_matches'] for c in review['changes'])

def main():
 sys.path.insert(0,str(ROOT));import audit as a
 assert (ROOT/'expected-sha').read_text().strip()==APP_SHA
 assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+APP_SHA
 review=json.loads((ROOT/'business-verification.json').read_text())
 raw=subprocess.check_output(['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--data-only','--table=accounts','-f','-'],input=(pathlib.Path(review['cutover_backup'])/'database.dump').read_bytes(),timeout=90).decode()
 escapes={'b':'\b','f':'\f','n':'\n','r':'\r','t':'\t','v':'\v','\\':'\\'}
 def value(s):return None if s=='\\N' else re.sub(r'\\(.)',lambda m:escapes.get(m[1],m[1]),s)
 old={};active=False
 for line in raw.splitlines():
  if line.startswith('COPY public.accounts ('):
   cols=[c.strip().strip('"') for c in line.split('(',1)[1].split(')',1)[0].split(',')];active=True;continue
  if active and line=='\\.':active=False;continue
  if active:
   row=dict(zip(cols,map(value,line.split('\t'))))
   if row['deleted_at'] is None:old[int(row['id'])]={'manual_pause':json.loads(row['extra'] or '{}').get('okad_manual_pause'),'mapping':json.loads(row['credentials'] or '{}').get('model_mapping')}
 rows=json.loads(a.sql("SELECT coalesce(json_agg(json_build_object('id',id,'manual_pause',extra->'okad_manual_pause','schedulable',schedulable,'mapping',credentials->'model_mapping')),'[]') FROM accounts WHERE deleted_at IS NULL"));current={r['id']:r for r in rows}
 pause=[i for i in old if i in current and old[i]['manual_pause']!=current[i]['manual_pause']]
 mapping=[i for i in old if i in current and old[i]['mapping']!=current[i]['mapping']]
 validate(review,pause,mapping,sorted(set(old)-set(current)))
 out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':APP_SHA,'user_confirmation':'是，我做了这些操作，保留现状','question_scope':{'manual_pause_count':63,'model_mapping_count':49},'status':'confirmed_operator_changes_retained','original_business_report_sha256':hashlib.sha256((ROOT/'business-verification.json').read_bytes()).hexdigest(),'approved_pause_ids':APPROVED_PAUSE_IDS,'approved_mapping_ids':APPROVED_MAPPING_IDS,'manual_pause_changes':[{'id':i,'before':old[i]['manual_pause'],'current':current[i]['manual_pause'],'schedulable':current[i]['schedulable']} for i in pause],'mapping_hashes':{str(i):hashlib.sha256(json.dumps(current[i]['mapping'],sort_keys=True).encode()).hexdigest() for i in mapping},'other_manual_pause_states_preserved':True,'manual_proxy_modes_preserved':True,'business_data_written':False,'restored_old_data':False}
 a.save('operator-changes-review.json',out)
 print(json.dumps({'confirmed_manual_pause_changes':len(pause),'confirmed_model_mapping_changes':len(mapping),'other_manual_pause_states_preserved':True,'business_data_written':False}))
if __name__=='__main__':main()
