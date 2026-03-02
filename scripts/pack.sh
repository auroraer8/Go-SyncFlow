#!/bin/bash
# Go-SyncFlow 统一身份同步与管理平台 - 打包脚本
# 用法: ./pack.sh [输出文件名]
# 打包内容：预编译二进制 + 前端静态文件 + VPN客户端安装包 + PostgreSQL + 部署脚本
# 打包结果：解压后可一键启动，内置 PostgreSQL，无需额外依赖

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
OUTPUT_NAME="${1:-go-syncflow-v3.5-$(date +%Y%m%d)}"
OUTPUT_FILE="/tmp/${OUTPUT_NAME}.tar.gz"

echo "=========================================="
echo "    Go-SyncFlow 统一身份同步与管理平台"
echo "      打包工具 v3.5 (全功能离线版)"
echo "=========================================="

cd "$PROJECT_DIR"

# ========== 步骤 1：编译最新版本 ==========
echo ""
echo "[1/7] 编译后端..."
cd "$PROJECT_DIR/backend"
CGO_ENABLED=1 go build -o go-syncflow . 2>&1
echo "  [✓] 后端编译完成"

echo "[2/7] 构建前端..."
cd "$PROJECT_DIR/frontend"
if [ ! -d "node_modules" ]; then
    npm install --prefer-offline 2>&1
fi
npm run build 2>&1
rm -rf "$PROJECT_DIR/backend/static"
cp -r dist "$PROJECT_DIR/backend/static"
echo "  [✓] 前端构建完成"
cd "$PROJECT_DIR"

# ========== 步骤 2：创建打包目录 ==========
echo "[3/7] 准备打包目录..."
PACK_DIR="/tmp/go-syncflow-pack"
rm -rf "$PACK_DIR"
mkdir -p "$PACK_DIR/Go-SyncFlow"

# ========== 步骤 3：复制文件 ==========
echo "[4/7] 复制文件..."

# 复制后端（含预编译二进制和静态文件）
cp -r backend "$PACK_DIR/Go-SyncFlow/"
# 清理可能存在的旧目录
rm -rf "$PACK_DIR/Go-SyncFlow/backend/web"
echo "  - 后端目录已复制"

# 复制脚本
cp -r scripts "$PACK_DIR/Go-SyncFlow/"
echo "  - 脚本已复制"

# 不打包 frontend（前端已编译到 backend/static/，运行时不需要源码和 node_modules）
# 不打包 tooling（Go 安装包，部署时不需要编译）

# ========== 内置 PostgreSQL ==========
echo "  - 打包内置 PostgreSQL..."
mkdir -p "$PACK_DIR/Go-SyncFlow/postgres"

PG_PACKED=false

