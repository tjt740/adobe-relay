import copy
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('account_errors_release', Path(__file__).with_name('account-errors-release.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class AccountErrorsRollbackTest(unittest.TestCase):
    def setUp(self):
        self.sha = 'a' * 40
        self.before = {'containers': {release.base.APP: {'image': release.OLD_IMAGE, 'id': 'old', 'image_id': 'old'},
            release.OKAD: {'id': 'okad-before', 'image_id': 'okad-before'}, 'redis': {'id': 'redis', 'image_id': 'redis'}},
            'config_hashes': {'config': 'hash', release.COMPOSE: 'old'}, 'env_without_image_hash': 'env',
            'okad_compose_without_image': 'compose', 'okad_settings_hash': 'settings',
            'settings_hashes': {'private_smtp': 'hash'}, 'rules_hash': 'rules'}
        self.final = copy.deepcopy(self.before)
        self.final['containers'][release.base.APP] = {'image': 'adobe-relay:' + self.sha, 'id': 'new', 'image_id': 'new'}
        self.final['containers'][release.OKAD] = {'id': 'okad-after', 'image_id': 'okad-after'}
        self.final['config_hashes'][release.COMPOSE] = 'new'

    def test_expected_image_changes_preserve_data(self):
        release.preserved(self.before, self.final)
        self.final['account_state'] = [{'id': 1, 'credentials_hash': 'live-new-value'}]
        release.validate_rollback(self.before, self.final, self.final, self.sha, 'old')

    def test_later_code_config_and_mail_are_rejected(self):
        for key in ['config_hashes', 'env_without_image_hash', 'okad_compose_without_image', 'okad_settings_hash', 'settings_hashes', 'rules_hash']:
            with self.subTest(key=key):
                current = copy.deepcopy(self.final)
                current[key] = {} if key.endswith('hashes') else 'changed'
                with self.assertRaises(AssertionError):
                    release.validate_rollback(self.before, current, self.final, self.sha, 'old')
        for app in [release.base.APP, release.OKAD, 'redis']:
            current = copy.deepcopy(self.final)
            current['containers'][app]['image_id'] = 'later'
            with self.assertRaises(AssertionError):
                release.validate_rollback(self.before, current, self.final, self.sha, 'old')
        with self.assertRaises(AssertionError):
            release.validate_rollback(self.before, self.final, self.final, self.sha, 'wrong-rollback')
