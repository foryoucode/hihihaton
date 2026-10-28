# syntax=docker/dockerfile:1

FROM node:22-bookworm-slim AS frontend
WORKDIR /build/client
COPY client/package.json client/package-lock.json ./
RUN --mount=type=secret,id=ca_bundle \
    if [ -f /run/secrets/ca_bundle ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/ca_bundle; fi; \
    npm ci --no-audit --no-fund --strict-ssl=true
COPY client/ ./
RUN npm run build

FROM golang:1.23.4-bookworm AS backend
WORKDIR /build/backend
ENV CGO_ENABLED=0 GOMAXPROCS=2
ARG GOPROXY=https://proxy.golang.org
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=secret,id=ca_bundle \
    --mount=type=cache,target=/go/pkg/mod \
    if [ -f /run/secrets/ca_bundle ]; then \
        export SSL_CERT_FILE=/run/secrets/ca_bundle GIT_SSL_CAINFO=/run/secrets/ca_bundle; \
    fi; \
    go mod download
COPY backend/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /out/server ./cmd && go build -o /out/migrate ./cmd/migrate

FROM debian:bookworm-slim
WORKDIR /app
ENV NORTH_ROOT=/app \
    STATIC_DIR=/app/client/dist \
    APP_ADDRESS=0.0.0.0:8080 \
    GIN_MODE=release
RUN install -d -o 10001 -g 10001 /app/runtime
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=backend --chmod=755 /out/server /out/migrate /app/
COPY --from=frontend --chown=10001:10001 /build/client/dist /app/client/dist
COPY --chown=10001:10001 data/ /app/data/
USER 10001:10001
EXPOSE 8080
CMD ["/app/server"]
