import copy
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('image_response_release', Path(__file__).with_name('image-response-release.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ImageResponseReleaseGuardsTest(unittest.TestCase):
    def setUp(self):
        self.sha = 'a' * 40
        self.before = {'containers': {release.APP: {'image': release.OLD_IMAGE, 'id': 'old', 'image_id': 'old-image'},
                       'okad': {'id': 'okad-id', 'image_id': 'okad-image'}},
                       'config_hashes': {'config': 'hash'}, 'env_without_image_hash': 'env',
                       'okad_settings_hash': 'link', 'settings_hashes': {'private_smtp': 'hash'}, 'rules_hash': 'rules'}
        self.current = copy.deepcopy(self.before)
        self.current['containers'][release.APP] = {'image': 'adobe-relay:' + self.sha, 'id': 'new', 'image_id': 'new-image'}

    def test_code_only_rollback_passes(self):
        self.assertTrue(all(release.validate_rollback(self.before, self.current, self.sha, 'old-image').values()))

    def test_later_release_or_changed_rollback_tag_is_rejected(self):
        with self.assertRaises(AssertionError):
            release.validate_rollback(self.before, self.current, 'b' * 40, 'old-image')
        with self.assertRaises(AssertionError):
            release.validate_rollback(self.before, self.current, self.sha, 'replaced-image')

    def test_configuration_and_other_service_changes_require_review(self):
        for key in ('config_hashes', 'env_without_image_hash', 'okad_settings_hash', 'settings_hashes', 'rules_hash'):
            with self.subTest(key=key):
                current = copy.deepcopy(self.current)
                current[key] = 'changed'
                with self.assertRaises(AssertionError):
                    release.validate_rollback(self.before, current, self.sha, 'old-image')
        self.current['containers']['okad']['id'] = 'new-okad'
        with self.assertRaises(AssertionError):
            release.validate_rollback(self.before, self.current, self.sha, 'old-image')

    def test_live_business_changes_do_not_restore_old_rows(self):
        self.before['account_state'] = [{'id': 1, 'credentials_hash': 'old'}]
        self.current['account_state'] = [{'id': 1, 'credentials_hash': 'new'}, {'id': 2}]
        release.validate_rollback(self.before, self.current, self.sha, 'old-image')
        self.assertEqual(self.current['account_state'][0]['credentials_hash'], 'new')


if __name__ == '__main__':
    unittest.main()
