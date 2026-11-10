"""Admit only the reviewed model selection and billing mode release, retaining model discovery and naming."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = '900bee9ed507ac75c7a84baf65caa911c163c9c4'
ALLOWED = {'frontend/src/views/admin/GroupsView.vue', 'backend/migrations/258_adobe_quality_pricing_enabled.sql', 'backend/internal/service/api_key_auth_cache_impl.go', '.github/workflows/deploy-adobe-relay.yml', 'backend/internal/domain/group_adobe_model.go', 'frontend/src/utils/__tests__/adobeModelOptions.spec.ts', 'frontend/src/i18n/locales/en/changelog.ts', 'frontend/src/data/adobe-website-capabilities.json', 'backend/internal/service/group.go', 'frontend/src/api/__tests__/admin.groups.model-options.spec.ts', 'frontend/src/utils/__tests__/adobeGroupPricing.spec.ts', 'backend/ent/group/where.go', 'backend/internal/handler/admin/group_handler_quality_pricing_mode_test.go', 'backend/ent/mutation.go', 'frontend/src/utils/__tests__/adobeModelCapabilities.spec.ts', 'backend/internal/service/admin_group.go', 'frontend/src/utils/adobePricingModels.ts', 'frontend/dev/adobe-groups.vite.ts', 'frontend/src/components/admin/group/__tests__/AdobeGroupNameFields.spec.ts', 'backend/internal/handler/dto/types.go', 'frontend/src/i18n/locales/en/admin/overview.ts', 'frontend/src/i18n/locales/zh/changelog.ts', 'backend/internal/domain/group_adobe_model_custom_test.go', 'backend/internal/pkg/adobe/website_models.go', 'frontend/src/components/admin/group/ImageQualityPricing.vue', 'frontend/src/views/admin/__tests__/GroupsView.duplicate.spec.ts', 'backend/ent/runtime/runtime.go', 'backend/ent/migrate/schema.go', 'frontend/src/components/admin/group/__tests__/ImageQualityPricing.spec.ts', 'frontend/src/components/admin/group/AdobePricingModeSwitch.vue', 'backend/internal/pkg/adobe/website_models_test.go', 'frontend/src/utils/adobeGroupPricing.ts', 'backend/ent/group/group.go', 'frontend/dev/adobe-groups.md', 'backend/internal/service/admin_service.go', 'frontend/dev/adobe-groups.ts', 'frontend/src/i18n/locales/zh/admin/overview.ts', 'services/docs-site/guide.md', 'backend/internal/repository/adobe_image_quality_pricing_test.go', 'backend/migrations/257_adobe_multi_model_group_names.sql', 'backend/internal/repository/api_key_repo.go', 'docs/ADOBE_IMAGE_CLIENT_GUIDE.md', 'frontend/src/content/changelog/daily.ts', 'backend/internal/handler/dto/mappers.go', 'backend/ent/group_create.go', 'backend/internal/service/openai_gateway_usage.go', 'backend/ent/schema/group.go', 'backend/internal/service/admin_group_duplicate.go', 'backend/internal/service/api_key_auth_cache.go', 'frontend/src/utils/adobeModelOptions.ts', 'backend/ent/group_update.go', 'backend/internal/handler/admin/group_handler.go', 'frontend/src/utils/adobeGroupName.ts', 'frontend/src/components/common/Select.vue', 'frontend/src/utils/adobeModelCapabilities.ts', 'backend/internal/repository/group_repo.go', 'backend/internal/service/image_quality_pricing_test.go', 'frontend/src/components/admin/group/AdobeGroupNameFields.vue', 'frontend/src/api/admin/groups.ts', 'backend/ent/group.go', 'frontend/src/components/admin/group/__tests__/GroupI18nField.spec.ts', 'frontend/src/types/index.ts'}


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
