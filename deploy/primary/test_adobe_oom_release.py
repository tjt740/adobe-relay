import copy
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('oom_release', Path(__file__).with_name('adobe-oom-release.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class OOMReleaseGuardTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.backup = Path(self.directory.name) / 'resources.yml'
        self.raw = b'services:\n  app:\n    mem_limit: 2g\n    environment:\n      GOMEMLIMIT: 1536MiB\n      GOMAXPROCS: "6"\n'
        self.backup.write_bytes(self.raw)
        self.patcher = patch.object(module, 'RESOURCE_BACKUP', self.backup)
        self.patcher.start()
        self.addCleanup(self.patcher.stop)
        self.before = {'containers': {module.release.APP: {'id': 'old', 'image_id': 'old'}, 'okad': {'id': 'unchanged', 'image_id': 'same'}},
                       'config_hashes': {str(module.RESOURCE): module.release.digest(self.raw), 'other': 'unchanged'},
                       'env_without_image_hash': 'same', 'okad_settings_hash': 'same', 'settings_hashes': {'smtp': 'same'}, 'rules_hash': 'same',
                       'app_memory': {'memory': 2*1024**3, 'go_memory_limit': '1536MiB', 'other_environment_hash': 'same'}}
        self.current = copy.deepcopy(self.before)
        self.current['config_hashes'][str(module.RESOURCE)] = module.release.digest(module.expanded_resources(self.raw))
        self.current['app_memory'].update(memory=8*1024**3, go_memory_limit='6144MiB')

    def test_only_approved_memory_change_passes(self):
        self.assertTrue(all(module.check_preserved(self.before, self.current).values()))
        self.assertEqual(self.before['app_memory']['memory'], 2*1024**3)

    def test_other_configuration_or_environment_is_rejected(self):
        for key in ['other_environment_hash', 'memory', 'go_memory_limit']:
            with self.subTest(key=key):
                current = copy.deepcopy(self.current)
                current['app_memory'][key] = 'unreviewed'
                with self.assertRaises(AssertionError):
                    module.check_preserved(self.before, current)
        self.current['config_hashes']['other'] = 'changed'
        with self.assertRaises(AssertionError):
            module.check_preserved(self.before, self.current)

    def test_unreviewed_resource_change_or_backup_replacement_is_rejected(self):
        self.current['config_hashes'][str(module.RESOURCE)] = 'unreviewed'
        with self.assertRaises(AssertionError):
            module.check_preserved(self.before, self.current)
        self.backup.write_bytes(self.raw + b'# changed\n')
        with self.assertRaises(AssertionError):
            module.check_preserved(self.before, self.current)

    def test_expansion_refuses_other_baselines(self):
        with self.assertRaises(AssertionError):
            module.expanded_resources(self.raw.replace(b'2g', b'4g'))


if __name__ == '__main__':
    unittest.main()
