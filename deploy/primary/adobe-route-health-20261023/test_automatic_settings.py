import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
from unittest import TestCase, mock

spec=importlib.util.spec_from_file_location('audit',Path(__file__).with_name('audit.py'))
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)

class AutomaticSettingReviewTests(TestCase):
    def setUp(self):
        self.old={'claude_code_client_version_synced':hashlib.md5(b'2.1.305').hexdigest(),'smtp':hashlib.md5(b'preserved').hexdigest()}
        self.review={'key':'claude_code_client_version_synced','old_value':'2.1.305','new_value':'2.1.309','old_hash':self.old['claude_code_client_version_synced'],'new_hash':hashlib.md5(b'2.1.309').hexdigest(),'sync_source_path':'backend/internal/service/claude_code_version_sync_service.go','sync_source_before':'b477e00bc6a393ee4cf985ae881ce3e036603e8e96f6bf3bdcb0a302b91fba5b','sync_source_candidate':'b477e00bc6a393ee4cf985ae881ce3e036603e8e96f6bf3bdcb0a302b91fba5b','event':'claude_code_version_synced','evidence_log_sha256':'a'*64}

    def test_original_snapshot_is_not_rewritten(self):
        original=copy.deepcopy(self.old)
        expected=m.reviewed_setting_hashes(self.old,self.review)
        self.assertEqual(self.old,original)
        self.assertNotEqual(expected,self.old)
        self.assertEqual(expected['smtp'],self.old['smtp'])

    def test_other_keys_mismatched_evidence_and_downgrades_are_rejected(self):
        for patch in [{'key':'smtp'},{'old_hash':'0'*32},{'new_hash':'0'*32},{'sync_source_candidate':'0'*64},{'new_value':'2.1.304'}]:
            with self.subTest(patch=patch), self.assertRaises(AssertionError):
                m.reviewed_setting_hashes(self.old,{**self.review,**patch})

    def test_full_guard_accepts_only_the_exact_reviewed_hash(self):
        before={'containers':{},'config_hashes':{},'env_without_image_hash':'same','okad_settings_hash':'same','settings_hashes':self.old,'clash_contract_hash':'same','rules_hash':'same'}
        current=copy.deepcopy(before)
        current['settings_hashes']=m.reviewed_setting_hashes(self.old,self.review)
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(m,'ROOT',Path(directory)):
            with self.assertRaises(AssertionError):m.check_preserved(before,current)
            (Path(directory)/'reviewed-automatic-settings.json').write_text(json.dumps(self.review))
            self.assertTrue(all(m.check_preserved(before,current).values()))
            current['settings_hashes']['smtp']='unreviewed'
            with self.assertRaises(AssertionError):m.check_preserved(before,current)
