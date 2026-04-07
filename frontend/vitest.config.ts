import { defineConfig } from 'vitest/config';
import vue from '@vitejs/plugin-vue';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  plugins: [vue()],
  define: {
    // Nuxt uses import.meta.client / import.meta.server at build time.
    // Define them for Vitest so composables can detect client-side context.
    'import.meta.client': true,
    'import.meta.server': false,
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    include: ['app/**/*.test.ts', 'app/**/*.spec.ts'],
  },
  resolve: {
    alias: {
      '~': fileURLToPath(new URL('./app', import.meta.url)),
      '@': fileURLToPath(new URL('./app', import.meta.url)),
      // ofetch is a transitive dependency (via nuxt) and not hoisted by pnpm.
      // Alias it so Vite's import-analysis can resolve it for vi.mock().
      ofetch: fileURLToPath(
        new URL(
          './node_modules/.pnpm/ofetch@1.5.1/node_modules/ofetch/dist/index.mjs',
          import.meta.url,
        ),
      ),
    },
  },
});
