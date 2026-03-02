package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"go-syncflow/internal/dingtalk"
	"go-syncflow/internal/ldapserver"
	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"
	syncer "go-syncflow/internal/sync"
)

// SyncResult 同步结果
type SyncResult struct {
	DepartmentsSynced int              `json:"departmentsSynced"`
	UsersCreated      int              `json:"usersCreated"`
	UsersUpdated      int              `json:"usersUpdated"`
	UsersDisabled     int              `json:"usersDisabled"`
	UsersTotal        int              `json:"usersTotal"`
	Duration          string           `json:"duration"`
	Error             string           `json:"error,omitempty"`
	UserDetails       []SyncUserDetail `json:"userDetails,omitempty"`
}

// SyncUserDetail 单个用户同步明细
type SyncUserDetail struct {
	DingTalkUID string `json:"dingtalkUid"`
	DingName    string `json:"dingName"`
	LocalUser   string `json:"localUser"`
	Department  string `json:"department"`
	Action      string `json:"action"` // created / updated / skipped / failed
	Message     string `json:"message,omitempty"`
}

// GetDingTalkDepartments 获取已同步的部门树
func GetDingTalkDepartments(c *gin.Context) {
	var depts []models.DingTalkDepartment
	if err := storage.DB.Order("\"order\" asc, id asc").Find(&depts).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询部门失败")
		return
	}
	respondOK(c, depts)
}

// GetDingTalkUsers 查询钉钉源用户（从 dingtalk_users 表读取，与本地用户隔离）
func GetDingTalkUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	deptID := c.Query("deptId")
	keyword := c.Query("keyword")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := storage.DB.Model(&models.DingTalkUser{})

	if deptID != "" {
		if id, err := strconv.ParseInt(deptID, 10, 64); err == nil {
			query = query.Where("department_id = ?", id)
		}
	}

	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR email LIKE ? OR mobile LIKE ? OR ding_talk_uid LIKE ?", like, like, like, like)
	}

	var total int64
	query.Count(&total)

	var users []models.DingTalkUser
	query.Offset((page - 1) * pageSize).Limit(pageSize).Order("id desc").Find(&users)

	respondList(c, users, total)
}

// TriggerDingTalkSync 手动触发钉钉同步
func TriggerDingTalkSync(c *gin.Context) {
	result, err := ExecuteDingTalkSync("manual")
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.RecordOperationLog(c, "钉钉同步", "手动同步组织架构",
		fmt.Sprintf("部门:%d 新增:%d 更新:%d 禁用:%d", result.DepartmentsSynced, result.UsersCreated, result.UsersUpdated, result.UsersDisabled), "")

	respondOK(c, result)
}

