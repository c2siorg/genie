import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./tests/setup.js'],
    include: ['tests/unit/**/*.test.js', 'tests/components/**/*.test.ts'],
    exclude: ['node_modules', 'dist'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      include: ['pkg/web/handlers/ui/**/*.js'],
      exclude: ['**/*.test.js', '**/*.test.ts'],
    },
    reporters: ['verbose', 'html'],
  },
});
