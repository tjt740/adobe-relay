import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('adobe_memory_burst_release', Path(__file__).with_name('adobe-memory-burst-release.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class MemoryBurstReleaseGuardsTest(unittest.TestCase):
    def test_snapshot_rejects_unapproved_memory_configuration(self):
        app = {'HostConfig': {'Memory': 12 * 1024**3, 'MemorySwap': 24 * 1024**3},
               'Config': {'Env': ['GOMEMLIMIT=9216MiB']}}
        with patch.object(guard, '_original_snapshot', return_value={}), patch.object(guard.release, 'inspect', return_value=app):
            self.assertEqual(guard.snapshot()['memory_limits']['go_memory_limit'], '9216MiB')
            app['HostConfig']['Memory'] = 14 * 1024**3
            with self.assertRaises(AssertionError):
                guard.snapshot()

    def test_new_repair_preserves_existing_limits_and_configuration_checks(self):
        before = {'memory_limits': {'memory': 12 * 1024**3}}
        with patch.object(guard, '_original_check', return_value={'configuration': True}) as protected:
            self.assertTrue(guard.check_preserved(before, before)['memory_limits_preserved'])
            protected.assert_called_once_with(before, before)
            with self.assertRaises(AssertionError):
                guard.check_preserved(before, {'memory_limits': {'memory': 8 * 1024**3}})


if __name__ == '__main__':
    unittest.main()
