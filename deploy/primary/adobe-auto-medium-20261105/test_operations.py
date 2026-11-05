"""Validate generated remote programs before running an archive or email action."""
import ast
from pathlib import Path
import runpy
import subprocess
import sys
from unittest import TestCase, mock


class OperationsTests(TestCase):
    def test_archive_and_notify_construct_valid_remote_programs(self):
        path = Path(__file__).with_name('operations.py')
        for phase in ['archive', 'notify', 'defer']:
            with self.subTest(phase), mock.patch.object(sys, 'argv', [str(path), phase, 'a' * 40, '123']), mock.patch.object(subprocess, 'run', return_value=subprocess.CompletedProcess([], 0)) as run:
                runpy.run_path(str(path), run_name='__main__')
                scripts = [call.kwargs['input'] for call in run.call_args_list if 'input' in call.kwargs]
                self.assertEqual(len(scripts), 1)
                ast.parse(scripts[0])
