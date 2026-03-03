# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Build argument for enterprise features
ARG BUILD_TAGS=""

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files (context is ./server)
COPY go.mod go.sum* ./
RUN go mod download

# Copy server source code (context is ./server)
COPY . ./

# Build the binary (with optional build tags for enterprise)
RUN if [ -n "$BUILD_TAGS" ]; then \
        CGO_ENABLED=0 GOOS=linux go build -tags="$BUILD_TAGS" -ldflags="-w -s" -o /knodex-server .; \
    else \
        CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /knodex-server .; \
    fi

# Runtime stage
FROM alpine:3.23

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy the binary
COPY --from=builder /knodex-server .

# Create non-root user with high UID to avoid conflicts with K8s distros
RUN addgroup -g 10001 -S appgroup && \
    adduser -S -u 10001 -G appgroup appuser && \
    chown -R appuser:appgroup /app
USER appuser

EXPOSE 8080

ENTRYPOINT ["./knodex-server"]
