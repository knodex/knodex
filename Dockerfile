# =============================================================================
# Unified Multi-Stage Dockerfile: Web + Server in a single image
# =============================================================================
# Produces a single Go binary with the React web app embedded via go:embed.
# Pattern: ArgoCD (argocd-server), Grafana (grafana-server), Headlamp.

# Stage 1: Build web (Node)
FROM node:22-alpine AS web-builder

WORKDIR /web

# Build argument for enterprise mode
ARG BUILD_MODE=""

# Copy package files and install dependencies
COPY web/package*.json ./
RUN npm ci

# Copy web source
COPY web/ ./

# Copy version.txt to parent directory where vite.config.ts expects it
COPY version.txt /version.txt

# Build the web app (enterprise or OSS based on BUILD_MODE)
RUN if [ "$BUILD_MODE" = "enterprise" ]; then \
        npm run build:enterprise; \
    else \
        npm run build; \
    fi

# Stage 2: Build server (Go) with embedded web
FROM golang:1.26-alpine AS server-builder

WORKDIR /build

# Build argument for enterprise features
ARG BUILD_TAGS=""

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY server/go.mod server/go.sum* ./
RUN go mod download

# Copy server source code
COPY server/ ./

# Copy web dist into the embed path
COPY --from=web-builder /web/dist /build/internal/static/dist

# Buildx injects TARGETOS/TARGETARCH automatically for multi-arch builds.
# Without --platform, these are empty and Go falls back to host OS/arch.
ARG TARGETOS
ARG TARGETARCH

# Build the binary (with optional build tags for enterprise)
RUN if [ -n "$BUILD_TAGS" ]; then \
        CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -tags="$BUILD_TAGS" -ldflags="-w -s" -o /knodex-server .; \
    else \
        CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o /knodex-server .; \
    fi

# Stage 3: Runtime
FROM alpine:3.23

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy the binary (web is embedded)
COPY --from=server-builder /knodex-server .

# Create non-root user with high UID to avoid conflicts with K8s distros
RUN addgroup -g 10001 -S appgroup && \
    adduser -S -u 10001 -G appgroup appuser && \
    chown -R appuser:appgroup /app
USER appuser

EXPOSE 8080

ENTRYPOINT ["./knodex-server"]
