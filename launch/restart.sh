#!/bin/bash
# restart.sh - 重启 Go 服务（不重新编译）

# 引入公共配置
source "./config.sh"
SCRIPT_DIR="$BIN_DIR"

echo "========================================"
echo "   AgentEarthMgr 重启服务"
echo "========================================"
echo "使用环境: $ENV"
echo

# 停止服务
echo "正在停止现有服务..."
"$SCRIPT_DIR/stop.sh"

echo
echo "等待服务完全停止..."
sleep 2


echo "正在重启服务..."
"$SCRIPT_DIR/start.sh"
