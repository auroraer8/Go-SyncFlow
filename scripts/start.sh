#!/bin/bash
# Go-SyncFlow 统一身份同步与管理平台 - 一键启动脚本
# 用法: ./start.sh
# 内置 PostgreSQL 数据库，开箱即用

set -e

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
SERVICE_NAME="go-syncflow"
PG_DATA_DIR="$PROJECT_DIR/pgdata"
PG_LOG_FILE="$PROJECT_DIR/pgdata/postgresql.log"

echo "=========================================="
echo "  Go-SyncFlow 统一身份同步与管理平台"
echo "     一键启动 v3.0 (PostgreSQL 内置)"
echo "=========================================="

# 检查是否为root用户
if [ "$EUID" -ne 0 ]; then
    echo "[警告] 建议使用root用户运行以支持systemd服务"
fi

# 进入项目目录
cd "$PROJECT_DIR"

# ============================================================
# PostgreSQL 专用用户
# ============================================================
PG_USER="syncflow"

# ============================================================
# 初始化内置 PostgreSQL
# ============================================================
init_postgresql() {
    echo "[*] 初始化内置 PostgreSQL..."
    
    # 设置 PostgreSQL 路径
    if [ -d "$PROJECT_DIR/postgres/bin" ]; then
        # 使用内置 PostgreSQL
        PG_BIN="$PROJECT_DIR/postgres/bin"
        export LD_LIBRARY_PATH="$PROJECT_DIR/postgres/lib:$LD_LIBRARY_PATH"
        export PATH="$PG_BIN:$PATH"
        echo "  - 使用内置 PostgreSQL"
        USE_EMBEDDED_PG=true
        
        # PostgreSQL 二进制编译时硬编码了 /usr/share/postgresql/16 路径
        # 需要创建符号链接让它能找到我们打包的 share 目录
        if [ -d "$PROJECT_DIR/postgres/share" ] && [ ! -d "/usr/share/postgresql/16" ]; then
            echo "  - 创建 PostgreSQL share 目录链接..."
            mkdir -p /usr/share/postgresql
            ln -sf "$PROJECT_DIR/postgres/share" /usr/share/postgresql/16
        fi
    elif command -v pg_ctl &> /dev/null; then
        # 使用系统 PostgreSQL
        PG_BIN=$(dirname $(which pg_ctl))
        echo "  - 使用系统 PostgreSQL"
        USE_EMBEDDED_PG=false
    else
        echo "[X] 未找到 PostgreSQL，请确保内置或系统已安装"
        exit 1
    fi
    
    # 如果是 root 用户运行且使用内置 PostgreSQL，需要创建专用用户
    if [ "$(id -u)" = "0" ] && [ "$USE_EMBEDDED_PG" = "true" ]; then
        echo "  - 检查 PostgreSQL 运行用户..."
        if ! id "$PG_USER" &>/dev/null; then
            useradd -r -s /bin/false -d "$PROJECT_DIR" "$PG_USER" 2>/dev/null || true
            echo "    [✓] 创建用户 $PG_USER"
        fi
        # 设置目录权限
        chown -R "$PG_USER:$PG_USER" "$PROJECT_DIR/postgres" 2>/dev/null || true
        chown -R "$PG_USER:$PG_USER" "$PG_DATA_DIR" 2>/dev/null || true
        mkdir -p "$PG_DATA_DIR"
        chown "$PG_USER:$PG_USER" "$PG_DATA_DIR"
        RUN_AS_USER="su - $PG_USER -s /bin/bash -c"
    else
        RUN_AS_USER="bash -c"
    fi
    
    # 检查数据目录是否已初始化
    if [ ! -f "$PG_DATA_DIR/PG_VERSION" ]; then
        echo "  - 初始化数据库目录..."
        mkdir -p "$PG_DATA_DIR"
        
        if [ "$(id -u)" = "0" ] && [ "$USE_EMBEDDED_PG" = "true" ]; then
            chown "$PG_USER:$PG_USER" "$PG_DATA_DIR"
        fi
        
        # 初始化数据库
        echo "  - 执行 initdb..."
        
        # 设置 PostgreSQL 环境变量（内置 PostgreSQL 需要指定路径）
        PG_SHARE_DIR="$PROJECT_DIR/postgres/share"
        
        # 构建环境变量设置
        PG_ENV="export LD_LIBRARY_PATH='$PROJECT_DIR/postgres/lib:\$LD_LIBRARY_PATH'"
        if [ -d "$PG_SHARE_DIR" ]; then
            # 设置 PGSHAREDIR 让 postgres 二进制找到 share 目录
            PG_ENV="$PG_ENV && export PGSHAREDIR='$PG_SHARE_DIR'"
            INITDB_OPTS="-L '$PG_SHARE_DIR'"
        else
            INITDB_OPTS=""
        fi
        
        INIT_CMD="$PG_ENV && '$PG_BIN/initdb' $INITDB_OPTS -D '$PG_DATA_DIR' -U postgres -E UTF8 --locale=C"
        
        if $RUN_AS_USER "$INIT_CMD" 2>&1 | tail -5; then
            echo "  [✓] initdb 完成"
        else
            echo "  [X] initdb 失败，尝试使用系统 locale..."
            INIT_CMD="$PG_ENV && '$PG_BIN/initdb' $INITDB_OPTS -D '$PG_DATA_DIR' -U postgres -E UTF8"
            if $RUN_AS_USER "$INIT_CMD" 2>&1 | tail -5; then
                echo "  [✓] initdb 完成（使用系统 locale）"
            else
                echo "  [X] PostgreSQL 初始化失败"
                exit 1
            fi
        fi
        
        # 配置 PostgreSQL
        cat >> "$PG_DATA_DIR/postgresql.conf" << PGCONF
# Go-SyncFlow 配置
listen_addresses = '127.0.0.1'
port = 5432
max_connections = 200
shared_buffers = 128MB
log_destination = 'stderr'
logging_collector = on
log_directory = '.'
log_filename = 'postgresql.log'
unix_socket_directories = '$PG_DATA_DIR'
PGCONF
        
        # 配置访问权限
        cat > "$PG_DATA_DIR/pg_hba.conf" << 'PGHBA'
# TYPE  DATABASE        USER            ADDRESS                 METHOD
local   all             all                                     trust
host    all             all             127.0.0.1/32            md5
host    all             all             ::1/128                 md5
PGHBA
        
        if [ "$(id -u)" = "0" ] && [ "$USE_EMBEDDED_PG" = "true" ]; then
            chown -R "$PG_USER:$PG_USER" "$PG_DATA_DIR"
        fi
        
        echo "  [✓] PostgreSQL 数据目录已初始化"
    fi
    
    # PostgreSQL 环境变量设置
    PG_SHARE_DIR="$PROJECT_DIR/postgres/share"
    if [ -d "$PG_SHARE_DIR" ]; then
        PG_ENV_BASE="export LD_LIBRARY_PATH='$PROJECT_DIR/postgres/lib:\$LD_LIBRARY_PATH' && export PGSHAREDIR='$PG_SHARE_DIR'"
    else
        PG_ENV_BASE="export LD_LIBRARY_PATH='$PROJECT_DIR/postgres/lib:\$LD_LIBRARY_PATH'"
    fi
    
    # 检查 PostgreSQL 是否已运行
    PG_STATUS_CMD="$PG_ENV_BASE && '$PG_BIN/pg_ctl' -D '$PG_DATA_DIR' status"
    PG_START_CMD="$PG_ENV_BASE && '$PG_BIN/pg_ctl' -D '$PG_DATA_DIR' -l '$PG_LOG_FILE' start"
    PG_STOP_CMD="$PG_ENV_BASE && '$PG_BIN/pg_ctl' -D '$PG_DATA_DIR' stop -m fast"
    
    INIT_PG_FOR_SETUP="false"
    
    # 如果已经在运行（由 systemd 启动），直接返回
    if $RUN_AS_USER "$PG_STATUS_CMD" > /dev/null 2>&1; then
        echo "  [✓] PostgreSQL 已在运行"
        return 0
    fi
    
    # 标记为脚本启动（需要在后面停止让 systemd 接管）
    INIT_PG_FOR_SETUP="true"
    
    # 启动 PostgreSQL 进行初始化
    echo "  - 启动 PostgreSQL..."
    $RUN_AS_USER "$PG_START_CMD" > /dev/null 2>&1
    sleep 2
    
    if ! $RUN_AS_USER "$PG_STATUS_CMD" > /dev/null 2>&1; then
        echo "  [X] PostgreSQL 启动失败，查看日志: $PG_LOG_FILE"
        cat "$PG_LOG_FILE" 2>/dev/null | tail -20
        exit 1
    fi
    echo "  [✓] PostgreSQL 启动成功"
    
    # 创建用户和数据库（如果不存在）
    echo "  - 检查数据库和用户..."
    
    # 检查用户是否存在
    PSQL_CMD="$PG_ENV_BASE && '$PG_BIN/psql' -h '$PG_DATA_DIR' -U postgres"
    if ! $RUN_AS_USER "$PSQL_CMD -tAc \"SELECT 1 FROM pg_roles WHERE rolname='syncflow'\"" 2>/dev/null | grep -q 1; then
        $RUN_AS_USER "$PSQL_CMD -c \"CREATE USER syncflow WITH PASSWORD 'syncflow';\"" > /dev/null 2>&1
        echo "    [✓] 创建用户 syncflow"
    fi
    
    # 检查主数据库是否存在
    if ! $RUN_AS_USER "$PSQL_CMD -tAc \"SELECT 1 FROM pg_database WHERE datname='go_syncflow'\"" 2>/dev/null | grep -q 1; then
        $RUN_AS_USER "$PSQL_CMD -c \"CREATE DATABASE go_syncflow OWNER syncflow;\"" > /dev/null 2>&1
        echo "    [✓] 创建数据库 go_syncflow"
    fi
    
    # 检查 VPN 数据库是否存在
    if ! $RUN_AS_USER "$PSQL_CMD -tAc \"SELECT 1 FROM pg_database WHERE datname='go_syncflow_vpn'\"" 2>/dev/null | grep -q 1; then
        $RUN_AS_USER "$PSQL_CMD -c \"CREATE DATABASE go_syncflow_vpn OWNER syncflow;\"" > /dev/null 2>&1
        echo "    [✓] 创建数据库 go_syncflow_vpn"
    fi
    
    # 授予权限
    $RUN_AS_USER "$PSQL_CMD -c \"GRANT ALL PRIVILEGES ON DATABASE go_syncflow TO syncflow;\"" > /dev/null 2>&1
    $RUN_AS_USER "$PSQL_CMD -c \"GRANT ALL PRIVILEGES ON DATABASE go_syncflow_vpn TO syncflow;\"" > /dev/null 2>&1
    
    # 如果脚本启动的 PostgreSQL，停止它让 systemd 管理
    if [ "$INIT_PG_FOR_SETUP" = "true" ]; then
        echo "  - 停止临时 PostgreSQL（systemd 将重新启动）..."
        $RUN_AS_USER "$PG_STOP_CMD" > /dev/null 2>&1 || true
        sleep 1
    fi
    
    echo "  [✓] PostgreSQL 初始化完成"
}

