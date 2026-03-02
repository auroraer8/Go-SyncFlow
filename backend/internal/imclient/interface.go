package imclient

import (
	"fmt"

	"go-syncflow/internal/models"
)

// IMDeptInfo IM平台部门信息
type IMDeptInfo struct {
	DeptID   string
	Name     string
	ParentID string
	Order    int
}

// IMUserInfo IM平台用户信息
type IMUserInfo struct {
	UserID   string
	Name     string
	Mobile   string
	Email    string
	Avatar   string
	JobTitle string
	DeptID   string
	DeptName string
	Active   bool
}

// IMClient IM平台统一接口
type IMClient interface {
	// TestConnection 测试连接
	TestConnection() error

	// GetAllDepartments 获取所有部门
	GetAllDepartments() ([]IMDeptInfo, error)

	// GetDepartmentUsers 获取指定部门的用户列表
	GetDepartmentUsers(deptID string) ([]IMUserInfo, error)

	// GetUserByAuthCode 通过免登授权码获取用户信息（SSO）
	GetUserByAuthCode(authCode string) (*IMUserInfo, error)

	// SendMessage 发送消息给指定用户
	SendMessage(userID string, content string) error

	// GetUserIDByMobile 通过手机号查询平台用户ID
	GetUserIDByMobile(mobile string) (string, error)

	// PlatformType 返回平台类型
	PlatformType() string
}

// NewIMClient 根据连接器创建 IM 客户端
func NewIMClient(conn models.Connector) (IMClient, error) {
	switch conn.Type {
	case "im_dingtalk":
		return NewDingTalkClient(conn), nil
	case "im_wechatwork":
		return NewWeChatWorkClient(conn), nil
	case "im_feishu":
		return NewFeishuClient(conn), nil
	case "im_welink":
		return NewWeLinkClient(conn), nil
	default:
		return nil, fmt.Errorf("不支持的 IM 平台类型: %s", conn.Type)
	}
}
