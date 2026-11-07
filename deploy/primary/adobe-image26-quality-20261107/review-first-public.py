"""Recover the first known successful output using existing evidence; no HTTP calls."""
import datetime, hashlib, json, pathlib, subprocess
from public_contract_guard import parse_pipeline_line, pipeline_matches, pipeline_evidence
ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-image26-quality-20261107')
PUBLIC=ROOT/'public'
source=PUBLIC/'flare-xhigh-result.json'
raw=source.read_bytes();row=json.loads(raw)
sha=(ROOT/'expected-sha').read_text().strip()
key=json.loads((ROOT/'business-key-created.json').read_text())
start=json.loads((PUBLIC/'flare-xhigh-started.json').read_text())
assert row['application_sha']==sha and row['model']=='gpt-image-2.6-flare' and row['quality']=='xhigh'
assert row['http_status']==200 and row['response_quality']=='xhigh' and row['outer_posts']==1
assert row['exception_type']=='AssertionError' and 'usage' in row and 'applied_submissions' not in row
assert start['api_key_id']==key['id'] and key['name']=='release-quality-26-20261107-temporary'
cleanup=json.loads((ROOT/'business-access-cleanup.json').read_text())
assert cleanup['temporary_key_deleted'] and cleanup['original_allowed_groups']==cleanup['current_allowed_groups']
assert not any((PUBLIC/(name+'-started.json')).exists() for name in ['flare-max','sunburst-xhigh','sunburst-max'])
image,=row['images'];data=(PUBLIC/'flare-xhigh.png').read_bytes()
assert image['png_fully_validated'] and image['width']==3840 and image['height']==2160
assert image['sha256']==hashlib.sha256(data).hexdigest() and image['bytes']==len(data)
matches={}
for command in [['docker','exec','adobe-relay-app-1','tail','-c','67108864','/app/data/logs/sub2api.log'],['docker','logs','--since','2h','--tail','50000','adobe-relay-app-1']]:
    p=subprocess.run(command,capture_output=True,text=True,timeout=30)
    for line in (p.stdout+'\n'+p.stderr).splitlines():
        entry=parse_pipeline_line(line)
        if pipeline_matches(entry,row['usage'],row['http_request_id'],key['id']):
            evidence=pipeline_evidence(entry);matches[json.dumps(evidence,sort_keys=True)]=evidence
assert len(matches)==1, 'Require one exact existing pipeline; never regenerate this case'
evidence,=matches.values();applied,=evidence['applied_submissions']
assert applied=={'submitted_model_version':'gpt-image-2.6-star-swirl','submitted_detail_level':6,'submit_attempts':1}
row.update(success=True,api_key_id=key['id'],pipeline=evidence,applied_submissions=evidence['applied_submissions'],pipeline_correlated=True,
           reviewed_without_generation=True,original_result_sha256=hashlib.sha256(raw).hexdigest(),reviewed_at=datetime.datetime.now(datetime.timezone.utc).isoformat())
row.pop('exception_type')
with (PUBLIC/'flare-xhigh-reviewed.json').open('x') as f:json.dump(row,f,indent=2)
print(json.dumps({'reviewed_without_generation':True,'api_key_id':key['id'],'actual_level':6,'original_result_preserved':True}))
