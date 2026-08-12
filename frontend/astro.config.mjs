import { defineConfig } from 'astro/config';
import preact from '@astrojs/preact';
import react from '@astrojs/react';

// The island and the .ics links talk to the Go API on the same origin in
// production (one Caddy in front of both), so development proxies instead of
// enabling CORS.
export default defineConfig({
  // Two JSX renderers share one project, so each is scoped to its own folder:
  // the public calendar stays Preact, the admin panel is React, and neither
  // ever sees the other's .tsx files.
  integrations: [
    preact({ include: ['**/islands/**'] }),
    react({ include: ['**/admin/**'] }),
  ],
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
