import { describe, expect, it } from 'vitest'
import { adobeGroupNameModels, formatAdobeGroupName } from '../adobeGroupName'
import { makeDemoGroups } from '../../../dev/adobe-groups-data'

describe('Adobe group name classification', () => {
  it.each([
    ['Adobe 2 - Aapeli', ['2']],
    ['Adobe 2.5 - Flare/Sunburst - blue', ['2.5']],
    ['adobe 2.6-Fridayz1024', ['2.6']],
    ['adobe 2.6 刘壮壮', ['2.6']],
    ['Adobe 2.5 and 2 ----南小镇', ['2.5', '2']],
    ['Adobe2.6 - test', ['2.6']],
    ['Adobe 2.60 - test', []],
    ['Adobe 20 - test', []],
    ['Adobe 2.7 - test', []],
    ['客户测试专用分组 Adobe 2', []],
    ['adobe账号转移分组', []],
  ])('classifies %s', (name, models) => expect(adobeGroupNameModels(name as string)).toEqual(models))

  it('classifies the reviewed 44-group fixture without confusing 2 and 2.x', () => {
    const groups = makeDemoGroups()
    expect(groups).toHaveLength(44)
    expect(['2', '2.5', '2.6'].map(model => groups.filter(g => adobeGroupNameModels(g.name).includes(model as '2')).length)).toEqual([8, 15, 21])
  })
  it('formats names without empty separators', () => {
    expect(formatAdobeGroupName('2.6', ' Flare/Sunburst ', ' 杂乱无章 ')).toBe('Adobe 2.6 - Flare/Sunburst - 杂乱无章')
    expect(formatAdobeGroupName('2', '', 'Aapeli')).toBe('Adobe 2 - Aapeli')
    expect(formatAdobeGroupName('2.5', '6分分组', '')).toBe('Adobe 2.5 - 6分分组')
  })
})
