"""Admit only reviewed Image 2.6 quality and public-name changes against the actual running app."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = '683404c7f2e9cc713d2b68f6bd204b7b85d05fe3'
ALLOWED = {'frontend/src/api/admin/groups.ts', 'backend/internal/service/adobe_image_output.go', 'frontend/src/components/admin/group/__tests__/GroupI18nField.spec.ts', 'backend/internal/domain/group_adobe_model_custom_test.go', 'backend/internal/pkg/adobe/payloads.go', 'services/docs-site/guide.md', 'backend/internal/pkg/adobe/website_models.go', 'frontend/src/utils/__tests__/adobeGroupPricing.spec.ts', 'docs/ADOBE_IMAGE_CLIENT_GUIDE.md', 'backend/internal/repository/group_repo_adobe_model_test.go', 'backend/internal/handler/admin/group_handler.go', 'backend/internal/pkg/adobe/website_models_test.go', 'frontend/src/components/admin/group/__tests__/AdobeGroupNameFields.spec.ts', 'frontend/src/utils/__tests__/adobeGroupName.spec.ts', 'frontend/src/utils/adobeGroupName.ts', '.github/workflows/deploy-adobe-relay.yml', 'frontend/src/api/__tests__/admin.groups.model-options.spec.ts', 'backend/internal/repository/group_repo.go', 'frontend/src/content/changelog/daily.ts', 'frontend/dev/adobe-groups.ts', 'frontend/src/components/admin/group/ImageQualityPricing.vue', 'backend/internal/domain/group_adobe_model_test.go', 'backend/internal/service/adobe_image_quality_test.go', 'frontend/src/views/admin/__tests__/GroupsView.duplicate.spec.ts', 'frontend/src/components/admin/group/__tests__/ImageQualityPricing.spec.ts', 'backend/internal/handler/admin/group_website_models.go', 'frontend/dev/adobe-groups.html', 'frontend/dev/adobe-model-capabilities-20261109.json', 'backend/internal/pkg/adobe/payloads_test.go', 'frontend/src/views/admin/GroupsView.vue', 'backend/internal/service/adobe_image_service.go', 'backend/internal/handler/admin/group_handler_adobe_model_test.go', 'backend/internal/server/routes/admin.go', 'backend/internal/service/image_quality_pricing_test.go', 'frontend/src/i18n/locales/zh/admin/overview.ts', 'frontend/dev/adobe-groups.md', 'frontend/src/components/admin/group/GroupI18nField.vue', 'frontend/src/i18n/locales/en/admin/overview.ts', 'frontend/src/components/admin/group/AdobePricingModeSwitch.vue', 'frontend/src/utils/__tests__/adobeModelOptions.spec.ts', 'frontend/dev/adobe-groups-data.ts', 'frontend/src/i18n/locales/en/changelog.ts', 'frontend/src/i18n/locales/zh/changelog.ts', 'frontend/src/utils/adobeGroupPricing.ts', 'frontend/src/utils/adobeModelOptions.ts', 'backend/internal/service/image_quality_pricing.go', 'frontend/package.json', 'backend/internal/domain/group_adobe_model.go', 'frontend/dev/adobe-groups.vite.ts', 'frontend/src/components/admin/group/AdobeGroupNameFields.vue'}


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
