import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('access', Path(__file__).with_name('customer-test-access.py'))
access = importlib.util.module_from_spec(spec)
spec.loader.exec_module(access)


class CustomerAccessTests(unittest.TestCase):
    def test_cleanup_removes_only_its_new_grant(self):
        self.assertEqual(access.cleanup_groups([3], [3, 9, 33], 0), [3, 9])

    def test_cleanup_rejects_preexisting_access(self):
        with self.assertRaises(AssertionError):
            access.cleanup_groups([33], [33], 0)

    def test_cleanup_rejects_concurrent_dependent_key(self):
        with self.assertRaises(AssertionError):
            access.cleanup_groups([], [33], 1)

    def test_cleanup_rejects_removed_original_permissions(self):
        with self.assertRaises(AssertionError):
            access.cleanup_groups([3], [33], 0)

    def test_cleanup_cannot_delete_customer_or_existing_key(self):
        good = dict(id=98765, user_id=1, group_id=33, name=access.NAME)
        access.validate_owned_key(good)
        for field, value in [('id', 12), ('user_id', 2), ('group_id', 7), ('name', 'existing')]:
            with self.subTest(field=field), self.assertRaises(AssertionError):
                access.validate_owned_key(dict(good, **{field: value}))
