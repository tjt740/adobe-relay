"""Check the generated remote guard programs without touching production."""
import ast
from pathlib import Path
import runpy
import subprocess
import sys
from unittest import TestCase, mock


class OperationsTests(TestCase):
    def test_supported_remote_programs_are_valid(self):
        path = Path(__file__).with_name('operations.py')
        for phase in ['preflight', 'start', 'public-test', 'public-test-remaining', 'verify', 'archive', 'status']:
            with self.subTest(phase), mock.patch.object(sys, 'argv', [str(path), phase, 'a' * 40, '123']), mock.patch.object(subprocess, 'run', return_value=subprocess.CompletedProcess([], 0, stdout='{}')) as run, mock.patch.object(Path, 'mkdir'), mock.patch.object(Path, 'write_text'):
                runpy.run_path(str(path), run_name='__main__')
                scripts = [call.kwargs['input'] for call in run.call_args_list if 'input' in call.kwargs]
                self.assertEqual(len(scripts), 1)
                ast.parse(scripts[0])

    def test_email_and_queue_deferral_are_not_release_phases(self):
        path = Path(__file__).with_name('operations.py')
        for phase in ['notify', 'defer']:
            with self.subTest(phase), mock.patch.object(sys, 'argv', [str(path), phase, 'a' * 40, '123']), mock.patch.object(subprocess, 'run') as run:
                with self.assertRaises(AssertionError):
                    runpy.run_path(str(path), run_name='__main__')
                run.assert_not_called()
