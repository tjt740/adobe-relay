"""Admit only the reviewed default-model change against the actual old app."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = 'f62d70935c5c17ad0bb316d725805b71751bab9a'
ALLOWED = {'backend/internal/repository/account_repo_okad_postgres_test.go', 'backend/internal/service/okad_cookie_recovery.go', 'backend/internal/service/okad_cookie_recovery_test.go', 'backend/internal/service/okad_managed_recovery.go', 'backend/internal/repository/account_repo_okad_attempts_test.go'}


def validate(changed, before, candidate, manifest):
    assert set(manifest) == ALLOWED, 'Reviewed file manifest changed'
    assert set(changed) == ALLOWED, 'Unexpected backend, migration or startup changes'
    assert before == {p: v['before'] for p, v in manifest.items()}, 'Actual baseline source changed'
    assert candidate == {p: v['candidate'] for p, v in manifest.items()}, 'Reviewed candidate source changed'


def main(sha):
    assert re.fullmatch('[0-9a-f]{40}', sha), 'Invalid application revision'
    manifest = json.loads(Path(__file__).with_name('backend-change-manifest.json').read_text())
    changed = subprocess.check_output(['git', 'diff', '--name-only', BASE, sha, '--', 'backend', 'Dockerfile', 'deploy/docker-entrypoint.sh', 'docs/legal', '.github/workflows/deploy-adobe-relay.yml'], text=True).splitlines()
    def hashes(ref):
        out={}
        for p in ALLOWED:
            result=subprocess.run(['git','show',ref+':'+p],capture_output=True)
            out[p]=hashlib.sha256(result.stdout).hexdigest() if result.returncode==0 else None
        return out
    validate(changed, hashes(BASE), hashes(sha), manifest)
    print(json.dumps({'baseline_sha': BASE, 'application_sha': sha, 'reviewed_backend_files': sorted(ALLOWED), 'migration_and_startup_unchanged': True}))


if __name__ == '__main__':
    main(sys.argv[1])
