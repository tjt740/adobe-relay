import { buildApiUrl } from '../url'
import { ADMIN_UI_REQUEST_HEADER } from '../adminUIRequest'

export interface AccountConnectionTestResult {
  success: boolean
  statusCode?: number
  error?: string
}

export class AccountTestSessionExpired extends Error {}

// The single-account connection test is an SSE POST. HTTP 200 alone is not a
// successful test: only its explicit terminal test_complete event counts.
export async function testAccountConnection(
  accountID: number,
  signal: AbortSignal,
  timeoutMs = 180_000
): Promise<AccountConnectionTestResult> {
  const controller = new AbortController()
  const abort = () => controller.abort()
  signal.addEventListener('abort', abort, { once: true })
  if (signal.aborted) abort()
  let timedOut = false
  const timer = setTimeout(() => { timedOut = true; controller.abort() }, timeoutMs)
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined
  try {
    const response = await fetch(buildApiUrl(`/admin/accounts/${accountID}/test`), {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${localStorage.getItem('auth_token')}`,
        'Content-Type': 'application/json',
        [ADMIN_UI_REQUEST_HEADER]: '1'
      },
      body: JSON.stringify({ model_id: '', prompt: '', mode: 'default' }),
      signal: controller.signal
    })
    // These are admin-session errors, not an upstream account's SSE result.
    if (response.status === 401 || response.status === 403) throw new AccountTestSessionExpired()
    if (!response.ok) return { success: false, error: `HTTP ${response.status}`, statusCode: response.status }
    reader = response.body?.getReader()
    if (!reader) return { success: false }
    const decoder = new TextDecoder()
    let buffer = ''
    let result: AccountConnectionTestResult | undefined
    const consume = (line: string) => {
      if (!line.startsWith('data:')) return
      try {
        const event = JSON.parse(line.slice(5).trim())
        if (event.type !== 'test_complete' && event.type !== 'error') return
        // Once an error has been observed, a later completion cannot erase it.
        if (result?.success === false) return
        const success = event.type === 'test_complete' && event.success === true
        const code = Number(event.status_code)
        result = {
          success,
          statusCode: success ? 200 : Number.isInteger(code) && code >= 400 && code <= 599 ? code : undefined,
          error: typeof event.error === 'string' ? event.error : undefined
        }
      } catch {
        // Ignore non-JSON keepalive/content; a missing completion fails closed.
      }
    }
    while (true) {
      const chunk = await reader.read()
      buffer += decoder.decode(chunk.value, { stream: !chunk.done })
      const lines = buffer.split(/\r?\n/)
      buffer = lines.pop() ?? ''
      lines.forEach(consume)
      if (chunk.done) { if (buffer) consume(buffer); break }
    }
    return result ?? { success: false }
  } catch (error) {
    if (signal.aborted) throw new DOMException('Aborted', 'AbortError')
    if (timedOut) return { success: false, statusCode: 408 }
    throw error
  } finally {
    clearTimeout(timer)
    signal.removeEventListener('abort', abort)
    reader?.releaseLock()
  }
}
