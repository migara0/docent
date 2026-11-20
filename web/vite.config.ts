import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { resolve } from 'node:path';
import { thirdPartyLicenses } from './licenses';

// The built app goes straight into the Go package that embeds it (internal/ui/dist), so `go build` always ships the current UI.
export default defineConfig({
  plugins: [svelte(), thirdPartyLicenses(resolve(__dirname, '../licenses/THIRD_PARTY_WEB.md'))],
  build: {
    outDir: resolve(__dirname, '../internal/ui/dist'),
    emptyOutDir: true,
    target: 'es2023',
    rollupOptions: { input: { main: resolve(__dirname, 'index.html'), diagram: resolve(__dirname, 'diagram.html') } },
  },
  server: {
    port: 5173,
    fs: { allow: ['..'] }, // the design tokens and fonts live in ../design
    proxy: { '/_api': 'http://127.0.0.1:8787' },
  },
  test: { environment: 'node', include: ['src/**/*.test.ts'] },
});
