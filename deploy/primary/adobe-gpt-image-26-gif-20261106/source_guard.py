"""Admit only reviewed 2.6 GIF and complexity changes against the actual running app."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = 'b97759edd35a80cc85dc868acce5e670a1f6efa0'
ALLOWED = {'backend/internal/service/adobe_image_buffer.go', 'docs/ADOBE_IMAGE_CLIENT_GUIDE.md', 'backend/internal/service/adobe_image_output.go', 'backend/internal/service/openai_images.go', 'frontend/src/content/changelog/daily.ts', 'frontend/src/i18n/locales/en/changelog.ts', 'frontend/src/i18n/locales/zh/changelog.ts', 'backend/internal/service/adobe_image_output_test.go', 'backend/internal/pkg/adobe/payloads.go', 'backend/internal/service/openai_images_responses.go', 'backend/internal/pkg/adobe/payloads_test.go', 'backend/internal/service/adobe_image_service.go', 'backend/internal/service/openai_images_test.go', 'backend/internal/service/openai_images_direct.go', 'backend/internal/service/adobe_image_buffer_test.go', 'backend/internal/service/adobe_image_service_test.go'}


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
