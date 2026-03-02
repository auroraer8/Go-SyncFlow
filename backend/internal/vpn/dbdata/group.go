package dbdata

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go-syncflow/internal/vpn/base"
	"github.com/songgao/water/waterutil"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	Allow = "allow"
	Deny  = "deny"
	ALL   = "all"
	TCP   = "tcp"
	UDP   = "udp"
	ICMP  = "icmp"
)

// 域名分流最大字符2万
const DsMaxLen = 20000

type GroupLinkAcl struct {
	// 自上而下匹配 默认 allow * *
	Action   string               `json:"action"`      // allow、deny
	Protocol string               `json:"protocol"`    // 支持 ALL、TCP、UDP、ICMP 协议
	IpProto  waterutil.IPProtocol `json:"ip_protocol"` // 判断协议使用
	Val      string               `json:"val"`
	Port     string               `json:"port"` // 兼容单端口历史数据类型uint16
	Ports    map[uint16]int8      `json:"ports"`
	IpNet    *net.IPNet           `json:"ip_net"`
	Note     string               `json:"note"`
}

type ValData struct {
	Val    string `json:"val"`
	IpMask string `json:"ip_mask"`
	Note   string `json:"note"`
}

type GroupNameId struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// type Group struct {
// 	Id               int                    `json:"id" xorm:"pk autoincr not null"`
// 	Name             string                 `json:"name" xorm:"varchar(60) not null unique"`
// 	Note             string                 `json:"note" xorm:"varchar(255)"`
// 	AllowLan         bool                   `json:"allow_lan" xorm:"Bool"`
// 	ClientDns        []ValData              `json:"client_dns" xorm:"Text"`
// 	RouteInclude     []ValData              `json:"route_include" xorm:"Text"`
// 	RouteExclude     []ValData              `json:"route_exclude" xorm:"Text"`
// 	DsExcludeDomains string                 `json:"ds_exclude_domains" xorm:"Text"`
// 	DsIncludeDomains string                 `json:"ds_include_domains" xorm:"Text"`
// 	LinkAcl          []GroupLinkAcl         `json:"link_acl" xorm:"Text"`
// 	Bandwidth        int                    `json:"bandwidth" xorm:"Int"`                           // 带宽限制
// 	Auth             map[string]interface{} `json:"auth" xorm:"not null default '{}' varchar(255)"` // 认证方式
// 	Status           int8                   `json:"status" xorm:"Int"`                              // 1正常
// 	CreatedAt        time.Time              `json:"created_at" xorm:"DateTime created"`
// 	UpdatedAt        time.Time              `json:"updated_at" xorm:"DateTime updated"`
// }

func GetGroupNames() []string {
	var datas []Group
	err := Find(&datas, 0, 0)
	if err != nil {
		base.Error(err)
		return nil
	}
	var names []string
	for _, v := range datas {
		names = append(names, v.Name)
	}
	return names
}

func GetGroupNamesNormal() []string {
	var datas []Group
	err := FindWhere(&datas, 0, 0, "status=1")
	if err != nil {
		base.Error(err)
		return nil
	}
	var names []string
	for _, v := range datas {
		names = append(names, v.Name)
	}
	return names
}

func GetGroupNamesIds() []GroupNameId {
	var datas []Group
	err := Find(&datas, 0, 0)
	if err != nil {
		base.Error(err)
		return nil
	}
	var names []GroupNameId
	for _, v := range datas {
		names = append(names, GroupNameId{Id: v.Id, Name: v.Name})
	}
	return names
}

