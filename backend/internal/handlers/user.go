package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"

	"go-syncflow/internal/ldapserver"
	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"
	syncer "go-syncflow/internal/sync"
)

// hashPasswordForStorage 将密码哈希为存储格式：bcrypt(SHA256(password))
func hashPasswordForStorage(password string, isEncrypted bool) (string, error) {
	var toHash string
	if isEncrypted {
		// 前端已经发送了SHA256哈希
		toHash = password
	} else {
		// 原始密码，需要先SHA256
		h := sha256.Sum256([]byte(password))
		toHash = hex.EncodeToString(h[:])
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(toHash), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func ListUsers(c *gin.Context) {
	pageIndex, _ := strconv.Atoi(c.DefaultQuery("pageIndex", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	keyword := c.Query("keyword")

	var users []models.User
	var total int64

	groupID := c.Query("groupId")

	query := storage.DB.Model(&models.User{}).Where("is_deleted = 0")
	if keyword != "" {
		query = query.Where("username LIKE ? OR nickname LIKE ? OR phone LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if groupID != "" {
		if gid, err := strconv.Atoi(groupID); err == nil {
			if gid == 0 {
				query = query.Where("group_id = 0")
			} else {
				query = query.Where("group_id = ?", gid)
			}
		}
	}

	query.Count(&total)
	query.Preload("Roles").Offset(pageIndex * pageSize).Limit(pageSize).Order("id desc").Find(&users)

	respondList(c, users, total)
}

type createUserRequest struct {
	Username    string     `json:"username" binding:"required"`
	Password    string     `json:"password" binding:"required,min=6"`
	RawPassword string     `json:"rawPassword"` // 明文密码（用于生成 NT Hash）
	Nickname    string     `json:"nickname"`
	Phone       string     `json:"phone"`
	Email       string     `json:"email"`
	Status      int8       `json:"status"`
	RoleIDs     []uint     `json:"roleIds"`
	GroupID     uint       `json:"groupId"`
	ExpiresAt   *time.Time `json:"expiresAt"` // 账户到期时间，nil表示永不过期
}

func CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 检查用户名是否存在
	var count int64
	storage.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		respondError(c, http.StatusBadRequest, "用户名已存在")
		return
	}

	hashed, _ := hashPasswordForStorage(req.Password, false)
	user := models.User{
		Username:  req.Username,
		Password:  hashed,
		Nickname:  req.Nickname,
		Phone:     req.Phone,
		Email:     req.Email,
		Status:    req.Status,
		Source:    "local",
		GroupID:   req.GroupID,
		ExpiresAt: req.ExpiresAt,
	}
	// 生成 Samba NT Hash（需要明文密码）
	if req.RawPassword != "" {
		user.SambaNTPassword = ldapserver.ComputeNTHash(req.RawPassword)
	} else {
		user.SambaNTPassword = ldapserver.ComputeNTHash(req.Password)
	}

	if err := storage.DB.Create(&user).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	// 分配角色：只有拥有 user:assign_role 权限时才使用前端传入的角色列表
	hasAssignPerm := middleware.CheckPermission(c, "user:assign_role")
	if hasAssignPerm && len(req.RoleIDs) > 0 {
		for _, roleID := range req.RoleIDs {
			storage.DB.Create(&models.UserRole{UserID: user.ID, RoleID: roleID})
		}
	} else {
		// 无角色分配权限或未指定角色时，自动分配"普通用户"角色（ID=2）
		var defaultRole models.Role
		if storage.DB.Where("code = ?", "user").First(&defaultRole).Error == nil {
			storage.DB.Create(&models.UserRole{UserID: user.ID, RoleID: defaultRole.ID})
		}
	}

	middleware.RecordOperationLog(c, "用户管理", "新增用户", req.Username, "")
	syncer.DispatchSyncEvent(models.SyncEventUserCreate, user.ID, req.Password)
	respondOK(c, user)
}

func GetUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var user models.User
	if err := storage.DB.Preload("Roles").First(&user, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}

	respondOK(c, user)
}

type updateUserRequest struct {
	Nickname       string     `json:"nickname"`
	Phone          string     `json:"phone"`
	Email          string     `json:"email"`
	Status         int8       `json:"status"`
	RoleIDs        []uint     `json:"roleIds"`
	GroupID        *uint      `json:"groupId"`
	ExpiresAt      *time.Time `json:"expiresAt"`      // 账户到期时间
	ClearExpiresAt bool       `json:"clearExpiresAt"` // 清除到期时间（设为永不过期）
}

func UpdateUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	var user models.User
	if err := storage.DB.First(&user, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}

	// 钉钉同步用户：基本信息（姓名、手机号、邮箱、分组）不允许手动修改，只能通过同步更新
	if user.Source == "dingtalk" || strings.HasPrefix(user.Source, "im_") {
		updates := map[string]interface{}{
			"status": req.Status,
		}
		selectFields := []string{"status"}
		// 允许设置到期时间
		if req.ClearExpiresAt {
			updates["expires_at"] = nil
			selectFields = append(selectFields, "expires_at")
		} else if req.ExpiresAt != nil {
			updates["expires_at"] = req.ExpiresAt
			selectFields = append(selectFields, "expires_at")
		}
		storage.DB.Model(&user).Select(selectFields).Updates(updates)
	} else {
		updates := map[string]interface{}{
			"nickname": req.Nickname,
			"phone":    req.Phone,
			"email":    req.Email,
			"status":   req.Status,
		}
		selectFields := []string{"nickname", "phone", "email", "status"}
		if req.GroupID != nil {
			updates["group_id"] = *req.GroupID
			selectFields = append(selectFields, "group_id")
		}
		// 处理到期时间
		if req.ClearExpiresAt {
			updates["expires_at"] = nil
			selectFields = append(selectFields, "expires_at")
		} else if req.ExpiresAt != nil {
			updates["expires_at"] = req.ExpiresAt
			selectFields = append(selectFields, "expires_at")
		}
		storage.DB.Model(&user).Select(selectFields).Updates(updates)
	}

	// 更新角色：只有拥有 user:assign_role 权限时才允许修改角色
	if middleware.CheckPermission(c, "user:assign_role") {
		storage.DB.Where("user_id = ?", id).Delete(&models.UserRole{})
		for _, roleID := range req.RoleIDs {
			storage.DB.Create(&models.UserRole{UserID: uint(id), RoleID: roleID})
		}
	}

	middleware.RecordOperationLog(c, "用户管理", "编辑用户", user.Username, "")
	syncer.DispatchSyncEvent(models.SyncEventUserUpdate, uint(id), "")
	respondOK(c, nil)
}

func DeleteUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// 不允许删除自己
	if uint(id) == middleware.GetUserID(c) {
		respondError(c, http.StatusBadRequest, "不能删除自己")
		return
	}

	// 不允许删除 admin
	var user models.User
	if err := storage.DB.First(&user, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}
	if user.Username == "admin" {
		respondError(c, http.StatusForbidden, "不能删除管理员账户")
		return
	}

	// 先加载角色（下游同步需要）再触发同步，等同步完成后再删除
	storage.DB.Preload("Roles").First(&user, id)
	syncer.DispatchSyncEventSync(models.SyncEventUserDelete, user, "")

	// 硬删除：清理关联数据并物理删除记录
	storage.DB.Where("user_id = ?", id).Delete(&models.UserRole{})
	storage.DB.Unscoped().Delete(&user)

	middleware.RecordOperationLog(c, "用户管理", "删除用户", user.Username, "")
	respondOK(c, nil)
}

func UpdateUserStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	storage.DB.Model(&models.User{}).Where("id = ?", id).Update("status", req.Status)

	middleware.RecordOperationLog(c, "用户管理", "更新状态", strconv.FormatUint(id, 10), "")
	if req.Status == 1 {
		syncer.DispatchSyncEvent(models.SyncEventUserEnable, uint(id), "")
	} else {
		syncer.DispatchSyncEvent(models.SyncEventUserDisable, uint(id), "")
	}
	respondOK(c, nil)
}

// UserExportItem 用户导出结构体（包含敏感字段）
type UserExportItem struct {
	ID              uint       `json:"id"`
	Username        string     `json:"username"`
	Nickname        string     `json:"nickname"`
	Phone           string     `json:"phone"`
	Email           string     `json:"email"`
	Avatar          string     `json:"avatar"`
	Status          int8       `json:"status"`
	Source          string     `json:"source"`
	GroupID         uint       `json:"groupId"`
	DingTalkUID     string     `json:"dingtalkUid"`
	DepartmentName  string     `json:"departmentName"`
	JobTitle        string     `json:"jobTitle"`
	Password        string     `json:"password"`        // bcrypt 哈希
	SambaNTPassword string     `json:"sambaNTPassword"` // Samba NT 哈希
	CreatedAt       string     `json:"createdAt"`
	UpdatedAt       string     `json:"updatedAt"`
	PasswordChangedAt *string  `json:"passwordChangedAt"`
	Roles           []RoleExportItem `json:"roles"`
}

