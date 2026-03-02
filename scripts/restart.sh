#!/bin/bash
# Go-SyncFlow 统一身份同步与管理平台 - 一键重启脚本
# 用法: ./restart.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_NAME="go-syncflow"

echo "=========================================="
echo "    Go-SyncFlow - 重启服务"
echo "=========================================="

if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
    echo "[*] 正在重启服务..."
    systemctl restart "$SERVICE_NAME"
    sleep 2
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        echo "[✓] 服务重启成功"
    else
        echo "[✗] 服务重启失败，查看日志: journalctl -u ${SERVICE_NAME} -f"
        exit 1
    fi
else
    echo "[*] 服务未运行，执行启动..."
    "$SCRIPT_DIR/start.sh"
fi

systemctl status "$SERVICE_NAME" --no-pager 2>/dev/null | head -10
