#!/bin/bash
#
# Go-SyncFlow PostgreSQL 外部连接管理脚本
# 用于开启或关闭数据库的外部连接访问
#
# 使用方法:
#   ./enable-external-db.sh           - 开启外部连接（默认）
#   ./enable-external-db.sh enable    - 开启外部连接
#   ./enable-external-db.sh disable   - 关闭外部连接
#   ./enable-external-db.sh status    - 查看当前状态
#   ./enable-external-db.sh help      - 显示帮助
#
# 特点：无需 Docker，直接操作内置 PostgreSQL
#

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# PostgreSQL 配置
PG_DATA_DIR="$PROJECT_DIR/pgdata"
PG_BIN="$PROJECT_DIR/postgres/bin"
PG_CONF="$PG_DATA_DIR/postgresql.conf"
PG_HBA="$PG_DATA_DIR/pg_hba.conf"
PG_USER="syncflow"

# 数据库连接信息
DB_NAME="go_syncflow"
DB_USER="syncflow"
DB_PASSWORD="syncflow"
DB_PORT="5432"

# 设置环境变量
export LD_LIBRARY_PATH="$PROJECT_DIR/postgres/lib:$LD_LIBRARY_PATH"
export PGSHAREDIR="$PROJECT_DIR/postgres/share"

# 检查是否以 root 运行
check_root() {
    if [ "$EUID" -ne 0 ]; then
        echo -e "${RED}请以 root 权限运行此脚本${NC}"
        exit 1
    fi
}

# 检查 PostgreSQL 是否可用
check_postgresql() {
    if [ ! -d "$PG_DATA_DIR" ]; then
        echo -e "${RED}PostgreSQL 数据目录不存在: $PG_DATA_DIR${NC}"
        echo -e "请先运行 ${CYAN}./start.sh${NC} 初始化系统"
        exit 1
    fi
    
    if [ ! -f "$PG_CONF" ]; then
        echo -e "${RED}PostgreSQL 配置文件不存在: $PG_CONF${NC}"
        exit 1
    fi
    
    if [ ! -f "$PG_BIN/pg_ctl" ]; then
        echo -e "${RED}PostgreSQL 二进制文件不存在: $PG_BIN/pg_ctl${NC}"
        exit 1
    fi
}

