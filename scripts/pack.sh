#!/bin/bash
# Go-SyncFlow 统一身份同步与管理平台 - 打包脚本
# 用法: ./pack.sh [输出文件名]
# 打包内容：预编译二进制 + 前端静态文件 + VPN客户端安装包 + PostgreSQL + 部署脚本
# 打包结果：解压后可一键启动，内置 PostgreSQL，无需额外依赖

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
OUTPUT_NAME="${1:-go-syncflow-v3.6-$(date +%Y%m%d)}"
OUTPUT_FILE="/tmp/${OUTPUT_NAME}.tar.gz"

echo "=========================================="
echo "    Go-SyncFlow 统一身份同步与管理平台"
echo "      打包工具 v3.6 (精简离线版)"
echo "=========================================="

cd "$PROJECT_DIR"

# ========== 步骤 1：编译最新版本 ==========
echo ""
echo "[1/7] 编译后端..."
cd "$PROJECT_DIR/backend"
CGO_ENABLED=1 go build -ldflags="-s -w" -o go-syncflow . 2>&1
echo "  [✓] 后端编译完成（已strip调试信息）"

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
mkdir -p "$PACK_DIR/Go-SyncFlow/backend"

# ========== 步骤 3：复制文件（精简模式：只复制必要文件）==========
echo "[4/7] 复制必要文件..."

DEST="$PACK_DIR/Go-SyncFlow"

# ---- 复制后端二进制 ----
cp "$PROJECT_DIR/backend/go-syncflow" "$DEST/backend/"
echo "  - 后端二进制已复制"

# ---- 复制前端静态资源 ----
cp -r "$PROJECT_DIR/backend/static" "$DEST/backend/"
echo "  - 前端静态资源已复制"

# ---- 复制必要的配置文件（但不包含敏感数据）----
mkdir -p "$DEST/backend/data"
# 复制默认数据库配置
cat > "$DEST/backend/data/database.yaml" << 'DBCONFIG'
# Go-SyncFlow PostgreSQL 数据库配置
# 内置 PostgreSQL，开箱即用

host: 127.0.0.1
port: 5432
user: syncflow
password: syncflow
database: go_syncflow
sslmode: disable
max_idle: 10
max_open: 100
DBCONFIG
echo "  - 默认配置文件已创建"

