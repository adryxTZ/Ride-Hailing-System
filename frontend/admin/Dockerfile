# Admin dashboard image. Build context is the frontend/ workspace alone (ADR 0007):
#   docker build -f admin/Dockerfile --build-arg NEXT_PUBLIC_API_URL=http://localhost:8080/v1 -t nexa-admin ./frontend
FROM node:22-alpine AS build
RUN corepack enable
WORKDIR /w
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY admin/package.json admin/
COPY packages ./packages
RUN pnpm install --frozen-lockfile
COPY admin ./admin
# NEXT_PUBLIC_* values are baked in at build time: the API the browser talks to.
ARG NEXT_PUBLIC_API_URL=http://localhost:8080/v1
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL NEXT_TELEMETRY_DISABLED=1
RUN pnpm --filter admin build
# public/ is optional in Next.js; make sure it exists so the runtime COPY below never fails.
RUN mkdir -p admin/public

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production PORT=3000 HOSTNAME=0.0.0.0
COPY --from=build /w/admin/.next/standalone ./
COPY --from=build /w/admin/.next/static ./admin/.next/static
COPY --from=build /w/admin/public ./admin/public
EXPOSE 3000
CMD ["node", "admin/server.js"]
