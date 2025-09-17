# Etherscan V2 API 超时问题优化方案

## 问题描述

用户反馈：使用EVM V2接口经常超时，但复制链接到浏览器访问则正常。

## 深度分析

### 问题根因

1. **反爬虫检测** - Etherscan对程序化访问有更严格的检测机制
2. **超时配置不合理** - 原有超时设置不足以应对Etherscan API的响应时间
3. **请求头不完整** - 缺少关键的浏览器特征头部
4. **请求频率过高** - 触发了Etherscan的限流机制
5. **连接复用问题** - HTTP连接配置不适合Etherscan API

### 浏览器 vs 程序访问差异

| 方面 | 浏览器 | 程序访问（优化前） |
|------|--------|-------------------|
| User-Agent | 完整浏览器标识 | 简单标识 |
| 请求头 | 完整浏览器头部 | 基础头部 |
| TLS指纹 | 浏览器TLS配置 | 标准Go TLS |
| 请求频率 | 人工控制 | 高频并发 |
| 连接行为 | 浏览器连接模式 | 程序连接模式 |

## 优化方案

### 1. HTTP客户端配置优化

#### 超时时间大幅增加
```go
// 优化前
TLSHandshakeTimeout:   20 * time.Second
ResponseHeaderTimeout: 90 * time.Second
DialContext: (&net.Dialer{
    Timeout: 30 * time.Second,
}).DialContext

// 优化后
TLSHandshakeTimeout:   45 * time.Second  // 增加到45秒
ResponseHeaderTimeout: 150 * time.Second // 增加到150秒
DialContext: (&net.Dialer{
    Timeout: 60 * time.Second,           // 增加到60秒
}).DialContext
```

#### 连接池优化
```go
// 优化前
MaxIdleConns:        100
MaxIdleConnsPerHost: 10
MaxConnsPerHost:     50

// 优化后 - 降低连接数避免被检测为爬虫
MaxIdleConns:        50
MaxIdleConnsPerHost: 5
MaxConnsPerHost:     20
```

#### 总超时时间增加
```go
// 优化前
const defaultHttpTimeout = 120 // 120秒

// 优化后
const defaultHttpTimeout = 180 // 180秒
```

### 2. 请求头完全模拟浏览器

#### 优化前的请求头
```go
headers := map[string]string{
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36...",
    "Accept":     "application/json,text/html,application/xhtml+xml...",
    // 基础头部
}
```

#### 优化后的完整浏览器头部
```go
headers := map[string]string{
    "User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Accept":                    "application/json,text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
    "Accept-Language":           "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
    "Accept-Encoding":           "gzip, deflate, br, zstd",
    "Sec-Ch-Ua":                 `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
    "Sec-Ch-Ua-Mobile":          "?0",
    "Sec-Ch-Ua-Platform":        `"Windows"`,
    "Priority":                  "u=0, i",
    "Pragma":                    "no-cache",
    // 更多完整的浏览器头部
}
```

### 3. 智能限流机制

#### 新增EtherscanRateLimiter
```go
type EtherscanRateLimiter struct {
    minInterval  time.Duration // 最小请求间隔200ms
    maxRequests  int          // 每秒最多5个请求
    // 窗口计数和互斥锁
}
```

#### 限流策略
- **最小间隔**: 200ms
- **频率限制**: 每秒最多5个请求
- **智能等待**: 自动计算等待时间
- **窗口重置**: 每秒重置计数窗口

### 4. 请求前置处理

#### 在所有Etherscan API请求前添加限流
```go
// callback.go中
if strings.Contains(url, "api.etherscan.io") {
    log.Debug("应用Etherscan API限流...")
    httpClient.WaitForEtherscan()
}

