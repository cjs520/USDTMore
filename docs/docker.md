# Docker安装教程

保证你的服务器已经安装了`Docker`和`Docker Compose`，如果没有请自行安装。

直接执行命令，注意下面的参数需要修改为你自己合适的参数，按照相同的格式进行修改。

```bash
docker run -d --restart=always --name usdtmore -p 6080:6080  -e TG_BOT_TOKEN=[机器人token] -e TG_BOT_ADMIN_ID=[管理员id]  -e AUTH_TOKEN=[验证密钥]  -e APP_URI=[USDT网关域名] -e REWRITE_HTTPS=true zxzx412/usdtmore:latest
```

执行成功后，访问`http://你的服务器IP:6080`如果能正常打开，则安装成功，你就可以使用机器人添加钱包地址了。