// ExecuteDingTalkSync 执行钉钉同步，triggerType: "manual" 或 "schedule"
func ExecuteDingTalkSync(triggerType string) (*SyncResult, error) {
	if triggerType == "" {
		triggerType = "manual"
	}
	client := dingtalk.GetClient()
	if !client.IsEnabled() {
		return nil, fmt.Errorf("钉钉未启用")
	}

	cfg, err := client.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("获取钉钉配置失败: %v", err)
	}

	start := time.Now()
	result := &SyncResult{}

	// 1. 同步部门
	depts, err := client.GetAllDepartments()
	if err != nil {
		errMsg := fmt.Sprintf("获取钉钉部门失败: %v", err)
		updateSyncStatus("failed", errMsg)
		storage.DB.Create(&models.SyncLog{
			TriggerType: triggerType, TriggerEvent: "dingtalk_sync",
			Status: "failed", Message: errMsg, Duration: time.Since(start).Milliseconds(),
		})
		return nil, fmt.Errorf("%s", errMsg)
	}

	for _, d := range depts {
		var existing models.DingTalkDepartment
		if storage.DB.Where("dept_id = ?", d.DeptID).First(&existing).Error == nil {
			storage.DB.Model(&existing).Updates(map[string]interface{}{
				"name":      d.Name,
				"parent_id": d.ParentID,
			})
		} else {
			storage.DB.Create(&models.DingTalkDepartment{
				DeptID:   d.DeptID,
				Name:     d.Name,
				ParentID: d.ParentID,
			})
		}
		result.DepartmentsSynced++
	}

	// 1.5 同步组织架构到本地 UserGroup
	// 先为所有钉钉部门创建/更新对应的本地分组
	deptToGroupID := make(map[int64]uint) // 钉钉部门ID -> 本地GroupID

	// 第一遍：创建/更新所有分组（不处理父级关系）
	for _, d := range depts {
		var group models.UserGroup
		if storage.DB.Where("ding_talk_dept_id = ?", d.DeptID).First(&group).Error == nil {
			// 已存在，更新名称
			storage.DB.Model(&group).Update("name", d.Name)
			deptToGroupID[d.DeptID] = group.ID
		} else {
			// 创建新分组
			group = models.UserGroup{
				Name:           d.Name,
				DingTalkDeptID: d.DeptID,
			}
			storage.DB.Create(&group)
			deptToGroupID[d.DeptID] = group.ID
		}
	}

	// 第二遍：更新父级关系
	for _, d := range depts {
		groupID := deptToGroupID[d.DeptID]
		var parentGroupID uint = 0
		if d.ParentID > 0 {
			if pgid, ok := deptToGroupID[d.ParentID]; ok {
				parentGroupID = pgid
			}
		}
		storage.DB.Model(&models.UserGroup{}).Where("id = ?", groupID).Update("parent_id", parentGroupID)
	}

	// 2. 获取用户名策略
	strategy := cfg.UsernameField
	if strategy == "" {
		strategy = dingtalk.StrategyEmailPrefix
	}

	// 3. 默认角色
	defaultRoleID := cfg.DefaultRoleID
	if defaultRoleID == 0 {
		defaultRoleID = 2 // 普通用户
	}

	// 4. 遍历部门同步用户
	processedUsers := make(map[string]bool) // 防止重复处理
	var userDetails []SyncUserDetail

	for _, dept := range depts {
		users, err := client.GetDepartmentUsers(dept.DeptID)
		if err != nil {
			log.Printf("获取部门 %s(%d) 用户失败: %v", dept.Name, dept.DeptID, err)
			continue
		}

		for _, u := range users {
			if processedUsers[u.UserID] {
				continue
			}
			processedUsers[u.UserID] = true
			result.UsersTotal++

			// 查找部门名称
			deptName := dept.Name
			if len(u.DeptIDList) > 0 {
				var deptModel models.DingTalkDepartment
				if storage.DB.Where("dept_id = ?", u.DeptIDList[0]).First(&deptModel).Error == nil {
					deptName = deptModel.Name
				}
			}

			primaryDeptID := dept.DeptID
			if len(u.DeptIDList) > 0 {
				primaryDeptID = u.DeptIDList[0]
			}

			// 查找对应的本地分组ID
			var userGroupID uint
			if gid, ok := deptToGroupID[primaryDeptID]; ok {
				userGroupID = gid
			}

			// ===== 写入钉钉源用户表（dingtalk_users），与本地用户完全隔离 =====
			var dtUser models.DingTalkUser
			if storage.DB.Where("ding_talk_uid = ?", u.UserID).First(&dtUser).Error == nil {
				// 更新已有钉钉源记录
				storage.DB.Model(&dtUser).Updates(map[string]interface{}{
					"name":            u.Name,
					"mobile":          u.Mobile,
					"email":           u.Email,
					"avatar":          u.Avatar,
					"job_title":       u.JobTitle,
					"department_id":   primaryDeptID,
					"department_name": deptName,
					"active":          true,
				})
			} else {
				// 创建新的钉钉源记录
				dtUser = models.DingTalkUser{
					DingTalkUID:    u.UserID,
					Name:           u.Name,
					Mobile:         u.Mobile,
					Email:          u.Email,
					Avatar:         u.Avatar,
					JobTitle:       u.JobTitle,
					DepartmentID:   primaryDeptID,
					DepartmentName: deptName,
					Active:         true,
				}
				storage.DB.Create(&dtUser)
			}

			// ===== 同步到本地用户表（users），本地修改不影响钉钉源 =====
			var existingUser models.User
			if storage.DB.Where("ding_talk_uid = ? AND is_deleted = 0", u.UserID).First(&existingUser).Error == nil {
				// 更新已有本地用户的钉钉信息（不覆盖 status、username、password 等本地属性）
				storage.DB.Model(&existingUser).Updates(map[string]interface{}{
					"nickname":        u.Name,
					"phone":           u.Mobile,
					"email":           u.Email,
					"avatar":          u.Avatar,
					"department_id":   primaryDeptID,
					"department_name": deptName,
					"job_title":       u.JobTitle,
					"group_id":        userGroupID,
				})
				// 关联钉钉源记录
				storage.DB.Model(&dtUser).Update("local_user_id", existingUser.ID)

				// 触发下游同步器（同步用户信息变更到 AD / 数据库等）
				syncer.DispatchSyncEvent(models.SyncEventUserUpdate, existingUser.ID, "")

				result.UsersUpdated++
				userDetails = append(userDetails, SyncUserDetail{
					DingTalkUID: u.UserID, DingName: u.Name, LocalUser: existingUser.Username,
					Department: deptName, Action: "updated", Message: "更新本地用户信息",
				})
			} else {
			// 创建新本地用户
			username := dingtalk.GenerateUsername(strategy, &u)

			// 生成随机密码
			randomPwd := fmt.Sprintf("dd_%d_%s", time.Now().UnixNano(), u.UserID)
			hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(randomPwd), bcrypt.DefaultCost)

			newUser := models.User{
				Username:        username,
				Password:        string(hashedPwd),
				SambaNTPassword: ldapserver.ComputeNTHash(randomPwd), // 同时生成 Samba NT Hash
				Nickname:        u.Name,
				Phone:           u.Mobile,
				Email:           u.Email,
				Avatar:          u.Avatar,
				Status:          1,
				Source:          "dingtalk",
				DingTalkUID:     u.UserID,
				DepartmentID:    primaryDeptID,
				DepartmentName:  deptName,
				JobTitle:        u.JobTitle,
				GroupID:         userGroupID,
			}

			// 使用 Select("*") 确保所有字段（包括 Status=1）都被正确保存
			if err := storage.DB.Select("*").Create(&newUser).Error; err != nil {
				log.Printf("创建钉钉用户 %s 失败: %v", u.Name, err)
					userDetails = append(userDetails, SyncUserDetail{
						DingTalkUID: u.UserID, DingName: u.Name, LocalUser: username,
						Department: deptName, Action: "failed", Message: fmt.Sprintf("创建失败: %v", err),
					})
					continue
				}

				// 关联钉钉源记录
				storage.DB.Model(&dtUser).Update("local_user_id", newUser.ID)

				// 分配默认角色
				storage.DB.Create(&models.UserRole{UserID: newUser.ID, RoleID: defaultRoleID})

				// 触发下游同步器（同步到 AD / 数据库等）
				syncer.DispatchSyncEvent(models.SyncEventUserCreate, newUser.ID, randomPwd)

				// 根据消息策略发送"账号开通通知"
				go sendAccountCreatedNotification(newUser, randomPwd)

				result.UsersCreated++
				userDetails = append(userDetails, SyncUserDetail{
					DingTalkUID: u.UserID, DingName: u.Name, LocalUser: username,
					Department: deptName, Action: "created", Message: "新建本地用户并分配默认角色",
				})
			}
		}
	}

	// 5. 禁用钉钉中已不存在的用户
	// 查询所有 source=dingtalk 且 status=1 的本地用户
	var activeDingtalkUsers []models.User
	storage.DB.Where("source = ? AND status = 1 AND is_deleted = 0", "dingtalk").Find(&activeDingtalkUsers)
	for _, u := range activeDingtalkUsers {
		if u.DingTalkUID != "" && !processedUsers[u.DingTalkUID] {
			// 该用户在本次同步中未出现（已从钉钉删除），禁用
			storage.DB.Model(&u).Update("status", 0)

			// 触发下游同步器（同步禁用状态到 AD / 数据库等）
			syncer.DispatchSyncEvent(models.SyncEventUserDisable, u.ID, "")

			result.UsersDisabled++
			userDetails = append(userDetails, SyncUserDetail{
				DingTalkUID: u.DingTalkUID, DingName: u.Nickname, LocalUser: u.Username,
				Department: u.DepartmentName, Action: "disabled", Message: "钉钉中已不存在，本地用户已禁用",
			})
			// 同时更新钉钉源表的 active 状态
			storage.DB.Model(&models.DingTalkUser{}).Where("ding_talk_uid = ?", u.DingTalkUID).Update("active", false)
		}
	}

	// 6. 同步本地群组名称和架构（处理钉钉部门重命名或调整层级的情况）
	// 已在 1.5 步骤中通过 deptToGroupID 处理了名称更新和父级关系更新

	// 7. 根据自动分配规则分配角色
	applyAutoAssignRules()

	result.Duration = time.Since(start).String()
	result.UserDetails = userDetails

	// 更新同步状态
	statusMsg := fmt.Sprintf("部门:%d 新增:%d 更新:%d 禁用:%d 总计:%d",
		result.DepartmentsSynced, result.UsersCreated, result.UsersUpdated, result.UsersDisabled, result.UsersTotal)
	updateSyncStatus("success", statusMsg)

	// 写入同步日志到 SyncLog 表
	detailJSON, _ := json.Marshal(userDetails)
	syncLog := models.SyncLog{
		SynchronizerID: 0, // 钉钉内置同步
		TriggerType:    triggerType,
		TriggerEvent:   "dingtalk_sync",
		Status:         "success",
		Message:        statusMsg,
		Detail:         string(detailJSON),
		AffectedCount:  result.UsersCreated + result.UsersUpdated + result.UsersDisabled,
		Duration:       time.Since(start).Milliseconds(),
	}
	storage.DB.Create(&syncLog)

	return result, nil
}

