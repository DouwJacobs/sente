FROM node:26.12.1-alpine@sha256:40cd011b55415bc894e45b02fdd5a5dfde056921b7d23a64c74f80497af48797 AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
COPY scripts/dependency-notices.py /src/scripts/dependency-notices.py
RUN npm run build
RUN apk add --no-cache python3 && python3 /src/scripts/dependency-notices.py web /licenses/web

FROM node:26.12.1-alpine@sha256:40cd011b55415bc894e45b02fdd5a5dfde056921b7d23a64c74f80497af48797 AS fnb
WORKDIR /opt/sente/fnb
COPY connectors/fnb/owner/package*.json ./
RUN npm ci --omit=dev --ignore-scripts
COPY connectors/fnb/owner/refresh.mjs connectors/fnb/owner/accounts.mjs connectors/fnb/owner/transactions.mjs connectors/fnb/owner/browser-runtime.mjs connectors/fnb/owner/virtual-display.mjs connectors/fnb/owner/runtime-check.mjs connectors/fnb/owner/login-diagnostics.mjs connectors/fnb/owner/navigation-check.mjs connectors/fnb/owner/browser-crash-diagnostics.mjs ./
COPY scripts/dependency-notices.py /src/scripts/dependency-notices.py
RUN apk add --no-cache python3 && python3 /src/scripts/dependency-notices.py web /licenses/fnb

FROM golang:1.27.2-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY scripts/dependency-notices.py ./scripts/dependency-notices.py
RUN apk add --no-cache python3 && python3 scripts/dependency-notices.py go /licenses/go
COPY cmd/ ./cmd/
COPY internal/ ./internal/
ARG SENTE_VERSION=dev
ARG SENTE_COMMIT=""
ARG SENTE_BUILD_TIME=""
COPY scripts/build-metadata.sh ./scripts/build-metadata.sh
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w $(sh scripts/build-metadata.sh)" -o /finance ./cmd/finance

# Native Chromium 155 fixes the reproduced Alpine 152 renderer trap.
# Keep all runtime packages on edge; never mix stable and edge repositories.
FROM alpine:edge@sha256:020dfcbaaf4cc1078bf2d9c7ba31a8466e334061dcd2f248001d68f79e52c000
ARG SENTE_VERSION=dev
ARG SENTE_COMMIT=""
ARG SENTE_BUILD_TIME=""
LABEL org.opencontainers.image.version="$SENTE_VERSION" \
      org.opencontainers.image.revision="$SENTE_COMMIT" \
      org.opencontainers.image.created="$SENTE_BUILD_TIME" \
      org.opencontainers.image.licenses="GPL-3.0-only" \
      org.opencontainers.image.title="Sente" \
      org.opencontainers.image.description="Self-hosted household finance and budgeting" \
      org.opencontainers.image.source="https://github.com/DouwJacobs/sente"
RUN apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata git chromium=155.0.8059.84-r0 xvfb libstdc++ && addgroup -g 10001 finance && adduser -D -u 10001 -G finance finance && mkdir -p /app/data /app/backups && chown -R finance:finance /app && mkdir -p /config/fnb && chown finance:finance /config/fnb && chmod 0700 /config/fnb
WORKDIR /app
COPY --from=fnb /usr/local/bin/node /usr/local/bin/node
COPY --from=fnb /opt/sente/fnb /opt/sente/fnb
COPY --from=fnb /licenses/ /usr/share/licenses/sente/dependencies/
COPY --from=backend /finance /usr/local/bin/finance
COPY --from=web /src/web/dist /app/web/dist
COPY LICENSE NOTICE /usr/share/licenses/sente/
COPY connectors/fnb/upstream/LICENSE /usr/share/licenses/sente/fnb-reference/LICENSE
COPY docker/PLAYWRIGHT-LICENSE /usr/share/licenses/sente/chromium-seccomp/LICENSE
COPY --from=backend /licenses/ /usr/share/licenses/sente/dependencies/
COPY --from=web /licenses/ /usr/share/licenses/sente/dependencies/
USER finance
ENV PORT=8080 DATABASE_PATH=/app/data/finance.sqlite BACKUP_DIR=/app/backups STATIC_DIR=/app/web/dist FNB_KEY_FILE=/config/fnb/fnb.key FNB_BROWSER_MODE=virtual-display FNB_NODE_EXECUTABLE=/usr/local/bin/node FNB_RUNNER_PATH=/opt/sente/fnb/refresh.mjs
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1
ENTRYPOINT ["finance"]
CMD ["serve"]
