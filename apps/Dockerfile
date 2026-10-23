# Mobile dev bundlers in a container. Build context is the apps/ workspace alone (ADR 0007).
# Serves the rider and driver apps to Expo Go on phones (ports 8081, 8082) and as web
# builds in a browser (8083, 8084). Phones need the host's LAN address:
#   docker build -t nexa-apps ./apps && docker run -e HOST_IP=192.168.1.20 -p 8081-8084:8081-8084 nexa-apps
FROM node:22-alpine
RUN corepack enable && apk add --no-cache bash git
WORKDIR /w
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY rider/package.json rider/
COPY driver/package.json driver/
COPY packages ./packages
RUN pnpm install --frozen-lockfile
COPY . .
ENV CI=1 EXPO_NO_TELEMETRY=1 API_BASE_URL=http://localhost:8080/v1 BRAND=nexa
EXPOSE 8081 8082 8083 8084
CMD ["bash", "tools/docker-start.sh"]
