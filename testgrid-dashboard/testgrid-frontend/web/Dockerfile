# Multi-stage build with pnpm
FROM node:22-slim AS base
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

WORKDIR /app

# Copy package files first for better caching.
# pnpm-workspace.yaml carries the build-script allowlist (onlyBuiltDependencies).
COPY package.json pnpm-lock.yaml* pnpm-workspace.yaml ./

FROM base AS build
# Install all dependencies (including devDependencies for build).
# pnpm 11 hard-fails `install` when deps have un-run build scripts
# (ERR_PNPM_IGNORED_BUILDS) even when allow-listed in pnpm-workspace.yaml, so we
# approve them explicitly. These scripts are non-essential (core-js funding
# notice + Vaadin telemetry) but approving them makes the install layer succeed.
RUN --mount=type=cache,id=pnpm,target=/pnpm/store \
    pnpm install --frozen-lockfile || pnpm approve-builds --all

# Copy source code
COPY . .

ARG API_URL
ENV API_URL=$API_URL

# Build the application
RUN pnpm run build

# Production stage with nginx
FROM nginx:alpine AS production

WORKDIR /usr/share/nginx/html

# Remove default nginx static files
RUN rm -rf ./*

# Copy built static files from build stage
COPY --from=build /app/dist .

# Self-unregistering service worker: the build no longer emits one, but older
# clients may still have the old Workbox SW registered. Serving this tombstone
# at /sw.js makes their next update check unregister it and clear its caches.
COPY sw.js .

# SPA fallback config: serve index.html for client-side routes so reloads and
# direct links to /dashboard/tab don't 404. Overrides the image's default
# server block at conf.d/default.conf (the stock nginx.conf includes conf.d/*).
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