// RoleExportItem 角色简要信息
type RoleExportItem struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// 旧 ExportUsers (JSON) 已替换为文件末尾的 Excel 版本

func ResetUserPassword(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		NotifyChannels []string `json:"notifyChannels"` // "sms" / "dingtalk"
	}
	c.ShouldBindJSON(&req)

	// 查找用户
	var user models.User
	if err := storage.DB.First(&user, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}

	// 自动生成符合密码策略的随机密码
	rawPassword := generateSecurePassword()

	hashed, _ := hashPasswordForStorage(rawPassword, false)
	updates := map[string]interface{}{
		"password":          hashed,
		"samba_nt_password": ldapserver.ComputeNTHash(rawPassword),
		"password_changed_at": time.Now(),
	}
	storage.DB.Model(&models.User{}).Where("id = ?", id).Updates(updates)

	// 使该用户 token 失效
	middleware.InvalidateUserTokens(uint(id))

	// 发送通知
	notifyResult := sendPasswordResetNotification(user, rawPassword, req.NotifyChannels)

	middleware.RecordOperationLog(c, "用户管理", "重置密码",
		fmt.Sprintf("用户: %s(ID:%d)", user.Username, id),
		fmt.Sprintf("自动生成密码, 通知: %v", req.NotifyChannels))

	syncer.DispatchSyncEvent(models.SyncEventPasswordChange, uint(id), rawPassword)

	respondOK(c, gin.H{
		"message":      "密码重置成功",
		"notifyResult": notifyResult,
	})
}

// BatchResetPassword 批量重置密码
func BatchResetPassword(c *gin.Context) {
	var req struct {
		UserIDs        []uint   `json:"userIds" binding:"required"`
		NotifyChannels []string `json:"notifyChannels"` // "sms" / "dingtalk"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	if len(req.UserIDs) == 0 {
		respondError(c, http.StatusBadRequest, "请选择要重置的用户")
		return
	}

	if len(req.UserIDs) > 200 {
		respondError(c, http.StatusBadRequest, "单次最多重置200个用户")
		return
	}

	var users []models.User
	storage.DB.Where("id IN ? AND is_deleted = 0", req.UserIDs).Find(&users)

	type resetResult struct {
		UserID       uint   `json:"userId"`
		Username     string `json:"username"`
		Nickname     string `json:"nickname"`
		Success      bool   `json:"success"`
		NotifyResult string `json:"notifyResult"`
	}

	results := make([]resetResult, 0, len(users))
	successCount := 0

	for _, user := range users {
		rawPassword := generateSecurePassword()
		hashed, err := hashPasswordForStorage(rawPassword, false)
		if err != nil {
			results = append(results, resetResult{
				UserID:   user.ID,
				Username: user.Username,
				Nickname: user.Nickname,
				Success:  false,
				NotifyResult: "密码生成失败",
			})
			continue
		}

		updates := map[string]interface{}{
			"password":          hashed,
			"samba_nt_password": ldapserver.ComputeNTHash(rawPassword),
			"password_changed_at": time.Now(),
		}
		storage.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates)
		middleware.InvalidateUserTokens(user.ID)

		notifyResult := sendPasswordResetNotification(user, rawPassword, req.NotifyChannels)
		successCount++

		results = append(results, resetResult{
			UserID:       user.ID,
			Username:     user.Username,
			Nickname:     user.Nickname,
			Success:      true,
			NotifyResult: notifyResult,
		})
	}

	middleware.RecordOperationLog(c, "用户管理", "批量重置密码",
		fmt.Sprintf("共%d人, 成功%d人", len(users), successCount),
		fmt.Sprintf("通知: %v", req.NotifyChannels))

	respondOK(c, gin.H{
		"total":   len(users),
		"success": successCount,
		"results": results,
	})
}

