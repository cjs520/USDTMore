# Multi-stage build for USDTMore
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o usdtmore \
    .

# Final stage - use alpine for tools support
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    netcat-openbsd \
    bash \
    postgresql-client \
    && rm -rf /var/cache/apk/*

# Set timezone and logging configuration
ENV TZ=Asia/Shanghai
ENV DOCKER_ENV=true
ENV LOG_TO_STDOUT=true
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# Create non-root user
RUN addgroup -g 1001 -S usdtmore && \
    adduser -u 1001 -S usdtmore -G usdtmore

# Copy the binary
COPY --from=builder /build/usdtmore /app/usdtmore

# Copy deploy script and make it executable
COPY --from=builder /build/deploy.sh /app/deploy.sh

# Copy static files and templates
COPY --from=builder /build/templates /app/templates
COPY --from=builder /build/static /app/static

# Set permissions
RUN chmod +x /app/usdtmore /app/deploy.sh && \
    mkdir -p /app/logs && \
    chown -R usdtmore:usdtmore /app

# Switch to non-root user
USER usdtmore

# Set working directory
WORKDIR /app

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD ["/app/usdtmore", "--health-check"]

# Expose port
EXPOSE 6080

# Start application
ENTRYPOINT ["/app/usdtmore"]