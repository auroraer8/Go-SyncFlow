package dingtalk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

const (
	apiGetToken         = "https://oapi.dingtalk.com/gettoken"
	apiGetUserInfo      = "https://oapi.dingtalk.com/topapi/v2/user/getuserinfo"
	apiGetUserDetail    = "https://oapi.dingtalk.com/topapi/v2/user/get"
	apiDeptListSub      = "https://oapi.dingtalk.com/topapi/v2/department/listsub"
	apiDeptUserList     = "https://oapi.dingtalk.com/topapi/v2/user/list"
	apiSendWorkMessage  = "https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2"
)

// Client 钉钉客户端
type Client struct {
	config      *models.DingTalkConfig
	accessToken string
	tokenExpire time.Time
	mu          sync.RWMutex
	httpClient  *http.Client
}

var (
	instance *Client
	once     sync.Once
)

// GetClient 获取钉钉客户端单例
func GetClient() *Client {
	once.Do(func() {
		instance = &Client{
			httpClient: &http.Client{
				Timeout: 10 * time.Second,
			},
		}
	})
	return instance
}

// getConfig 获取钉钉配置
func (c *Client) getConfig() (*models.DingTalkConfig, error) {
	value, err := storage.GetConfig("dingtalk")
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, errors.New("钉钉配置未设置")
	}

	var cfg models.DingTalkConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// IsEnabled 检查钉钉是否启用
func (c *Client) IsEnabled() bool {
	cfg, err := c.getConfig()
	if err != nil {
		return false
	}
	return cfg.Enabled && cfg.AppKey != "" && cfg.AppSecret != ""
}

// GetConfig 获取钉钉配置（公开）
func (c *Client) GetConfig() (*models.DingTalkConfig, error) {
	return c.getConfig()
}

// GetAccessToken 获取企业应用access_token
func (c *Client) GetAccessToken() (string, error) {
	c.mu.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpire) {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpire) {
		return c.accessToken, nil
	}

	cfg, err := c.getConfig()
	if err != nil {
		return "", err
	}

	if cfg.AppKey == "" || cfg.AppSecret == "" {
		return "", errors.New("钉钉AppKey或AppSecret未配置")
	}

	url := fmt.Sprintf("%s?appkey=%s&appsecret=%s", apiGetToken, cfg.AppKey, cfg.AppSecret)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("请求钉钉API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析钉钉响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取access_token失败: %s", result.ErrMsg)
	}

	c.accessToken = result.AccessToken
	c.tokenExpire = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	return c.accessToken, nil
}

// UserInfo 钉钉用户信息
type UserInfo struct {
	UserID       string  `json:"userid"`
	UnionID      string  `json:"unionid"`
	Name         string  `json:"name"`
	Mobile       string  `json:"mobile"`
	Email        string  `json:"email"`
	Avatar       string  `json:"avatar"`
	JobTitle     string  `json:"title"`
	DeptIDList   []int64 `json:"dept_id_list"`
}

// DeptInfo 钉钉部门信息
type DeptInfo struct {
	DeptID   int64  `json:"dept_id"`
	Name     string `json:"name"`
	ParentID int64  `json:"parent_id"`
}

// GetUserInfoByAuthCode 通过免登授权码获取用户信息
func (c *Client) GetUserInfoByAuthCode(authCode string) (*UserInfo, error) {
	token, err := c.GetAccessToken()
	if err != nil {
		return nil, err
	}

	userIDResp, err := c.getUserIDByCode(token, authCode)
	if err != nil {
		return nil, err
	}

	return c.getUserDetail(token, userIDResp.UserID)
}

