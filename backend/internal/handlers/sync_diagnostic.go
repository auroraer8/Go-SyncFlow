package handlers

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	ldapv3 "github.com/go-ldap/ldap/v3"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

// SyncDiagnosticResult 同步诊断结果
type SyncDiagnosticResult struct {
	LocalTotal      int                   `json:"localTotal"`      // 本地用户总数
	LocalActive     int                   `json:"localActive"`     // 本地启用用户数
	LocalDisabled   int                   `json:"localDisabled"`   // 本地禁用用户数
	LocalDeleted    int                   `json:"localDeleted"`    // 本地已删除用户数
	ADTotal         int                   `json:"adTotal"`         // AD 用户总数
	Synced          int                   `json:"synced"`          // 已同步用户数
	NotSynced       int                   `json:"notSynced"`       // 未同步用户数
	NotSyncedUsers  []NotSyncedUserInfo   `json:"notSyncedUsers"`  // 未同步用户列表
	OnlyInAD        int                   `json:"onlyInAD"`        // 仅存在于 AD 的用户数
	OnlyInADUsers   []string              `json:"onlyInADUsers"`   // 仅存在于 AD 的用户列表
	SyncableCount   int                   `json:"syncableCount"`   // 可同步用户数（满足同步条件）
	Reasons         map[string]int        `json:"reasons"`         // 各原因统计
}

// NotSyncedUserInfo 未同步用户信息
type NotSyncedUserInfo struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Status    int    `json:"status"`
	IsDeleted int    `json:"isDeleted"`
	GroupName string `json:"groupName"`
	Reason    string `json:"reason"` // 未同步原因
}

// DiagnoseDownstreamSync 诊断下游同步状态
func DiagnoseDownstreamSync(c *gin.Context) {
	connectorID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	var conn models.Connector
	if err := storage.DB.First(&conn, connectorID).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}
	
	if conn.Type != "ldap_ad" {
		respondError(c, http.StatusBadRequest, "此诊断功能仅支持 AD/LDAP 连接器")
		return
	}
	
	result := SyncDiagnosticResult{
		Reasons: make(map[string]int),
	}
	
	// 1. 统计本地用户
	var total, active, disabled, deleted int64
	storage.DB.Model(&models.User{}).Count(&total)
	storage.DB.Model(&models.User{}).Where("is_deleted = 0 AND status = 1").Count(&active)
	storage.DB.Model(&models.User{}).Where("is_deleted = 0 AND status = 0").Count(&disabled)
	storage.DB.Model(&models.User{}).Where("is_deleted = 1").Count(&deleted)
	result.LocalTotal = int(total)
	result.LocalActive = int(active)
	result.LocalDisabled = int(disabled)
	result.LocalDeleted = int(deleted)
	
	// 可同步用户数 = 启用且未删除
	result.SyncableCount = result.LocalActive
	
	// 2. 连接 AD 获取用户列表
	l, err := dialLDAPForDiag(conn)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "连接 AD 失败: "+err.Error())
		return
	}
	defer l.Close()
	
	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		respondError(c, http.StatusInternalServerError, "AD 认证失败: "+err.Error())
		return
	}
	
	// 搜索 AD 中所有用户
	adUsers := make(map[string]bool)
	searchReq := ldapv3.NewSearchRequest(
		conn.BaseDN,
		ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 0, 0, false,
		"(&(objectClass=user)(objectCategory=person))",
		[]string{"sAMAccountName"},
		nil,
	)
	
	sr, err := l.Search(searchReq)
	if err != nil {
		// 尝试分页搜索
		sr, err = pagedSearch(l, conn.BaseDN, "(&(objectClass=user)(objectCategory=person))", []string{"sAMAccountName"})
		if err != nil {
			respondError(c, http.StatusInternalServerError, "搜索 AD 用户失败: "+err.Error())
			return
		}
	}
	
	for _, entry := range sr.Entries {
		sam := entry.GetAttributeValue("sAMAccountName")
		if sam != "" {
			adUsers[strings.ToLower(sam)] = true
		}
	}
	result.ADTotal = len(adUsers)
	
	// 3. 查询本地所有用户并比对
	var localUsers []models.User
	storage.DB.Find(&localUsers)
	
	// 预加载所有分组
	var allGroups []models.UserGroup
	storage.DB.Find(&allGroups)
	groupNameMap := make(map[uint]string)
	for _, g := range allGroups {
		groupNameMap[g.ID] = g.Name
	}
	
	localUserMap := make(map[string]models.User)
	for _, u := range localUsers {
		localUserMap[strings.ToLower(u.Username)] = u
		
		// 检查是否在 AD 中存在
		if adUsers[strings.ToLower(u.Username)] {
			result.Synced++
		} else {
			// 分析未同步原因
			var reason string
			if u.IsDeleted == 1 {
				reason = "本地已删除"
				result.Reasons["本地已删除"]++
			} else if u.Status == 0 {
				reason = "本地已禁用"
				result.Reasons["本地已禁用"]++
			} else {
				reason = "未知原因（应该已同步但 AD 中不存在）"
				result.Reasons["同步失败或未执行"]++
			}
			
			groupName := ""
			if u.GroupID > 0 {
				groupName = groupNameMap[u.GroupID]
			}
			
			result.NotSyncedUsers = append(result.NotSyncedUsers, NotSyncedUserInfo{
				ID:        u.ID,
				Username:  u.Username,
				Nickname:  u.Nickname,
				Status:    int(u.Status),
				IsDeleted: int(u.IsDeleted),
				GroupName: groupName,
				Reason:    reason,
			})
		}
	}
	result.NotSynced = len(result.NotSyncedUsers)
	
	// 4. 检查仅存在于 AD 的用户
	for adUser := range adUsers {
		if _, exists := localUserMap[adUser]; !exists {
			result.OnlyInAD++
			if len(result.OnlyInADUsers) < 100 { // 最多返回100个
				result.OnlyInADUsers = append(result.OnlyInADUsers, adUser)
			}
		}
	}
	
	respondOK(c, result)
}