# 方式1：优先使用项目内置的 PostgreSQL（从之前的打包中获取）
if [ -d "$PROJECT_DIR/postgres/bin" ] && [ -f "$PROJECT_DIR/postgres/bin/postgres" ]; then
    echo "    - 检测到项目内置 PostgreSQL"
    cp -r "$PROJECT_DIR/postgres"/* "$PACK_DIR/Go-SyncFlow/postgres/"
    echo "    [✓] 项目内置 PostgreSQL 已复制"
    PG_PACKED=true
fi

# 方式2：如果项目没有内置，尝试从系统安装复制
if [ "$PG_PACKED" = false ]; then
    PG_VERSION=""
    for v in 16 15 14 13 12; do
        if [ -d "/usr/lib/postgresql/$v/bin" ]; then
            PG_VERSION="$v"
            break
        fi
    done

    if [ -n "$PG_VERSION" ]; then
        PG_BIN_DIR="/usr/lib/postgresql/$PG_VERSION/bin"
        PG_LIB_DIR="/usr/lib/postgresql/$PG_VERSION/lib"
        PG_SHARE_DIR="/usr/share/postgresql/$PG_VERSION"
        
        echo "    - 检测到系统 PostgreSQL $PG_VERSION"
        
        # 复制二进制
        mkdir -p "$PACK_DIR/Go-SyncFlow/postgres/bin"
        for bin in postgres initdb pg_ctl psql createdb createuser pg_dump pg_restore; do
            if [ -f "$PG_BIN_DIR/$bin" ]; then
                cp -f "$PG_BIN_DIR/$bin" "$PACK_DIR/Go-SyncFlow/postgres/bin/"
            fi
        done
        echo "    [✓] PostgreSQL 二进制已复制"
        
        # 复制库文件
        mkdir -p "$PACK_DIR/Go-SyncFlow/postgres/lib"
        cp -a "$PG_LIB_DIR"/* "$PACK_DIR/Go-SyncFlow/postgres/lib/" 2>/dev/null || true
        
        # 复制系统库依赖
        for lib in libpq libssl libcrypto libreadline libtinfo libncurses liblz4 libzstd libicuuc libicudata libicui18n; do
            find /usr/lib/x86_64-linux-gnu -name "${lib}.so*" -exec cp -a {} "$PACK_DIR/Go-SyncFlow/postgres/lib/" \; 2>/dev/null || true
        done
        echo "    [✓] PostgreSQL 库文件已复制"
        
        # 复制 share 目录
        if [ -d "$PG_SHARE_DIR" ]; then
            cp -r "$PG_SHARE_DIR" "$PACK_DIR/Go-SyncFlow/postgres/share"
            echo "    [✓] PostgreSQL share 目录已复制"
        fi
        PG_PACKED=true
    fi
fi

if [ "$PG_PACKED" = false ]; then
    echo "    [!] 警告：未找到 PostgreSQL，部署时需要手动安装"
fi

# 复制文档（只复制用户文档，排除开发/集成文档）
if [ -d "docs" ]; then
    mkdir -p "$PACK_DIR/Go-SyncFlow/docs"
    for doc in "系统使用手册.md" "VPN使用手册.md" "SSO使用手册.md" "功能文档.md" "技术架构文档.md" "API接口文档.md" "SSO接入指南（CAS-SAML-OIDC）.md" "ThirdParty-API-SSO-Auth.md"; do
        [ -f "docs/$doc" ] && cp "docs/$doc" "$PACK_DIR/Go-SyncFlow/docs/"
    done
    find docs -maxdepth 1 -name "*.pdf" -exec cp {} "$PACK_DIR/Go-SyncFlow/docs/" \; 2>/dev/null || true
    echo "  [✓] 用户文档已复制（已排除开发/集成/模板文档）"
fi
cp -f README.md "$PACK_DIR/Go-SyncFlow/" 2>/dev/null || true
cp -f 快速部署说明.txt "$PACK_DIR/Go-SyncFlow/" 2>/dev/null || true


# ========== 步骤 4：清理敏感数据 ==========
echo "[5/7] 清理敏感数据..."

DEST="$PACK_DIR/Go-SyncFlow"

# ---- 清理旧数据 ----
rm -rf "$DEST/backend/data/certs"
rm -rf "$DEST/backend/data/jwt_secret"
# 数据库配置保持默认（使用内置 PostgreSQL）
echo "  - 数据目录已清理"
# 清理 VPN 目录中的敏感数据，但保留配置模板
if [ -d "$DEST/backend/data/vpn/conf" ]; then
    # 保留 vpn.db 中的 setting 表配置（页面标题等），清除用户数据
    if [ -f "$DEST/backend/data/vpn/conf/vpn.db" ]; then
        # 创建干净的默认配置数据库
        python3 -c "
import sqlite3
import json
import os

db_path = '$DEST/backend/data/vpn/conf/vpn.db'
conn = sqlite3.connect(db_path)
cursor = conn.cursor()

# 清除用户敏感数据
cursor.execute('DELETE FROM user')
cursor.execute('DELETE FROM user_act_log')
cursor.execute('DELETE FROM access_audit')
cursor.execute('DELETE FROM ip_map')
cursor.execute('DELETE FROM stats_network')
cursor.execute('DELETE FROM stats_cpu')
cursor.execute('DELETE FROM stats_mem')
cursor.execute('DELETE FROM stats_online')

# 重置序列
cursor.execute(\"DELETE FROM sqlite_sequence WHERE name IN ('user', 'user_act_log', 'access_audit', 'ip_map')\")

conn.commit()
conn.close()
print('  - VPN 数据库已清理用户数据，保留服务配置')
" 2>/dev/null || {
            # 如果 python 清理失败，直接删除
            rm -f "$DEST/backend/data/vpn/conf/vpn.db"
            echo "  - VPN 数据库已删除（Python 清理失败）"
        }
    fi
    rm -f "$DEST/backend/data/vpn/conf/vpn_cert.pem"
    rm -f "$DEST/backend/data/vpn/conf/vpn_cert.key"
    echo "  - VPN 证书已清除"
    # 去除重复的 VPN 客户端安装包（根目录与子目录重复）
    if [ -d "$DEST/backend/data/vpn/conf/files" ]; then
        rm -f "$DEST/backend/data/vpn/conf/files/anyconnect-macos-intel.dmg"
        rm -f "$DEST/backend/data/vpn/conf/files/cisco-secure-client-macos-apple-silicon.dmg"
        echo "  - 已去除根目录重复的 macOS 安装包（macos/ 子目录保留）"
        VPN_FILES_SIZE=$(du -sh "$DEST/backend/data/vpn/conf/files" | cut -f1)
        echo "  [✓] VPN 客户端安装包已保留 ($VPN_FILES_SIZE)"
    else
        echo "  [!] 警告：未找到 VPN 客户端安装包，443下载页面将无文件可下载"
    fi
fi

# ---- 证书和密钥 ----
rm -rf "$DEST/backend/certs"
find "$DEST" \( -name "jwt_secret" -o -name "*.pem" -o -name "*.key" -o -name "*.crt" \) ! -path "*/vpn/conf/*" -exec rm -f {} + 2>/dev/null || true
echo "  - 证书和密钥已清除"

# ---- 日志文件 ----
find "$DEST" -name "*.log" -exec rm -f {} + 2>/dev/null || true
echo "  - 日志文件已清除"

# ---- 旧编译产物 ----
rm -f "$DEST/backend/server"
rm -f "$DEST/backend/bi-dashboard"

# ---- Go 源码（新机器不需要编译）----
find "$DEST/backend" -name "*.go" -exec rm -f {} + 2>/dev/null || true
rm -f "$DEST/backend/go.mod" "$DEST/backend/go.sum"
echo "  - Go 源码已清除（使用预编译二进制）"

# ---- 其他 ----
rm -rf "$DEST/.git"

# ---- 清理多余的打包脚本 ----
rm -f "$DEST/scripts/pack-full.sh"
rm -f "$DEST/docs/generate-pdf.sh"
rm -f "$DEST/docs/template.tex"
rm -f "$DEST/docs/vpn-landing-page-sample.html"

# ---- 清理 SQLite 相关（已废弃，但保留 VPN 配置数据库）----
find "$DEST" \( -name "*.db" -o -name "*.sqlite" -o -name "*.sqlite3" \) ! -name "vpn.db" -exec rm -f {} + 2>/dev/null || true

echo "  [OK] 全部敏感数据已清理"

# ========== 步骤 5：确保脚本可执行 ==========
echo "[6/7] 设置文件权限..."
chmod +x "$DEST/scripts/"*.sh
chmod +x "$DEST/backend/go-syncflow"

# ========== 步骤 6：打包 ==========
echo "[7/7] 压缩打包..."

cd /tmp
tar -czf "$OUTPUT_FILE" -C go-syncflow-pack Go-SyncFlow

# 清理临时目录
rm -rf "$PACK_DIR"

# 复制到项目目录
cp "$OUTPUT_FILE" "$PROJECT_DIR/"

FILE_SIZE=$(du -h "$PROJECT_DIR/${OUTPUT_NAME}.tar.gz" | cut -f1)

echo ""
echo "=========================================="
echo "    打包完成！"
echo "=========================================="
echo ""
echo "输出文件: $PROJECT_DIR/${OUTPUT_NAME}.tar.gz"
echo "文件大小: $FILE_SIZE"
echo ""
echo "包含内容："
echo "  [✓] 后端预编译二进制（go-syncflow）"
echo "  [✓] 前端静态资源（backend/static/）"
echo "  [✓] VPN 客户端安装包（443下载页面）"
echo "  [✓] PostgreSQL 数据库依赖（postgres/）"
echo "  [✓] 部署脚本（scripts/）"
echo "  [✓] 用户文档（docs/）"
echo ""
echo "=========================================="
echo "  部署方法（一键部署）"
echo "=========================================="
echo ""
echo "  1. 上传到新服务器"
echo "  2. 解压："
echo "     tar -xzf ${OUTPUT_NAME}.tar.gz -C /opt/"
echo ""
echo "  3. 一键启动："
echo "     cd /opt/Go-SyncFlow && chmod +x scripts/*.sh"
echo "     ./scripts/start.sh"
echo ""
echo "  4. 其他命令："
echo "     ./scripts/stop.sh           # 停止服务"
echo "     ./scripts/restart.sh        # 重启服务"
echo "     ./scripts/reset-admin.sh    # 重置管理员密码"
echo ""
echo "  默认管理员: admin / Admin@2024"
echo "  管理后台: https://服务器IP:8443"
echo "  VPN入口: https://服务器IP (443端口，含客户端下载页)"
echo ""
echo "  说明："
echo "  - 内置 PostgreSQL 数据库，开箱即用"
echo "  - 包含预编译二进制，无需在新机器上编译"
echo "  - VPN 443端口自带客户端下载页面及安装包"
echo "  - 首次启动自动初始化 PostgreSQL 和应用数据"
echo "  - 通知渠道、同步连接器等需在界面中配置"
echo ""
