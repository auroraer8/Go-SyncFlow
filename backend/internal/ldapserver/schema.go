package ldapserver

import (
	"crypto/sha1"
	"fmt"
	"strconv"
	"strings"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

// generateEntryUUID 根据类型和 ID 生成确定性 UUID
func generateEntryUUID(entryType string, id uint) string {
	data := fmt.Sprintf("%s-%d", entryType, id)
	h := sha1.Sum([]byte(data))
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

// formatLDAPTimestamp 格式化时间为 LDAP 标准时间格式 (YYYYMMDDHHmmSSZ)
func formatLDAPTimestamp(t time.Time) string {
	return t.UTC().Format("20060102150405") + "Z"
}

// generateCSN 生成 Change Sequence Number
func generateCSN(t time.Time) string {
	return t.UTC().Format("20060102150405.000000") + "Z#000000#001#000000"
}

// addOperationalAttrs 为条目添加 LDAP 操作属性
func addOperationalAttrs(attrs map[string][]string, dn string, structuralClass string, createdAt, updatedAt time.Time, adminDN string, entryType string, id uint) {
	attrs["entryDN"] = []string{dn}
	attrs["entryUUID"] = []string{generateEntryUUID(entryType, id)}
	attrs["structuralObjectClass"] = []string{structuralClass}
	attrs["subschemaSubentry"] = []string{"cn=subschema"}
	attrs["createTimestamp"] = []string{formatLDAPTimestamp(createdAt)}
	attrs["modifyTimestamp"] = []string{formatLDAPTimestamp(updatedAt)}
	attrs["creatorsName"] = []string{adminDN}
	attrs["modifiersName"] = []string{adminDN}
	attrs["entryCSN"] = []string{generateCSN(updatedAt)}
}

// GroupDNMap 存储群组 ID -> 完整层级 DN 的映射
type GroupDNMap struct {
	GroupDN   map[uint]string    // groupID -> 完整 DN
	GroupName map[uint]string    // groupID -> 群组名称
	Groups    []models.UserGroup // 所有群组（不含根部门）
	RootID    uint               // 根部门 ID（将被跳过）
}

// BuildGroupDNMap 构建群组的层级 DN 映射
// 自动跳过唯一的根部门，其子群组直接挂在 baseDN 下
func BuildGroupDNMap(baseDN string) *GroupDNMap {
	var allGroups []models.UserGroup
	storage.DB.Order("parent_id asc, id asc").Find(&allGroups)

	groupMap := make(map[uint]models.UserGroup)
	nameMap := make(map[uint]string)
	for _, g := range allGroups {
		groupMap[g.ID] = g
		nameMap[g.ID] = g.Name
	}

	dnMap := make(map[uint]string)

	// 递归构建 DN：parentID=0 的群组直接挂在 baseDN 下
	var buildDN func(id uint) string
	buildDN = func(id uint) string {
		if dn, ok := dnMap[id]; ok {
			return dn
		}
		g, exists := groupMap[id]
		if !exists {
			return baseDN
		}
		// 顶层群组直接挂在 baseDN 下
		if g.ParentID == 0 {
			dn := fmt.Sprintf("ou=%s,%s", ldapv3.EscapeDN(g.Name), baseDN)
			dnMap[id] = dn
			return dn
		}
		parentDN := buildDN(g.ParentID)
		dn := fmt.Sprintf("ou=%s,%s", ldapv3.EscapeDN(g.Name), parentDN)
		dnMap[id] = dn
		return dn
	}

	for _, g := range allGroups {
		buildDN(g.ID)
	}

	return &GroupDNMap{
		GroupDN:   dnMap,
		GroupName: nameMap,
		Groups:   allGroups,
		RootID:   0,
	}
}

// BuildUserEntry 将用户模型转换为 LDAP 属性映射
// 用户的 DN 为 uid=username,{groupDN}（如果有群组）或 uid=username,{baseDN}
func BuildUserEntry(user models.User, baseDN string, adminDN string, groupDNMap *GroupDNMap, roleNames []string, sambaEnabled bool, sambaSID string) (string, map[string][]string) {
	// 确定用户所在的 DN 位置
	var dn string
	escapedUID := ldapv3.EscapeDN(user.Username)
	if user.GroupID > 0 {
		if groupDN, ok := groupDNMap.GroupDN[user.GroupID]; ok {
			dn = fmt.Sprintf("uid=%s,%s", escapedUID, groupDN)
		} else {
			dn = fmt.Sprintf("uid=%s,%s", escapedUID, baseDN)
		}
	} else {
		dn = fmt.Sprintf("uid=%s,%s", escapedUID, baseDN)
	}

	// objectClass
	objectClasses := []string{"top", "person", "organizationalPerson", "inetOrgPerson", "posixAccount", "shadowAccount"}
	if sambaEnabled {
		objectClasses = append(objectClasses, "sambaSamAccount", "sambaIdmapEntry")
	}

	// cn = 用户名, sn = 姓名全称, displayName = 姓名全称
	displayName := user.Nickname
	if displayName == "" {
		displayName = user.Username
	}

	uidNumber := strconv.Itoa(int(user.ID)*2 + 10000)
	gidNumber := "10000"
	if user.GroupID > 0 {
		gidNumber = strconv.Itoa(int(user.GroupID)*2 + 10000)
	}

	homeDirectory := "/home/" + user.Username
	loginShell := "/bin/bash"

	attrs := map[string][]string{
		"objectClass":   objectClasses,
		"uid":           {user.Username},
		"cn":            {user.Username},
		"sn":            {displayName},
		"displayName":   {displayName},
		"uidNumber":     {uidNumber},
		"gidNumber":     {gidNumber},
		"homeDirectory": {homeDirectory},
		"loginShell":    {loginShell},
		"gecos":         {displayName},
	}

	// userPassword - 存储密码哈希
	// 注意：本地 LDAP 服务使用 bcrypt 格式密码，如果密码是 {LDAP} 格式则跳过
	// {LDAP} 格式密码需要用户首次登录后才会转换为 bcrypt
	if user.Password != "" && !strings.HasPrefix(user.Password, "{LDAP}") {
		attrs["userPassword"] = []string{"{CRYPT}" + user.Password}
	}

	if user.Email != "" {
		attrs["mail"] = []string{user.Email}
	}
	if user.Phone != "" {
		attrs["telephoneNumber"] = []string{user.Phone}
	}
	if user.JobTitle != "" {
		attrs["title"] = []string{user.JobTitle}
	}
	// ou 属性 -> 用户所属群组名称
	if user.GroupID > 0 {
		if name, ok := groupDNMap.GroupName[user.GroupID]; ok {
			attrs["ou"] = []string{name}
		}
	}

	// memberOf -> 角色 DN
	if len(roleNames) > 0 {
		memberOf := make([]string, 0, len(roleNames))
		for _, roleName := range roleNames {
			memberOf = append(memberOf, fmt.Sprintf("cn=%s,ou=roles,%s", roleName, baseDN))
		}
		attrs["memberOf"] = memberOf
	}

	// Samba 属性
	if sambaEnabled && sambaSID != "" {
		userSID := GenerateUserSID(sambaSID, user.ID)
		attrs["sambaSID"] = []string{userSID}
		attrs["sambaAcctFlags"] = []string{"[U          ]"}
		if user.SambaNTPassword != "" {
			attrs["sambaNTPassword"] = []string{user.SambaNTPassword}
		}
		if user.PasswordChangedAt != nil {
			attrs["sambaPwdLastSet"] = []string{strconv.FormatInt(user.PasswordChangedAt.Unix(), 10)}
		} else {
			attrs["sambaPwdLastSet"] = []string{strconv.FormatInt(time.Now().Unix(), 10)}
		}
	}

	// 操作属性
	addOperationalAttrs(attrs, dn, "inetOrgPerson", user.CreatedAt, user.UpdatedAt, adminDN, "user", user.ID)

	return dn, attrs
}

// BuildGroupEntry 将用户群组转换为 LDAP 条目（同时作为 organizationalUnit 和 groupOfNames）
func BuildGroupEntry(group models.UserGroup, adminDN string, groupDNMap *GroupDNMap, memberDNs []string) (string, map[string][]string) {
	dn := groupDNMap.GroupDN[group.ID]
	if dn == "" {
		return "", nil
	}
	attrs := map[string][]string{
		"objectClass": {"top", "organizationalUnit", "groupOfNames"},
		"ou":          {group.Name},
		"cn":          {group.Name},
		"description": {group.Name},
	}

	// member 属性：该群组下的所有用户 DN
	if len(memberDNs) > 0 {
		attrs["member"] = memberDNs
	}

	// 操作属性
	addOperationalAttrs(attrs, dn, "groupOfNames", group.CreatedAt, group.UpdatedAt, adminDN, "group", group.ID)

	return dn, attrs
}

// BuildRoleEntry 将角色转换为 LDAP posixGroup 条目
func BuildRoleEntry(role models.Role, baseDN string, adminDN string, memberUsernames []string, sambaEnabled bool, sambaSID string) (string, map[string][]string) {
	code := role.Code
	if code == "" {
		code = role.Name
	}
	dn := fmt.Sprintf("cn=%s,ou=roles,%s", code, baseDN)

	objectClasses := []string{"top", "posixGroup"}
	if sambaEnabled {
		objectClasses = append(objectClasses, "sambaGroupMapping")
	}

	gidNumber := strconv.Itoa(int(role.ID)*2 + 10000)
	desc := role.Description
	if desc == "" {
		desc = role.Name
	}

	attrs := map[string][]string{
		"objectClass": objectClasses,
		"cn":          {code},
		"gidNumber":   {gidNumber},
		"description": {desc},
	}

	if len(memberUsernames) > 0 {
		attrs["memberUid"] = memberUsernames
	}

	if sambaEnabled && sambaSID != "" {
		attrs["sambaSID"] = []string{GenerateGroupSID(sambaSID, role.ID)}
		attrs["sambaGroupType"] = []string{"2"}
	}

	// 操作属性
	addOperationalAttrs(attrs, dn, "posixGroup", role.CreatedAt, role.UpdatedAt, adminDN, "role", role.ID)

	return dn, attrs
}

// BuildSambaDomainEntry 构建 sambaDomain 条目，群晖 NAS 需要此条目来确认 Samba 支持
func BuildSambaDomainEntry(baseDN string, adminDN string, domainName string, sambaSID string) (string, map[string][]string) {
	// 从域名提取简短域名（如 example.com -> EXAMPLE）
	shortDomain := domainName
	if idx := strings.Index(domainName, "."); idx > 0 {
		shortDomain = domainName[:idx]
	}
	shortDomain = strings.ToUpper(shortDomain)

	dn := fmt.Sprintf("sambaDomainName=%s,%s", shortDomain, baseDN)
	attrs := map[string][]string{
		"objectClass":             {"top", "sambaDomain"},
		"sambaDomainName":        {shortDomain},
		"sambaSID":               {sambaSID},
		"sambaAlgorithmicRidBase": {"1000"},
		"sambaNextUserRid":        {"50000"},
		"sambaNextGroupRid":       {"50000"},
		"sambaNextRid":            {"50000"},
	}
	now := time.Now()
	addOperationalAttrs(attrs, dn, "sambaDomain", now, now, adminDN, "sambaDomain", 0)
	return dn, attrs
}

// BuildOUEntry 构建 organizationalUnit 容器条目
func BuildOUEntry(ouName, baseDN string, adminDN string) (string, map[string][]string) {
	dn := fmt.Sprintf("ou=%s,%s", ouName, baseDN)
	attrs := map[string][]string{
		"objectClass": {"top", "organizationalUnit"},
		"ou":          {ouName},
	}
	// 容器条目的操作属性使用固定时间和 ID
	now := time.Now()
	addOperationalAttrs(attrs, dn, "organizationalUnit", now, now, adminDN, "ou-"+ouName, 0)
	return dn, attrs
}

// BuildBaseDNEntry 构建 Base DN 根条目
func BuildBaseDNEntry(baseDN, domain string, adminDN string) (string, map[string][]string) {
	attrs := map[string][]string{
		"objectClass": {"top", "dcObject", "organization"},
		"o":           {domain},
	}
	// 提取 dc 值
	parts := parseDN(baseDN)
	for _, p := range parts {
		if p[0] == "dc" {
			attrs["dc"] = []string{p[1]}
			break
		}
	}
	// 根条目的操作属性
	now := time.Now()
	addOperationalAttrs(attrs, baseDN, "organization", now, now, adminDN, "baseDN", 0)
	return baseDN, attrs
}

// parseDN 简单解析 DN 为 [][]string{{key, value}, ...}
func parseDN(dn string) [][2]string {
	var result [][2]string
	for _, part := range splitDN(dn) {
		idx := indexOf(part, '=')
		if idx > 0 {
			result = append(result, [2]string{part[:idx], part[idx+1:]})
		}
	}
	return result
}

func splitDN(dn string) []string {
	var parts []string
	current := ""
	for _, ch := range dn {
		if ch == ',' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func indexOf(s string, ch byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			return i
		}
	}
	return -1
}
