import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import subprocess
import sys
from unittest import TestCase, mock

from clash_contract_guard import BEFORE, AFTER, reviewed_hash

HERE = Path(__file__).parent
spec = importlib.util.spec_from_file_location('clash_review_audit', HERE / 'audit.py')
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)


class ClashContractReviewTests(TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.raw = (HERE / 'clash-concurrent-import-review.json').read_bytes()
        self.review = json.loads(self.raw)
        self.observation = {key: value for key, value in self.review.items()
                            if key not in ['unchanged_application_sources', 'server_observation_file', 'interpretation']}

    def install(self):
        (self.root / 'clash-concurrent-import-review.json').write_bytes(self.raw)
        (self.root / self.review['server_observation_file']).write_text(json.dumps(self.observation))

    def test_no_review_keeps_original_guard(self):
        self.assertEqual(reviewed_hash(BEFORE, self.root), BEFORE)

    def test_audit_loads_by_absolute_path_outside_release_directory(self):
        script = "import importlib.util,sys; s=importlib.util.spec_from_file_location('remote_audit',sys.argv[1]); m=importlib.util.module_from_spec(s); s.loader.exec_module(m); assert callable(m.check_preserved)"
        subprocess.run([sys.executable, '-I', '-c', script, str((HERE / 'audit.py').resolve())], cwd=self.root, check=True, capture_output=True, text=True)

    def test_exact_review_and_independent_observation(self):
        self.install()
        self.assertEqual(reviewed_hash(BEFORE, self.root), AFTER)
        with self.assertRaises(AssertionError):
            reviewed_hash('different-baseline', self.root)

    def test_forged_receipt_or_extra_node_change_is_rejected(self):
        self.install()
        for field, value in [('current_contract_hash', '0' * 32), ('node_changed_field_counts', {'password': 1}), ('import_log_entries', [])]:
            with self.subTest(field=field):
                changed = {**self.review, field: value}
                (self.root / 'clash-concurrent-import-review.json').write_text(json.dumps(changed))
                with self.assertRaises(AssertionError):
                    reviewed_hash(BEFORE, self.root)

    def test_missing_or_changed_server_evidence_is_rejected(self):
        self.install()
        path = self.root / self.review['server_observation_file']
        path.unlink()
        with self.assertRaises(FileNotFoundError):
            reviewed_hash(BEFORE, self.root)
        path.write_text(json.dumps({**self.observation, 'current_revision': 34}))
        with self.assertRaises(AssertionError):
            reviewed_hash(BEFORE, self.root)

    def test_full_guard_still_rejects_new_import_and_other_changes(self):
        self.install()
        before = {'containers': {}, 'config_hashes': {}, 'env_without_image_hash': 'same', 'okad_settings_hash': 'same',
                  'settings_hashes': {}, 'clash_contract_hash': BEFORE, 'rules_hash': 'same'}
        current = {**before, 'clash_contract_hash': AFTER}
        original = copy.deepcopy(before)
        with mock.patch.object(audit, 'ROOT', self.root):
            self.assertTrue(all(audit.check_preserved(before, current).values()))
            for patch in [{'clash_contract_hash': 'unreviewed'}, {'rules_hash': 'changed'}, {'settings_hashes': {'smtp': 'changed'}}]:
                with self.subTest(patch=patch), self.assertRaises(AssertionError):
                    audit.check_preserved(before, {**current, **patch})
        self.assertEqual(before, original)
