#!/bin/bash

# wait-for-db.sh - 等待数据库服务启动的脚本

set -e

host="$1"
port="$2"
shift 2
cmd="$@"

# 默认值
host=${host:-localhost}
port=${port:-5432}
timeout=${WAIT_TIMEOUT:-30}

echo "等待数据库 $host:$port 启动..."

# 等待数据库连接
for i in $(seq 1 $timeout); do
    if nc -z "$host" "$port" > /dev/null 2>&1; then
        echo "数据库 $host:$port 已就绪！"
        break
    fi
    
    if [ $i -eq $timeout ]; then
        echo "错误: 等待数据库 $host:$port 超时 ($timeout 秒)"
        exit 1
    fi
    
    echo "等待数据库启动... ($i/$timeout)"
    sleep 1
done

# 如果提供了命令，则执行
if [ -n "$cmd" ]; then
    echo "执行命令: $cmd"
    exec $cmd
fi