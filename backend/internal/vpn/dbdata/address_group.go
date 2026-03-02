package dbdata

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"go-syncflow/internal/vpn/base"
)

// isDomain 判断一个地址条目是否为域名（非 IP、非 CIDR）
func isDomain(val string) bool {
	if strings.Contains(val, "/") {
		return false
	}
	if net.ParseIP(val) != nil {
		return false
	}
	return strings.Contains(val, ".")
}

type AddressGroupNameId struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

func GetAddressGroupNamesIds() []AddressGroupNameId {
	var datas []AddressGroup
	err := Find(&datas, 0, 0)
	if err != nil {
		base.Error(err)
		return nil
	}
	var names []AddressGroupNameId
	for _, v := range datas {
		names = append(names, AddressGroupNameId{Id: v.Id, Name: v.Name})
	}
	return names
}

func SetAddressGroup(ag *AddressGroup) error {
	if ag.Name == "" {
		return errors.New("地址组名称不能为空")
	}

	addresses := []ValData{}
	for _, v := range ag.Addresses {
		v.Val = strings.TrimSpace(v.Val)
		if v.Val == "" {
			continue
		}

		if strings.ToLower(v.Val) == ALL {
			v.IpMask = ""
			addresses = append(addresses, v)
			continue
		}

		// 域名条目：跳过 CIDR 校验，原样保存
		if isDomain(v.Val) {
			v.IpMask = ""
			addresses = append(addresses, v)
			continue
		}

		ipMask, ipNet, err := parseIpNet(v.Val)
		if err != nil {
			return fmt.Errorf("地址 %s 格式错误: %v", v.Val, err)
		}

		if strings.Split(ipMask, "/")[0] != ipNet.IP.String() {
			return fmt.Errorf("网络地址错误，建议: %s 改为 %s", v.Val, ipNet)
		}

		v.IpMask = ipMask
		addresses = append(addresses, v)
	}

	if len(addresses) == 0 {
		return errors.New("至少需要一个地址")
	}

	ag.Addresses = addresses
	ag.UpdatedAt = time.Now()

	var err error
	if ag.Id > 0 {
		err = Set(ag)
	} else {
		err = Add(ag)
	}
	return err
}

func GetAddressGroupByIds(ids []int) ([]AddressGroup, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var groups []AddressGroup
	err := xdb.In("id", ids).Find(&groups)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// ResolveAddressGroupRoutes 将地址组 ID 列表解析为 ValData 路由列表
func ResolveAddressGroupRoutes(groupIDs []int) ([]ValData, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}

	addrGroups, err := GetAddressGroupByIds(groupIDs)
	if err != nil {
		return nil, err
	}

	var routes []ValData
	seen := make(map[string]bool)
	for _, ag := range addrGroups {
		for _, addr := range ag.Addresses {
			if addr.Val == "" {
				continue
			}

			key := addr.Val
			if seen[key] {
				continue
			}
			seen[key] = true

			if strings.ToLower(addr.Val) == ALL {
				routes = append(routes, addr)
				continue
			}

			// 跳过域名条目，只返回 IP 路由
			if isDomain(addr.Val) {
				continue
			}

			if addr.IpMask == "" {
				ipMask, _, err := parseIpNet(addr.Val)
				if err != nil {
					continue
				}
				addr.IpMask = ipMask
			}
			routes = append(routes, addr)
		}
	}
	return routes, nil
}

// ResolveAddressGroupDomains 从地址组中提取域名条目
func ResolveAddressGroupDomains(groupIDs []int) ([]string, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}

	addrGroups, err := GetAddressGroupByIds(groupIDs)
	if err != nil {
		return nil, err
	}

	var domains []string
	seen := make(map[string]bool)
	for _, ag := range addrGroups {
		for _, addr := range ag.Addresses {
			val := strings.TrimSpace(addr.Val)
			if val == "" || seen[val] {
				continue
			}
			if isDomain(val) {
				seen[val] = true
				domains = append(domains, val)
			}
		}
	}
	return domains, nil
}

// IsAddressGroupInUse 检查地址组是否正在被用户组引用
func IsAddressGroupInUse(agId int) (bool, string) {
	var groups []Group
	err := Find(&groups, 0, 0)
	if err != nil {
		return false, ""
	}

	for _, g := range groups {
		for _, id := range g.RouteIncludeGroupIDs {
			if id == agId {
				return true, g.Name
			}
		}
		for _, id := range g.RouteExcludeGroupIDs {
			if id == agId {
				return true, g.Name
			}
		}
	}
	return false, ""
}
