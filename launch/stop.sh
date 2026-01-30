#!/bin/bash
# stop.sh - 停止 Go 服务

# 引入公共配置
source "./config.sh"

echo "========================================"
echo "   AgentEarthMgr 停止服务"
echo "========================================"
echo "使用环境: $ENV"
echo

if [ ! -f "$PID_FILE" ]; then
    echo "未找到 PID 文件，$SERVICE_NAME 可能未在运行。"
    exit 1
fi

PID=$(cat "$PID_FILE")
echo "正在停止服务: $SERVICE_NAME，PID: $PID ..."
kill -9 "$PID"

# 删除 PID 文件
rm -f "$PID_FILE"
echo "服务已停止。"
