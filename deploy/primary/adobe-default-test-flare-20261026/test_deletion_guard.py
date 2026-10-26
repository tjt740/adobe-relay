from copy import deepcopy
from unittest import TestCase
from deletion_guard import validate


class DeletionGuardTests(TestCase):
    def setUp(self):
        self.business = {'missing_ids': [1, 2], 'cutover_backup': 'backup', 'proxy_events_since': '2026-10-26T05:24:41+00:00'}
        self.scope = {'expected_missing_ids': [1, 2], 'cutover_backup': 'backup', 'application_sha': 'a'*40, 'original_business_digest': 'b'*64}
        self.evidence = {'cutover_backup': 'backup', 'application_image': 'adobe-relay:'+'a'*40, 'read_only': True, 'original_business_digest': 'b'*64,
                         'rows': [{'id': i, 'deleted_at': '2026-10-26T05:25:00+00:00'} for i in [1, 2]],
                         'events': [{'id': 10, 'actor_user_id': 1, 'actor_role': 'admin', 'auth_method': 'jwt', 'status_code': 200,
                                     'created_at': '2026-10-26T05:25:01+00:00', 'latency_ms': 2000, 'method': 'POST',
                                     'path': '/api/v1/admin/accounts/batch-delete', 'request_body': '{"account_ids":[1,2]}'}]}

    def test_accepts_only_audited_post_cutover_soft_deletes(self):
        result = validate(self.business, self.evidence, self.scope)
        self.assertEqual(result['reviewed_deleted_ids'], [1, 2])
        self.assertEqual(result['audit_ids'], [10])
        self.assertFalse(result['restored_old_data'])

    def test_rejects_additional_missing_accounts(self):
        with self.assertRaises(AssertionError):
            validate({**self.business, 'missing_ids': [1, 2, 3]}, self.evidence, self.scope)

    def test_rejects_missing_physical_rows_or_active_rows(self):
        for rows in [[], [{'id': 1, 'deleted_at': None}, self.evidence['rows'][1]]]:
            with self.assertRaises(AssertionError):
                validate(self.business, {**self.evidence, 'rows': rows}, self.scope)

    def test_rejects_failed_non_admin_and_non_jwt_requests(self):
        for change in [{'status_code': 403}, {'actor_role': 'user'}, {'auth_method': 'api_key'}, {'actor_user_id': None}, {'request_body': '{"account_ids":[1]}'}]:
            evidence = deepcopy(self.evidence)
            evidence['events'][0].update(change)
            with self.assertRaises(AssertionError):
                validate(self.business, evidence, self.scope)

    def test_rejects_deletion_outside_request_or_before_cutover(self):
        for when in ['2026-10-26T05:24:00+00:00', '2026-10-26T05:26:00+00:00']:
            evidence = deepcopy(self.evidence)
            evidence['rows'][0]['deleted_at'] = when
            with self.assertRaises(AssertionError):
                validate(self.business, evidence, self.scope)

    def test_rejects_changed_backup_application_or_original_evidence(self):
        for change in [{'cutover_backup': 'other'}, {'application_image': 'later'}, {'original_business_digest': 'changed'}, {'read_only': False}]:
            with self.assertRaises(AssertionError):
                validate(self.business, {**self.evidence, **change}, self.scope)

    def test_rejects_missing_audit(self):
        with self.assertRaises(AssertionError):
            validate(self.business, {**self.evidence, 'events': []}, self.scope)