# ============================================================
# 判断是否已有预编译产物
# ============================================================
HAS_BINARY=false
BINARY_PATH=""
if [ -f "$PROJECT_DIR/backend/go-syncflow" ]; then
    HAS_BINARY=true
    BINARY_PATH="$PROJECT_DIR/backend/go-syncflow"
elif [ -f "$PROJECT_DIR/backend/server" ]; then
    HAS_BINARY=true
    BINARY_PATH="$PROJECT_DIR/backend/server"
fi

HAS_STATIC=false
if [ -d "$PROJECT_DIR/backend/static" ] && [ -f "$PROJECT_DIR/backend/static/index.html" ]; then
    HAS_STATIC=true
fi

NEED_BUILD=false
if [ "$HAS_BINARY" = false ] || [ "$HAS_STATIC" = false ]; then
    NEED_BUILD=true
fi

# ============================================================
# 仅在需要编译时才检查/安装编译依赖
# ============================================================
if [ "$NEED_BUILD" = true ]; then
    echo "[*] 未检测到预编译产物，需要编译..."
    echo ""

    # ---------- 检查 Go ----------
    if [ "$HAS_BINARY" = false ]; then
        if command -v go &> /dev/null; then
            echo "[OK] Go已安装: $(go version)"
        elif [ -f "$PROJECT_DIR/tooling/go1.22.6.linux-amd64.tar.gz" ]; then
            echo "[*] 正在从tooling安装Go环境..."
            tar -C /usr/local -xzf "$PROJECT_DIR/tooling/go1.22.6.linux-amd64.tar.gz"
            export PATH=$PATH:/usr/local/go/bin
            echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
            echo "[OK] Go安装完成: $(go version)"
        else
            echo "[X] 未找到Go环境且无离线安装包，请先安装Go 1.22+"
            exit 1
        fi

        echo "[*] 编译后端..."
        cd "$PROJECT_DIR/backend"
        go mod download 2>/dev/null || true
        CGO_ENABLED=1 go build -o go-syncflow .
        BINARY_PATH="$PROJECT_DIR/backend/go-syncflow"
        echo "[OK] 后端编译完成"
        cd "$PROJECT_DIR"
    fi

    # ---------- 检查 Node.js ----------
    if [ "$HAS_STATIC" = false ]; then
        if command -v node &> /dev/null; then
            echo "[OK] Node.js已安装: $(node --version)"
        elif [ -f "$PROJECT_DIR/tooling/node-v18.20.2-linux-x64.tar.xz" ]; then
            echo "[*] 正在从tooling安装Node.js..."
            tar -xJf "$PROJECT_DIR/tooling/node-v18.20.2-linux-x64.tar.xz" -C /usr/local --strip-components=1
            echo "[OK] Node.js安装完成: $(node --version)"
        else
            echo "[X] 未找到Node.js环境，请先安装Node.js 18+"
            exit 1
        fi

        echo "[*] 构建前端..."
        cd "$PROJECT_DIR/frontend"
        if [ ! -d "node_modules" ]; then
            echo "[*] 安装前端依赖..."
            npm install --prefer-offline
        fi
        npm run build
        rm -rf "$PROJECT_DIR/backend/static"
        cp -r dist "$PROJECT_DIR/backend/static"
        echo "[OK] 前端构建完成"
        cd "$PROJECT_DIR"
    fi
