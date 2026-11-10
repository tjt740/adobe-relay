import type { ImageQualityPrices } from '@/types'

// The independent editor saves both 2.6 model drafts together. When collapsed,
// omit the field so that an ordinary group edit preserves all stored prices.
export function adobeQualityPricePayload(
  draft: ImageQualityPrices,
  enabled: boolean,
): ImageQualityPrices | undefined {
  return enabled ? JSON.parse(JSON.stringify(draft)) : undefined
}
