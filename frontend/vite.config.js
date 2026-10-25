import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  return {
    plugins: [react()],
    server: {
      proxy: {
        '/api': {
          target: env.API_PROXY_TARGET || 'http://127.0.0.1:8080',
          changeOrigin: true,
          timeout: 240000,
          proxyTimeout: 240000,
        },
        '/g': { target: env.API_PROXY_TARGET || 'http://127.0.0.1:8080', changeOrigin: true },
        '/e': { target: env.API_PROXY_TARGET || 'http://127.0.0.1:8080', changeOrigin: true },
        '/healthz': { target: env.API_PROXY_TARGET || 'http://127.0.0.1:8080', changeOrigin: true },
      },
    },
  };
});