// AutoAssignResult 自动分配执行结果
type AutoAssignResult struct {
	TotalMatched  int                    `json:"totalMatched"`  // 匹配到的用户总数
	TotalAssigned int                    `json:"totalAssigned"` // 新分配角色的用户数
	TotalSkipped  int                    `json:"totalSkipped"`  // 已有角色跳过的用户数
	Details       []AutoAssignRoleDetail `json:"details"`       // 每个角色的详情
}

type AutoAssignRoleDetail struct {
	RoleID   uint   `json:"roleId"`
	RoleName string `json:"roleName"`
	Matched  int    `json:"matched"`  // 匹配到的用户数
	Assigned int    `json:"assigned"` // 新分配的用户数
	Skipped  int    `json:"skipped"`  // 已有角色跳过的
}

// applyAutoAssignRules 根据角色自动分配规则分配角色（保持向后兼容）
func applyAutoAssignRules() {
	applyAutoAssignRulesWithResult()
}

// applyAutoAssignRulesWithResult 根据角色自动分配规则分配角色，返回执行结果
func applyAutoAssignRulesWithResult() AutoAssignResult {
	result := AutoAssignResult{}

	var rules []models.RoleAutoAssignRule
	storage.DB.Find(&rules)
	if len(rules) == 0 {
		return result
	}

	// 按角色分组规则
	roleRulesMap := make(map[uint][]models.RoleAutoAssignRule)
	for _, rule := range rules {
		roleRulesMap[rule.RoleID] = append(roleRulesMap[rule.RoleID], rule)
	}

	for roleID, roleRules := range roleRulesMap {
		// 获取角色名称
		var role models.Role
		if err := storage.DB.First(&role, roleID).Error; err != nil {
			continue
		}

		detail := AutoAssignRoleDetail{
			RoleID:   roleID,
			RoleName: role.Name,
		}

		// 收集所有匹配用户（去重）
		matchedUsers := make(map[uint]bool)
		for _, rule := range roleRules {
			var users []models.User
			switch rule.RuleType {
			case "group":
				groupID, err := strconv.ParseUint(rule.RuleValue, 10, 32)
				if err != nil {
					continue
				}
				if groupID == 0 {
					storage.DB.Where("is_deleted = 0").Find(&users)
				} else {
					storage.DB.Where("group_id = ? AND is_deleted = 0", groupID).Find(&users)
				}
			case "job_title":
				storage.DB.Where("job_title = ? AND is_deleted = 0", rule.RuleValue).Find(&users)
			default:
				continue
			}
			for _, u := range users {
				matchedUsers[u.ID] = true
			}
		}

		detail.Matched = len(matchedUsers)
		result.TotalMatched += detail.Matched

		// 为匹配的用户分配角色
		for userID := range matchedUsers {
			var count int64
			storage.DB.Model(&models.UserRole{}).Where("user_id = ? AND role_id = ?", userID, roleID).Count(&count)
			if count == 0 {
				if err := storage.DB.Create(&models.UserRole{UserID: userID, RoleID: roleID}).Error; err != nil {
					log.Printf("角色自动分配失败: user_id=%d, role_id=%d, err=%v", userID, roleID, err)
				} else {
					detail.Assigned++
					result.TotalAssigned++
				}
			} else {
				detail.Skipped++
				result.TotalSkipped++
			}
		}

		result.Details = append(result.Details, detail)
	}

	return result
}

