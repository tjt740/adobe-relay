"""Recognize one independently observed import; never restore or broadly allow changes."""
import hashlib
import json

BEFORE = '1e32db4c3008faf77f9bd36394327b81'
AFTER = '1546d8db13442c6cc818740c03172343'
REVIEW_DIGEST = 'ec1832e289c98450702096d5345c7f0e0f63d4e7958eb0f3d982ba203ec034b1'
OBSERVATION_DIGEST = '6ea997bc0264e7ddb48109049bfc11d510412d9c5f87360d135d9f3d1398a932'


def canonical_digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def reviewed_hash(original, root):
    review_path = root / 'clash-concurrent-import-review.json'
    if not review_path.exists():
        return original
    raw = review_path.read_bytes()
    assert hashlib.sha256(raw).hexdigest() == REVIEW_DIGEST, 'Unreviewed subscription exception'
    review = json.loads(raw)
    assert original == BEFORE == review['before_contract_hash']
    assert review['current_contract_hash'] == AFTER
    observation = json.loads((root / review['server_observation_file']).read_text())
    assert canonical_digest(observation) == OBSERVATION_DIGEST, 'Original server observation differs'
    for field, value in observation.items():
        assert review[field] == value
    assert review['before_revision'] == 32 and review['current_revision'] == 33
    assert review['before_nodes'] == review['current_nodes'] == 597
    assert review['before_disabled_nodes'] == review['current_disabled_nodes'] == 0
    assert review['added_names'] == review['removed_names'] == 0 and review['url_unchanged']
    assert review['node_changed_field_counts'] == {'config': 24}
    assert review['config_changed_field_counts'] == {'port': 24}
    receipt, = review['import_log_entries']
    assert receipt['method'] == 'POST' and receipt['path'] == '/api/v1/admin/proxies/clash/import'
    assert receipt['status_code'] == 200 and receipt['request_id'] == '8c435c8a-1d82-4619-aa58-e632d7fa68de'
    assert all(row['baseline_sha256'] == row['application_sha256'] for row in review['unchanged_application_sources'])
    assert review['read_only'] and not review['restored_old_data'] and not review['credentials_in_output']
    return AFTER
