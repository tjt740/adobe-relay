import importlib.util
from pathlib import Path
from unittest import TestCase
p=Path(__file__).with_name('migration_guard.py')
spec=importlib.util.spec_from_file_location('migration_guard',p);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)

class MigrationGuardTests(TestCase):
    def test_uses_exact_checked_migration_and_production_trim_rule(self):
        path=Path(__file__).resolve().parents[3]/'backend/migrations'/m.NAME
        self.assertEqual(m.SQL,path.read_text())
        self.assertEqual(m.migration_checksum(m.SQL),m.migration_checksum(' \n'+m.SQL+' \n'))
        self.assertNotEqual(m.migration_checksum(m.SQL),m.migration_checksum(m.SQL.replace('0 AND 3','0 AND 4')))
    def test_exact_addition_preserves_all_previous_checksums(self):
        before={'001.sql':'old'}
        m.validate_migrations({**before,m.NAME:m.migration_checksum(m.SQL)},before)
    def test_rejects_old_checksum_change_and_unexpected_migration(self):
        before={'001.sql':'old'}
        for installed in [{'001.sql':'changed',m.NAME:m.migration_checksum(m.SQL)},
                          {**before,m.NAME:m.migration_checksum(m.SQL),'256.sql':'unexpected'}]:
            with self.assertRaises(AssertionError):m.validate_migrations(installed,before)
