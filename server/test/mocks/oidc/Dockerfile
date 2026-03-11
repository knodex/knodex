# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Buildx injects TARGETOS/TARGETARCH automatically for multi-arch builds.
ARG TARGETOS
ARG TARGETARCH

# Build the mock OIDC server
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /mock-oidc ./test/mocks/oidc/cmd/main.go

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary from builder
COPY --from=builder /mock-oidc /mock-oidc

# Run as non-root user (distroless default: 65532)
USER 65532:65532

EXPOSE 8081

ENTRYPOINT ["/mock-oidc"]
CMD ["--port=8081"]
