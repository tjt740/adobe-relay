"""Admit only the reviewed default-model change against the actual old app."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = '682fac7564377c8467fe2b4151ed3031de6a9cd5'
ALLOWED = {
    'backend/internal/pkg/adobe/models.go',
    'backend/internal/service/account_test_adobe.go',
    'backend/internal/service/account_test_adobe_test.go',
    'backend/internal/service/adobe_account_health.go',
    'backend/internal/service/adobe_proxy_test_lease_test.go',
    'backend/internal/service/adobe_route_review_test.go',
    'backend/internal/service/okad_connection_check_test.go',
}


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
        return {p: hashlib.sha256(subprocess.check_output(['git', 'show', ref + ':' + p])).hexdigest() for p in ALLOWED}
    validate(changed, hashes(BASE), hashes(sha), manifest)
    print(json.dumps({'baseline_sha': BASE, 'application_sha': sha, 'reviewed_backend_files': sorted(ALLOWED), 'migration_and_startup_unchanged': True}))


if __name__ == '__main__':
    main(sys.argv[1])