func SetGroup(g *Group) error {
	var err error
	if g.Name == "" {
		return errors.New("用户组名错误")
	}

	// 判断数据
	routeInclude := []ValData{}
	for _, v := range g.RouteInclude {
		if v.Val != "" {
			if v.Val == ALL {
				routeInclude = append(routeInclude, v)
				continue
			}

			ipMask, ipNet, err := parseIpNet(v.Val)

			if err != nil {
				return errors.New("RouteInclude 错误" + err.Error())
			}

			// 给Mac系统下发路由时，必须是标准的网络地址
			if strings.Split(ipMask, "/")[0] != ipNet.IP.String() {
				errMsg := fmt.Sprintf("RouteInclude 错误: 网络地址错误，建议： %s 改为 %s", v.Val, ipNet)
				return errors.New(errMsg)
			}

			v.IpMask = ipMask
			routeInclude = append(routeInclude, v)
		}
	}
	g.RouteInclude = routeInclude
	routeExclude := []ValData{}
	for _, v := range g.RouteExclude {
		if v.Val != "" {
			ipMask, ipNet, err := parseIpNet(v.Val)
			if err != nil {
				return errors.New("RouteExclude 错误" + err.Error())
			}

			if strings.Split(ipMask, "/")[0] != ipNet.IP.String() {
				errMsg := fmt.Sprintf("RouteInclude 错误: 网络地址错误，建议： %s 改为 %s", v.Val, ipNet)
				return errors.New(errMsg)
			}

			v.IpMask = ipMask
			routeExclude = append(routeExclude, v)
		}
	}
	g.RouteExclude = routeExclude

	// 校验地址组引用
	if len(g.RouteIncludeGroupIDs) > 0 {
		ags, err := GetAddressGroupByIds(g.RouteIncludeGroupIDs)
		if err != nil {
			return errors.New("查询路由包含地址组失败: " + err.Error())
		}
		if len(ags) != len(g.RouteIncludeGroupIDs) {
			return errors.New("部分路由包含地址组不存在，请检查")
		}
	}
	if len(g.RouteExcludeGroupIDs) > 0 {
		ags, err := GetAddressGroupByIds(g.RouteExcludeGroupIDs)
		if err != nil {
			return errors.New("查询路由排除地址组失败: " + err.Error())
		}
		if len(ags) != len(g.RouteExcludeGroupIDs) {
			return errors.New("部分路由排除地址组不存在，请检查")
		}
	}

	// 转换数据
	linkAcl := []GroupLinkAcl{}
	for _, v := range g.LinkAcl {
		if v.Val != "" {
			_, ipNet, err := parseIpNet(v.Val)
			if err != nil {
				return errors.New("GroupLinkAcl 错误" + err.Error())
			}
			v.IpNet = ipNet

			// 设置协议数据
			switch v.Protocol {
			case TCP:
				v.IpProto = waterutil.TCP
			case UDP:
				v.IpProto = waterutil.UDP
			case ICMP:
				v.IpProto = waterutil.ICMP
			default:
				// 其他类型都是 all
				v.Protocol = ALL
			}

			portsStr := v.Port
			v.Port = strings.TrimSpace(portsStr)
			// switch vp := v.Port.(type) {
			// case float64:
			// 	portsStr = strconv.Itoa(int(vp))
			// case string:
			// 	portsStr = vp
			// }

			if regexp.MustCompile(`^\d{1,5}(-\d{1,5})?(,\d{1,5}(-\d{1,5})?)*$`).MatchString(portsStr) {
				ports := map[uint16]int8{}
				for _, p := range strings.Split(portsStr, ",") {
					if p == "" {
						continue
					}
					if regexp.MustCompile(`^\d{1,5}-\d{1,5}$`).MatchString(p) {
						rp := strings.Split(p, "-")
						// portfrom, err := strconv.Atoi(rp[0])
						portfrom, err := strconv.ParseUint(rp[0], 10, 16)
						if err != nil {
							return errors.New("端口:" + rp[0] + " 格式错误, " + err.Error())
						}
						// portto, err := strconv.Atoi(rp[1])
						portto, err := strconv.ParseUint(rp[1], 10, 16)
						if err != nil {
							return errors.New("端口:" + rp[1] + " 格式错误, " + err.Error())
						}
						for i := portfrom; i <= portto; i++ {
							ports[uint16(i)] = 1
						}

					} else {
						port, err := strconv.ParseUint(p, 10, 16)
						if err != nil {
							return errors.New("端口:" + p + " 格式错误, " + err.Error())
						}
						ports[uint16(port)] = 1
					}
				}
				v.Ports = ports
				linkAcl = append(linkAcl, v)
			} else {
				return errors.New("端口: " + portsStr + " 格式错误,请用逗号分隔的端口,比如: 22,80,443 连续端口用-,比如:1234-5678")
			}

		}
	}

	g.LinkAcl = linkAcl

	// DNS 判断
	clientDns := []ValData{}
	for _, v := range g.ClientDns {
		v.Val = strings.TrimSpace(v.Val)
		if v.Val != "" {
			ip := net.ParseIP(v.Val)
			if ip.String() != v.Val {
				return errors.New("DNS IP 错误")
			}
			clientDns = append(clientDns, v)
		}
	}
	// 是否默认路由（有地址组引用时不视为默认路由）
	hasAddrGroupInclude := len(g.RouteIncludeGroupIDs) > 0
	isDefRoute := !hasAddrGroupInclude && (len(routeInclude) == 0 || (len(routeInclude) == 1 && routeInclude[0].Val == "all"))
	if isDefRoute && len(clientDns) == 0 {
		return errors.New("默认路由，必须设置一个DNS")
	}
	g.ClientDns = clientDns

	splitDns := []ValData{}
	for _, v := range g.SplitDns {
		v.Val = strings.TrimSpace(v.Val)
		if v.Val != "" {
			ValidateDomainName(v.Val)
			if !ValidateDomainName(v.Val) {
				return errors.New("域名 错误")
			}
			splitDns = append(splitDns, v)
		}
	}
	g.SplitDns = splitDns

	// 域名拆分隧道，不能同时填写
	g.DsIncludeDomains = strings.TrimSpace(g.DsIncludeDomains)
	g.DsExcludeDomains = strings.TrimSpace(g.DsExcludeDomains)
	if g.DsIncludeDomains != "" && g.DsExcludeDomains != "" {
		return errors.New("包含/排除域名不能同时填写")
	}
	// 校验包含域名的格式
	err = CheckDomainNames(g.DsIncludeDomains)
	if err != nil {
		return errors.New("包含域名有误：" + err.Error())
	}
	// 校验排除域名的格式
	err = CheckDomainNames(g.DsExcludeDomains)
	if err != nil {
		return errors.New("排除域名有误：" + err.Error())
	}
	if isDefRoute && g.DsIncludeDomains != "" {
		return errors.New("默认路由, 不允许设置\"包含域名\", 请重新配置")
	}
	// 处理登入方式的逻辑
	// 默认使用 syncflow（系统本地用户）认证方式
	defAuth := map[string]interface{}{
		"type": "syncflow",
	}
	if len(g.Auth) == 0 {
		g.Auth = defAuth
	}
	authType := g.Auth["type"].(string)
	smsEnabled := g.Auth["sms_enabled"]

	// local 和 syncflow 是简单认证类型，不需要额外配置
	if authType == "local" || authType == "syncflow" {
		g.Auth = map[string]interface{}{
			"type": authType,
		}
		if smsEnabled != nil {
			g.Auth["sms_enabled"] = smsEnabled
		}
	} else if authType == "ldap_connector" || authType == "connector" {
		// ldap_connector / connector 类型直接使用 connector_id 字段
		// connector 是前端新版使用的类型名称，统一处理
		connectorID := g.Auth["connector_id"]
		learnPassword := g.Auth["learn_password"]
		
		// 验证 connector_id 是否有效
		if connectorID == nil {
			return errors.New("请选择认证连接器")
		}
		
		// 保存时统一使用 connector 类型（前端兼容）
		g.Auth = map[string]interface{}{
			"type":         "connector",
			"connector_id": connectorID,
		}
		if learnPassword != nil {
			g.Auth["learn_password"] = learnPassword
		}
		if smsEnabled != nil {
			g.Auth["sms_enabled"] = smsEnabled
		}
	} else {
		if _, ok := authRegistry[authType]; !ok {
			return errors.New("未知的认证方式: " + authType)
		}
		auth := makeInstance(authType).(IUserAuth)
		err = auth.checkData(g.Auth)
		if err != nil {
			return err
		}
		// 重置Auth，删除多余的key，但保留 sms_enabled
		g.Auth = map[string]interface{}{
			"type":   authType,
			authType: g.Auth[authType],
		}
		if smsEnabled != nil {
			g.Auth["sms_enabled"] = smsEnabled
		}
	}

	g.UpdatedAt = time.Now()
	if g.Id > 0 {
		err = Set(g)
	} else {
		err = Add(g)
	}

	return err
}

