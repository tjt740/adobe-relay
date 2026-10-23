import importlib.util
import json
from pathlib import Path
import tempfile
from unittest import TestCase

spec = importlib.util.spec_from_file_location('candidate_history', Path(__file__).with_name('candidate_history.py'))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

class CandidateHistoryTests(TestCase):
    old = 'a' * 40
    new = 'b' * 40

    def setup_root(self, path):
        root = Path(path)
        (root / 'artifact').mkdir()
        evidence = {'sha': self.old, 'image_id': 'sha256:old', 'workflow_run': 42}
        (root / 'image-loaded.json').write_text(json.dumps(evidence))
        (root / 'artifact/ci-proof.json').write_text(json.dumps({'application_sha': self.old}))
        return root, evidence

    def test_repeated_prepare_keeps_receipt_and_removes_only_active_metadata(self):
        with tempfile.TemporaryDirectory() as directory:
            root, evidence = self.setup_root(directory)
            m.archive_candidate(root, self.new)
            m.archive_candidate(root, self.new)
            self.assertFalse((root / 'image-loaded.json').exists())
            self.assertEqual(json.loads((root / 'candidate-history' / self.old / 'image-loaded.json').read_text()), evidence)

    def test_partial_prior_prepare_keeps_original_proof(self):
        with tempfile.TemporaryDirectory() as directory:
            root, evidence = self.setup_root(directory)
            history = root / 'candidate-history' / self.old
            history.mkdir(parents=True)
            (history / 'image-loaded.json').write_text(json.dumps(evidence))
            (history / 'ci-proof.json').write_text(json.dumps({'application_sha': self.old}))
            (root / 'artifact/ci-proof.json').write_text(json.dumps({'application_sha': 'c' * 40}))
            m.archive_candidate(root, self.new)
            self.assertEqual(json.loads((history / 'ci-proof.json').read_text())['application_sha'], self.old)
            self.assertFalse((root / 'image-loaded.json').exists())

    def test_started_candidate_is_never_replaced(self):
        with tempfile.TemporaryDirectory() as directory:
            root, _ = self.setup_root(directory)
            (root / 'progress.json').write_text('{}')
            with self.assertRaises(AssertionError):
                m.archive_candidate(root, self.new)
            self.assertTrue((root / 'image-loaded.json').exists())

    def test_conflicting_history_is_not_overwritten(self):
        with tempfile.TemporaryDirectory() as directory:
            root, evidence = self.setup_root(directory)
            history = root / 'candidate-history' / self.old
            history.mkdir(parents=True)
            (history / 'image-loaded.json').write_text(json.dumps({**evidence, 'image_id': 'different'}))
            with self.assertRaises(AssertionError):
                m.archive_candidate(root, self.new)
            self.assertTrue((root / 'image-loaded.json').exists())
