# USDTMore项目安全配置

## 环境变量配置

创建 `.env` 文件：

```bash
# 应用配置
LISTEN=:6080
ENVIRONMENT=production

# 认证配置
AUTH_TOKEN=your-secure-auth-token-here

# 支付配置
PAYMENT_AMOUNT_RANGE=0.01,99999
EXPIRE_TIME=1800
USDT_RATE==7.4

# TRON配置
TRON_SERVER_API=TRON_SCAN
TRON_SCAN_API_KEY=your-tron-scan-api-key
TRON_GRID_API_KEY=your-tron-grid-api-key

# Polygon配置
POLYGON_SCAN_API_KEY=your-polygon-scan-api-key

# Optimism配置
OPTIMISM_EXPLORER_API_KEY=your-optimism-api-key

# BSC配置
BSC_SCAN_API_KEY=your-bsc-api-key

# Telegram配置
TG_BOT_TOKEN=your-telegram-bot-token
TG_BOT_ADMIN_ID=your-admin-user-id
TG_BOT_GROUP_ID=your-notification-group-id

# 文件路径配置
DB_DIR=/var/lib/usdtmore
LOG_DIR=/var/log/usdtmore
HTML_DIR=/usr/share/usdtmore

# 交易配置
TRADE_IS_CONFIRMED=false
ETH_CONFIRMATION=50

# 应用配置
APP_URI=https://yourdomain.com
REWRITE_HTTPS=true
```

## Docker安全配置

### docker-compose.yml

```yaml
version: '3.8'

services:
  usdtmore:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: usdtmore
    restart: unless-stopped
    environment:
      - ENVIRONMENT=production
    env_file:
      - .env
    ports:
      - "6080:6080"
    volumes:
      - ${DB_DIR}:/app/data
      - ${LOG_DIR}:/app/logs
    networks:
      - usdtmore-network
    # 安全设置
    cap_drop:
      - ALL
    cap_add:
      - CHOWN
      - SETGID
      - SETUID
    read_only: true
    tmpfs:
      - /tmp
      - /var/tmp
    user: "1000:1000"  # 非root用户

  nginx:
    image: nginx:alpine
    container_name: usdtmore-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - usdtmore
    networks:
      - usdtmore-network
    # 安全设置
    cap_drop:
      - ALL
    cap_add:
      - CHOWN
      - SETGID
      - NET_BIND_SERVICE
    read_only: true

volumes:
  usdtmore_data:
    driver: local

networks:
  usdtmore-network:
    driver: bridge
    internal: false
```

### Dockerfile

```dockerfile
FROM golang:1.21-alpine AS builder

# 安装必要的工具
RUN apk add --no-cache git ca-certificates

# 创建非root用户
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

# 设置工作目录
WORKDIR /app

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o usdtmore .

# 最终镜像
FROM alpine:latest

# 安装ca-certificates
RUN apk --no-cache add ca-certificates

# 创建非root用户
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

# 创建必要的目录
RUN mkdir -p /app/data /app/logs && \
    chown -R appuser:appgroup /app

# 从builder复制二进制文件
COPY --from=builder /app/usdtmore /app/usdtmore

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 6080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:6080/ || exit 1

# 启动应用
CMD ["/app/usdtmore"]
```

## Nginx安全配置

```nginx
user nginx;
worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /var/run/nginx.pid;

events {
    worker_connections 1024;
    use epoll;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
    
    # 隐藏版本信息
    server_tokens off;
    
    # 日志格式
    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent" "$http_x_forwarded_for"';
    
    access_log /var/log/nginx/access.log main;
    
    # 基础设置
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    client_max_body_size 1M;
    
    # Gzip压缩
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;
    
    # 上游
    upstream usdtmore {
        server usdtmore:6080;
    }
    
    # HTTP重定向
    server {
        listen 80;
        server_name yourdomain.com;
        return 301 https://$server_name$request_uri;
    }
    
    # HTTPS服务器
    server {
        listen 443 ssl http2;
        server_name yourdomain.com;
        
        # SSL配置
        ssl_certificate /etc/nginx/ssl/cert.pem;
        ssl_certificate_key /etc/nginx/ssl/key.pem;
        ssl_session_cache shared:SSL:1m;
        ssl_session_timeout 5m;
        ssl_ciphers HIGH:!aNULL:!MD5;
        ssl_prefer_server_ciphers on;
        
        # HSTS
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
        
        # 安全头
        add_header X-Frame-Options "DENY" always;
        add_header X-Content-Type-Options "nosniff" always;
        add_header X-XSS-Protection "1; mode=block" always;
        add_header Referrer-Policy "strict-origin-when-cross-origin" always;
        add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:;" always;
        
        # 代理到应用
        location / {
            proxy_pass http://usdtmore;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            
            # 超时设置
            proxy_connect_timeout 30s;
            proxy_send_timeout 30s;
            proxy_read_timeout 30s;
        }
        
        # 静态文件
        location /static/ {
            alias /usr/share/usdtmore/static/;
            expires 1y;
            add_header Cache-Control "public, immutable";
        }
        
        # 健康检查
        location /health {
            access_log off;
            return 200 "OK";
            add_header Content-Type text/plain;
        }
    }
}
```

## 系统服务配置

创建 systemd 服务文件 `/etc/systemd/system/usdtmore.service`：

```ini
[Unit]
Description=USDTMore Payment Service
After=network.target

[Service]
Type=simple
User=usdtmore
Group=usdtmore
WorkingDirectory=/opt/usdtmore
ExecStart=/opt/usdtmore/usdtmore
Restart=always
RestartSec=5
Environment=ENVIRONMENT=production
EnvironmentFile=/etc/usdtmore/.env

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/usdtmore/data /var/log/usdtmore
ReadOnlyPaths=/opt/usdtmore

[Install]
WantedBy=multi-user.target
```

## 防火墙配置

```bash
# 允许SSH
sudo ufw allow 22/tcp

# 允许HTTP和HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# 默认拒绝所有入站
sudo ufw default deny incoming

# 启用防火墙
sudo ufw enable
```

## 日志轮转配置

创建 `/etc/logrotate.d/usdtmore`：

```
/var/log/usdtmore/*.log {
    daily
    missingok
    rotate 52
    compress
    delaycompress
    notifempty
    create 640 usdtmore usdtmore
    postrotate
        systemctl reload usdtmore
    endscript
}
```

## 监控配置

### Prometheus配置

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'usdtmore'
    static_configs:
      - targets: ['localhost:6080']
    metrics_path: /metrics
    scrape_interval: 30s
```

### 告警规则

```yaml
groups:
- name: usdtmore
  rules:
  - alert: USDTMoreDown
    expr: up{job="usdtmore"} == 0
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "USDTMore service is down"
      
  - alert: HighErrorRate
    expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High error rate detected"
```

## 安全最佳实践

1. **定期更新**：保持系统和依赖库最新
2. **监控日志**：实时监控异常活动
3. **备份策略**：定期备份配置和数据库
4. **访问控制**：限制对服务器的访问
5. **SSL证书**：使用有效的SSL证书
6. **密码策略**：使用强密码并定期更换
7. **API密钥**：定期轮换API密钥
8. **网络隔离**：使用防火墙和网络隔离