import { afterEach, describe, expect, it, vi } from 'vitest'
import { AccountTestSessionExpired, testAccountConnection } from '../accountTest'
import { ADMIN_UI_REQUEST_HEADER } from '../../adminUIRequest'

const response = (chunks: string[], status = 200) => {
  const encoder = new TextEncoder()
  let i = 0
  return {
    ok: status === 200,
    status,
    body: { getReader: () => ({
      read: async () => i < chunks.length ? { done: false, value: encoder.encode(chunks[i++]) } : { done: true },
      releaseLock: vi.fn()
    }) }
  } as unknown as Response
}
afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers() })

describe('account connection test stream', () => {
  it('uses the existing authenticated test endpoint and waits for explicit success across chunks', async () => {
    const fetchMock = vi.fn().mockResolvedValue(response(['data: {"type":"test_', 'complete","success":true}\r', '\n\r\n']))
    vi.stubGlobal('fetch', fetchMock)
    localStorage.setItem('auth_token', 'fixture-token')
    expect(await testAccountConnection(42, new AbortController().signal)).toEqual({ success: true, statusCode: 200, error: undefined })
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/accounts/42/test', expect.objectContaining({
      method: 'POST',
      headers: expect.objectContaining({ Authorization: 'Bearer fixture-token', [ADMIN_UI_REQUEST_HEADER]: '1' }),
      body: JSON.stringify({ model_id: '', prompt: '', mode: 'default' })
    }))
  })
  it('handles HTTP 200 containing an upstream 401 and an unterminated final line', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response(['data:{"type":"error","error":"Expired cookie","status_code":401}'])))
    expect(await testAccountConnection(1, new AbortController().signal)).toEqual({ success: false, statusCode: 401, error: 'Expired cookie' })
  })
  it('does not treat empty/truncated output as success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response(['data: {"type":"test_start"}\n', 'data: {broken\n'])))
    expect((await testAccountConnection(1, new AbortController().signal)).success).toBe(false)
  })
  it('does not overwrite a terminal failure with a later success event', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response(['data: {"type":"error","error":"failed"}\n', 'data: {"type":"test_complete","success":true}\n'])))
    expect((await testAccountConnection(1, new AbortController().signal)).success).toBe(false)
  })
  it.each([401, 403])('distinguishes admin HTTP %s from an upstream account failure', async status => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response([], status)))
    await expect(testAccountConnection(1, new AbortController().signal)).rejects.toBeInstanceOf(AccountTestSessionExpired)
  })
  it('bounds a hanging test with a timeout', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('fetch', vi.fn((_url, options) => new Promise((_resolve, reject) => {
      options.signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))
    })))
    const result = testAccountConnection(1, new AbortController().signal, 100)
    await vi.advanceTimersByTimeAsync(100)
    expect(await result).toEqual({ success: false, statusCode: 408 })
  })
  it('propagates a user cancellation separately from an upstream timeout', async () => {
    vi.stubGlobal('fetch', vi.fn((_url, options) => new Promise((_resolve, reject) => {
      options.signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))
    })))
    const controller = new AbortController()
    const result = testAccountConnection(1, controller.signal)
    const assertion = expect(result).rejects.toMatchObject({ name: 'AbortError' })
    controller.abort()
    await assertion
  })
})