// generateSecurePassword 生成符合密码策略的随机密码（12位）
func generateSecurePassword() string {
	upper := "ABCDEFGHJKLMNPQRSTUVWXYZ"
	lower := "abcdefghjkmnpqrstuvwxyz"
	digits := "23456789"
	special := "!@#$%&*"

	// 确保至少各包含一个
	password := make([]byte, 12)
	password[0] = upper[randInt(len(upper))]
	password[1] = lower[randInt(len(lower))]
	password[2] = digits[randInt(len(digits))]
	password[3] = special[randInt(len(special))]

	all := upper + lower + digits + special
	for i := 4; i < 12; i++ {
		password[i] = all[randInt(len(all))]
	}

	// Fisher-Yates shuffle
	for i := len(password) - 1; i > 0; i-- {
		j := randInt(i + 1)
		password[i], password[j] = password[j], password[i]
	}

	return string(password)
}

func randInt(max int) int {
	b := make([]byte, 1)
	rand.Read(b)
	return int(b[0]) % max
}

// sendPasswordResetNotification 发送密码重置通知
// channels 为通知渠道类型（sms/dingtalk/email/webhook 等）
func sendPasswordResetNotification(user models.User, newPassword string, channels []string) string {
	if len(channels) == 0 {
		return "未选择通知方式"
	}

	// 获取密码重置消息模板（必须在前端配置）
	var tpl models.MessageTemplate
	if storage.DB.Where("scene = ?", "password_reset_notify").First(&tpl).Error != nil {
		log.Printf("[通知] 消息模板 password_reset_notify 不存在，请在消息模板管理中创建，跳过通知: %s", user.Username)
		return "消息模板不存在，无法发送通知"
	}
	content := tpl.Content
	content = strings.ReplaceAll(content, "{{username}}", user.Username)
	content = strings.ReplaceAll(content, "{{nickname}}", user.Nickname)
	content = strings.ReplaceAll(content, "{{name}}", user.Nickname)
	content = strings.ReplaceAll(content, "{{password}}", newPassword)
	content = strings.ReplaceAll(content, "{{department}}", user.DepartmentName)
	content = strings.ReplaceAll(content, "{{time}}", time.Now().Format("2006-01-02 15:04:05"))
	content = strings.ReplaceAll(content, "{{app_name}}", "统一身份认证平台")

	results := services.SendNotificationByChannels(user, "密码重置通知", content, channels)
	return services.FormatNotifyResults(results)
}

