import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig(({ mode }) => ({
    base: './',
    plugins: [vue()],
    build: {
        assetsInlineLimit: mode === 'demo' ? 10 * 1024 * 1024 : 4096,
        rollupOptions: {
            output: { inlineDynamicImports: true }
        }
    },
    server: {
        proxy: {
            '/api': 'http://127.0.0.1:8080'
        }
    }
}));
