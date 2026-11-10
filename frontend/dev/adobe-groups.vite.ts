import { defineConfig, mergeConfig, type Plugin } from 'vite'
import { readFile } from 'node:fs/promises'
import baseConfig from './account-recovery-demo.vite'

// Preview group data stays in memory. Only this fixed public Adobe directory
// is fetched remotely; no Sub2 credentials, account data or writes are proxied.
const sourceURL = 'https://gateway.ff.adobe.io/v2/models/discovery?resolveSchema=true&includeFirstParty=true'
const adobeDirectory: Plugin = {
  name: 'adobe-public-directory-preview',
  configureServer(server) {
    // An explicit private local snapshot is the only source of production group
    // data. No request from this preview is forwarded to Sub2.
    server.middlewares.use('/__preview/group-snapshot', async (req, res) => {
      res.setHeader('Content-Type', 'application/json')
      res.setHeader('Cache-Control', 'no-store')
      if (req.method !== 'GET') { res.statusCode = 405; res.end('{}'); return }
      const snapshotPath = process.env.ADOBE_PREVIEW_SNAPSHOT
      if (!snapshotPath) { res.statusCode = 404; res.end('{}'); return }
      try {
        const snapshot = JSON.parse(await readFile(snapshotPath, 'utf8'))
        if (snapshot.source !== 'production_snapshot' || !Array.isArray(snapshot.groups) || !snapshot.groups.length) throw new Error('Invalid snapshot')
        res.end(JSON.stringify(snapshot))
      } catch {
        res.statusCode = 500
        res.end(JSON.stringify({ error: 'Local group snapshot could not be loaded' }))
      }
    })
    server.middlewares.use('/__preview/adobe-models', async (req, res) => {
      res.setHeader('Content-Type', 'application/json')
      res.setHeader('Cache-Control', 'no-store')
      if (req.method !== 'GET') {
        res.statusCode = 405
        res.end('{}')
        return
      }
      try {
        const response = await fetch(sourceURL, {
          signal: AbortSignal.timeout(8000), redirect: 'error',
          headers: {
            Accept: 'application/json', 'User-Agent': 'Mozilla/5.0',
            'x-api-key': 'clio-playground-web',
            Origin: 'https://firefly.adobe.com', Referer: 'https://firefly.adobe.com/',
          },
        })
        if (!response.ok || !response.body) throw new Error('Adobe discovery unavailable')
        const reader = response.body.getReader()
        const chunks: Uint8Array[] = []
        let size = 0
        while (true) {
          const { value, done } = await reader.read()
          if (done) break
          size += value.byteLength
          if (size > 4 * 1024 * 1024) {
            await reader.cancel()
            throw new Error('Adobe discovery response is too large')
          }
          chunks.push(value)
        }
        const data = JSON.parse(Buffer.concat(chunks).toString('utf8'))
        if (!Array.isArray(data.models) || !data.models.length) throw new Error('Empty Adobe discovery')
        res.end(JSON.stringify({ source: 'adobe', source_url: sourceURL, fetched_at: new Date().toISOString(), models: data.models, commonRequestSchema: data.commonRequestSchema }))
      } catch {
        res.statusCode = 503
        res.end(JSON.stringify({ error: 'Adobe model list is temporarily unavailable' }))
      }
    })
  },
}

export default mergeConfig(baseConfig, defineConfig({ plugins: [adobeDirectory] }))
