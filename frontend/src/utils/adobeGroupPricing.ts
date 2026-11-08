import { adobeModels, adobeGroupNameModelIds, adobeGroupNameModels } from './adobeGroupName'
import type { GroupModelOption } from '@/api/admin/groups'
import type { ImageQualityPrices } from '@/types'

const qualityModels = adobeModels

export function adobeQualityModel(selection: string, options: GroupModelOption[]): GroupModelOption | undefined {
  const value = selection.trim().toLowerCase()
  const model = qualityModels.find(model => model.id === value ||
    model.label.toLowerCase() === value ||
    options.some(option => option.id === model.id && option.display_name.toLowerCase() === value))
  if (!model) return undefined
  return { id: model.id, display_name: options.find(option => option.id === model.id)?.display_name || model.label }
}

// Old mixed/ambiguous names must not silently choose Flare or Sunburst.
export function adobeGroupQualityModel(name: string, options: GroupModelOption[]): GroupModelOption | undefined {
  const value = name.trim().toLowerCase()
  for (const option of options) {
    const prefix = 'adobe ' + option.display_name.toLowerCase()
    if (value === prefix || value.startsWith(prefix + ' - ')) return adobeQualityModel(option.id, options)
  }
  const models = adobeGroupNameModelIds(name)
  if (models.length !== 1 || adobeGroupNameModels(name).length !== 1) return undefined
  return adobeQualityModel(models[0], options)
}

// Only the currently visible model draft may be saved. Existing prices for
// other models are retained because the server replaces this entire field.
export function adobeQualityPricePayload(
  draft: ImageQualityPrices,
  model: GroupModelOption | undefined,
  enabled: boolean,
  saved: ImageQualityPrices = {},
): ImageQualityPrices | undefined {
  if (!enabled || !model) return undefined
  const prices: ImageQualityPrices = JSON.parse(JSON.stringify(saved))
  delete prices[model.id]
  if (draft[model.id]) prices[model.id] = JSON.parse(JSON.stringify(draft[model.id]))
  return prices
}