func (c *Client) getUserIDByCode(token, authCode string) (*struct {
	UserID   string `json:"userid"`
	DeviceID string `json:"device_id"`
	UnionID  string `json:"unionid"`
}, error) {
	url := fmt.Sprintf("%s?access_token=%s", apiGetUserInfo, token)

	reqBody := fmt.Sprintf(`{"code":"%s"}`, authCode)
	resp, err := c.httpClient.Post(url, "application/json", strings.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("请求钉钉用户信息失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			UserID   string `json:"userid"`
			DeviceID string `json:"device_id"`
			UnionID  string `json:"unionid"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析钉钉响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("获取用户ID失败: %s (code: %d)", result.ErrMsg, result.ErrCode)
	}

	return &result.Result, nil
}

func (c *Client) getUserDetail(token, userID string) (*UserInfo, error) {
	url := fmt.Sprintf("%s?access_token=%s", apiGetUserDetail, token)

	reqBody := fmt.Sprintf(`{"userid":"%s"}`, userID)
	resp, err := c.httpClient.Post(url, "application/json", strings.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("请求钉钉用户详情失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			UserID     string  `json:"userid"`
			UnionID    string  `json:"unionid"`
			Name       string  `json:"name"`
			Mobile     string  `json:"mobile"`
			Email      string  `json:"email"`
			Avatar     string  `json:"avatar"`
			Title      string  `json:"title"`
			DeptIDList []int64 `json:"dept_id_list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析钉钉响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("获取用户详情失败: %s (code: %d)", result.ErrMsg, result.ErrCode)
	}

	return &UserInfo{
		UserID:     result.Result.UserID,
		UnionID:    result.Result.UnionID,
		Name:       result.Result.Name,
		Mobile:     result.Result.Mobile,
		Email:      result.Result.Email,
		Avatar:     result.Result.Avatar,
		JobTitle:   result.Result.Title,
		DeptIDList: result.Result.DeptIDList,
	}, nil
}

// GetDepartmentList 获取子部门列表
func (c *Client) GetDepartmentList(parentID int64) ([]DeptInfo, error) {
	token, err := c.GetAccessToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s?access_token=%s", apiDeptListSub, token)
	reqBody := fmt.Sprintf(`{"dept_id":%d}`, parentID)
	resp, err := c.httpClient.Post(url, "application/json", strings.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("请求钉钉部门列表失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  []struct {
			DeptID   int64  `json:"dept_id"`
			Name     string `json:"name"`
			ParentID int64  `json:"parent_id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析钉钉响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("获取部门列表失败: %s", result.ErrMsg)
	}

	depts := make([]DeptInfo, len(result.Result))
	for i, d := range result.Result {
		depts[i] = DeptInfo{
			DeptID:   d.DeptID,
			Name:     d.Name,
			ParentID: d.ParentID,
		}
	}
	return depts, nil
}

// GetAllDepartments 递归获取完整组织架构
func (c *Client) GetAllDepartments() ([]DeptInfo, error) {
	var allDepts []DeptInfo
	err := c.fetchDepartmentsRecursive(1, &allDepts) // 1 = 根部门
	if err != nil {
		return nil, err
	}
	// 添加根部门
	allDepts = append([]DeptInfo{{DeptID: 1, Name: "根部门", ParentID: 0}}, allDepts...)
	return allDepts, nil
}

func (c *Client) fetchDepartmentsRecursive(parentID int64, allDepts *[]DeptInfo) error {
	depts, err := c.GetDepartmentList(parentID)
	if err != nil {
		return err
	}

	for _, d := range depts {
		*allDepts = append(*allDepts, d)
		if err := c.fetchDepartmentsRecursive(d.DeptID, allDepts); err != nil {
			return err
		}
	}
	return nil
}

// GetDepartmentUsers 获取部门用户列表
func (c *Client) GetDepartmentUsers(deptID int64) ([]UserInfo, error) {
	token, err := c.GetAccessToken()
	if err != nil {
		return nil, err
	}

	var allUsers []UserInfo
	cursor := int64(0)

	for {
		url := fmt.Sprintf("%s?access_token=%s", apiDeptUserList, token)
		reqBody := fmt.Sprintf(`{"dept_id":%d,"cursor":%d,"size":100}`, deptID, cursor)
		resp, err := c.httpClient.Post(url, "application/json", strings.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("请求钉钉用户列表失败: %v", err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
			Result  struct {
				HasMore    bool  `json:"has_more"`
				NextCursor int64 `json:"next_cursor"`
				List       []struct {
					UserID     string  `json:"userid"`
					UnionID    string  `json:"unionid"`
					Name       string  `json:"name"`
					Mobile     string  `json:"mobile"`
					Email      string  `json:"email"`
					Avatar     string  `json:"avatar"`
					Title      string  `json:"title"`
					DeptIDList []int64 `json:"dept_id_list"`
				} `json:"list"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("解析钉钉响应失败: %v", err)
		}

		if result.ErrCode != 0 {
			return nil, fmt.Errorf("获取部门用户失败: %s", result.ErrMsg)
		}

		for _, u := range result.Result.List {
			allUsers = append(allUsers, UserInfo{
				UserID:     u.UserID,
				UnionID:    u.UnionID,
				Name:       u.Name,
				Mobile:     u.Mobile,
				Email:      u.Email,
				Avatar:     u.Avatar,
				JobTitle:   u.Title,
				DeptIDList: u.DeptIDList,
			})
		}

		if !result.Result.HasMore {
			break
		}
		cursor = result.Result.NextCursor
	}

	return allUsers, nil
}

// SendWorkMessage 发送钉钉工作消息（异步）
// userID: 钉钉用户ID, content: 消息内容（纯文本）
func (c *Client) SendWorkMessage(userID, content string) error {
	token, err := c.GetAccessToken()
	if err != nil {
		return fmt.Errorf("获取access_token失败: %v", err)
	}

	cfg, err := c.getConfig()
	if err != nil {
		return fmt.Errorf("获取钉钉配置失败: %v", err)
	}

	if cfg.AgentID == "" {
		return fmt.Errorf("钉钉AgentID未配置")
	}

	// AgentID 需要转为整数
	agentID, err := strconv.ParseInt(strings.TrimSpace(cfg.AgentID), 10, 64)
	if err != nil {
		return fmt.Errorf("钉钉AgentID格式错误（应为数字）: %v", err)
	}

	apiURL := fmt.Sprintf("%s?access_token=%s", apiSendWorkMessage, token)

	// 构造请求体
	msgBody := map[string]interface{}{
		"agent_id":    agentID,
		"userid_list": userID,
		"msg": map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": content,
			},
		},
	}
	bodyJSON, _ := json.Marshal(msgBody)

	log.Printf("[钉钉工作消息] 发送到用户 %s, AgentID: %d", userID, agentID)

	resp, err := c.httpClient.Post(apiURL, "application/json", strings.NewReader(string(bodyJSON)))
	if err != nil {
		return fmt.Errorf("请求钉钉工作消息API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("[钉钉工作消息] 响应: %s", string(body))

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		TaskID  int64  `json:"task_id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析钉钉响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		// token 过期或无效时，清空缓存让下次重新获取
		if result.ErrCode == 40014 || result.ErrCode == 40001 || result.ErrCode == 42001 {
			log.Printf("[钉钉工作消息] Token无效(code=%d)，清空缓存", result.ErrCode)
			c.mu.Lock()
			c.accessToken = ""
			c.tokenExpire = time.Time{}
			c.mu.Unlock()
		}
		return fmt.Errorf("发送工作消息失败: %s (code: %d)", result.ErrMsg, result.ErrCode)
	}

	log.Printf("[钉钉工作消息] 发送成功, TaskID: %d", result.TaskID)
	return nil
}

// ResetToken 重置access_token
func (c *Client) ResetToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
	c.tokenExpire = time.Time{}
}

// TestConnection 测试钉钉连接
func (c *Client) TestConnection() error {
	_, err := c.GetAccessToken()
	return err
}
