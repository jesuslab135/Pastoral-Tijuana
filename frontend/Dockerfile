FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

# The Caddyfile is bind-mounted from /opt/pastoral at runtime, so routing
# changes are a file edit + container restart, never an image rebuild.
FROM caddy:2-alpine
COPY --from=build /app/dist /srv
