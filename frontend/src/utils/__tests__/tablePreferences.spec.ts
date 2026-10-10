import { afterEach, describe, expect, it } from 'vitest'

import {
  DEFAULT_TABLE_PAGE_SIZE,
  DEFAULT_TABLE_PAGE_SIZE_OPTIONS,
  getConfiguredTableDefaultPageSize,
  getConfiguredTablePageSizeOptions,
  normalizeTablePageSize
} from '@/utils/tablePreferences'

describe('tablePreferences', () => {
  afterEach(() => {
    delete window.__APP_CONFIG__
  })

  it('returns built-in defaults when app config is missing', () => {
    expect(getConfiguredTableDefaultPageSize()).toBe(DEFAULT_TABLE_PAGE_SIZE)
    expect(getConfiguredTablePageSizeOptions()).toEqual(DEFAULT_TABLE_PAGE_SIZE_OPTIONS)
  })

  it('uses configured defaults when app config is valid', () => {
    window.__APP_CONFIG__ = {
      table_default_page_size: 300,
      table_page_size_options: [200, 300, 500]
    } as any

    expect(getConfiguredTableDefaultPageSize()).toBe(300)
    expect(getConfiguredTablePageSizeOptions()).toEqual([200, 300, 500])
  })

  it('normalizes a default page size outside the configured options', () => {
    window.__APP_CONFIG__ = {
      table_default_page_size: 500,
      table_page_size_options: [200, 300]
    } as any

    expect(getConfiguredTableDefaultPageSize()).toBe(500)
    expect(getConfiguredTablePageSizeOptions()).toEqual([200, 300])
    expect(normalizeTablePageSize(500)).toBe(300)
    expect(normalizeTablePageSize(250)).toBe(300)
  })

  it('normalizes invalid options and falls back to the built-in range', () => {
    window.__APP_CONFIG__ = {
      table_default_page_size: 100,
      table_page_size_options: [1001, 50, 10, 10, 2, 0]
    } as any

    expect(getConfiguredTableDefaultPageSize()).toBe(DEFAULT_TABLE_PAGE_SIZE)
    expect(getConfiguredTablePageSizeOptions()).toEqual(DEFAULT_TABLE_PAGE_SIZE_OPTIONS)
    expect(normalizeTablePageSize(undefined)).toBe(DEFAULT_TABLE_PAGE_SIZE)
  })

  it('normalizes page size against configured options by rounding up', () => {
    window.__APP_CONFIG__ = {
      table_default_page_size: 200,
      table_page_size_options: [200, 300, 500]
    } as any

    expect(normalizeTablePageSize(200)).toBe(200)
    expect(normalizeTablePageSize(250)).toBe(300)
    expect(normalizeTablePageSize(400)).toBe(500)
    expect(normalizeTablePageSize(1500)).toBe(500)
    expect(normalizeTablePageSize(undefined)).toBe(200)
  })

  it('keeps built-in selectable defaults at 200, 300, 400, 500', () => {
    window.__APP_CONFIG__ = {
      table_default_page_size: 100
    } as any

    expect(getConfiguredTablePageSizeOptions()).toEqual([200, 300, 400, 500])
  })
})
