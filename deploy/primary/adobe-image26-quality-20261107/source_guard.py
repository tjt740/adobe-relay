"""Admit only reviewed Image 2.6 quality and public-name changes against the actual running app."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = 'da24f5634a0fd332f95a14d666fa2e9cea19db53'
ALLOWED = {'backend/internal/pkg/adobe/catalog_test.go', 'backend/internal/service/adobe_model_names_test.go', 'frontend/src/composables/useModelWhitelist.ts', 'frontend/src/i18n/locales/en/changelog.ts', 'backend/internal/service/adobe_image_quality_test.go', 'backend/internal/pkg/adobe/quality26_client_test.go', 'frontend/src/composables/__tests__/useModelWhitelist.spec.ts', 'backend/internal/pkg/adobe/payloads_test.go', 'backend/internal/service/account.go', 'docs/ADOBE_IMAGE_CLIENT_GUIDE.md', 'backend/internal/pkg/adobe/catalog.go', 'backend/internal/service/gateway_service.go', 'backend/internal/handler/admin/account_handler_adobe_models_test.go', 'backend/internal/pkg/adobe/quality26_test.go', 'backend/internal/service/gateway_hotpath_optimization_test.go', 'backend/internal/service/adobe_image_output.go', 'frontend/src/content/changelog/daily.ts', 'backend/internal/domain/constants.go', 'backend/internal/service/group_model_allowlist.go', 'backend/internal/pkg/adobe/payloads.go', 'backend/internal/service/adobe_image_service.go', 'backend/internal/pkg/adobe/models.go', 'frontend/src/i18n/locales/zh/changelog.ts', 'backend/internal/pkg/adobe/client.go'}


def validate(changed, before, candidate, manifest):
    assert set(manifest) == ALLOWED, 'Reviewed file manifest changed'
    assert set(changed) == ALLOWED, 'Unexpected backend, migration or startup changes'
    assert before == {p: v['before'] for p, v in manifest.items()}, 'Actual baseline source changed'
    assert candidate == {p: v['candidate'] for p, v in manifest.items()}, 'Reviewed candidate source changed'


def main(sha):
    assert re.fullmatch('[0-9a-f]{40}', sha), 'Invalid application revision'
    manifest = json.loads(Path(__file__).with_name('backend-change-manifest.json').read_text())
    changed = subprocess.check_output(['git', 'diff', '--name-only', BASE, sha, '--', 'backend', 'frontend', 'Dockerfile', 'deploy/docker-entrypoint.sh', 'docs/legal', 'docs/ADOBE_IMAGE_CLIENT_GUIDE.md', '.github/workflows/deploy-adobe-relay.yml'], text=True).splitlines()
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
