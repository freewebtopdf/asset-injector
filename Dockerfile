ARG GO_DOCKER_TAG=1.25.5-alpine3.23

# Build arguments for version info and metadata
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE
ARG GO_VERSION

# Multi-stage build for Asset Injector Microservice
# Stage 1: Build stage using Alpine with Go
FROM golang:${GO_DOCKER_TAG} AS builder

# Accept HTTP proxy as a build argument
ARG HTTP_PROXY

# Set the HTTP proxy environment variables
ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTP_PROXY}

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for build with specific UID (65534 is reserved for nobody)
RUN adduser -D -g '' -u 10001 appuser

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Create required directories
RUN mkdir -p ./data/rules ./rules/local ./rules/community ./rules/overrides \
    && chown -R 10001:10001 ./data ./rules

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o asset-injector \
    ./cmd/server

# Stage 2: Runtime stage using scratch for minimal image
FROM scratch

# OCI image labels for security and metadata
LABEL org.opencontainers.image.title="Asset Injector Microservice"
LABEL org.opencontainers.image.description="High-performance microservice for injecting assets into web pages with caching and CORS support"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${COMMIT_SHA}"
LABEL org.opencontainers.image.created="${BUILD_DATE}"
LABEL org.opencontainers.image.source="https://github.com/user/asset-injector"
LABEL org.opencontainers.image.url="https://github.com/user/asset-injector"
LABEL org.opencontainers.image.documentation="https://github.com/user/asset-injector/blob/main/README.md"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.vendor="Asset Injector Team"
LABEL org.opencontainers.image.authors="Asset Injector Team"
LABEL maintainer="Asset Injector Team"

# Build metadata labels
LABEL build.go-version="${GO_VERSION}"
LABEL build.commit="${COMMIT_SHA}"
LABEL build.date="${BUILD_DATE}"

# Copy timezone data from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy passwd file for non-root user
COPY --from=builder /etc/passwd /etc/passwd

# Copy the binary from builder stage
COPY --from=builder /build/asset-injector /asset-injector
COPY --from=builder --chown=10001:10001 /build/data /data

# Create required directories with correct ownership
COPY --from=builder --chown=10001:10001 /build/rules /rules

# Switch to non-root user for security
USER appuser

# Expose port (default 8080)
EXPOSE 8080

# Health check using the /health endpoint
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/asset-injector", "-health-check"] || exit 1

# Set entrypoint
ENTRYPOINT ["/asset-injector"]
