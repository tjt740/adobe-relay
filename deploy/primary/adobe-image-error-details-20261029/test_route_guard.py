from unittest import TestCase
from route_guard import validate_excluded_route, validate_subscription_exclusion


class ExcludedRouteTests(TestCase):
    def test_retired_subscription_is_not_recreated(self):
        self.assertTrue(validate_subscription_exclusion([{'id':'current','proxy_ids':[9],'blocked_proxy_ids':[]}])['not_in_current_subscriptions'])

    def test_a_present_excluded_node_must_still_be_blocked(self):
        with self.assertRaises(AssertionError):
            validate_subscription_exclusion([{'id':'current','proxy_ids':[1290],'blocked_proxy_ids':[]}])
        self.assertEqual(validate_subscription_exclusion([{'id':'current','proxy_ids':[1290],'blocked_proxy_ids':[1290]}])['source_ids'],['current'])

    def review(self, status):
        return {'proxy_id': 1290, 'read_only': True, 'backup_excluded': True, 'backup_status': status}

    def current(self, status='active', **changes):
        return {'excluded': True, 'status': status, 'bindings': 0, 'leases': 0, **changes}

    def test_preserves_both_existing_statuses_with_the_permanent_exclusion(self):
        for status in ['active', 'inactive']:
            self.assertTrue(validate_excluded_route(self.review(status), self.current(status))['status_matches_cutover_backup'])

    def test_rejects_a_status_change_from_the_actual_backup(self):
        with self.assertRaises(AssertionError):
            validate_excluded_route(self.review('inactive'), self.current('active'))

    def test_rejects_removing_the_exclusion(self):
        with self.assertRaises(AssertionError):
            validate_excluded_route(self.review('active'), self.current(excluded=False))

    def test_rejects_new_bindings_or_detection_leases(self):
        for field in ['bindings', 'leases']:
            with self.assertRaises(AssertionError):
                validate_excluded_route(self.review('active'), self.current(**{field: 1}))

    def test_rejects_a_backup_without_the_permanent_exclusion(self):
        with self.assertRaises(AssertionError):
            validate_excluded_route({**self.review('active'), 'backup_excluded': False}, self.current())