else
    echo ""
    echo "[OK] 检测到预编译二进制: $(basename $BINARY_PATH)"
    echo "[OK] 检测到前端静态文件: backend/static/"
    echo ""
fi

# ============================================================
# 初始化 PostgreSQL
# ============================================================
init_postgresql

# ============================================================
# 设置时区
# ============================================================
timedatectl set-timezone Asia/Shanghai 2>/dev/null || ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime 2>/dev/null || true
echo "[OK] 时区: $(date '+%Y-%m-%d %H:%M:%S %Z')"

# ============================================================
# 初始化运行时目录
# ============================================================
echo "[*] 初始化运行环境..."
mkdir -p "$PROJECT_DIR/backend/data"
mkdir -p "$PROJECT_DIR/backend/certs"
echo "[OK] 数据目录就绪"

# ============================================================
# 配置 systemd 服务
# ============================================================
echo "[*] 配置系统服务..."

# 确定最终二进制路径
if [ -f "$PROJECT_DIR/backend/go-syncflow" ]; then
    EXEC_PATH="$PROJECT_DIR/backend/go-syncflow"
else
    EXEC_PATH="$PROJECT_DIR/backend/server"
fi

# 兼容：如果旧服务存在则停止并移除
if systemctl is-active --quiet bi-dashboard 2>/dev/null; then
    systemctl stop bi-dashboard 2>/dev/null || true
    systemctl disable bi-dashboard 2>/dev/null || true
    rm -f /etc/systemd/system/bi-dashboard.service
