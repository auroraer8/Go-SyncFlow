#!/bin/bash
# Go-SyncFlow 统一身份同步与管理平台 - 一键停止脚本
# 用法: ./stop.sh

SERVICE_NAME="go-syncflow"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=========================================="
echo "    Go-SyncFlow - 停止服务"
echo "=========================================="

# 停止主应用服务
for svc in "$SERVICE_NAME" "bi-dashboard"; do
    if systemctl is-active --quiet "$svc" 2>/dev/null; then
        echo "[*] 正在停止服务 $svc ..."
        systemctl stop "$svc"
        echo "[✓] 服务 $svc 已停止"
    fi
done

# 询问是否停止 PostgreSQL
echo ""
read -p "是否同时停止内置 PostgreSQL? (y/N): " stop_pg
if [[ "$stop_pg" =~ ^[Yy]$ ]]; then
    if systemctl is-active --quiet go-syncflow-pg 2>/dev/null; then
        systemctl stop go-syncflow-pg
        echo "[✓] PostgreSQL 已停止"
    elif [ -f "$PROJECT_DIR/pgdata/postmaster.pid" ]; then
        "$PROJECT_DIR/postgres/bin/pg_ctl" -D "$PROJECT_DIR/pgdata" stop 2>/dev/null || true
        echo "[✓] PostgreSQL 已停止"
    fi
fi

# 显示状态
systemctl status ${SERVICE_NAME} --no-pager 2>/dev/null || true

echo ""
echo "如需完全卸载服务，请运行:"
echo "  systemctl disable ${SERVICE_NAME} go-syncflow-pg"
echo "  rm /etc/systemd/system/${SERVICE_NAME}.service"
echo "  rm /etc/systemd/system/go-syncflow-pg.service"
echo "  systemctl daemon-reload"
