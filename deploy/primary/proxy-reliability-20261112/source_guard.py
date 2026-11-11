"""Admit reviewed reliability changes plus current SS support; preserve startup and migrations."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = '106e911e98fdf22f8610983a709584440296d585'
# Preserve independently deployed documentation already present on the starting main.
ALLOWED = {'backend/internal/handler/ops_error_logger.go', 'backend/internal/pkg/adobe/client_test.go', 'backend/internal/handler/adobe_final_attempt_attribution_test.go', 'frontend/src/i18n/locales/zh/admin/ops.ts', 'backend/internal/clash/manager.go', 'services/docs-site/verify-live.py', 'backend/internal/clash/subscription.go', 'frontend/src/views/admin/ops/components/OpsAlertRulesCard.vue', 'backend/internal/service/ops_upstream_context.go', 'backend/internal/clash/shadowsocks_test.go', 'frontend/src/content/changelog/daily.ts', 'backend/internal/handler/admin/ops_alerts_handler.go', 'backend/internal/clash/testdata/shadowsocks-base64.txt', 'backend/internal/service/ops_alert_evaluator_service.go', 'frontend/src/i18n/locales/zh/changelog.ts', 'frontend/src/api/admin/ops.ts', 'backend/internal/clash/feed_batch_test.go', 'backend/internal/pkg/adobe/request_diagnostics.go', 'backend/internal/service/ops_mihomo_runtime_test.go', 'backend/internal/pkg/adobe/client.go', 'frontend/src/i18n/locales/en/admin/ops.ts', 'backend/internal/pkg/adobe/diagnostics.go', 'docs/ADOBE_IMAGE_CLIENT_GUIDE.md', 'backend/internal/service/ops_mihomo_alert_test.go', 'frontend/src/i18n/locales/en/changelog.ts', 'backend/internal/service/adobe_image_diagnostics_test.go', 'backend/internal/service/ops_mihomo_runtime.go', 'backend/internal/clash/shadowsocks_mihomo_test.go', 'backend/internal/clash/shadowsocks.go', 'frontend/src/views/admin/ops/components/__tests__/OpsAlertRulesCard.spec.ts', 'backend/internal/pkg/adobe/request_diagnostics_test.go', 'backend/internal/proxyfailover/https_probe_test.go', '.github/workflows/deploy-adobe-relay.yml', 'backend/internal/clash/node_count_test.go', 'backend/internal/proxyfailover/manager_test.go', 'backend/internal/service/ops_service.go', 'backend/internal/service/adobe_image_diagnostics.go', 'services/docs-site/guide.md', 'backend/internal/clash/manager_integration_test.go', 'backend/internal/proxyfailover/probe.go'}


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
    print(json.dumps({'baseline_sha': BASE, 'application_sha': sha, 'reviewed_backend_files': sorted(ALLOWED), 'new_migrations': [], 'startup_unchanged': True}))


if __name__ == '__main__':
    main(sys.argv[1])
