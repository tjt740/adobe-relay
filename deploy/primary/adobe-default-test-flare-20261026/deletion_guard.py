"""Explain only the pinned post-cutover soft deletions using authenticated audit."""
import datetime
import json


def moment(value):
    return datetime.datetime.fromisoformat(value.replace('Z', '+00:00'))


def validate(business, evidence, scope):
    missing = set(business['missing_ids'])
    assert missing == set(scope['expected_missing_ids']), 'Unreviewed missing account IDs'
    assert business['cutover_backup'] == evidence['cutover_backup'] == scope['cutover_backup']
    assert evidence['application_image'] == 'adobe-relay:' + scope['application_sha']
    assert evidence['read_only'] is True
    assert evidence['original_business_digest'] == scope['original_business_digest'], 'Original evidence changed'
    rows = {row['id']: row for row in evidence['rows']}
    assert set(rows) == missing, 'Missing physical rows cannot be accepted as soft deletes'
    cutover = moment(business['proxy_events_since'])
    matches = {}
    for account_id, row in rows.items():
        assert row['deleted_at'], 'Account remains active'
        deleted = moment(row['deleted_at'])
        assert deleted >= cutover, 'Deletion predates the cutover'
        for event in evidence['events']:
            if not (event['actor_user_id'] and event['actor_role'] == 'admin' and event['auth_method'] == 'jwt' and event['status_code'] == 200):
                continue
            end = moment(event['created_at'])
            start = end - datetime.timedelta(milliseconds=event['latency_ms'])
            if not (start - datetime.timedelta(seconds=2) <= deleted <= end + datetime.timedelta(seconds=2)):
                continue
            if event['method'] == 'POST' and event['path'] == '/api/v1/admin/accounts/batch-delete':
                requested = json.loads(event['request_body']).get('account_ids', [])
            elif event['method'] == 'DELETE' and event['path'] == '/api/v1/admin/accounts/:id':
                requested = [int(event['params']['id'])]
            else:
                continue
            if account_id in requested:
                matches[account_id] = event['id']
                break
    assert set(matches) == missing, 'Each soft deletion needs a matching successful admin JWT request'
    return {'reviewed_deleted_ids': sorted(missing), 'audit_ids': sorted(set(matches.values())), 'soft_delete_rows_preserved': True, 'restored_old_data': False}