// updateSyncStatus 更新同步状态到配置
func updateSyncStatus(status, message string) {
	value, _ := storage.GetConfig("dingtalk")
	var cfg models.DingTalkConfig
	if value != "" {
		json.Unmarshal([]byte(value), &cfg)
	}

	cfg.LastSyncAt = time.Now().Format("2006-01-02 15:04:05")
	cfg.LastSyncStatus = status
	cfg.LastSyncMessage = message

	data, _ := json.Marshal(cfg)
	storage.SetConfig("dingtalk", string(data))
}

// GetDingTalkSyncStatus 获取同步状态
func GetDingTalkSyncStatus(c *gin.Context) {
	value, _ := storage.GetConfig("dingtalk")
	var cfg models.DingTalkConfig
	if value != "" {
		json.Unmarshal([]byte(value), &cfg)
	}

	respondOK(c, gin.H{
		"lastSyncAt":      cfg.LastSyncAt,
		"lastSyncStatus":  cfg.LastSyncStatus,
		"lastSyncMessage": cfg.LastSyncMessage,
		"syncInterval":    cfg.SyncInterval,
	})
}

// GetDingTalkSyncSettings 获取钉钉同步配置
func GetDingTalkSyncSettings(c *gin.Context) {
	value, _ := storage.GetConfig("dingtalk")
	var cfg models.DingTalkConfig
	if value != "" {
		json.Unmarshal([]byte(value), &cfg)
	}

	respondOK(c, gin.H{
		"enabled":       cfg.Enabled,
		"usernameField": cfg.UsernameField,
		"syncInterval":  cfg.SyncInterval,
		"defaultRoleId": cfg.DefaultRoleID,
		"autoRegister":  cfg.AutoRegister,
		"matchField":    cfg.MatchField,
		"lastSyncAt":    cfg.LastSyncAt,
	})
}