fi

# PostgreSQL 服务（使用 syncflow 用户运行）
cat > /etc/systemd/system/go-syncflow-pg.service << EOF
[Unit]
Description=Go-SyncFlow PostgreSQL
After=network.target

[Service]
Type=forking
User=$PG_USER
Group=$PG_USER
ExecStart=$PROJECT_DIR/postgres/bin/pg_ctl -D $PG_DATA_DIR -l $PG_LOG_FILE start
ExecStop=$PROJECT_DIR/postgres/bin/pg_ctl -D $PG_DATA_DIR stop
ExecReload=$PROJECT_DIR/postgres/bin/pg_ctl -D $PG_DATA_DIR reload
Environment=LD_LIBRARY_PATH=$PROJECT_DIR/postgres/lib
Environment=PGSHAREDIR=$PROJECT_DIR/postgres/share
PIDFile=$PG_DATA_DIR/postmaster.pid
Restart=on-failure
TimeoutStartSec=30

[Install]
WantedBy=multi-user.target
EOF

# 主应用服务
cat > /etc/systemd/system/${SERVICE_NAME}.service << EOF
[Unit]
Description=Go-SyncFlow Unified Identity Sync Platform
After=network.target go-syncflow-pg.service
Requires=go-syncflow-pg.service

[Service]
Type=simple
User=root
WorkingDirectory=$PROJECT_DIR/backend
ExecStart=$EXEC_PATH
Restart=always
RestartSec=5
Environment=GIN_MODE=release
Environment=DB_HOST=127.0.0.1
Environment=DB_PORT=5432
Environment=DB_USER=syncflow
Environment=DB_PASSWORD=syncflow
Environment=DB_DATABASE=go_syncflow
StandardOutput=journal
StandardError=journal
SyslogIdentifier=go-syncflow
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable go-syncflow-pg 2>/dev/null || true
systemctl enable ${SERVICE_NAME} 2>/dev/null
echo "[OK] 系统服务配置完成"

