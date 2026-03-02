package services

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"go-syncflow/internal/models"

	"github.com/go-ldap/ldap/v3"
	"golang.org/x/crypto/bcrypt"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"

	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/sijms/go-ora/v2"
)

// AuthenticateWithConnector 统一认证入口
// 根据连接器类型调用对应的认证方法
func AuthenticateWithConnector(conn *models.Connector, username, password, matchField string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("连接器为空")
	}

	switch {
	case conn.IsLDAP():
		return authenticateWithLDAP(conn, username, password, matchField)
	case conn.IsDatabase():
		return authenticateWithDatabase(conn, username, password, matchField)
	case conn.IsRADIUS():
		return authenticateWithRADIUS(conn, username, password)
	case conn.IsHTTPAPI():
		return authenticateWithHTTPAPI(conn, username, password)
	default:
		return false, fmt.Errorf("不支持的连接器类型: %s", conn.Type)
	}
}

// authenticateWithLDAP 使用 LDAP 连接器进行认证
func authenticateWithLDAP(conn *models.Connector, username, password, matchField string) (bool, error) {
	// 连接 LDAP
	l, err := dialLDAPForAuth(conn)
	if err != nil {
		return false, fmt.Errorf("LDAP 连接失败: %v", err)
	}
	defer l.Close()

	// 管理员绑定
	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		return false, fmt.Errorf("LDAP 绑定失败: %v", err)
	}

	// 确定搜索属性
	searchAttr := conn.LDAPLoginAttr
	if searchAttr == "" {
		if conn.Type == "ldap_openldap" {
			searchAttr = "uid"
		} else {
			searchAttr = "sAMAccountName"
		}
	}

	objectClass := conn.LDAPUserObjectClass
	if objectClass == "" {
		if conn.Type == "ldap_openldap" {
			objectClass = "inetOrgPerson"
		} else {
			objectClass = "user"
		}
	}

	// 根据匹配字段确定搜索值和属性
	searchValue := username
	switch matchField {
	case "email":
		searchAttr = conn.LDAPEmailAttr
		if searchAttr == "" {
			searchAttr = "mail"
		}
	case "phone":
		searchAttr = conn.LDAPMobileAttr
		if searchAttr == "" {
			searchAttr = "mobile"
		}
	}

	if searchValue == "" {
		return false, fmt.Errorf("搜索值为空")
	}

	// 搜索用户
	filter := fmt.Sprintf("(&(objectClass=%s)(%s=%s))", objectClass, searchAttr, ldap.EscapeFilter(searchValue))
	sr, err := l.Search(ldap.NewSearchRequest(
		conn.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 10, false,
		filter, []string{"dn"}, nil,
	))
	if err != nil {
		return false, fmt.Errorf("LDAP 搜索失败: %v", err)
	}

	if len(sr.Entries) == 0 {
		return false, fmt.Errorf("用户在 LDAP 中不存在")
	}
	if len(sr.Entries) > 1 {
		return false, fmt.Errorf("在 LDAP 中找到多个匹配用户")
	}

	userDN := sr.Entries[0].DN

	// 使用用户 DN + 密码验证
	if err := l.Bind(userDN, password); err != nil {
		return false, fmt.Errorf("LDAP 密码验证失败")
	}

	return true, nil
}

