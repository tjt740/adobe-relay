import type { ImageQualityPrices } from '@/types'

// Store the price draft independently of the billing switch. Disabling quality
// billing must preserve edited prices so that enabling it later restores them.
export function adobeQualityPricePayload(
  draft: ImageQualityPrices,
  selectedIds?: string[],
): ImageQualityPrices {
  const prices = selectedIds ? Object.fromEntries(Object.entries(draft).filter(([id]) => selectedIds.includes(id))) : draft
  return JSON.parse(JSON.stringify(prices))
}
