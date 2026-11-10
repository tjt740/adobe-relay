"""Admit only the reviewed model selection and billing mode release, retaining model discovery and naming."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = '900bee9ed507ac75c7a84baf65caa911c163c9c4'
ALLOWED = {'frontend/dev/adobe-groups.vite.ts', 'backend/internal/pkg/adobe/website_models.go', 'services/docs-site/guide.md', 'backend/ent/group/group.go', 'backend/internal/repository/api_key_repo.go', 'backend/internal/service/api_key_auth_cache_impl.go', 'frontend/src/components/admin/group/ImageQualityPricing.vue', 'frontend/src/components/admin/group/AdobeGroupNameFields.vue', 'frontend/src/components/admin/group/AdobePricingModeSwitch.vue', 'backend/internal/service/admin_group.go', 'backend/internal/handler/dto/mappers.go', 'frontend/src/i18n/locales/zh/changelog.ts', 'frontend/src/utils/__tests__/adobeGroupPricing.spec.ts', 'backend/ent/group_update.go', 'docs/ADOBE_IMAGE_CLIENT_GUIDE.md', 'frontend/src/i18n/locales/zh/admin/overview.ts', 'backend/ent/group_create.go', 'frontend/src/api/admin/groups.ts', 'frontend/src/utils/__tests__/adobeModelCapabilities.spec.ts', 'backend/internal/repository/adobe_image_quality_pricing_test.go', 'backend/internal/pkg/adobe/website_models_test.go', 'backend/internal/service/admin_group_duplicate.go', 'frontend/src/views/admin/__tests__/GroupsView.duplicate.spec.ts', 'frontend/src/components/common/Select.vue', 'backend/ent/migrate/schema.go', 'frontend/src/utils/adobeModelCapabilities.ts', 'backend/migrations/258_adobe_quality_pricing_enabled.sql', 'backend/ent/schema/group.go', 'backend/internal/service/admin_service.go', 'frontend/src/i18n/locales/en/admin/overview.ts', 'frontend/src/content/changelog/daily.ts', 'backend/internal/domain/group_adobe_model_custom_test.go', 'backend/internal/service/group.go', 'backend/internal/repository/group_repo.go', 'backend/migrations/257_adobe_multi_model_group_names.sql', 'backend/internal/domain/group_adobe_model.go', 'backend/ent/group.go', 'frontend/src/components/admin/group/__tests__/ImageQualityPricing.spec.ts', 'backend/internal/handler/admin/group_handler_quality_pricing_mode_test.go', 'backend/internal/service/api_key_auth_cache.go', 'frontend/src/i18n/locales/en/changelog.ts', 'backend/internal/handler/admin/group_handler.go', 'frontend/src/types/index.ts', 'backend/internal/service/image_quality_pricing_test.go', 'backend/internal/service/openai_gateway_usage.go', 'frontend/src/utils/adobeModelOptions.ts', 'frontend/dev/adobe-groups.ts', 'frontend/dev/adobe-groups.md', 'frontend/src/utils/adobeGroupName.ts', 'backend/ent/group/where.go', 'frontend/src/views/admin/GroupsView.vue', 'frontend/src/utils/__tests__/adobeModelOptions.spec.ts', 'backend/ent/runtime/runtime.go', 'frontend/src/data/adobe-website-capabilities.json', 'backend/internal/handler/dto/types.go', 'frontend/src/utils/adobeGroupPricing.ts', 'backend/ent/mutation.go', 'frontend/src/components/admin/group/__tests__/AdobeGroupNameFields.spec.ts', 'frontend/src/utils/adobePricingModels.ts', '.github/workflows/deploy-adobe-relay.yml'}


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
    print(json.dumps({'baseline_sha': BASE, 'application_sha': sha, 'reviewed_backend_files': sorted(ALLOWED), 'new_migrations': ['257_adobe_multi_model_group_names.sql','258_adobe_quality_pricing_enabled.sql'], 'startup_unchanged': True}))


if __name__ == '__main__':
    main(sys.argv[1])
