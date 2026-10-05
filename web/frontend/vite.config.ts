/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

const srcDir = import.meta.dirname
// Local development proxies the API to the gateway; production is served by
// the gateway itself (same origin), so no URL is baked into the bundle.
const apiTarget = process.env.API_URL || 'http://127.0.0.1:8080'

export default defineConfig(({ mode }) => ({
  base: '/',
  build: {
    outDir: path.resolve(srcDir, 'dist'),
    emptyOutDir: true,
    sourcemap: mode === 'development',
    target: 'es2020',
  },
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': path.resolve(srcDir, 'src') } },
  server: {
    host: process.env.HOST || '127.0.0.1',
    port: 5173,
    proxy: {
      '/api': { target: apiTarget, changeOrigin: true },
      '/healthz': { target: apiTarget, changeOrigin: true },
    },
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    // Several journeys chain dozens of awaited user actions; on a busy machine the 5 s default made two of them flaky.
    testTimeout: 20_000,
  },
}))
