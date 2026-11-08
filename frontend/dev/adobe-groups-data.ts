// Synthetic identities and counts; legacy naming patterns mirror the reviewed list.
// Pricing and usage are synthetic; edits stay in memory and reset on reload.
import type { AdminGroup } from '../src/types'
import { adobeGroupNameProposal } from '../src/utils/adobeGroupName'

const records: Array<[number, string, number, number]> = [
  [101, "Adobe 2 - 低中质量 124K 全参分组 - 示例客户01", 3, 2],
  [102, "Adobe 2 - 低中质量 124K 全参分组（7分） - 示例客户02", 6, 4],
  [103, "Adobe 2 - 示例客户03", 9, 6],
  [104, "Adobe 2 - 示例客户04", 12, 8],
  [105, "Adobe 2 - 示例客户05", 15, 10],
  [106, "Adobe 2 - 示例客户06", 18, 12],
  [107, "Adobe 2.5 - 6分分组", 0, 0],
  [108, "Adobe 2.5 - Flare/Sunburst - 示例客户07", 3, 2],
  [109, "Adobe 2.5 - Flare/Sunburst - 示例客户06", 6, 4],
  [110, "Adobe 2.5 - Flare/Sunburst - 示例客户08", 9, 6],
  [111, "Adobe 2.5 - Flare/Sunburst - 示例客户09", 12, 8],
  [112, "Adobe 2.5 - Flare/Sunburst - 示例客户10", 15, 10],
  [113, "Adobe 2.5 - Flare/Sunburst - 示例客户11", 18, 12],
  [114, "Adobe 2.5 - Flare/Sunburst - 示例客户12", 0, 0],
  [115, "Adobe 2.5 - Flare/Sunburst - 示例客户04", 3, 2],
  [116, "Adobe 2.5 - Flare/Sunburst - 示例客户13", 6, 4],
  [117, "Adobe 2.5 - Flare/Sunburst - 示例客户05", 9, 6],
  [118, "Adobe 2.5 - Flare/Sunburst - 示例客户01", 12, 8],
  [119, "Adobe 2.5 - Flare/Sunburst - 示例客户14", 15, 10],
  [120, "Adobe 2.5 and 2 ----示例客户15", 18, 12],
  [121, "Adobe 2.5 and 2 ----示例客户16", 0, 0],
  [122, "Adobe 2.6 - Flare/Sunburst - 示例客户06", 3, 2],
  [123, "adobe账号转移分组", 6, 4],
  [124, "客户测试专用分组", 9, 6],
  [125, "Adobe 2.6 - 低中质量 124K 全参分组 - 示例客户01", 12, 8],
  [126, "Adobe 2.6 - 低中质量 124K 全参分组（7分） - 示例客户02", 15, 10],
  [127, "Adobe 2.6 - Flare/Sunburst - 示例客户07", 18, 12],
  [128, "Adobe 2.6 - Flare/Sunburst - 示例客户15", 0, 0],
  [129, "Adobe 2.6 - Flare/Sunburst - 示例客户16", 3, 2],
  [130, "Adobe 2.6 - Flare/Sunburst - 示例客户03", 6, 4],
  [131, "Adobe 2.6 - Flare/Sunburst - 示例客户08", 9, 6],
  [132, "Adobe 2.6 - Flare/Sunburst - 示例客户09", 12, 8],
  [133, "Adobe 2.6 - Flare/Sunburst - 示例客户10", 15, 10],
  [134, "Adobe 2.6 - Flare/Sunburst - 示例客户11", 18, 12],
  [135, "Adobe 2.6 - Flare/Sunburst - 示例客户12", 0, 0],
  [136, "Adobe 2.6 - Flare/Sunburst - 示例客户04", 3, 2],
  [137, "Adobe 2.6 - Flare/Sunburst - 示例客户13", 6, 4],
  [138, "Adobe 2.6 - Flare/Sunburst - 示例客户05", 9, 6],
  [139, "Adobe 2.6 - Flare/Sunburst - 示例客户01", 12, 8],
  [140, "Adobe 2.6 - Flare/Sunburst - 示例客户14", 15, 10],
  [141, "Adobe 2.6 - 6分分组", 18, 12],
  [142, "adobe 2.6 示例客户04", 0, 0],
  [143, "adobe 2.6 示例客户05", 3, 2],
  [144, "adobe 2.6-示例客户06", 6, 4]
]

export const demoNameChanges = records.map(([id, name]) => ({ id, before: name, ...adobeGroupNameProposal(name) }))

export function makeDemoGroups(): AdminGroup[] {
  return records.map(([id, name, total, active], index) => ({
    id, name: adobeGroupNameProposal(name).name, platform: 'adobe', description: null, i18n: {},
    status: index >= 24 ? 'inactive' : 'active',
    rate_multiplier: 1, rpm_limit: 0, is_exclusive: index !== 23,
    subscription_type: 'standard', daily_limit_usd: null, weekly_limit_usd: null, monthly_limit_usd: null,
    allow_image_generation: true, allow_batch_image_generation: true,
    image_rate_independent: false, image_rate_multiplier: 1,
    batch_image_discount_multiplier: 0.5, batch_image_hold_multiplier: 0.6,
    image_price_1k: null, image_price_2k: null, image_price_4k: null, image_quality_prices: {},
    video_rate_independent: false, video_rate_multiplier: 1,
    video_price_480p: null, video_price_720p: null, video_price_1080p: null,
    web_search_price_per_call: null, peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1,
    claude_code_only: false, fallback_group_id: null, fallback_group_id_on_invalid_request: null,
    allow_messages_dispatch: false, default_mapped_model: '', require_oauth_only: false, require_privacy_set: false,
    model_routing: null, model_routing_enabled: false, mcp_xml_inject: true,
    supported_model_scopes: [], account_count: total, active_account_count: active, rate_limited_account_count: 0,
    sort_order: index, created_at: '2026-11-08T00:00:00Z', updated_at: '2026-11-08T00:00:00Z',
  }))
}
