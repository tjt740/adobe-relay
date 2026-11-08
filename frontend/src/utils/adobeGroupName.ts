// Display names verified in Firefly's model picker; IDs are already supported.
export const adobeModels = [
  { id: 'gpt-image-2', label: 'GPT Image 2', version: '2' },
  { id: 'gpt-image-2.5-flare', label: 'GPT Image 2.5 Flare', version: '2.5' },
  { id: 'gpt-image-2.5-sunburst', label: 'GPT Image 2.5 Sunburst', version: '2.5' },
  { id: 'gpt-image-2.6-flare', label: 'GPT Image 2.6 Flare', version: '2.6' },
  { id: 'gpt-image-2.6-sunburst', label: 'GPT Image 2.6 Sunburst', version: '2.6' },
] as const
export type AdobeGroupModel = typeof adobeModels[number]['id']
export const adobeGroupModels = ['2', '2.5', '2.6'] as const
export type AdobeGroupVersion = typeof adobeGroupModels[number]

// Retain legacy version filters for existing API callers.
export function adobeGroupNameModels(name: string): AdobeGroupVersion[] {
  const match = name.trim().match(/^adobe\s*(?:gpt\s+image\s+)?(2(?:\.[56])?)(?=$|[\s/&-])(?:\s*(?:and|&|\/)\s*(2(?:\.[56])?)(?=$|[\s-]))?/i)
  return match ? [...new Set([match[1], match[2]].filter(Boolean))] as AdobeGroupVersion[] : []
}

export function adobeGroupNameModelIds(name: string): AdobeGroupModel[] {
  const versions = adobeGroupNameModels(name)
  const result: AdobeGroupModel[] = versions.includes('2') ? ['gpt-image-2'] : []
  // A variant must be next to the version, never inferred from a customer name.
  const variant = name.trim().match(/^adobe\s*(?:gpt\s+image\s+)?(2\.[56])\s*-?\s*(flare(?:\s*\/\s*sunburst)?|sunburst(?:\s*\/\s*flare)?)(?=$|[\s-])/i)
  if (variant) {
    for (const value of variant[2].toLowerCase().split(/\s*\/\s*/)) {
      result.push(('gpt-image-' + variant[1] + '-' + value) as AdobeGroupModel)
    }
  }
  return result
}

export function formatAdobeGroupName(model: string, category: string, customer: string): string {
  const label = adobeModels.find(option => option.id === model)?.label || model
  return ['Adobe ' + label, category.trim(), customer.trim()].filter(Boolean).join(' - ')
}

export function parseAdobeGroupName(name: string): { model: AdobeGroupModel; category: string; customer: string } | null {
  const models = adobeGroupNameModelIds(name)
  if (models.length !== 1 || adobeGroupNameModels(name).length !== 1) return null
  const model = models[0]
  const version = adobeModels.find(option => option.id === model)!.version
  let rest = name.trim().replace(/^adobe\s*(?:gpt\s+image\s+)?2(?:\.[56])?/i, '').trim()
  if (version !== '2') rest = rest.replace(/^-?\s*(flare|sunburst)/i, '').trim()
  rest = rest.replace(/^-+\s*/, '')
  const parts = rest.split(/\s+-\s+|--+/).map(part => part.trim()).filter(Boolean)
  if (!parts.length) return null
  return { model, category: parts.length > 1 || parts[0] === '6分分组' ? parts[0] : '', customer: parts.length > 1 ? parts.slice(1).join(' - ') : parts[0] === '6分分组' ? '' : parts[0] }
}

export function adobeGroupNameProposal(name: string): { name: string; reason: 'clear' | 'ambiguous' | 'utility' | 'unchanged' } {
  const parsed = parseAdobeGroupName(name)
  if (!parsed) return { name, reason: adobeGroupNameModels(name).length ? 'ambiguous' : 'utility' }
  const proposed = formatAdobeGroupName(parsed.model, parsed.category, parsed.customer)
  return { name: proposed, reason: proposed === name ? 'unchanged' : 'clear' }
}
