import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('selected_access', Path(__file__).with_name('selected-test-access.py'))
access = importlib.util.module_from_spec(spec)
spec.loader.exec_module(access)


class SelectedAccessTests(unittest.TestCase):
    def test_existing_groups_cannot_be_deleted(self):
        for gid in [7, 33, 0]:
            with self.subTest(gid=gid), self.assertRaises(AssertionError):
                access.validate_group(dict(id=gid, name=access.NAME, platform='adobe', is_exclusive=True))

    def test_other_group_names_and_platforms_cannot_be_deleted(self):
        good = dict(id=999, name=access.NAME, platform='adobe', is_exclusive=True)
        access.validate_group(good)
        for field, value in [('name', 'customer-group'), ('platform', 'openai'), ('is_exclusive', False)]:
            with self.subTest(field=field), self.assertRaises(AssertionError):
                access.validate_group(dict(good, **{field: value}))

    def test_foreign_keys_cannot_be_deleted(self):
        good = dict(id=999, user_id=1, group_id=1000, name=access.NAME)
        access.validate_key(good, 1000)
        for field, value in [('id', 12), ('user_id', 2), ('group_id', 33), ('name', 'customer-key')]:
            with self.subTest(field=field), self.assertRaises(AssertionError):
                access.validate_key(dict(good, **{field: value}), 1000)
