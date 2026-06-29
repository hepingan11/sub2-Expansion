import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const apiTarget = process.env.VITE_PROXY_TARGET ?? 'http://localhost:8080';
const apiProxy = {
  '/api': {
    target: apiTarget,
    changeOrigin: true
  }
};

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: apiProxy
  },
  preview: {
    port: 5173,
    host: '0.0.0.0',
    proxy: apiProxy
  }
});