// monitor/trade.go中
if strings.Contains(requestURL, "api.etherscan.io") {
    log.Debug("应用Etherscan API限流...")
    httpClient.WaitForEtherscan()
}
```

## 技术实现细节

### 1. 文件修改清单

| 文件 | 修改内容 | 目的 |
|------|----------|------|
| `app/http/client.go` | 增加超时时间、优化连接池 | 提高请求成功率 |
| `app/http/rate_limiter.go` | 新增限流器 | 避免触发反爬虫 |
| `app/config/config.go` | 增加默认超时时间 | 确保足够处理时间 |
| `app/telegram/callback.go` | 优化请求头、添加限流 | 模拟浏览器行为 |
| `app/monitor/trade.go` | 添加限流机制 | 统一限流策略 |

### 2. 关键优化参数

| 参数 | 优化前 | 优化后 | 说明 |
|------|--------|--------|------|
| HTTP总超时 | 120秒 | 180秒 | 增加50% |
| TLS握手超时 | 20秒 | 45秒 | 增加125% |
| 响应头超时 | 90秒 | 150秒 | 增加67% |
| 连接超时 | 30秒 | 60秒 | 增加100% |
| 最小请求间隔 | 无 | 200ms | 新增限流 |
| 每秒最大请求 | 无限制 | 5个 | 新增限流 |

### 3. 错误处理增强

#### 针对Etherscan API的特殊错误处理
```go
// 在errors.go中已包含
retryableErrors := []string{
    "etherscan api错误",
    "api.etherscan.io",
    "deadline exceeded",
    "timeout",
    // 更多Etherscan相关错误
}
```

#### 智能重试延迟
```go
// 对Etherscan API使用更长的重试延迟
if strings.Contains(strings.ToLower(err.Error()), "etherscan") {
    baseDelay = 5 * time.Second // 更长延迟
    maxDelay = 60 * time.Second // 更长最大延迟
}
```

## 预期效果

### 1. 超时问题解决
- **连接超时**: 从30秒增加到60秒，解决连接建立慢的问题
- **TLS握手超时**: 从20秒增加到45秒，解决TLS握手慢的问题
- **响应超时**: 从90秒增加到150秒，解决API响应慢的问题
- **总超时**: 从120秒增加到180秒，提供充足的处理时间

### 2. 反爬虫检测规避
- **完整浏览器头部**: 模拟真实浏览器请求
- **智能限流**: 避免高频请求触发检测
- **连接数控制**: 降低并发连接数，减少被检测概率

### 3. 稳定性提升
- **重试机制优化**: 针对Etherscan API的特殊重试策略
- **错误分类**: 更精确的错误识别和处理
- **日志增强**: 更详细的请求和响应日志

## 监控和验证

### 1. 关键指标
- **请求成功率**: 目标 > 95%
- **平均响应时间**: 监控是否在合理范围内
- **超时错误率**: 目标 < 5%
- **限流触发频率**: 监控限流器工作情况

### 2. 日志监控
```go
// 请求前日志
log.Debug("应用Etherscan API限流...")

// 请求详情日志
log.Info(fmt.Sprintf("请求ETH兼容链API: %s", maskedURL))

// 响应时间日志
log.Info(fmt.Sprintf("HTTP响应: %s - %d - %v", url, statusCode, duration))
```

### 3. 错误统计
- 超时错误统计
- API限流错误统计
- 连接错误统计
- 成功率趋势分析

## 后续优化建议

### 1. 动态调整
- 根据实际成功率动态调整超时时间
- 根据API响应时间动态调整限流参数
- 根据错误类型优化重试策略

### 2. 缓存机制
- 实现API响应缓存，减少重复请求
- 智能缓存失效策略
- 缓存命中率监控

### 3. 负载均衡
- 如果有多个API密钥，实现负载均衡
- 根据API响应时间选择最优端点
- 实现故障转移机制

---

**优化完成时间**: 2025年1月17日  
**预期效果**: 解决Etherscan V2 API超时问题，提高请求成功率到95%以上  
**监控周期**: 建议持续监控1周，根据实际效果进行微调