"""Admit reviewed subscription metadata, incremental imports and manual priority; retain submission and migration guards."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = 'bc0bea5250ca8f24ce01fdf04e370e4e2fb8924d'
ALLOWED = {'frontend/dev/clash-subscriptions.ts', 'backend/internal/clash/subscription_edit_test.go', 'backend/internal/proxyfailover/manager.go', 'frontend/src/i18n/locales/en/admin/resources.ts', 'frontend/src/__tests__/integration/clash-subscription.spec.ts', 'backend/internal/clash/subscription_edit.go', 'backend/internal/handler/admin/proxy_clash.go', 'backend/internal/proxyfailover/allocation.go', 'frontend/src/i18n/locales/zh/admin/resources.ts', 'backend/internal/proxyfailover/subscription_priority_test.go', 'frontend/src/components/admin/proxy/SubscriptionSourceEditor.vue', 'backend/internal/clash/source_health.go', 'frontend/src/content/changelog/daily.ts', 'frontend/src/i18n/locales/en/changelog.ts', 'frontend/src/i18n/locales/zh/changelog.ts', 'frontend/src/components/admin/proxy/ClashSubscriptionModal.vue', 'frontend/src/api/admin/clash.ts', 'backend/internal/server/routes/admin.go', 'backend/internal/clash/manager_integration_test.go', 'backend/internal/clash/manager.go'}


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
