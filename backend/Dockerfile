# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.26-bookworm AS build
WORKDIR /src

# Dependency layer cache
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# sqlite driver (mattn/go-sqlite3) requires cgo
ENV CGO_ENABLED=1
RUN go build -trimpath -ldflags="-s -w" -o /out/openagent ./cmd/server

# ---- Runtime stage ----
# Use debian-slim (with glibc) instead of scratch: cgo binary needs dynamic libc,
# ca-certificates for invokeUpstreamMCP to reach external MCP servers over HTTPS.
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/openagent /app/openagent
RUN mkdir -p /app/data

# Console REST + MCP Gateway
EXPOSE 8084 8085

# Production MUST override these sensitive values via -e (otherwise security warnings are printed at startup):
#   JWT_SECRET / ENCRYPTION_KEY / BOOTSTRAP_PASSWORD
ENTRYPOINT ["/app/openagent"]