func ContainsInPorts(ports map[uint16]int8, port uint16) bool {
	_, ok := ports[port]
	if ok {
		return true
	} else {
		return false
	}
}

func GroupAuthLogin(name, pwd string, authData map[string]interface{}) error {
	g := &Group{Auth: authData}
	authType := g.Auth["type"].(string)
	if _, ok := authRegistry[authType]; !ok {
		return errors.New("未知的认证方式: " + authType)
	}
	auth := makeInstance(authType).(IUserAuth)
	err := auth.checkData(g.Auth)
	if err != nil {
		return err
	}
	ext := map[string]interface{}{}
	err = auth.checkUser(name, pwd, g, ext)
	return err
}

func parseIpNet(s string) (string, *net.IPNet, error) {
	ip, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		return "", nil, err
	}

	mask := net.IP(ipNet.Mask)
	ipMask := fmt.Sprintf("%s/%s", ip, mask)

	return ipMask, ipNet, nil
}

func CheckDomainNames(domains string) error {
	if domains == "" {
		return nil
	}
	strLen := 0
	str_slice := strings.Split(domains, ",")
	for _, val := range str_slice {
		if val == "" {
			return errors.New(val + " 请以逗号分隔域名")
		}
		if !ValidateDomainName(val) {
			return errors.New(val + " 域名有误")
		}
		strLen += len(val)
	}
	if strLen > DsMaxLen {
		p := message.NewPrinter(language.English)
		return fmt.Errorf("字符长度超出限制，最大%s个(不包含逗号), 请删减一些域名", p.Sprintf("%d", DsMaxLen))
	}
	return nil
}

func ValidateDomainName(domain string) bool {
	RegExp := regexp.MustCompile(`^([a-zA-Z0-9][-a-zA-Z0-9]{0,62}\.)+[A-Za-z]{2,18}$`)
	return RegExp.MatchString(domain)
}
