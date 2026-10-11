import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class OkadReleaseTest(unittest.TestCase):
    def release(self, failure=""):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        (root / "data").mkdir()
        (root / "data" / "settings.txt").write_text("preserve")
        (root / ".env").write_text("PRESERVE=yes\n")
        (root / "compose.yml").write_text(
            "services:\n  app:\n    image: adobe-team-cookie:old\n"
            "    ports: [127.0.0.1:9500:9500]\n    env_file: .env\n")
        bins = root / "bin"
        bins.mkdir()
        mock = '''#!/usr/bin/env bash
set -euo pipefail
printf '%s %s\\n' "${0##*/}" "$*" >> "$TEST_ROOT/commands"
case "${0##*/}:$*" in
  docker:compose*'ps -q app') echo app ;;
  docker:exec*'python -') cat >/dev/null; [[ "$FAILURE" != backup ]] ;;
  docker:cp*) touch "${@: -1}" ;;
  docker:compose*'up '*)
    if [[ "$FAILURE" == health ]] && grep -q 'image: adobe-team-cookie:new' "$TEST_ROOT/compose.yml"; then exit 1; fi ;;
esac
'''
        for name in ("docker", "curl", "flock"):
            file = bins / name
            file.write_text(mock)
            file.chmod(0o755)
        script = root / "release.sh"
        script.write_text(Path(__file__).with_name("okad-release.sh").read_text().replace(
            "root=/opt/adobe-team-cookie", 'root="$TEST_ROOT"'))
        result = subprocess.run(["bash", str(script), "adobe-team-cookie:new"],
                                capture_output=True, text=True,
                                env=dict(os.environ, TEST_ROOT=str(root), FAILURE=failure,
                                         PATH=str(bins) + os.pathsep + os.environ["PATH"]))
        return root, result

    def test_success_preserves_configuration_and_backs_up_runtime(self):
        root, result = self.release()
        self.assertEqual(result.returncode, 0, result.stderr)
        config = (root / "compose.yml").read_text()
        self.assertIn("image: adobe-team-cookie:new\n", config)
        self.assertIn("ports: [127.0.0.1:9500:9500]", config)
        self.assertEqual((root / ".env").read_text(), "PRESERVE=yes\n")
        backup, = (root / "backups").iterdir()
        self.assertTrue((backup / "app.db").exists())
        self.assertTrue((backup / "data-config.tar.gz").exists())
        commands = (root / "commands").read_text()
        self.assertIn("docker commit --pause=false app adobe-team-cookie:rollback-", commands)
        self.assertIn("--no-build --no-deps --wait", commands)

    def test_backup_failure_does_not_restart_app(self):
        root, result = self.release("backup")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("image: adobe-team-cookie:old", (root / "compose.yml").read_text())
        self.assertNotIn(" up -d ", (root / "commands").read_text())

    def test_failed_health_restores_runtime_snapshot(self):
        root, result = self.release("health")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("image: adobe-team-cookie:rollback-", (root / "compose.yml").read_text())
        self.assertEqual((root / "commands").read_text().count(" up -d "), 2)


if __name__ == "__main__":
    unittest.main()
