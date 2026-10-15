import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('oom_release', Path(__file__).with_name('adobe-memory-adaptive-release.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class AdaptiveMemoryReleaseGuardTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.backup = Path(self.directory.name) / 'resources.yml'
        self.raw = b'services:\n  app:\n    mem_limit: 8g\n    environment:\n      GOMEMLIMIT: 6144MiB\n      GOMAXPROCS: "6"\n'
        self.backup.write_bytes(self.raw)
        self.patcher = patch.object(module, 'RESOURCE_BACKUP', self.backup)
        self.patcher.start()
        self.addCleanup(self.patcher.stop)
        root_patcher = patch.object(module.release, 'ROOT', Path(self.directory.name))
        root_patcher.start()
        self.addCleanup(root_patcher.stop)
        self.before = {'containers': {module.release.APP: {'id': 'old', 'image_id': 'old'}, 'okad': {'id': 'unchanged', 'image_id': 'same'}},
                       'config_hashes': {str(module.RESOURCE): module.release.digest(self.raw), 'other': 'unchanged'},
                       'env_without_image_hash': 'same', 'okad_settings_hash': 'same', 'settings_hashes': {'smtp': 'same'}, 'rules_hash': 'same',
                       'app_memory': {'memory': 8*1024**3, 'go_memory_limit': '6144MiB', 'other_environment_hash': 'same'}}
        self.current = copy.deepcopy(self.before)
        self.current['config_hashes'][str(module.RESOURCE)] = module.release.digest(module.expanded_resources(self.raw))
        self.current['app_memory'].update(memory=12*1024**3, memory_swap=24*1024**3, go_memory_limit='9216MiB')

    def test_only_approved_memory_change_passes(self):
        self.assertTrue(all(module.check_preserved(self.before, self.current).values()))
        self.assertEqual(self.before['app_memory']['memory'], 8*1024**3)

    def test_other_configuration_or_environment_is_rejected(self):
        for key in ['other_environment_hash', 'memory', 'memory_swap', 'go_memory_limit']:
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
            module.expanded_resources(self.raw.replace(b'8g', b'4g'))

    def test_reviewed_concurrent_okad_update_is_exactly_preserved(self):
        name = 'adobe-team-cookie-app-1'
        compose = '/opt/adobe-team-cookie/compose.yml'
        old = {'id': 'old-okad', 'image': 'old-tag', 'image_id': 'old-image'}
        new = {'id': 'new-okad', 'image': 'new-tag', 'image_id': 'new-image'}
        self.before['containers'][name] = old
        self.before['config_hashes'][compose] = 'old-compose'
        self.current['containers'][name] = new
        self.current['config_hashes'][compose] = 'new-compose'
        review = {'before_container': old, 'after_container': new,
                  'before_compose_hash': 'old-compose', 'after_compose_hash': 'new-compose'}
        with self.assertRaises(AssertionError):
            module.check_preserved(self.before, self.current)
        (module.release.ROOT / 'reviewed-concurrent-okad.json').write_text(json.dumps(review))
        self.assertTrue(module.check_preserved(self.before, self.current)['reviewed_concurrent_okad_preserved'])
        for field in ('id', 'image', 'image_id'):
            changed = copy.deepcopy(self.current)
            changed['containers'][name][field] = 'later-release'
            with self.assertRaises(AssertionError):
                module.check_preserved(self.before, changed)
        self.current['config_hashes'][compose] = 'later-config'
        with self.assertRaises(AssertionError):
            module.check_preserved(self.before, self.current)



if __name__ == '__main__':
    unittest.main()
