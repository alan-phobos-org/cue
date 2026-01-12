import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import fs from 'fs'
import https from 'https'
import path from 'path'

// Load client certificates for mTLS proxy (if they exist)
function loadCerts() {
  const certDir = path.resolve(__dirname, '../certs')
  const clientCert = path.join(certDir, 'client.crt')
  const clientKey = path.join(certDir, 'client.key')
  const caCert = path.join(certDir, 'ca.crt')

  if (fs.existsSync(clientCert) && fs.existsSync(clientKey)) {
    console.log('[vite] Loading client certificates for mTLS proxy')
    return {
      cert: fs.readFileSync(clientCert),
      key: fs.readFileSync(clientKey),
      ca: fs.existsSync(caCert) ? fs.readFileSync(caCert) : undefined,
    }
  }
  console.log('[vite] No client certificates found, proxy will not use mTLS')
  return null
}

const certs = loadCerts()

// Create HTTPS agent with client certs
const httpsAgent = certs
  ? new https.Agent({
      cert: certs.cert,
      key: certs.key,
      ca: certs.ca,
      rejectUnauthorized: false,
    })
  : new https.Agent({ rejectUnauthorized: false })

// Default port - keep in sync with backend/cmd/cue/main.go DefaultPort
const DEFAULT_PORT = '31337'
const port = process.env.CUE_PORT || DEFAULT_PORT

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: `https://localhost:${port}`,
        changeOrigin: true,
        secure: false,
        agent: httpsAgent,
      }
    }
  },
  build: {
    outDir: '../backend/cmd/cue/dist',
    emptyOutDir: true
  }
})