# ============================================================
# 启动服务
# ============================================================
echo "[*] 启动服务..."

if systemctl is-active --quiet ${SERVICE_NAME}; then
    echo "[*] 服务已在运行，重启中..."
    systemctl restart ${SERVICE_NAME}
else
    systemctl start ${SERVICE_NAME}
fi

sleep 3

if systemctl is-active --quiet ${SERVICE_NAME}; then
    echo "[OK] 服务启动成功"
else
    echo "[X] 服务启动失败"
    echo "    查看日志: journalctl -u ${SERVICE_NAME} --no-pager -n 20"
    journalctl -u ${SERVICE_NAME} --no-pager -n 10
    exit 1
fi

# ============================================================
# 显示访问信息
# ============================================================
LOCAL_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
[ -z "$LOCAL_IP" ] && LOCAL_IP="127.0.0.1"

echo ""
echo "=========================================="
echo "  部署完成！"
echo "=========================================="
echo ""
echo "  访问地址:"
echo "    HTTP:  http://${LOCAL_IP}:8080"
echo "    HTTPS: https://${LOCAL_IP}:8443"
echo ""
echo "  数据库: PostgreSQL (内置)"
echo ""
echo "  默认管理员账号:"
echo "    用户名: admin"
echo "    密码:   Admin@2024"
echo ""
echo "  常用命令:"
echo "    查看状态: systemctl status ${SERVICE_NAME}"
echo "    查看日志: journalctl -u ${SERVICE_NAME} -f"
echo "    停止服务: $SCRIPT_DIR/stop.sh"
echo "    重启服务: $SCRIPT_DIR/restart.sh"
echo "    重置密码: $SCRIPT_DIR/reset-admin.sh"
