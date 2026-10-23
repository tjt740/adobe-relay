import copy
import importlib.util
from pathlib import Path
from unittest import TestCase

spec = importlib.util.spec_from_file_location('capacity', Path(__file__).with_name('capacity.py'))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class CapacityTests(TestCase):
    def fixture(self):
        before = {'at': 'before', 'accounts': [dict(id=i, proxy_id=1, quarantined=True, temporary=False,
                    manual_pause=False, intent_hash='same', status='active', schedulable=False) for i in range(1, 4)],
                  'allocation': {'nodes': [{'id': 1, 'status': 'healthy'}], 'available_slots': 0}}
        after = copy.deepcopy(before)
        after['at'] = 'after'
        for row in after['accounts']:
            row['proxy_id'] = None
        after['allocation']['available_slots'] = 3
        return before, after

    def test_released_accounts_count_once_and_empty_node_count_is_separate(self):
        before, after = self.fixture()
        out = m.compare(before, after)
        self.assertEqual(out['initial_408_bindings_now_released'], 3)
        self.assertEqual(out['released_on_initially_healthy_nodes'], 3)
        self.assertEqual(out['previously_all_408_nodes_now_empty'], 1)
        self.assertEqual(out['free_healthy_slots_net_change'], 3)

    def test_running_test_is_counted_as_occupied(self):
        before, after = self.fixture()
        after['accounts'][0].update(proxy_id=1, temporary=True)
        out = m.compare(before, after)
        self.assertEqual(out['initial_408_bindings_now_released'], 2)
        self.assertEqual(out['initial_408_bindings_currently_testing'], 1)
        self.assertEqual(out['previously_all_408_nodes_now_empty'], 0)

    def test_capacity_reused_by_regular_account_is_not_an_empty_node(self):
        before, after = self.fixture()
        row = dict(before['accounts'][0], id=4, quarantined=False)
        after['accounts'].append(row)
        out = m.compare(before, after)
        self.assertEqual(out['initial_408_bindings_now_released'], 3)
        self.assertEqual(out['previously_all_408_nodes_now_empty'], 0)

    def test_unhealthy_node_is_not_counted_as_healthy_capacity(self):
        before, after = self.fixture()
        before['allocation']['nodes'][0]['status'] = 'unknown'
        out = m.compare(before, after)
        self.assertEqual(out['released_on_initially_healthy_nodes'], 0)
        self.assertEqual(out['previously_all_408_nodes_healthy_before_and_after_now_empty'], 0)

    def test_manual_state_and_configuration_changes_are_not_silently_accepted(self):
        for field, value in [('manual_pause', True), ('intent_hash', 'changed'), ('proxy_id', 1), ('schedulable', True)]:
            with self.subTest(field=field):
                before, after = self.fixture()
                after['accounts'][0][field] = value
                with self.assertRaises(AssertionError):
                    m.compare(before, after)
