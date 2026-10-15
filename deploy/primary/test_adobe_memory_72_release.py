import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('adobe_memory_72_release', Path(__file__).with_name('adobe-memory-72-release.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class Memory72ReleaseGuardsTest(unittest.TestCase):
    def test_separate_audit_and_actual_previous_image(self):
        self.assertEqual(guard.release.ROOT.name, 'adobe-memory-72-20261015')
        self.assertEqual(guard.release.OLD_IMAGE, 'adobe-relay:6868148a26779e94c097eb9626cdd51233a5ac98')
        self.assertEqual(guard.release.ROLLBACK_IMAGE, 'adobe-relay:before-adobe-memory-72-20261015')

    def test_runtime_guard_still_requires_12_and_9_gib(self):
        app = {'HostConfig': {'Memory': 12 * 1024**3, 'MemorySwap': 24 * 1024**3},
               'Config': {'Env': ['GOMEMLIMIT=9216MiB']}}
        with patch.object(guard.guard, '_original_snapshot', return_value={}), patch.object(guard.release, 'inspect', return_value=app):
            self.assertEqual(guard.snapshot()['memory_limits']['memory'], 12 * 1024**3)
            app['Config']['Env'] = ['GOMEMLIMIT=10240MiB']
            with self.assertRaises(AssertionError):
                guard.snapshot()


if __name__ == '__main__':
    unittest.main()
