"""Admit only reviewed account status filters and presentation; retain submission and migration guards."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = 'e1c9b83ace0472ad7e97b35d4bf2c4dd5dc1574b'
ALLOWED = {'frontend/src/utils/accountStatus.ts', 'frontend/src/i18n/locales/en/changelog.ts', 'frontend/src/components/account/AccountStatusIndicator.vue', 'frontend/src/utils/__tests__/accountStatusEvidence.spec.ts', 'frontend/src/i18n/locales/zh/changelog.ts', 'frontend/src/utils/adobeCurrentResult.ts', 'frontend/src/components/admin/account/__tests__/AccountTableFilters.spec.ts', 'backend/internal/handler/admin/account_runtime_filters.go', 'frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts', 'frontend/src/content/changelog/daily.ts', 'frontend/src/utils/__tests__/fixtures/account-current-status.json', 'frontend/src/components/admin/account/AccountTableFilters.vue', 'backend/internal/handler/admin/account_runtime_filters_test.go'}


def validate(changed, before, candidate, manifest):
    assert set(manifest) == ALLOWED, 'Reviewed file manifest changed'
    assert set(changed) == ALLOWED, 'Unexpected backend, migration or startup changes'
    assert before == {p: v['before'] for p, v in manifest.items()}, 'Actual baseline source changed'
    assert candidate == {p: v['candidate'] for p, v in manifest.items()}, 'Reviewed candidate source changed'


def main(sha):
    assert re.fullmatch('[0-9a-f]{40}', sha), 'Invalid application revision'
    manifest = json.loads(Path(__file__).with_name('source-manifest.json').read_text())
    changed = subprocess.check_output(['git', 'diff', '--name-only', BASE, sha, '--', 'backend', 'frontend', 'Dockerfile', 'deploy/docker-entrypoint.sh', 'docs/legal', 'docs/ADOBE_IMAGE_CLIENT_GUIDE.md', 'services/docs-site', '.github/workflows/deploy-adobe-relay.yml'], text=True).splitlines()
    def hashes(ref):
        out={}
        for p in ALLOWED:
            result=subprocess.run(['git','show',ref+':'+p],capture_output=True)
            out[p]=hashlib.sha256(result.stdout).hexdigest() if result.returncode==0 else None
        return out
    validate(changed, hashes(BASE), hashes(sha), manifest)
    from retry_guard import validate_retry_guards
    validate_retry_guards(BASE,sha)
    print(json.dumps({'baseline_sha': BASE, 'application_sha': sha, 'reviewed_backend_files': sorted(ALLOWED), 'new_migrations': [], 'startup_unchanged': True}))


if __name__ == '__main__':
    main(sys.argv[1])
