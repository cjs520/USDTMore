# USDTMore 独角数卡插件

## 使用方法

### 1. 安装路由
将`routes`目录里面的代码增加到独角数卡的route上

```php
// USDTMore
Route::get('usdtmore/{payway}/{orderSN}', 'USDTMoreController@gateway');
Route::post('usdtmore/notify_url', 'USDTMoreController@notifyUrl');
Route::get('usdtmore/return_url', 'USDTMoreController@returnUrl')->name('usdtmore-return');
```

### 2. 安装控制器
将`app/Http/Controllers/Pay/USDTMoreController.php`复制到独角数卡的`app/Http/Controllers/Pay/`目录下

### 3. 配置支付方式
在独角数卡后台添加您需要的支付方式：

| 支付选项     | 商户id | 商户key | 商户密钥 | 支付标识               | 备注                                                                                        |     
|:---------| :----- | :----- | :----- |--------------------|:------------------------------------------------------------------------------------------|       
| USDTMore | API接口认证token | 空 | USDTMore收银台地址+/api/v1/order/create-transaction| TRON\|POLY\|OP\|BSC | 如果独角数卡和USDTMore在同一服务器则填写`127.0.0.1`不要填域名，例如`http://127.0.0.1:6080/api/v1/order/create-transaction` |

## 重要更新

### 签名算法变更 (v2.1.0+)
- **旧版本**: 使用 MD5 签名算法
- **新版本**: 使用 HMAC-SHA256 签名算法

**升级说明**：
1. 如果您使用的是USDTMore v2.1.0或更高版本，请确保使用最新的插件文件
2. 新的签名算法提供更高的安全性
3. 插件会自动使用正确的签名算法，无需手动配置

### 支持的区块链网络
- **TRON** (TRC20-USDT)
- **Ethereum** (ERC20-USDT) 
- **BSC** (BEP20-USDT)
- **Polygon** (Polygon-USDT)
- **Arbitrum** (Arbitrum-USDT)
- **Optimism** (Optimism-USDT)
- **X-Layer** (X-Layer-USDT)

## 技术支持
如遇到问题，请检查：
1. USDTMore服务是否正常运行
2. API接口地址是否正确
3. 认证token是否有效
4. 网络连接是否正常