// pagedSearch 分页搜索 LDAP
func pagedSearch(l *ldapv3.Conn, baseDN, filter string, attrs []string) (*ldapv3.SearchResult, error) {
	pagingControl := ldapv3.NewControlPaging(500)
	var allEntries []*ldapv3.Entry
	
	for {
		searchReq := ldapv3.NewSearchRequest(
			baseDN,
			ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 0, 0, false,
			filter, attrs, []ldapv3.Control{pagingControl},
		)
		
		sr, err := l.Search(searchReq)
		if err != nil {
			return nil, err
		}
		
		allEntries = append(allEntries, sr.Entries...)
		
		// 检查是否有下一页
		var pagingResult *ldapv3.ControlPaging
		for _, ctrl := range sr.Controls {
			if p, ok := ctrl.(*ldapv3.ControlPaging); ok {
				pagingResult = p
				break
			}
		}
		
		if pagingResult == nil || len(pagingResult.Cookie) == 0 {
			break
		}
		pagingControl.SetCookie(pagingResult.Cookie)
	}
	
	return &ldapv3.SearchResult{Entries: allEntries}, nil
}

func dialLDAPForDiag(conn models.Connector) (*ldapv3.Conn, error) {
	addr := fmt.Sprintf("%s:%d", conn.Host, conn.Port)
	if conn.UseTLS {
		return ldapv3.DialTLS("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	}
	return ldapv3.Dial("tcp", addr)
}

// GetSyncableUsers 获取可同步用户列表（满足同步条件的用户）
func GetSyncableUsers(c *gin.Context) {
	var users []models.User
	storage.DB.Where("is_deleted = 0 AND status = 1").
		Preload("Roles").
		Order("created_at DESC").
		Find(&users)
	
	// 预加载所有分组
	var allGroups []models.UserGroup
	storage.DB.Find(&allGroups)
	groupNameMap := make(map[uint]string)
	for _, g := range allGroups {
		groupNameMap[g.ID] = g.Name
	}
	
	type UserInfo struct {
		ID           uint     `json:"id"`
		Username     string   `json:"username"`
		Nickname     string   `json:"nickname"`
		Email        string   `json:"email"`
		Phone        string   `json:"phone"`
		GroupName    string   `json:"groupName"`
		Roles        []string `json:"roles"`
		HasPassword  bool     `json:"hasPassword"`
	}
	
	result := make([]UserInfo, 0, len(users))
	for _, u := range users {
		roles := make([]string, 0, len(u.Roles))
		for _, r := range u.Roles {
			roles = append(roles, r.Name)
		}
		
		groupName := ""
		if u.GroupID > 0 {
			groupName = groupNameMap[u.GroupID]
		}
		
		result = append(result, UserInfo{
			ID:          u.ID,
			Username:    u.Username,
			Nickname:    u.Nickname,
			Email:       u.Email,
			Phone:       u.Phone,
			GroupName:   groupName,
			Roles:       roles,
			HasPassword: u.Password != "",
		})
	}
	
	respondOK(c, gin.H{
		"total": len(result),
		"users": result,
	})
}

// GetUnsyncableUsers 获取不可同步用户列表（禁用或删除的用户）
func GetUnsyncableUsers(c *gin.Context) {
	var users []models.User
	storage.DB.Where("is_deleted = 1 OR status = 0").
		Order("updated_at DESC").
		Find(&users)
	
	// 预加载所有分组
	var allGroups []models.UserGroup
	storage.DB.Find(&allGroups)
	groupNameMap := make(map[uint]string)
	for _, g := range allGroups {
		groupNameMap[g.ID] = g.Name
	}
	
	type UserInfo struct {
		ID        uint   `json:"id"`
		Username  string `json:"username"`
		Nickname  string `json:"nickname"`
		Status    int    `json:"status"`
		IsDeleted int    `json:"isDeleted"`
		GroupName string `json:"groupName"`
		Reason    string `json:"reason"`
	}
	
	result := make([]UserInfo, 0, len(users))
	for _, u := range users {
		reason := ""
		if u.IsDeleted == 1 {
			reason = "已删除"
		} else if u.Status == 0 {
			reason = "已禁用"
		}
		
		groupName := ""
		if u.GroupID > 0 {
			groupName = groupNameMap[u.GroupID]
		}
		
		result = append(result, UserInfo{
			ID:        u.ID,
			Username:  u.Username,
			Nickname:  u.Nickname,
			Status:    int(u.Status),
			IsDeleted: int(u.IsDeleted),
			GroupName: groupName,
			Reason:    reason,
		})
	}
	
	respondOK(c, gin.H{
		"total": len(result),
		"users": result,
	})
}
