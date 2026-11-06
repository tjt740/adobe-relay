import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('bindings', Path(__file__).with_name('selected-binding-review.py'))
bindings = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bindings)


class SelectedBindingTests(unittest.TestCase):
    def test_normal_api_timestamp_change_keeps_group_and_priority(self):
        self.assertTrue(bindings.validate([dict(group_id=33, priority=1, created_at='old')], [dict(group_id=33, priority=1, created_at='new')], [33])['original_group_priorities_preserved'])

    def test_changed_priority_or_group_is_rejected(self):
        before = [dict(group_id=33, priority=1)]
        for current in [[dict(group_id=33, priority=2)], [dict(group_id=36, priority=1)], []]:
            with self.subTest(current=current), self.assertRaises(AssertionError):
                bindings.validate(before, current, [33])
