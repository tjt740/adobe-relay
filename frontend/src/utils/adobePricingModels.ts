import type { GroupModelOption } from '@/api/admin/groups'
import { adobeFallbackModelOptions } from './adobeModelOptions'
import type { ImageQualityPrices } from '@/types'
import { adobeModels, adobeGroupModelNames, adobeGroupNameModelIds } from './adobeGroupName'

export function adobePricingModelsForName(name: string, options: GroupModelOption[], prices: ImageQualityPrices = {}): GroupModelOption[] {
  const knownIds = adobeGroupNameModelIds(name)
  const names = adobeGroupModelNames(name)
  const ids = names.length > 1 || (names.length === 1 && !/^2(?:\.[56])?(?:\s|$)/i.test(names[0]))
    ? names.map(label => options.find(m => m.display_name === label || m.id === label)?.id || adobeModels.find(m => m.label === label)?.id || label)
    : knownIds
  // Existing prices remain accessible even if their model was removed from the name.
  const all = [...new Set([...ids, ...Object.keys(prices)])]
  if (!all.length) all.push('gpt-image-2.6-flare', 'gpt-image-2.6-sunburst')
  return all.map(id => options.find(m => m.id === id) || adobeFallbackModelOptions.find(m => m.id === id) || { id, display_name: id })
}
