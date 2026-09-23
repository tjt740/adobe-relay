import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("clash_init_env", Path(__file__).with_name("init_env.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class EnvironmentUpgradeTest(unittest.TestCase):
    def test_adds_secret_once_and_preserves_existing_credentials(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / ".env"
            existing = "ADMIN_PASSWORD=unchanged\nJWT_SECRET=keep-this\nSUB2API_IMAGE=old"
            path.write_text(existing)
            self.assertTrue(module.ensure_secret(path))
            updated = path.read_text()
            self.assertTrue(updated.startswith(existing + "\n"))
            secret = updated.split("CLASH_CONTROLLER_SECRET=")[1].strip()
            self.assertEqual(len(secret), 64)
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertFalse(module.ensure_secret(path))
            self.assertEqual(path.read_text(), updated)

    def test_refuses_missing_or_empty_configuration(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / ".env"
            with self.assertRaises(FileNotFoundError):
                module.ensure_secret(path)
            path.write_text('CLASH_CONTROLLER_SECRET=""\n')
            with self.assertRaises(ValueError):
                module.ensure_secret(path)
            self.assertEqual(path.read_text(), 'CLASH_CONTROLLER_SECRET=""\n')


if __name__ == "__main__":
    unittest.main()
