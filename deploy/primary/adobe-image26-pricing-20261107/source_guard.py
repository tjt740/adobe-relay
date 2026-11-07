"""Admit only reviewed Image 2.6 quality and public-name changes against the actual running app."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

BASE = '2dc7a2a3ae0761d88b2d7e89964c93b2dc86ff06'
ALLOWED = {'services/docs-site/guide.md', 'backend/ent/group_update.go', 'backend/ent/usagelog/where.go', 'backend/ent/group/group.go', 'frontend/src/views/admin/GroupsView.vue', 'frontend/src/i18n/locales/en/dashboard.ts', 'backend/ent/group/where.go', 'backend/internal/service/api_key_auth_cache_impl.go', 'backend/ent/usagelog_create.go', 'backend/ent/usagelog_update.go', 'backend/internal/handler/dto/types.go', 'backend/internal/repository/usage_log_repo_insert.go', 'backend/internal/service/image_quality_pricing.go', 'backend/internal/repository/group_repo.go', 'backend/ent/schema/group.go', 'backend/internal/repository/adobe_image_quality_pricing_test.go', 'backend/internal/service/admin_service.go', 'frontend/src/i18n/locales/zh/admin/overview.ts', 'services/docs-site/README.md', 'backend/ent/usagelog/usagelog.go', 'services/docs-site/verify-live.py', 'backend/ent/group.go', 'backend/internal/service/image_quality_pricing_test.go', 'backend/internal/service/api_key_auth_cache.go', 'backend/ent/mutation.go', 'frontend/src/i18n/locales/zh/dashboard.ts', 'backend/internal/handler/admin/group_handler.go', 'backend/internal/repository/usage_log_session_id_unit_test.go', 'frontend/src/components/admin/usage/UsageTable.vue', 'frontend/src/content/changelog/daily.ts', 'backend/internal/handler/dto/mappers.go', 'docs/ADOBE_IMAGE_CLIENT_GUIDE.md', 'backend/ent/migrate/schema.go', 'backend/migrations/256_adobe_image_quality_pricing.sql', 'backend/internal/repository/usage_log_repo_request_type_test.go', 'frontend/src/utils/imageUsage.ts', 'frontend/src/i18n/locales/en/admin/overview.ts', 'backend/ent/runtime/runtime.go', 'backend/internal/service/openai_gateway_usage.go', 'backend/internal/service/group.go', 'frontend/src/components/admin/group/ImageQualityPricing.vue', 'backend/internal/repository/usage_log_repo_insert_shape_unit_test.go', 'backend/internal/service/usage_log.go', 'backend/internal/repository/api_key_repo.go', 'backend/internal/repository/usage_log_repo_query.go', 'backend/internal/service/admin_group_duplicate.go', 'backend/ent/usagelog.go', 'frontend/src/i18n/locales/en/changelog.ts', 'frontend/src/types/index.ts', 'frontend/src/components/admin/group/__tests__/ImageQualityPricing.spec.ts', 'backend/internal/service/adobe_image_service.go', 'backend/ent/group_create.go', 'frontend/src/i18n/locales/zh/changelog.ts', 'backend/internal/service/admin_group.go', 'backend/internal/service/openai_gateway_service.go', 'backend/ent/schema/usage_log.go'}


def validate(changed, before, candidate, manifest):
    assert set(manifest) == ALLOWED, 'Reviewed file manifest changed'
    assert set(changed) == ALLOWED, 'Unexpected backend, migration or startup changes'
    assert before == {p: v['before'] for p, v in manifest.items()}, 'Actual baseline source changed'
    assert candidate == {p: v['candidate'] for p, v in manifest.items()}, 'Reviewed candidate source changed'


def main(sha):
    assert re.fullmatch('[0-9a-f]{40}', sha), 'Invalid application revision'
    manifest = json.loads(Path(__file__).with_name('backend-change-manifest.json').read_text())
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
    print(json.dumps({'baseline_sha': BASE, 'application_sha': sha, 'reviewed_backend_files': sorted(ALLOWED), 'additive_migration': '256_adobe_image_quality_pricing.sql', 'startup_unchanged': True}))


if __name__ == '__main__':
    main(sys.argv[1])
