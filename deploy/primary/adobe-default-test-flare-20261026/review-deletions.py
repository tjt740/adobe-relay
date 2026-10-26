"""Collect deletion metadata only; no credentials or business writes."""
import datetime
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path('/opt/adobe-relay/releases/adobe-default-test-flare-20261026')


def collect(a):
    business = json.loads((ROOT / 'business-verification.json').read_text())
    ids = business['missing_ids']
    assert ids and all(isinstance(value, int) for value in ids)
    cut = business['proxy_events_since']
    rows = json.loads(a.sql("SELECT coalesce(json_agg(json_build_object('id',id,'deleted_at',deleted_at)),'[]') FROM accounts WHERE id IN (" + ','.join(map(str, ids)) + ')'))
    events = json.loads(a.sql("SELECT coalesce(json_agg(json_build_object('id',id,'created_at',created_at,'actor_user_id',actor_user_id,'actor_role',actor_role,'auth_method',auth_method,'action',action,'method',method,'path',path,'status_code',status_code,'latency_ms',latency_ms,'request_body',request_body,'params',extra->'params') ORDER BY id),'[]') FROM audit_logs WHERE created_at>='" + cut + "'::timestamptz AND ((path='/api/v1/admin/accounts/batch-delete' AND method='POST') OR (path='/api/v1/admin/accounts/:id' AND method='DELETE'))"))
    return {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'application_image': a.inspect(a.APP)['Config']['Image'], 'cutover_backup': business['cutover_backup'], 'original_business_digest': hashlib.sha256(json.dumps(business, sort_keys=True, separators=(',', ':')).encode()).hexdigest(), 'rows': rows, 'events': events, 'read_only': True}


if __name__ == '__main__':
    spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
    a = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(a)
    evidence = collect(a)
    if not (ROOT / 'deletion-evidence.json').exists():
        a.save('deletion-evidence.json', evidence)
    print(json.dumps(evidence))