// dialLDAPForAuth 连接 LDAP 服务器
func dialLDAPForAuth(conn *models.Connector) (*ldap.Conn, error) {
	address := fmt.Sprintf("%s:%d", conn.Host, conn.Port)

	if conn.UseTLS {
		return ldap.DialURL(fmt.Sprintf("ldaps://%s", address), ldap.DialWithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	}
	return ldap.DialURL(fmt.Sprintf("ldap://%s", address))
}

// authenticateWithDatabase 使用数据库连接器进行认证
func authenticateWithDatabase(conn *models.Connector, username, password, matchField string) (bool, error) {
	db, err := dialDBForAuth(conn)
	if err != nil {
		return false, fmt.Errorf("数据库连接失败: %v", err)
	}
	defer db.Close()

	if conn.UserTable == "" {
		return false, fmt.Errorf("未配置用户表名")
	}

	// 确定查询字段
	queryField := "username"
	switch matchField {
	case "email":
		queryField = "email"
	case "phone":
		queryField = "phone"
	}

	// 确定密码字段
	pwdField := "password"

	// 查询用户密码
	dbType := conn.EffectiveDBType()
	var query string
	switch dbType {
	case "postgresql":
		query = fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1 LIMIT 1", pwdField, conn.UserTable, queryField)
	case "oracle":
		query = fmt.Sprintf("SELECT %s FROM %s WHERE %s = :1 AND ROWNUM <= 1", pwdField, conn.UserTable, queryField)
	case "sqlserver":
		query = fmt.Sprintf("SELECT TOP 1 %s FROM %s WHERE %s = @p1", pwdField, conn.UserTable, queryField)
	default: // mysql
		query = fmt.Sprintf("SELECT %s FROM %s WHERE %s = ? LIMIT 1", pwdField, conn.UserTable, queryField)
	}

	var storedPassword string
	err = db.QueryRow(query, username).Scan(&storedPassword)
	if err == sql.ErrNoRows {
		return false, fmt.Errorf("用户不存在")
	}
	if err != nil {
		return false, fmt.Errorf("查询失败: %v", err)
	}

	// 根据密码格式验证
	return verifyDatabasePassword(password, storedPassword, conn.PwdFormat)
}

// verifyDatabasePassword 验证数据库密码
func verifyDatabasePassword(inputPassword, storedPassword, pwdFormat string) (bool, error) {
	switch pwdFormat {
	case "plain":
		return inputPassword == storedPassword, nil
	case "md5":
		hash := md5.Sum([]byte(inputPassword))
		return hex.EncodeToString(hash[:]) == strings.ToLower(storedPassword), nil
	case "sha256":
		hash := sha256.Sum256([]byte(inputPassword))
		return hex.EncodeToString(hash[:]) == strings.ToLower(storedPassword), nil
	case "bcrypt":
		err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(inputPassword))
		return err == nil, nil
	default:
		// 默认尝试 bcrypt
		err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(inputPassword))
		if err == nil {
			return true, nil
		}
		// 回退到明文对比
		return inputPassword == storedPassword, nil
	}
}

// dialDBForAuth 连接数据库
func dialDBForAuth(conn *models.Connector) (*sql.DB, error) {
	dbType := conn.EffectiveDBType()
	var dsn string

	switch dbType {
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true",
			conn.DBUser, conn.DBPassword, conn.Host, conn.Port, conn.Database, conn.Charset)
		return sql.Open("mysql", dsn)

	case "postgresql":
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			conn.Host, conn.Port, conn.DBUser, conn.DBPassword, conn.Database)
		return sql.Open("postgres", dsn)

	case "sqlserver":
		dsn = fmt.Sprintf("server=%s;port=%d;database=%s;user id=%s;password=%s",
			conn.Host, conn.Port, conn.Database, conn.DBUser, conn.DBPassword)
		return sql.Open("sqlserver", dsn)

	case "oracle":
		dsn = fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
			conn.DBUser, conn.DBPassword, conn.Host, conn.Port, conn.ServiceName)
		return sql.Open("oracle", dsn)

	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
}

// authenticateWithRADIUS 使用 RADIUS 连接器进行认证
func authenticateWithRADIUS(conn *models.Connector, username, password string) (bool, error) {
	if conn.RadiusServer == "" {
		return false, fmt.Errorf("RADIUS 服务器地址未配置")
	}

	port := conn.RadiusPort
	if port == 0 {
		port = 1812
	}

	timeout := conn.RadiusTimeout
	if timeout == 0 {
		timeout = 5
	}

	// 创建 RADIUS 请求
	packet := radius.New(radius.CodeAccessRequest, []byte(conn.RadiusSecret))
	if err := rfc2865.UserName_SetString(packet, username); err != nil {
		return false, fmt.Errorf("设置用户名失败: %v", err)
	}
	if err := rfc2865.UserPassword_SetString(packet, password); err != nil {
		return false, fmt.Errorf("设置密码失败: %v", err)
	}

	// 设置 NAS-IP-Address
	if conn.RadiusNasIP != "" {
		ip := net.ParseIP(conn.RadiusNasIP)
		if ip != nil {
			rfc2865.NASIPAddress_Set(packet, ip)
		}
	}

	// 发送请求
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	serverAddr := fmt.Sprintf("%s:%d", conn.RadiusServer, port)
	response, err := radius.Exchange(ctx, packet, serverAddr)
	if err != nil {
		return false, fmt.Errorf("RADIUS 请求失败: %v", err)
	}

	return response.Code == radius.CodeAccessAccept, nil
}

