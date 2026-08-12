import { defineConfig } from 'astro/config';
import preact from '@astrojs/preact';

// The island and the .ics links talk to the Go API on the same origin in
// production (one Caddy in front of both), so development proxies instead of
// enabling CORS.
export default defineConfig({
  integrations: [preact()],
  vite: {
    server: {
      proxy: {
        '/api': 'http://localhost:8080',
        '/calendario.ics': 'http://localhost:8080',
        '/calendario': 'http://localhost:8080',
      },
    },
  },
});
