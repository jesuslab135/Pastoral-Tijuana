import { defineConfig } from 'vitest/config';
import preact from '@preact/preset-vite';

export default defineConfig({
  plugins: [preact()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    // The parish clock is what the code renders in. Pinning the runner to a
    // different zone keeps a machine set to Tijuana from hiding a bug where
    // the code used local time instead of parish time.
    env: { TZ: 'Europe/Madrid' },
  },
});
