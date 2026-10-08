FROM node:22.21.1-alpine@sha256:0340fa682d72068edf603c305bfbc10e23219fb0e40df58d9ea4d6f33a9798bf AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
COPY scripts/dependency-notices.py /src/scripts/dependency-notices.py
RUN npm run build
RUN apk add --no-cache python3 && python3 /src/scripts/dependency-notices.py web /licenses/web

FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS backend
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

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
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
RUN apk add --no-cache ca-certificates tzdata git && addgroup -g 10001 finance && adduser -D -u 10001 -G finance finance && mkdir -p /app/data /app/backups && chown -R finance:finance /app
WORKDIR /app
COPY --from=backend /finance /usr/local/bin/finance
COPY --from=web /src/web/dist /app/web/dist
COPY LICENSE NOTICE /usr/share/licenses/sente/
COPY --from=backend /licenses/ /usr/share/licenses/sente/dependencies/
COPY --from=web /licenses/ /usr/share/licenses/sente/dependencies/
USER finance
ENV PORT=8080 DATABASE_PATH=/app/data/finance.sqlite BACKUP_DIR=/app/backups STATIC_DIR=/app/web/dist
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1
ENTRYPOINT ["finance"]
CMD ["serve"]