// ExportUsers 导出用户列表为 Excel
func ExportUsers(c *gin.Context) {
	var users []models.User
	storage.DB.Where("is_deleted = 0").Order("id asc").Find(&users)

	// 预加载分组信息用于构建完整路径
	var groups []models.UserGroup
	storage.DB.Find(&groups)
	groupMap := make(map[uint]models.UserGroup)
	for _, g := range groups {
		groupMap[g.ID] = g
	}

	f := excelize.NewFile()
	sheet := "用户列表"
	f.SetSheetName("Sheet1", sheet)

	// 表头
	headers := []string{"ID", "用户名", "密码", "姓名", "手机号", "邮箱", "部门路径", "职位", "状态", "来源", "IM UserID", "到期时间", "创建时间"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// 表头样式
	style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#E0E0E0"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	lastCol, _ := excelize.CoordinatesToCellName(len(headers), 1)
	f.SetCellStyle(sheet, "A1", lastCol, style)

	// 数据行
	for i, u := range users {
		row := i + 2
		statusStr := "启用"
		if u.Status != 1 {
			statusStr = "禁用"
		}
		sourceStr := u.Source
		switch u.Source {
		case "local":
			sourceStr = "本地"
		case "im_dingtalk":
			sourceStr = "钉钉"
		case "im_wechatwork":
			sourceStr = "企微"
		case "im_feishu":
			sourceStr = "飞书"
		case "im_welink":
			sourceStr = "WeLink"
		}

		// 构建完整部门路径
		deptPath := u.DepartmentPath
		if deptPath == "" && u.GroupID > 0 {
			deptPath = buildGroupPath(u.GroupID, groupMap)
		}
		if deptPath == "" {
			deptPath = u.DepartmentName
		}

		// IM UserID：优先使用通用字段，其次使用钉钉字段
		imUserID := u.IMUserID
		if imUserID == "" {
			imUserID = u.DingTalkUID
		}

		// 到期时间
		expiresAtStr := "永不过期"
		if u.ExpiresAt != nil {
			expiresAtStr = u.ExpiresAt.Format("2006-01-02 15:04:05")
		}

		f.SetCellValue(sheet, cellName(1, row), u.ID)
		f.SetCellValue(sheet, cellName(2, row), u.Username)
		f.SetCellValue(sheet, cellName(3, row), u.Password) // 密码（哈希值）
		f.SetCellValue(sheet, cellName(4, row), u.Nickname)
		f.SetCellValue(sheet, cellName(5, row), u.Phone)
		f.SetCellValue(sheet, cellName(6, row), u.Email)
		f.SetCellValue(sheet, cellName(7, row), deptPath)
		f.SetCellValue(sheet, cellName(8, row), u.JobTitle)
		f.SetCellValue(sheet, cellName(9, row), statusStr)
		f.SetCellValue(sheet, cellName(10, row), sourceStr)
		f.SetCellValue(sheet, cellName(11, row), imUserID)
		f.SetCellValue(sheet, cellName(12, row), expiresAtStr)
		f.SetCellValue(sheet, cellName(13, row), u.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	// 设置列宽
	widths := map[string]float64{
		"A": 8, "B": 18, "C": 64, "D": 12, "E": 15, "F": 28,
		"G": 30, "H": 12, "I": 8, "J": 10, "K": 28, "L": 20, "M": 20,
	}
	for col, w := range widths {
		f.SetColWidth(sheet, col, col, w)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=users_%s.xlsx", time.Now().Format("20060102_150405")))
	if err := f.Write(c.Writer); err != nil {
		log.Printf("[导出] 用户导出失败: %v", err)
	}
}

// buildGroupPath 构建分组的完整路径
func buildGroupPath(groupID uint, groupMap map[uint]models.UserGroup) string {
	var path []string
	currentID := groupID
	visited := make(map[uint]bool)

	for currentID > 0 && !visited[currentID] {
		visited[currentID] = true
		if group, ok := groupMap[currentID]; ok {
			path = append([]string{group.Name}, path...)
			currentID = group.ParentID
		} else {
			break
		}
	}

	return strings.Join(path, "/")
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// CheckExpiredUsers 手动触发过期用户检查
func CheckExpiredUsers(c *gin.Context) {
	count, err := services.CheckUserExpiry()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "检查失败: "+err.Error())
		return
	}
	middleware.RecordOperationLog(c, "用户管理", "过期检查", "", fmt.Sprintf("禁用 %d 个过期用户", count))
	respondOK(c, gin.H{
		"message":       "过期用户检查完成",
		"disabledCount": count,
	})
}
