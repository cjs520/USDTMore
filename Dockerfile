# Multi-architecture build support
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

FROM --platform=$BUILDPLATFORM golang:1.22.2 AS builder

ENV GO111MODULE=on
ENV CGO_ENABLED=0
WORKDIR /go/release

# Copy source code
COPY . .

# Build for target architecture
RUN set -x \
    && echo "Building for platform: $TARGETPLATFORM (OS: $TARGETOS, ARCH: $TARGETARCH)" \
    && GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
        -trimpath \
        -ldflags="-s -w -buildid=" \
        -o usdtmore ./main

FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive
ENV DEBCONF_NOWARNINGS="yes"
ENV TZ=Asia/Shanghai
ENV HTML_DIR=/runtime

COPY --from=builder /go/release/usdtmore /runtime/usdtmore

ADD ./templates /runtime/templates
ADD ./static /runtime/static

# Install runtime dependencies for PostgreSQL deployment
RUN apt-get update && apt-get install -y --no-install-recommends \
        tzdata \
        ca-certificates \
        curl \
        postgresql-client \
    && ln -fs /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && dpkg-reconfigure -f noninteractive tzdata \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /tmp/* /var/tmp/*

# Create non-root user for security
RUN groupadd -r usdtmore && useradd -r -g usdtmore usdtmore \
    && chown -R usdtmore:usdtmore /runtime

# Switch to non-root user
USER usdtmore

# Set working directory
WORKDIR /runtime

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/health || exit 1

# Expose port
EXPOSE 8080

# Start application
CMD ["./usdtmore"]