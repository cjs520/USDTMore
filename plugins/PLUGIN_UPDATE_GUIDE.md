# USDTMore 插件签名算法更新指南

## 概述

USDTMore v2.1.0+ 已将签名算法从 MD5 升级为 HMAC-SHA256，以提供更高的安全性。本文档说明了插件的更新情况和使用方法。

## 签名算法变更

### 旧版本 (v2.0.x)
```php
// 旧的MD5签名方式
$sign = md5($sign . $signKey);
```

### 新版本 (v2.1.0+)
```php
// 新的HMAC-SHA256签名方式
$sign = hash_hmac('sha256', $sign, $signKey);
```

## 插件更新状态

### ✅ ACG-FAKA 插件
- **文件位置**: `plugins/acg-faka/USDTMore/`
- **状态**: 已更新为HMAC-SHA256
- **签名实现**: `plugins/acg-faka/USDTMore/Impl/Signature.php`
- **兼容版本**: USDTMore v2.1.0+

### ✅ 独角数卡 (Dujiaoka) 插件
- **文件位置**: `plugins/dujiaoka/app/Http/Controllers/Pay/`
- **状态**: 已更新为HMAC-SHA256
- **签名实现**: `plugins/dujiaoka/app/Http/Controllers/Pay/USDTMoreController.php`
- **兼容版本**: USDTMore v2.1.0+

## 升级步骤

### 1. 备份现有插件
```bash
# 备份现有插件文件
cp -r plugins/ plugins_backup/
```

### 2. 更新插件文件
- 对于 **ACG-FAKA**: 复制 `plugins/acg-faka/USDTMore/` 目录到你的ACG-FAKA安装目录
- 对于 **独角数卡**: 复制 `plugins/dujiaoka/app/Http/Controllers/Pay/USDTMoreController.php` 到对应目录

### 3. 验证配置
确保USDTMore的`.env`文件包含正确的配置：
```env
# API认证令牌
API_TOKEN=your_api_token_here

# EVM API配置
ETHERSCAN_API_KEY=your_etherscan_api_key
```

### 4. 测试支付流程
1. 创建测试订单
2. 验证签名生成和验证
3. 确认回调通知正常

## 签名算法实现细节

### 签名生成流程
1. **参数排序**: 对所有参数按键名进行字典序排序
2. **拼接字符串**: 按 `key=value&key=value` 格式拼接
3. **移除signature**: 排除signature参数本身
4. **HMAC签名**: 使用HMAC-SHA256算法和密钥生成签名

### 示例代码
```php
function generateSignature(array $data, string $key): string
{
    ksort($data);
    $sign = '';
    foreach ($data as $k => $v) {
        if ($v == '' || $k == 'signature') continue;
        $sign .= $k . '=' . $v . '&';
    }
    $sign = trim($sign, '&');
    return hash_hmac('sha256', $sign, $key);
}
```

## 支持的区块链网络

所有插件均支持以下区块链网络：

| 网络 | 代币标准 | 网络ID |
|------|----------|--------|
| TRON | TRC20-USDT | - |
| Ethereum | ERC20-USDT | 1 |
| BSC | BEP20-USDT | 56 |
| Polygon | Polygon-USDT | 137 |
| Arbitrum | Arbitrum-USDT | 42161 |
| Optimism | Optimism-USDT | 10 |
| X-Layer | X-Layer-USDT | 196 |

## 故障排除

### 常见问题

1. **签名验证失败**
   - 检查是否使用了最新的插件文件
   - 确认API_TOKEN配置正确
   - 验证参数拼接顺序

2. **回调通知失败**
   - 检查notify_url是否可访问
   - 确认服务器防火墙设置
   - 查看USDTMore日志

3. **支付页面无法访问**
   - 确认USDTMore服务正常运行
   - 检查API接口地址配置
   - 验证网络连接

### 调试方法

1. **启用调试日志**
```php
// 在插件中添加日志记录
error_log('USDTMore Debug: ' . json_encode($data));
```

2. **验证签名**
```php
// 手动验证签名
$expectedSign = hash_hmac('sha256', $signString, $apiToken);
if ($receivedSign !== $expectedSign) {
    error_log('Signature mismatch: expected=' . $expectedSign . ', received=' . $receivedSign);
}
```

## 版本兼容性

| USDTMore版本 | 签名算法 | 插件兼容性 |
|-------------|----------|------------|
| v2.1.0+ | HMAC-SHA256 | ✅ 使用新插件 |
| v2.0.x | MD5 | ❌ 已弃用 |

## 技术支持

如果在升级过程中遇到问题：

1. 检查本文档的故障排除部分
2. 查看USDTMore的日志文件
3. 确认所有配置参数正确
4. 测试网络连接和API访问

## 安全建议

1. **定期更新**: 保持USDTMore和插件为最新版本
2. **密钥管理**: 定期更换API_TOKEN
3. **HTTPS**: 在生产环境中使用HTTPS
4. **监控**: 设置支付异常监控和告警

---

**更新日期**: 2025-09-15  
**适用版本**: USDTMore v2.1.0+  
**文档版本**: 1.0