# syntax=docker/dockerfile:1
# Production image of the Next.js frontend (standalone output). Build context:
# src/frontend. BACKEND_URL is a BUILD-TIME setting: Next.js serializes the
# /api/v1 rewrite destination into the build. It is server-only (never in
# browser code); compose.yaml passes the API service address.

FROM node:24.21.0-alpine AS deps
RUN npm install -g pnpm@12.6.0
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN --mount=type=cache,id=pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

FROM deps AS build
COPY . .
ARG BACKEND_URL=http://api:8080
ENV BACKEND_URL=${BACKEND_URL} NEXT_TELEMETRY_DISABLED=1 NEXT_OUTPUT=standalone
RUN pnpm build

# Runtime: only the traced server files and static assets; no pnpm, no
# node_modules beyond what the standalone server needs. Runs as "node".
FROM node:24.21.0-alpine
# The standalone server needs only the node binary: drop the base image's
# package managers from the runtime.
RUN rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack \
      /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack \
      /opt/yarn-* /usr/local/bin/yarn /usr/local/bin/yarnpkg
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
WORKDIR /app
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
USER node
EXPOSE 3000
CMD ["node", "server.js"]
