"""Compile every generated remote command without connecting to production."""
import importlib.util
import os
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile
from unittest import TestCase, mock
import hashlib

ROOT = Path(__file__).parent
SHA = 'a' * 40


class OperationTests(TestCase):
    def test_all_remote_phases_generate_valid_python_and_use_the_dedicated_release(self):
        for phase in ['prepare', 'start', 'status', 'verify', 'archive']:
            with self.subTest(phase=phase), tempfile.TemporaryDirectory() as directory:
                previous = Path.cwd()
                try:
                    os.chdir(directory)
                    artifact = Path('release-artifact/release-images.tar.gz')
                    artifact.parent.mkdir()
                    artifact.write_bytes(b'isolated fixture')
                    Path(str(artifact) + '.sha256').write_text(hashlib.sha256(artifact.read_bytes()).hexdigest() + ' ' + artifact.name)
                    docs = Path('deploy/primary')
                    docs.mkdir(parents=True)
                    for name in ['README.md', 'adobe-408-timeout-scheduling-20261024.md']:
                        (docs / name).write_text('Verified release documentation')
                    (docs / 'release.sh').write_text('fixture')
                    with mock.patch.dict(os.environ, {'TARGET_ACCOUNT_IDS':'1,2'}), mock.patch.object(sys, 'argv', ['operations.py', phase, SHA, '123']), mock.patch.object(subprocess, 'run', return_value=subprocess.CompletedProcess([], 0, stdout='{}')) as run:
                        runpy.run_path(str(ROOT / 'operations.py'), run_name='__main__')
                    scripts = [call.kwargs['input'] for call in run.call_args_list if 'input' in call.kwargs]
                    self.assertTrue(scripts)
                    for script in scripts:
                        compile(script, '<remote-command>', 'exec')
                        self.assertIn('adobe-408-timeout-scheduling-20261024', script)
                        self.assertNotIn('account-manual-ip-release-20261023', script)
                        self.assertNotIn('edit-gate.py', script)
                    if phase == 'prepare':
                        self.assertIn('.adobe-deploy.lock', scripts[-1])
                    if phase == 'verify':
                        self.assertIn('check-rollback', scripts[-1])
                finally:
                    os.chdir(previous)

    def test_previous_application_supports_existing_manual_proxy_accounts(self):
        spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
        audit = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(audit)
        before = {'containers': {audit.APP: {'image_id': 'saved'}}}
        current = {'containers': {audit.APP: {'image': 'adobe-relay:' + SHA}}, 'manual_proxy_account_ids': [10]}
        with mock.patch.object(audit, 'check_preserved', return_value={'preserved': True}):
            self.assertEqual(audit.validate_rollback(before, current, SHA, 'saved'), {'preserved': True})
        with self.assertRaises(AssertionError):
            audit.validate_rollback(before, current, 'b' * 40, 'saved')
