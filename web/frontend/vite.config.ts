import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

const base = process.env.BASE_URL && process.env.BASE_URL !== '/' ? process.env.BASE_URL : undefined
const srcDir = import.meta.dirname

export default defineConfig(({ mode }) => {
  const isDev = mode === 'development'

  return {
    base,
    build: {
      outDir: path.resolve(srcDir, 'dist'),
      emptyOutDir: true,
      sourcemap: isDev,
      minify: !isDev,
      rollupOptions: {
        output: {
          manualChunks: undefined,
        },
      },
    },
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        '@': path.resolve(srcDir, 'src'),
      },
    },
    server: {
      host: process.env.HOST || '0.0.0.0',
      port: 5173,
      strictPort: false,
      proxy: {
        '/api': {
          target: process.env.API_URL || 'http://127.0.0.1:8080',
          changeOrigin: true,
        },
        '/auth': {
          target: process.env.API_URL || 'http://127.0.0.1:8080',
          changeOrigin: true,
        },
        '/health': {
          target: process.env.API_URL || 'http://127.0.0.1:8080',
          changeOrigin: true,
        },
      },
    },
  }
})
