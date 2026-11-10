// Firefly discovery defines parameter bounds; its public UI adapter defines
// human-readable tiers. Verified against the release recorded in the snapshot.
import snapshot from '@/data/adobe-website-capabilities.json'

export interface AdobeModelCapabilities {
  source: 'adobe' | 'verified-fallback'
  checkedAt: string
  resolutions: string[]
  resolutionMode: 'selectable' | 'default' | 'unknown'
  qualities: Array<{ id: string; label: string }>
  qualityMode: 'selectable' | 'none' | 'unknown'
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}
function branches(schema: unknown, common: unknown, depth = 0): Record<string, unknown>[] {
  if (!isRecord(schema) || depth > 12) return []
  if (typeof schema.$ref === 'string') {
    if (!schema.$ref.startsWith('#/')) return []
    let value = common
    for (const key of schema.$ref.slice(2).split('/')) value = isRecord(value) ? value[key.replace(/~1/g, '/').replace(/~0/g, '~')] : undefined
    if (!isRecord(value)) return []
    const merged = { ...value, ...schema }
    delete merged.$ref
    // A referenced schema may itself be a reference; preserve that chain.
    if (typeof value.$ref === 'string') merged.$ref = value.$ref
    return branches(merged, common, depth + 1)
  }
  return [schema, ...['anyOf', 'oneOf', 'allOf'].flatMap(key => Array.isArray(schema[key])
    ? schema[key].flatMap(value => branches(value, common, depth + 1)) : [])]
}
function property(schema: unknown, key: string, common: unknown): unknown {
  for (const branch of branches(schema, common)) {
    if (isRecord(branch.properties) && key in branch.properties) return branch.properties[key]
  }
}
function enumValues(schema: unknown, common: unknown): unknown[] {
  return branches(schema, common).flatMap(branch => Array.isArray(branch.enum) ? branch.enum : 'const' in branch ? [branch.const] : [])
}
const qualityLevels = [
  { id: 'low', label: 'Low', detail: 1 }, { id: 'medium', label: 'Medium', detail: 3 },
  { id: 'high', label: 'High', detail: 5 }, { id: 'xhigh', label: 'Extra High', detail: 6 }, { id: 'max', label: 'Max', detail: 7 },
]
// Exact named tiers in Firefly's size-schema-functions, not relay billing sizes.
function sizeLabel(width: number, height: number): string | undefined {
  const edge = Math.min(width, height)
  if (edge <= 0) return undefined
  const exact: Record<number, string> = { 360: '360p', 480: '480p', 512: '512', 540: '540p', 720: '720p', 1080: '1080p', 1024: '1K', 2048: '2K', 4096: '4K' }
  return exact[edge] || (edge <= 512 ? '512' : edge <= 1024 ? '1K' : edge <= 2048 ? '2K' : '4K')
}
const continuousGptVersions = new Set(['gpt-image-2.5-flare', 'gpt-image-2.5-prism', 'gpt-image-2.6-star-swirl', 'gpt-image-2.6-aurora'])

export function readAdobeModelCapabilities(schema: unknown, family: string, version: string, common?: unknown, checkedAt = '', source: AdobeModelCapabilities['source'] = 'adobe'): AdobeModelCapabilities {
  const result: AdobeModelCapabilities = { source, checkedAt, resolutions: [], resolutionMode: 'unknown', qualities: [], qualityMode: 'unknown' }
  if (!branches(schema, common).some(b => isRecord(b.properties))) return result
  const settings = property(schema, 'generationSettings', common)
  const detail = property(settings, 'detailLevel', common)
  const quality = property(schema, 'quality', common) ?? property(settings, 'quality', common)
  if (detail !== undefined) {
    const options = branches(detail, common)
    const allowed = enumValues(detail, common)
    result.qualities = qualityLevels.filter(q => options.some(b => typeof b.minimum === 'number' && typeof b.maximum === 'number' && q.detail >= b.minimum && q.detail <= b.maximum) || allowed.includes(q.detail)).map(({ id, label }) => ({ id, label }))
    if (result.qualities.length) result.qualityMode = 'selectable'
  } else if (quality !== undefined) {
    result.qualities = enumValues(quality, common).filter((v): v is string => typeof v === 'string').map(value => {
      const known = qualityLevels.find(q => q.id === value || (q.id === 'xhigh' && value === 'extraHigh'))
      return { id: known?.id || value, label: known?.label || value }
    })
    if (result.qualities.length) result.qualityMode = 'selectable'
  } else if (settings === undefined || branches(settings, common).some(b => isRecord(b.properties))) {
    result.qualityMode = 'none'
  }
  const resolution = property(schema, 'resolution', common)
  result.resolutions = [...new Set(enumValues(resolution, common).filter((v): v is string => typeof v === 'string' && !!v.trim()))]
  if (result.resolutions.length) result.resolutionMode = 'selectable'
  else if (resolution === undefined) {
    const size = property(schema, 'size', common)
    const sizes = enumValues(size, common).filter(isRecord)
    result.resolutions = [...new Set(sizes.flatMap(s => {
      const label = typeof s.width === 'number' && typeof s.height === 'number' ? sizeLabel(s.width, s.height) : undefined
      return label ? [label] : []
    }))]
    if (result.resolutions.length) result.resolutionMode = 'selectable'
    // These are website UI presets, not a guess from version numbers or a
    // pixel-range-to-K conversion. Keep unknown continuous models unconfirmed.
    const width = branches(property(size, 'width', common), common).find(b => typeof b.minimum === 'number' && typeof b.maximum === 'number')
    const height = branches(property(size, 'height', common), common).find(b => typeof b.minimum === 'number' && typeof b.maximum === 'number')
    const knownBounds = width?.minimum === 480 && width.maximum === 3840 && height?.minimum === 480 && height.maximum === 3840
    if (family === 'gpt-image' && knownBounds) {
      if (version === '2') { result.resolutions = ['1K']; result.resolutionMode = 'default' }
      else if (continuousGptVersions.has(version)) { result.resolutions = ['1K', '2K', '4K']; result.resolutionMode = 'selectable' }
    }
  }
  return result
}

export const adobeWebsiteSnapshot = snapshot
