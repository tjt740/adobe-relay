from pathlib import Path
import importlib.util
from unittest import TestCase
s=importlib.util.spec_from_file_location('backup_guard',Path(__file__).with_name('backup_guard.py'));m=importlib.util.module_from_spec(s);s.loader.exec_module(m)
class BackupGuardTests(TestCase):
    def test_verified_empty_or_soft_deleted_baseline_is_valid(self):
        m.validate_accounts_copy(True,True,0,0)
        m.validate_accounts_copy(True,True,100,0)
        m.validate_accounts_copy(True,True,100,30)
    def test_missing_copy_or_truncated_copy_is_rejected(self):
        for args in [(False,False,0,0),(True,False,0,0),(True,True,1,2)]:
            with self.assertRaises(AssertionError):m.validate_accounts_copy(*args)
