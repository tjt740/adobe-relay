"""Admit only the reviewed default-model change against the actual old app."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = 'df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c'
ALLOWED = {'backend/internal/service/adobe_image_input.go', 'backend/internal/pkg/adobe/errors.go', 'backend/internal/pkg/adobe/public_errors.go', 'backend/internal/pkg/adobe/transport.go', 'backend/internal/handler/gemini_v1beta_handler.go', 'backend/internal/service/adobe_image_diagnostics.go', 'backend/internal/pkg/adobe/client.go', 'backend/internal/service/gateway_service.go', 'backend/internal/pkg/adobe/public_errors_test.go', 'backend/internal/handler/adobe_images_error_details_test.go', 'frontend/src/i18n/locales/zh/changelog.ts', 'backend/internal/service/adobe_image_output.go', 'backend/internal/service/adobe_error_classifier.go', 'frontend/src/content/changelog/daily.ts', 'backend/internal/pkg/adobe/diagnostics.go', 'backend/internal/handler/adobe_images.go', 'backend/internal/pkg/adobe/payloads.go', 'backend/internal/pkg/adobe/catalog.go', 'frontend/src/i18n/locales/en/changelog.ts', 'frontend/src/i18n/__tests__/dailyChangelog.spec.ts', 'backend/internal/service/adobe_public_errors_test.go', 'backend/internal/service/adobe_image_service.go'}


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
