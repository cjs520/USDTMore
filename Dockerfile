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
COPY ./wait-for-db.sh /runtime/wait-for-db.sh

ADD ./templates /runtime/templates
ADD ./static /runtime/static

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
        tzdata \
        ca-certificates \
        curl \
        postgresql-client \
        netcat-openbsd \
    && ln -fs /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && dpkg-reconfigure -f noninteractive tzdata \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /tmp/* /var/tmp/*

# Create non-root user for security
RUN groupadd -r usdtmore && useradd -r -g usdtmore usdtmore \
    && chmod +x /runtime/wait-for-db.sh \
    && chown -R usdtmore:usdtmore /runtime

# Switch to non-root user
USER usdtmore

# Set working directory
WORKDIR /runtime

# Health check - verify both HTTP service and database connectivity
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8080/api/health && \
        pg_isready -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME || exit 1

# Expose port
EXPOSE 8080

# Start application
CMD ["./usdtmore"]