package dbdata

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"reflect"
	"time"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"

	"github.com/go-ldap/ldap/v3"
)

type AuthLdapConnector struct {
	ConnectorID uint `json:"connector_id"`
}

func init() {
	authRegistry["ldap_connector"] = reflect.TypeOf(AuthLdapConnector{})
}

func (auth AuthLdapConnector) checkData(authData map[string]interface{}) error {
	connectorIDFloat, ok := authData["connector_id"].(float64)
	if !ok || connectorIDFloat <= 0 {
		return errors.New("请选择上游 LDAP 连接器")
	}
	connectorID := uint(connectorIDFloat)
	var connector models.Connector
	if err := storage.DB.Where("id = ? AND status = 1", connectorID).First(&connector).Error; err != nil {
		return errors.New("指定的 LDAP 连接器不存在或未启用")
	}
	if connector.Type != "ldap_ad" && connector.Type != "ldap_openldap" {
		return errors.New("指定的连接器不是 LDAP 类型")
	}
	return nil
}

func (auth AuthLdapConnector) checkUser(name, pwd string, g *Group, ext map[string]interface{}) error {
	if name == "" || len(pwd) < 1 {
		return fmt.Errorf("%s 用户名或密码不能为空", name)
	}
	connectorIDFloat, ok := g.Auth["connector_id"].(float64)
	if !ok {
		return fmt.Errorf("%s LDAP 连接器配置错误", name)
	}
	connectorID := uint(connectorIDFloat)
	var connector models.Connector
	if err := storage.DB.Where("id = ? AND status = 1", connectorID).First(&connector).Error; err != nil {
		return fmt.Errorf("%s 指定的 LDAP 连接器不存在或未启用", name)
	}
	return authenticateWithConnector(name, pwd, &connector)
}

func authenticateWithConnector(username, password string, connector *models.Connector) error {
	addr := fmt.Sprintf("%s:%d", connector.Host, connector.Port)
	con, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("LDAP 服务器连接失败: %s", err.Error())
	}
	con.Close()

	var l *ldap.Conn
	if connector.UseTLS {
		l, err = ldap.DialURL(fmt.Sprintf("ldaps://%s", addr), ldap.DialWithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	} else {
		l, err = ldap.DialURL(fmt.Sprintf("ldap://%s", addr))
	}
	if err != nil {
		return fmt.Errorf("LDAP 连接失败: %s", err.Error())
	}
	defer l.Close()

	if err := l.Bind(connector.BindDN, connector.BindPassword); err != nil {
		return fmt.Errorf("LDAP 管理员绑定失败: %s", err.Error())
	}

	var searchAttr, objectClass string
	if connector.Type == "ldap_ad" {
		searchAttr = "sAMAccountName"
		objectClass = "user"
	} else {
		searchAttr = connector.LDAPLoginAttr
		if searchAttr == "" {
			searchAttr = "uid"
		}
		objectClass = connector.LDAPUserObjectClass
		if objectClass == "" {
			objectClass = "inetOrgPerson"
		}
	}

	filter := fmt.Sprintf("(&(objectClass=%s)(%s=%s))", objectClass, searchAttr, ldap.EscapeFilter(username))
	searchRequest := ldap.NewSearchRequest(
		connector.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 10, false,
		filter,
		[]string{"dn"},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		return fmt.Errorf("LDAP 用户搜索失败: %s", err.Error())
	}
	if len(sr.Entries) == 0 {
		return fmt.Errorf("LDAP 未找到用户 %s", username)
	}
	if len(sr.Entries) > 1 {
		return fmt.Errorf("LDAP 发现多个匹配用户 %s", username)
	}

	userDN := sr.Entries[0].DN
	if err := l.Bind(userDN, password); err != nil {
		return fmt.Errorf("LDAP 密码验证失败: %s", err.Error())
	}
	return nil
}

func GetLdapConnectorUserPhone(username string, connectorID uint) (string, error) {
	var connector models.Connector
	if err := storage.DB.Where("id = ? AND status = 1", connectorID).First(&connector).Error; err != nil {
		return "", fmt.Errorf("连接器不存在或未启用")
	}

	addr := fmt.Sprintf("%s:%d", connector.Host, connector.Port)
	var l *ldap.Conn
	var err error
	if connector.UseTLS {
		l, err = ldap.DialURL(fmt.Sprintf("ldaps://%s", addr), ldap.DialWithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	} else {
		l, err = ldap.DialURL(fmt.Sprintf("ldap://%s", addr))
	}
	if err != nil {
		return "", fmt.Errorf("LDAP 连接失败: %s", err.Error())
	}
	defer l.Close()

	if err := l.Bind(connector.BindDN, connector.BindPassword); err != nil {
		return "", fmt.Errorf("LDAP 管理员绑定失败: %s", err.Error())
	}

	var searchAttr, objectClass, phoneAttr string
	if connector.Type == "ldap_ad" {
		searchAttr = "sAMAccountName"
		objectClass = "user"
		phoneAttr = "telephoneNumber"
	} else {
		searchAttr = connector.LDAPLoginAttr
		if searchAttr == "" {
			searchAttr = "uid"
		}
		objectClass = connector.LDAPUserObjectClass
		if objectClass == "" {
			objectClass = "inetOrgPerson"
		}
		phoneAttr = connector.LDAPMobileAttr
		if phoneAttr == "" {
			phoneAttr = "mobile"
		}
	}

	filter := fmt.Sprintf("(&(objectClass=%s)(%s=%s))", objectClass, searchAttr, ldap.EscapeFilter(username))
	searchRequest := ldap.NewSearchRequest(
		connector.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 10, false,
		filter,
		[]string{phoneAttr},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		return "", fmt.Errorf("LDAP 用户搜索失败: %s", err.Error())
	}
	if len(sr.Entries) != 1 {
		return "", fmt.Errorf("用户未找到或存在多个")
	}
	return sr.Entries[0].GetAttributeValue(phoneAttr), nil
}
