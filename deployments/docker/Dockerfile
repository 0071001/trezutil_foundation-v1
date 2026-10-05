# Trezutil platform image: every service binary, the migration runner, the
# SQL migrations and the built web SPA. Each container selects its process
# with the command, e.g. `command: ["/app/bin/search-service"]`.
#
# Build from the repository root:
#   docker build -f deployments/docker/Dockerfile -t trezutil:latest .

# ---- web SPA ------------------------------------------------------------
FROM node:22-alpine AS web
WORKDIR /src/web/frontend
RUN corepack enable
# Public build-time value (digits only); empty hides WhatsApp calls-to-action.
ARG VITE_WHATSAPP_NUMBER=""
ENV VITE_WHATSAPP_NUMBER=$VITE_WHATSAPP_NUMBER
# Public pages of the mobile app on the stores (https), once it is published; empty means it is not mentioned.
ARG VITE_PLAY_STORE_URL=""
ENV VITE_PLAY_STORE_URL=$VITE_PLAY_STORE_URL
ARG VITE_APP_STORE_URL=""
ENV VITE_APP_STORE_URL=$VITE_APP_STORE_URL
# "true" shows placeholders for the channels not connected yet (staging only; leave empty in production).
ARG VITE_CHANNEL_PLACEHOLDERS=""
ENV VITE_CHANNEL_PLACEHOLDERS=$VITE_CHANNEL_PLACEHOLDERS
# Environment label shown in the administration header ("staging"); empty or "production" shows none.
ARG VITE_APP_ENV=""
ENV VITE_APP_ENV=$VITE_APP_ENV
COPY web/frontend/package.json web/frontend/pnpm-lock.yaml web/frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/frontend/ ./
RUN pnpm run typecheck && pnpm run build

# ---- Go services --------------------------------------------------------
FROM golang:1.26-alpine AS go
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY infrastructure ./infrastructure
COPY packages ./packages
RUN mkdir -p /out && for s in api-gateway identity-service search-service conversation-service order-service payment-service catalog-service migrate trezutil-admin; do \
      go build -ldflags="-s -w" -o /out/$s ./cmd/$s || exit 1; \
    done

# ---- runtime ------------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 trezutil
WORKDIR /app
COPY --from=go /out/ /app/bin/
COPY migrations /app/migrations
COPY --from=web /src/web/frontend/dist /app/web
ENV MIGRATIONS_DIR=/app/migrations \
    WEB_ROOT=/app/web \
    APP_ENV=production
USER trezutil
# The gateway is the only port that should be published.
EXPOSE 8080
CMD ["/app/bin/api-gateway"]
