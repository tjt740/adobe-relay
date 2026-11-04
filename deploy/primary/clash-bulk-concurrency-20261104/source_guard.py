"""Admit only reviewed subscription capacity and bulk concurrency changes against the actual old app."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = '66d9734be3d67baa4fa47cb2df505f0bb7f31193'
ALLOWED = {'backend/internal/clash/node_count_test.go', 'backend/internal/clash/manager.go', 'backend/internal/clash/source_health.go', 'frontend/src/i18n/locales/zh/admin/accounts.ts', 'backend/internal/clash/feed_batch.go', 'backend/internal/clash/subscription.go', 'frontend/src/content/changelog/daily.ts', 'frontend/src/i18n/__tests__/dailyChangelog.spec.ts', 'frontend/src/i18n/locales/en/changelog.ts', 'frontend/src/i18n/locales/zh/changelog.ts', 'frontend/src/components/admin/account/__tests__/AccountBulkTestModal.spec.ts', 'frontend/src/components/admin/account/AccountBulkTestModal.vue', 'frontend/src/i18n/locales/en/admin/accounts.ts'}


def validate(changed, before, candidate, manifest):
    assert set(manifest) == ALLOWED, 'Reviewed file manifest changed'
    assert set(changed) == ALLOWED, 'Unexpected backend, migration or startup changes'
    assert before == {p: v['before'] for p, v in manifest.items()}, 'Actual baseline source changed'
    assert candidate == {p: v['candidate'] for p, v in manifest.items()}, 'Reviewed candidate source changed'


def main(sha):
    assert re.fullmatch('[0-9a-f]{40}', sha), 'Invalid application revision'
    manifest = json.loads(Path(__file__).with_name('backend-change-manifest.json').read_text())
    changed = subprocess.check_output(['git', 'diff', '--name-only', BASE, sha, '--', 'backend', 'frontend', 'Dockerfile', 'deploy/docker-entrypoint.sh', 'docs/legal', '.github/workflows/deploy-adobe-relay.yml'], text=True).splitlines()
    def hashes(ref):
        out={}
        for p in ALLOWED:
            result=subprocess.run(['git','show',ref+':'+p],capture_output=True)
            out[p]=hashlib.sha256(result.stdout).hexdigest() if result.returncode==0 else None
        return out
    validate(changed, hashes(BASE), hashes(sha), manifest)
    from retry_guard import validate_retry_guards
    validate_retry_guards(BASE,sha)
    print(json.dumps({'baseline_sha': BASE, 'application_sha': sha, 'reviewed_backend_files': sorted(ALLOWED), 'migration_and_startup_unchanged': True}))


if __name__ == '__main__':
    main(sys.argv[1])
