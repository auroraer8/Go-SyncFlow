#!/bin/bash
# go-SyncFlow 统一用户管理平台 - 重置管理员密码脚本
# 用法: ./reset-admin.sh [新密码]
# 如不指定新密码，默认重置为 Admin@2024

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
DB_PATH="$PROJECT_DIR/backend/data/app.db"
NEW_PASSWORD="${1:-Admin@2024}"

echo "=========================================="
echo "    go-SyncFlow - 重置管理员密码"
echo "=========================================="

# 检查数据库文件
if [ ! -f "$DB_PATH" ]; then
    echo "[✗] 数据库文件不存在: $DB_PATH"
    echo "    请确认服务已至少启动过一次"
    exit 1
fi

# 检查sqlite3是否可用
if ! command -v sqlite3 &> /dev/null; then
    echo "[*] 安装 sqlite3..."
    if command -v apt &> /dev/null; then
        apt-get install -y sqlite3
    elif command -v yum &> /dev/null; then
        yum install -y sqlite
    else
        echo "[✗] 无法自动安装 sqlite3，请手动安装"
        exit 1
    fi
fi

# 使用Go生成bcrypt密码哈希，或者使用Python
generate_hash() {
    # 优先尝试使用Python3
    if command -v python3 &> /dev/null; then
        python3 -c "
import subprocess, sys
try:
    import bcrypt
    h = bcrypt.hashpw('$NEW_PASSWORD'.encode(), bcrypt.gensalt()).decode()
    print(h)
except ImportError:
    # 使用 passlib 作为备选
    try:
        from passlib.hash import bcrypt as pb
        print(pb.hash('$NEW_PASSWORD'))
    except ImportError:
        sys.exit(1)
" 2>/dev/null
        return $?
    fi
    return 1
}

# 尝试通过Go程序重置
reset_via_go() {
    if command -v go &> /dev/null; then
        cd "$PROJECT_DIR/backend"
        # 创建临时重置程序
        cat > /tmp/reset_admin.go << 'GOEOF'
package main

import (
    "database/sql"
    "fmt"
    "os"

    "golang.org/x/crypto/bcrypt"
    _ "github.com/mattn/go-sqlite3"
)

func main() {
    if len(os.Args) < 3 {
        fmt.Println("Usage: reset_admin <db_path> <new_password>")
        os.Exit(1)
    }
    dbPath := os.Args[1]
    newPass := os.Args[2]

    hash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
    if err != nil {
        fmt.Printf("Error generating hash: %v\n", err)
        os.Exit(1)
    }

    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        fmt.Printf("Error opening database: %v\n", err)
        os.Exit(1)
    }
    defer db.Close()

    result, err := db.Exec("UPDATE users SET password = ?, status = 1 WHERE username = 'admin'", string(hash))
    if err != nil {
        fmt.Printf("Error updating password: %v\n", err)
        os.Exit(1)
    }
    rows, _ := result.RowsAffected()
    if rows == 0 {
        fmt.Println("Warning: admin user not found in database")
        os.Exit(1)
    }
    fmt.Println("SUCCESS")
}
GOEOF
        go run /tmp/reset_admin.go "$DB_PATH" "$NEW_PASSWORD" 2>/dev/null
        local result=$?
        rm -f /tmp/reset_admin.go
        return $result
    fi
    return 1
}

echo "[*] 正在重置管理员密码..."

if reset_via_go; then
    echo "[✓] 管理员密码已重置"
    echo ""
    echo "新的登录信息:"
    echo "  用户名: admin"
    echo "  密码:   $NEW_PASSWORD"
    echo ""
    echo "如果服务正在运行，新密码立即生效。"
else
    HASH=$(generate_hash)
    if [ -n "$HASH" ]; then
        sqlite3 "$DB_PATH" "UPDATE users SET password = '$HASH', status = 1 WHERE username = 'admin';"
        echo "[✓] 管理员密码已重置"
        echo ""
        echo "新的登录信息:"
        echo "  用户名: admin"
        echo "  密码:   $NEW_PASSWORD"
        echo ""
        echo "如果服务正在运行，新密码立即生效。"
    else
        echo "[✗] 无法生成密码哈希"
        echo "    请确保已安装 Go 或 Python3 (with bcrypt)"
        echo ""
        echo "手动重置方法:"
        echo "  1. 安装: pip3 install bcrypt"
        echo "  2. 重新运行此脚本"
        exit 1
    fi
fi
