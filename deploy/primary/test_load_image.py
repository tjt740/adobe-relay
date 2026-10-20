import gzip
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class CompressedImageTransferTest(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        binaries = self.root / "bin"
        binaries.mkdir()
        self.payload = b"example docker image layer\n" * 4096
        self.archive = self.root / "image with spaces.tar.gz"
        self.archive.write_bytes(gzip.compress(self.payload))
        mocks = {
            "ssh": """#!/usr/bin/env python3
import os, pathlib, subprocess, sys
root=pathlib.Path(os.environ['TRANSFER_TEST_ROOT'])
data=sys.stdin.buffer.read()
(root/'wire').write_bytes(data)
if os.environ.get('TRANSFER_SSH_FAIL'): sys.exit(255)
if sys.argv[-2] != 'root@173.234.15.150': sys.exit(90)
sys.exit(subprocess.run(sys.argv[-1], shell=True, input=data).returncode)
""",
            "docker": """#!/usr/bin/env python3
import os, pathlib, sys
if sys.argv[1:] != ['load']: sys.exit(91)
pathlib.Path(os.environ['TRANSFER_TEST_ROOT'], 'loaded').write_bytes(sys.stdin.buffer.read())
sys.exit(int(os.environ.get('TRANSFER_DOCKER_EXIT', '0')))
""",
        }
        for name, source in mocks.items():
            path = binaries / name
            path.write_text(source)
            path.chmod(0o755)
        self.env = dict(os.environ, TRANSFER_TEST_ROOT=str(self.root),
                        PATH=str(binaries) + os.pathsep + os.environ["PATH"])

    def transfer(self, target="root@173.234.15.150", archive=None, **env):
        return subprocess.run(
            ["bash", str(Path(__file__).with_name("load-image.sh")),
             str(archive or self.archive), target, "-o", "StrictHostKeyChecking=yes"],
            env=dict(self.env, **env), capture_output=True, text=True, timeout=10)

    def test_compressed_wire_and_exact_loaded_bytes(self):
        result = self.transfer()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.root / "wire").read_bytes(), self.archive.read_bytes())
        self.assertLess((self.root / "wire").stat().st_size, len(self.payload))
        self.assertEqual((self.root / "loaded").read_bytes(), self.payload)

    def test_corrupt_trailer_fails_even_when_docker_accepts_bytes(self):
        self.archive.write_bytes(self.archive.read_bytes()[:-4])
        result = self.transfer()
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue((self.root / "loaded").exists())

    def test_docker_failure_reaches_caller(self):
        self.assertEqual(self.transfer(TRANSFER_DOCKER_EXIT="23").returncode, 23)

    def test_ssh_failure_reaches_caller(self):
        self.assertEqual(self.transfer(TRANSFER_SSH_FAIL="1").returncode, 255)
        self.assertFalse((self.root / "loaded").exists())

    def test_other_server_is_rejected_before_connecting(self):
        self.assertNotEqual(self.transfer(target="root@192.0.2.1").returncode, 0)
        self.assertFalse((self.root / "wire").exists())

    def test_missing_archive_is_rejected_before_connecting(self):
        self.assertNotEqual(self.transfer(archive=self.root / "missing").returncode, 0)
        self.assertFalse((self.root / "wire").exists())


if __name__ == "__main__":
    unittest.main()
