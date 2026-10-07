FROM node:22.21.1-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27.1-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
ARG SENTE_VERSION=dev
ARG SENTE_COMMIT=""
ARG SENTE_BUILD_TIME=""
COPY scripts/build-metadata.sh ./scripts/build-metadata.sh
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w $(sh scripts/build-metadata.sh)" -o /finance ./cmd/finance

FROM alpine:3.23
LABEL org.opencontainers.image.title="Sente" \
      org.opencontainers.image.description="Self-hosted household finance and budgeting" \
      org.opencontainers.image.source="https://github.com/DouwJacobs/sente"
RUN apk add --no-cache ca-certificates tzdata && addgroup -g 10001 finance && adduser -D -u 10001 -G finance finance && mkdir -p /app/data /app/backups && chown -R finance:finance /app
WORKDIR /app
COPY --from=backend /finance /usr/local/bin/finance
COPY --from=web /src/web/dist /app/web/dist
USER finance
ENV PORT=8080 DATABASE_PATH=/app/data/finance.sqlite BACKUP_DIR=/app/backups STATIC_DIR=/app/web/dist
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1
ENTRYPOINT ["finance"]
CMD ["serve"]