// authenticateWithHTTPAPI 使用 HTTP API 连接器进行认证
func authenticateWithHTTPAPI(conn *models.Connector, username, password string) (bool, error) {
	if conn.HTTPURL == "" {
		return false, fmt.Errorf("HTTP 认证 URL 未配置")
	}

	// 构造请求体，替换变量
	body := conn.HTTPBodyTemplate
	body = strings.ReplaceAll(body, "{USERNAME}", username)
	body = strings.ReplaceAll(body, "{PASSWORD}", password)
	body = strings.ReplaceAll(body, "{MD5PASSWORD}", md5Hash(password))
	body = strings.ReplaceAll(body, "{SHA256PASSWORD}", sha256Hash(password))

	// 根据加密方式额外处理
	if conn.HTTPEncryptMethod == "md5" && conn.HTTPEncryptKey != "" {
		body = strings.ReplaceAll(body, "{ENCRYPTED}", md5Hash(password+conn.HTTPEncryptKey))
	} else if conn.HTTPEncryptMethod == "sha256" && conn.HTTPEncryptKey != "" {
		body = strings.ReplaceAll(body, "{ENCRYPTED}", sha256Hash(password+conn.HTTPEncryptKey))
	}

	// 创建 HTTP 请求
	method := conn.HTTPMethod
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, conn.HTTPURL, strings.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置 Content-Type
	contentType := conn.HTTPContentType
	if contentType == "" {
		contentType = "application/json"
	}
	req.Header.Set("Content-Type", contentType)

	// 添加自定义请求头
	if conn.HTTPHeaders != "" {
		// 尝试解析为 map[string]string 格式
		var headersMap map[string]string
		if err := json.Unmarshal([]byte(conn.HTTPHeaders), &headersMap); err == nil {
			for k, v := range headersMap {
				req.Header.Set(k, v)
			}
		} else {
			// 尝试解析为数组格式 [{"key":"X-App-ID","value":"xxx"}]
			var headersArray []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}
			if err := json.Unmarshal([]byte(conn.HTTPHeaders), &headersArray); err == nil {
				for _, h := range headersArray {
					if h.Key != "" {
						req.Header.Set(h.Key, h.Value)
					}
				}
			}
		}
	}

	// TLS 配置
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !conn.HTTPVerifyCert,
		},
	}

	timeout := conn.Timeout
	if timeout == 0 {
		timeout = 10
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeout) * time.Second,
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("HTTP 请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("读取响应失败: %v", err)
	}

	// 检查成功标识
	if conn.HTTPSuccessFlag != "" {
		return strings.Contains(string(respBody), conn.HTTPSuccessFlag), nil
	}

	// 默认：HTTP 200 表示成功
	return resp.StatusCode == 200, nil
}

// md5Hash 计算 MD5 哈希
func md5Hash(s string) string {
	hash := md5.Sum([]byte(s))
	return hex.EncodeToString(hash[:])
}

// sha256Hash 计算 SHA256 哈希
func sha256Hash(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// TestConnectorAuth 测试连接器认证
func TestConnectorAuth(conn *models.Connector, username, password string) (bool, string) {
	ok, err := AuthenticateWithConnector(conn, username, password, "username")
	if err != nil {
		return false, err.Error()
	}
	if ok {
		return true, "认证成功"
	}
	return false, "认证失败"
}
