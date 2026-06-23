import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  // In production, the Go server serves everything. In dev, proxy API calls.
  server: {
    host: '0.0.0.0',
    port: 10003,
    proxy: {
      '/api': {
        target: 'http://localhost:10002',
        changeOrigin: true,
      },
    },
  },
})
