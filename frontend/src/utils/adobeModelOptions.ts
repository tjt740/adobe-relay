import { adobeModels } from './adobeGroupName'
import type { GroupModelOption } from '@/api/admin/groups'

export const adobeFallbackModelOptions: GroupModelOption[] = adobeModels.map(m => ({ id: m.id, display_name: m.label }))

// Compatibility IDs only connect known variants to the existing pricing editor.
// Display names always come from Adobe; unknown future models stay selectable.
const gptCompatibilityIds: Record<string, string> = {
  '1.5': 'gpt-image-1.5',
  '2': 'gpt-image-2',
  'gpt-image-2.5-flare': 'gpt-image-2.5-flare',
  'gpt-image-2.5-prism': 'gpt-image-2.5-sunburst',
  'gpt-image-2.6-star-swirl': 'gpt-image-2.6-flare',
  'gpt-image-2.6-aurora': 'gpt-image-2.6-sunburst',
}

function record(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

export function parseAdobeWebsiteModelOptions(value: unknown): GroupModelOption[] {
  if (!Array.isArray(value)) return []
  const names = new Set<string>()
  const options: GroupModelOption[] = []
  for (const family of value) {
    if (!record(family) || typeof family.modelId !== 'string' || !family.modelId.trim() || !record(family.modelVersions)) continue
    for (const [version, model] of Object.entries(family.modelVersions)) {
      if (!record(model) || model.enabled !== true || model.isDeprecated === true ||
        !Array.isArray(model.outputModality) || !model.outputModality.includes('image')) continue
      const name = typeof model.modelDisplayName === 'string' ? model.modelDisplayName.trim() : ''
      if (!name || names.has(name.toLowerCase())) continue
      names.add(name.toLowerCase())
      const id = (family.modelId === 'gpt-image' && Object.prototype.hasOwnProperty.call(gptCompatibilityIds, version) && gptCompatibilityIds[version]) || family.modelId + '@' + version
      options.push({ id, display_name: name })
    }
  }
  // Keep the familiar GPT Image variants first without excluding newer entries.
  const rank = (id: string) => {
    const index = adobeModels.findIndex(model => model.id === id)
    return index < 0 ? adobeModels.length : index
  }
  return options.sort((a, b) => rank(a.id) - rank(b.id) || a.display_name.localeCompare(b.display_name))
}

// An API response may contain a newer model that is not in the website fallback.
// Keep it, prefer its display name, and preserve user-entered options separately.
export function normalizeAdobeModelOptions(value: unknown): GroupModelOption[] {
  if (!Array.isArray(value)) return []
  const seen = new Set<string>()
  return value.flatMap(item => {
    if (!item || typeof item.id !== 'string' || !item.id.trim()) return []
    const id = item.id.trim()
    if (seen.has(id)) return []
    seen.add(id)
    const apiName = typeof item.display_name === 'string' ? item.display_name.trim() : ''
    const fallback = adobeModels.find(m => m.id === id)?.label
    return [{ id, display_name: apiName && apiName !== id ? apiName : fallback || apiName || id }]
  })
}
