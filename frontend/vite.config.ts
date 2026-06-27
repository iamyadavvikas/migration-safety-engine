import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/healthz': 'http://localhost:8080',
      '/metrics': 'http://localhost:8080',
      '/plans': 'http://localhost:8080',
      '/drift-scan': 'http://localhost:8080',
      '/migrations': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
  },
})