// UpdateDingTalkSyncSettings 更新钉钉同步配置
func UpdateDingTalkSyncSettings(c *gin.Context) {
	var req struct {
		UsernameField string `json:"usernameField"`
		SyncInterval  int    `json:"syncInterval"`
		DefaultRoleID uint   `json:"defaultRoleId"`
		AutoRegister  bool   `json:"autoRegister"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	value, _ := storage.GetConfig("dingtalk")
	var cfg models.DingTalkConfig
	if value != "" {
		json.Unmarshal([]byte(value), &cfg)
	}

	cfg.UsernameField = req.UsernameField
	cfg.SyncInterval = req.SyncInterval
	cfg.DefaultRoleID = req.DefaultRoleID
	cfg.AutoRegister = req.AutoRegister

	data, _ := json.Marshal(cfg)
	if err := storage.SetConfig("dingtalk", string(data)); err != nil {
		respondError(c, http.StatusInternalServerError, "保存失败")
		return
	}

	// 仅在已完成过首次同步后才启动定时同步
	if cfg.LastSyncAt != "" {
		RestartSyncScheduler(cfg.SyncInterval)
	}

	middleware.RecordOperationLog(c, "钉钉同步", "更新同步配置", "", "")
	respondOK(c, nil)
}

// ========== 定时同步 ==========

var syncTicker *time.Ticker
var syncDone chan bool

// StartSyncScheduler 启动定时同步（仅在已完成过至少一次同步后才生效）
func StartSyncScheduler() {
	value, _ := storage.GetConfig("dingtalk")
	var cfg models.DingTalkConfig
	if value != "" {
		json.Unmarshal([]byte(value), &cfg)
	}

	// 只有同步过至少一次 且 配置了同步间隔 才启动
	if cfg.SyncInterval > 0 && cfg.LastSyncAt != "" {
		startTicker(cfg.SyncInterval)
	}
}

// RestartSyncScheduler 重启定时同步
func RestartSyncScheduler(intervalMinutes int) {
	stopTicker()
	if intervalMinutes > 0 {
		startTicker(intervalMinutes)
	}
}

func startTicker(intervalMinutes int) {
	syncTicker = time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	syncDone = make(chan bool)

	go func() {
		for {
			select {
			case <-syncDone:
				return
			case <-syncTicker.C:
				log.Println("定时同步钉钉组织架构...")
				if result, err := ExecuteDingTalkSync("schedule"); err != nil {
					log.Printf("定时同步失败: %v", err)
				} else {
					log.Printf("定时同步完成: 部门=%d 新增=%d 更新=%d",
						result.DepartmentsSynced, result.UsersCreated, result.UsersUpdated)
				}
			}
		}
	}()
}

func stopTicker() {
	if syncTicker != nil {
		syncTicker.Stop()
	}
	if syncDone != nil {
		close(syncDone)
		syncDone = nil
	}
}

// sendAccountCreatedNotification 根据 account_created 消息策略发送账号开通通知
func sendAccountCreatedNotification(user models.User, rawPassword string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[账号通知] panic: %v", r)
		}
	}()

	// 按消息策略获取允许的通知渠道，未配置则不发送
	channelTypes := ResolveAllowedChannelTypes("account_created", user.GroupID)
	if len(channelTypes) == 0 {
		return // 消息策略中未配置账号开通通知渠道，不发送
	}

	// 获取消息模板（必须在前端配置，不存在则记录错误并跳过）
	var tpl models.MessageTemplate
	if storage.DB.Where("scene = ?", "account_created").First(&tpl).Error != nil {
		log.Printf("[账号通知] 消息模板 account_created 不存在，请在消息模板管理中创建，跳过通知: %s", user.Username)
		return
	}
	content := tpl.Content
	content = strings.ReplaceAll(content, "{{username}}", user.Username)
	content = strings.ReplaceAll(content, "{{nickname}}", user.Nickname)
	content = strings.ReplaceAll(content, "{{name}}", user.Nickname)
	content = strings.ReplaceAll(content, "{{password}}", rawPassword)
	content = strings.ReplaceAll(content, "{{time}}", time.Now().Format("2006-01-02 15:04:05"))
	content = strings.ReplaceAll(content, "{{app_name}}", "统一身份认证平台")
	content = strings.ReplaceAll(content, "{{department}}", user.DepartmentName)

	results := services.SendNotificationByChannels(user, "账号开通通知", content, channelTypes)
	for _, r := range results {
		if r.Success {
			log.Printf("[账号通知] %s 发送成功 -> %s", r.Channel, user.Username)
		} else {
			log.Printf("[账号通知] %s 发送失败 %s: %s", r.Channel, user.Username, r.Message)
		}
	}
}