# ---- 复制 VPN 客户端安装包（如存在）----
if [ -d "$PROJECT_DIR/backend/data/vpn/conf/files" ]; then
    mkdir -p "$DEST/backend/data/vpn/conf/files"
    # 复制所有子目录（android/, macos/, windows/）
    cp -r "$PROJECT_DIR/backend/data/vpn/conf/files"/* "$DEST/backend/data/vpn/conf/files/" 2>/dev/null || true
    # 去除根目录重复的 macOS 安装包（子目录已有）
    rm -f "$DEST/backend/data/vpn/conf/files/anyconnect-macos-intel.dmg" 2>/dev/null
    rm -f "$DEST/backend/data/vpn/conf/files/cisco-secure-client-macos-apple-silicon.dmg" 2>/dev/null
    VPN_FILES_SIZE=$(du -sh "$DEST/backend/data/vpn/conf/files" 2>/dev/null | cut -f1)
    echo "  - VPN 客户端安装包已复制 ($VPN_FILES_SIZE)"
else
    echo "  [!] 警告：未找到 VPN 客户端安装包"
fi

# ---- 复制脚本（只复制部署必须的）----
mkdir -p "$DEST/scripts"
for script in start.sh stop.sh restart.sh reset-admin.sh enable-external-db.sh; do
    [ -f "$PROJECT_DIR/scripts/$script" ] && cp "$PROJECT_DIR/scripts/$script" "$DEST/scripts/"
done
echo "  - 部署脚本已复制"

# ---- 内置 PostgreSQL ----
echo "  - 打包内置 PostgreSQL..."
mkdir -p "$DEST/postgres"

PG_PACKED=false

# 方式1：优先使用项目内置的 PostgreSQL
if [ -d "$PROJECT_DIR/postgres/bin" ] && [ -f "$PROJECT_DIR/postgres/bin/postgres" ]; then
    echo "    - 检测到项目内置 PostgreSQL"
    cp -r "$PROJECT_DIR/postgres"/* "$DEST/postgres/"
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
        mkdir -p "$DEST/postgres/bin"
        for bin in postgres initdb pg_ctl psql createdb createuser pg_dump pg_restore; do
            [ -f "$PG_BIN_DIR/$bin" ] && cp -f "$PG_BIN_DIR/$bin" "$DEST/postgres/bin/"
        done
        echo "    [✓] PostgreSQL 二进制已复制"
        
        # 复制库文件
        mkdir -p "$DEST/postgres/lib"
        cp -a "$PG_LIB_DIR"/* "$DEST/postgres/lib/" 2>/dev/null || true
        
        # 复制系统库依赖
        for lib in libpq libssl libcrypto libreadline libtinfo libncurses liblz4 libzstd libicuuc libicudata libicui18n; do
            find /usr/lib/x86_64-linux-gnu -name "${lib}.so*" -exec cp -a {} "$DEST/postgres/lib/" \; 2>/dev/null || true
        done
        echo "    [✓] PostgreSQL 库文件已复制"
        
        # 复制 share 目录
        if [ -d "$PG_SHARE_DIR" ]; then
            cp -r "$PG_SHARE_DIR" "$DEST/postgres/share"
            echo "    [✓] PostgreSQL share 目录已复制"
        fi
        PG_PACKED=true
    fi
fi

if [ "$PG_PACKED" = false ]; then
    echo "    [!] 警告：未找到 PostgreSQL，部署时需要手动安装"
fi

# ---- 复制必要的文档（只保留用户手册 PDF）----
mkdir -p "$DEST/docs"
for pdf in "系统使用手册.pdf" "VPN使用手册.pdf" "SSO使用手册.pdf" "功能文档.pdf" "API接口文档.pdf"; do
    [ -f "$PROJECT_DIR/docs/$pdf" ] && cp "$PROJECT_DIR/docs/$pdf" "$DEST/docs/"
done
echo "  - 用户文档已复制（仅PDF）"

# ---- 复制 README ----
cp -f "$PROJECT_DIR/README.md" "$DEST/" 2>/dev/null || true
cp -f "$PROJECT_DIR/快速部署说明.txt" "$DEST/" 2>/dev/null || true

# ========== 步骤 5：清理敏感数据 ==========
echo "[5/7] 清理敏感/无用数据..."

# ---- 删除所有源码相关文件（后端已是精简复制，但double check）----
find "$DEST" -name "*.go" -delete 2>/dev/null || true
rm -f "$DEST/backend/go.mod" "$DEST/backend/go.sum" 2>/dev/null || true
echo "  - Go 源码已清除"

# ---- 删除所有日志文件 ----
find "$DEST" -name "*.log" -delete 2>/dev/null || true
echo "  - 日志文件已清除"

# ---- 删除证书和密钥 ----
find "$DEST" \( -name "jwt_secret" -o -name "*.pem" -o -name "*.key" -o -name "*.crt" \) -delete 2>/dev/null || true
echo "  - 证书和密钥已清除"

# ---- 删除 SQLite 数据库（VPN 数据库也清除，首次运行会自动创建）----
find "$DEST" \( -name "*.db" -o -name "*.sqlite" -o -name "*.sqlite3" \) -delete 2>/dev/null || true
echo "  - 数据库文件已清除"

# ---- 删除旧编译产物名称 ----
rm -f "$DEST/backend/server" "$DEST/backend/bi-dashboard" 2>/dev/null || true

# ---- 删除 .git ----
rm -rf "$DEST/.git" 2>/dev/null || true

# ---- 删除旧打包脚本（保留 pack.sh 用于二次打包）----
rm -f "$DEST/scripts/pack-full.sh" 2>/dev/null || true

echo "  [OK] 清理完成"

# ========== 步骤 6：确保脚本可执行 ==========
echo "[6/7] 设置文件权限..."
chmod +x "$DEST/scripts/"*.sh
chmod +x "$DEST/backend/go-syncflow"

# ========== 步骤 7：打包 ==========
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
echo "  [✓] 后端预编译二进制（go-syncflow，已strip）"
echo "  [✓] 前端静态资源（backend/static/）"
echo "  [✓] VPN 客户端安装包（backend/data/vpn/）"
echo "  [✓] PostgreSQL 数据库依赖（postgres/）"
echo "  [✓] 部署脚本（scripts/）"
echo "  [✓] 用户文档 PDF（docs/）"
echo ""
echo "已排除内容："
echo "  [×] Go/Node.js 源代码"
echo "  [×] 开发文档、模板文件"
echo "  [×] 数据库数据、日志文件"
echo "  [×] 证书、密钥、敏感配置"
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
echo "  - 首次启动自动初始化 PostgreSQL 和应用数据"
echo ""
