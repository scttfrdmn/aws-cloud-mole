# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binary
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X github.com/research-computing/mole/internal/version.Version=${VERSION} \
              -X github.com/research-computing/mole/internal/version.Commit=${COMMIT} \
              -X github.com/research-computing/mole/internal/version.Date=${DATE}" \
    -o mole ./cmd/mole

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    wireguard-tools \
    iptables \
    iproute2 \
    ethtool \
    curl

# Create non-root user
RUN addgroup -g 1000 mole && \
    adduser -u 1000 -G mole -s /bin/sh -D mole

# Copy binary and configs
COPY --from=builder /app/mole /usr/local/bin/mole
COPY configs/ /etc/mole/

# Create directories
RUN mkdir -p /var/lib/mole /var/log/mole && \
    chown -R mole:mole /var/lib/mole /var/log/mole /etc/mole

# Switch to non-root user
USER mole

# Set working directory
WORKDIR /var/lib/mole

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD mole version || exit 1

# Default command
ENTRYPOINT ["mole"]
CMD ["--help"]