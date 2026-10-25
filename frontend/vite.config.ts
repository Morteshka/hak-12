import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// В dev-режиме /api проксируется на локальный бэкенд (go run ./cmd/server).
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: { '/api': { target: process.env.VITE_API_PROXY ?? 'http://localhost:8080', changeOrigin: true } },
  },
})