# 检查 PostgreSQL 是否运行
check_running() {
    if su - "$PG_USER" -s /bin/bash -c "$PG_BIN/pg_ctl -D '$PG_DATA_DIR' status" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# 获取本机 IP
get_local_ip() {
    hostname -I 2>/dev/null | awk '{print $1}' || echo "服务器IP"
}

# 获取当前状态
get_status() {
    local listen_addr=$(grep "^listen_addresses" "$PG_CONF" 2>/dev/null | grep -oP "'\K[^']+")
    local hba_external=$(grep -c "0.0.0.0/0" "$PG_HBA" 2>/dev/null || echo "0")
    
    if [ "$listen_addr" = "*" ] && [ "$hba_external" -gt 0 ]; then
        echo "enabled"
    else
        echo "disabled"
    fi
}

# 显示状态
show_status() {
    local status=$(get_status)
    local local_ip=$(get_local_ip)
    local is_running="否"
    
    if check_running; then
        is_running="是"
    fi
    
    echo ""
    echo -e "${BLUE}╔═══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║          Go-SyncFlow PostgreSQL 外部连接状态                      ║${NC}"
    echo -e "${BLUE}╠═══════════════════════════════════════════════════════════════════╣${NC}"
    
    if [ "$status" = "enabled" ]; then
        echo -e "${BLUE}║${NC}  外部连接: ${GREEN}已开启${NC}                                                ${BLUE}║${NC}"
        echo -e "${BLUE}║${NC}  监听地址: 0.0.0.0:5432 (所有网络接口)                           ${BLUE}║${NC}"
    else
        echo -e "${BLUE}║${NC}  外部连接: ${YELLOW}已关闭${NC}                                                ${BLUE}║${NC}"
        echo -e "${BLUE}║${NC}  监听地址: 127.0.0.1:5432 (仅本地)                               ${BLUE}║${NC}"
    fi
    
    echo -e "${BLUE}║${NC}  运行状态: ${is_running}                                                    ${BLUE}║${NC}"
    echo -e "${BLUE}╠═══════════════════════════════════════════════════════════════════╣${NC}"
    echo -e "${BLUE}║${NC}  ${CYAN}数据库连接信息${NC}                                                  ${BLUE}║${NC}"
    echo -e "${BLUE}╠═══════════════════════════════════════════════════════════════════╣${NC}"
    
    if [ "$status" = "enabled" ]; then
        echo -e "${BLUE}║${NC}  主机地址: ${GREEN}${local_ip}${NC}                                         ${BLUE}║${NC}"
    else
        echo -e "${BLUE}║${NC}  主机地址: ${YELLOW}127.0.0.1${NC} (仅本地访问)                               ${BLUE}║${NC}"
    fi
    
    echo -e "${BLUE}║${NC}  端口:     ${GREEN}${DB_PORT}${NC}                                                ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  数据库:   ${GREEN}${DB_NAME}${NC}                                          ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  用户名:   ${GREEN}${DB_USER}${NC}                                             ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  密码:     ${GREEN}${DB_PASSWORD}${NC}                                             ${BLUE}║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════════════════╝${NC}"
    
    if [ "$status" = "enabled" ]; then
        echo ""
        echo -e "${CYAN}连接示例:${NC}"
        echo ""
        echo "  # psql 命令行"
        echo -e "  ${GREEN}PGPASSWORD=${DB_PASSWORD} psql -h ${local_ip} -p ${DB_PORT} -U ${DB_USER} -d ${DB_NAME}${NC}"
        echo ""
        echo "  # JDBC URL"
        echo -e "  ${GREEN}jdbc:postgresql://${local_ip}:${DB_PORT}/${DB_NAME}${NC}"
        echo ""
        echo "  # Python (psycopg2)"
        echo -e "  ${GREEN}postgresql://${DB_USER}:${DB_PASSWORD}@${local_ip}:${DB_PORT}/${DB_NAME}${NC}"
        echo ""
        echo -e "${YELLOW}⚠ 安全提示:${NC}"
        echo "  1. 请配置防火墙，仅允许可信 IP 访问 5432 端口"
        echo "  2. 建议修改数据库密码（当前使用默认密码）"
        echo "  3. 定期检查数据库连接日志"
        echo ""
    fi
}

# 开启外部连接
enable_external() {
    local current_status=$(get_status)
    
    if [ "$current_status" = "enabled" ]; then
        echo -e "${YELLOW}外部连接已经是开启状态${NC}"
        show_status
        return 0
    fi
    
    echo -e "${BLUE}正在开启外部连接...${NC}"
    
    # 备份配置文件
    cp "$PG_CONF" "$PG_CONF.bak.$(date +%Y%m%d%H%M%S)"
    cp "$PG_HBA" "$PG_HBA.bak.$(date +%Y%m%d%H%M%S)"
    
    # 修改 postgresql.conf - 监听所有地址
    if grep -q "^listen_addresses" "$PG_CONF"; then
        sed -i "s/^listen_addresses.*/listen_addresses = '*'/" "$PG_CONF"
    else
        echo "listen_addresses = '*'" >> "$PG_CONF"
    fi
    echo "  [✓] 配置监听地址: 0.0.0.0"
    
    # 修改 pg_hba.conf - 允许外部连接
    if ! grep -q "0.0.0.0/0" "$PG_HBA"; then
        echo "" >> "$PG_HBA"
        echo "# 允许外部连接 (由 enable-external-db.sh 添加)" >> "$PG_HBA"
        echo "host    all             all             0.0.0.0/0               md5" >> "$PG_HBA"
        echo "host    all             all             ::/0                    md5" >> "$PG_HBA"
    fi
    echo "  [✓] 配置访问权限: 允许所有 IP"
    
    # 设置文件权限
    chown "$PG_USER:$PG_USER" "$PG_CONF" "$PG_HBA"
    
    # 重启 PostgreSQL
    echo "  [*] 重启 PostgreSQL..."
    if check_running; then
        systemctl restart go-syncflow-pg 2>/dev/null || {
            su - "$PG_USER" -s /bin/bash -c "$PG_BIN/pg_ctl -D '$PG_DATA_DIR' restart" 2>/dev/null
        }
    else
        systemctl start go-syncflow-pg 2>/dev/null || {
            su - "$PG_USER" -s /bin/bash -c "$PG_BIN/pg_ctl -D '$PG_DATA_DIR' -l '$PG_DATA_DIR/postgresql.log' start" 2>/dev/null
        }
    fi
    
    sleep 2
    
    if check_running; then
        echo -e "  ${GREEN}[✓] PostgreSQL 重启成功${NC}"
    else
        echo -e "  ${RED}[X] PostgreSQL 重启失败，请检查日志${NC}"
        exit 1
    fi
    
    echo ""
    echo -e "${GREEN}✓ 外部连接已开启${NC}"
    show_status
}

# 关闭外部连接
disable_external() {
    local current_status=$(get_status)
    
    if [ "$current_status" = "disabled" ]; then
        echo -e "${YELLOW}外部连接已经是关闭状态${NC}"
        show_status
        return 0
    fi
    
    echo -e "${BLUE}正在关闭外部连接...${NC}"
    
    # 修改 postgresql.conf - 仅监听本地
    sed -i "s/^listen_addresses.*/listen_addresses = '127.0.0.1'/" "$PG_CONF"
    echo "  [✓] 配置监听地址: 127.0.0.1"
    
    # 修改 pg_hba.conf - 移除外部连接规则
    sed -i '/# 允许外部连接/d' "$PG_HBA"
    sed -i '\|0.0.0.0/0|d' "$PG_HBA"
    sed -i '\|::/0|d' "$PG_HBA"
    echo "  [✓] 移除外部访问权限"
    
    # 设置文件权限
    chown "$PG_USER:$PG_USER" "$PG_CONF" "$PG_HBA"
    
    # 重启 PostgreSQL
    echo "  [*] 重启 PostgreSQL..."
    if check_running; then
        systemctl restart go-syncflow-pg 2>/dev/null || {
            su - "$PG_USER" -s /bin/bash -c "$PG_BIN/pg_ctl -D '$PG_DATA_DIR' restart" 2>/dev/null
        }
    fi
    
    sleep 2
    
    if check_running; then
        echo -e "  ${GREEN}[✓] PostgreSQL 重启成功${NC}"
    else
        echo -e "  ${RED}[X] PostgreSQL 重启失败，请检查日志${NC}"
        exit 1
    fi
    
    echo ""
    echo -e "${GREEN}✓ 外部连接已关闭${NC}"
    show_status
}

# 显示帮助
show_help() {
    echo ""
    echo -e "${BLUE}Go-SyncFlow PostgreSQL 外部连接管理脚本${NC}"
    echo ""
    echo "使用方法:"
    echo "  $0 [命令]"
    echo ""
    echo "命令:"
    echo "  enable     开启外部连接（允许远程访问数据库）"
    echo "  disable    关闭外部连接（仅允许本地访问）"
    echo "  status     查看当前外部连接状态和连接信息"
    echo "  help       显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0              # 进入交互式菜单"
    echo "  $0 enable       # 开启后可使用 Navicat/DBeaver 等工具连接"
    echo "  $0 status       # 查看连接信息和账号密码"
    echo "  $0 disable      # 关闭远程访问（安全）"
    echo ""
    echo -e "${YELLOW}安全建议:${NC}"
    echo "  开启外部连接后，请务必配置防火墙规则限制访问来源:"
    echo "  - ufw allow from <可信IP> to any port 5432"
    echo "  - firewall-cmd --add-rich-rule='rule family=ipv4 source address=<可信IP> port port=5432 protocol=tcp accept'"
    echo ""
}

# 交互式菜单（无参数时显示）
interactive_menu() {
    check_postgresql
    local status=$(get_status)

    echo ""
    echo -e "${BLUE}╔═══════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║    Go-SyncFlow PostgreSQL 外部连接管理            ║${NC}"
    echo -e "${BLUE}╠═══════════════════════════════════════════════════╣${NC}"
    if [ "$status" = "enabled" ]; then
        echo -e "${BLUE}║${NC}  当前状态: ${GREEN}外部连接已开启${NC}                       ${BLUE}║${NC}"
    else
        echo -e "${BLUE}║${NC}  当前状态: ${YELLOW}仅本机访问${NC}                           ${BLUE}║${NC}"
    fi
    echo -e "${BLUE}╠═══════════════════════════════════════════════════╣${NC}"
    echo -e "${BLUE}║${NC}  ${CYAN}1${NC}) 开启外部连接（允许远程访问数据库）          ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  ${CYAN}2${NC}) 关闭外部连接（仅本机可访问，更安全）        ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  ${CYAN}3${NC}) 查看连接信息                                ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  ${CYAN}0${NC}) 退出                                        ${BLUE}║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════╝${NC}"
    echo ""
    read -p "请选择 [0-3]: " choice

    case "$choice" in
        1)
            check_root
            enable_external
            ;;
        2)
            check_root
            disable_external
            ;;
        3)
            show_status
            ;;
        0)
            echo "已退出"
            ;;
        *)
            echo -e "${RED}无效选择${NC}"
            exit 1
            ;;
    esac
}

# 主函数
main() {
    case "${1:-}" in
        enable)
            check_root
            check_postgresql
            enable_external
            ;;
        disable)
            check_root
            check_postgresql
            disable_external
            ;;
        status)
            check_postgresql
            show_status
            ;;
        help|--help|-h)
            show_help
            ;;
        "")
            interactive_menu
            ;;
        *)
            echo -e "${RED}未知命令: $1${NC}"
            show_help
            exit 1
            ;;
    esac
}

main "$@"
