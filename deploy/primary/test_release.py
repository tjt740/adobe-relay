import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class PrimaryReleaseTest(unittest.TestCase):
    def run_release(self, failure=""):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        root = Path(directory.name)
        shared = root / "shared"
        shared.mkdir()
        (shared / ".env").write_text("SUB2API_IMAGE=adobe-relay:old\nPRESERVE=value\n")
        for name in ("compose.yml", "clash.yml", "resources.yml"):
            (shared / name).write_text("existing configuration\n")
        binaries = root / "bin"
        binaries.mkdir()
        mock = '''#!/usr/bin/env bash
set -euo pipefail
name=${0##*/}
printf '%s %s\\n' "$name" "$*" >> "$TEST_ROOT/commands"
case "$name:$*" in
  'adobe-relay:ps -q app') echo app ;;
  'adobe-relay:ps -q postgres') echo postgres ;;
  'docker:inspect app --format '{{.Config.Image}}) echo adobe-relay:old ;;
  'docker:exec postgres pg_dump '*)
    [[ "$FAILURE" != backup ]] || exit 1
    echo database ;;
  'docker:exec -i postgres pg_restore '*) cat >/dev/null ;;
  'docker:exec app tar '*) echo app-data ;;
  'adobe-relay:up '*)
    if [[ "$FAILURE" == health ]] && grep -q 'SUB2API_IMAGE=adobe-relay:new' "$TEST_ROOT/shared/.env"; then
      exit 1
    fi ;;
esac
'''
        for name in ("docker", "adobe-relay", "flock", "curl"):
            p = binaries / name
            p.write_text(mock)
            p.chmod(0o755)
        script = root / "release.sh"
        script.write_text(Path(__file__).with_name("release.sh").read_text().replace(
            "root=/opt/adobe-relay", 'root="$TEST_ROOT"'))
        env = dict(os.environ, TEST_ROOT=str(root), FAILURE=failure,
                   PATH=str(binaries) + os.pathsep + os.environ["PATH"])
        result = subprocess.run(["bash", str(script), "adobe-relay:new"], env=env,
                                capture_output=True, text=True)
        return root, result

    def test_release_preserves_configuration_and_updates_only_app(self):
        root, result = self.run_release()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((root / "shared/.env").read_text(),
                         "SUB2API_IMAGE=adobe-relay:new\nPRESERVE=value\n")
        backups = list((root / "backups").iterdir())
        self.assertEqual(len(backups), 1)
        self.assertIn("adobe-relay:old", (backups[0] / "environment").read_text())
        self.assertTrue((backups[0] / "database.dump").is_file())
        commands = (root / "commands").read_text().splitlines()
        updates = [line for line in commands if line.startswith("adobe-relay up ")]
        self.assertEqual(len(updates), 1)
        self.assertIn("--no-deps", updates[0])
        self.assertTrue(updates[0].endswith(" app"))

    def test_failed_backup_does_not_change_image(self):
        root, result = self.run_release("backup")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("SUB2API_IMAGE=adobe-relay:old", (root / "shared/.env").read_text())
        self.assertNotIn("adobe-relay up ", (root / "commands").read_text())

    def test_failed_health_check_restores_previous_image(self):
        root, result = self.run_release("health")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((root / "shared/.env").read_text(),
                         "SUB2API_IMAGE=adobe-relay:old\nPRESERVE=value\n")
        self.assertEqual((root / "commands").read_text().count("adobe-relay up "), 2)


if __name__ == "__main__":
    unittest.main()
