package imclient

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-syncflow/internal/models"
)

const (
	dtAPIGetToken        = "https://oapi.dingtalk.com/gettoken"
	dtAPIGetUserInfo     = "https://oapi.dingtalk.com/topapi/v2/user/getuserinfo"
	dtAPIGetUserDetail   = "https://oapi.dingtalk.com/topapi/v2/user/get"
	dtAPIDeptListSub     = "https://oapi.dingtalk.com/topapi/v2/department/listsub"
	dtAPIDeptUserList    = "https://oapi.dingtalk.com/topapi/v2/user/list"
	dtAPISendWorkMessage = "https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2"
)

// DingTalkClient 钉钉 IM 客户端
type DingTalkClient struct {
	conn        models.Connector
	accessToken string
	tokenExpire time.Time
	mu          sync.RWMutex
	httpClient  *http.Client
}

// NewDingTalkClient 创建钉钉客户端
func NewDingTalkClient(conn models.Connector) *DingTalkClient {
	return &DingTalkClient{
		conn: conn,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *DingTalkClient) PlatformType() string {
	return "im_dingtalk"
}

// getAccessToken 获取 AccessToken
func (c *DingTalkClient) getAccessToken() (string, error) {
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

	if c.conn.IMAppID == "" || c.conn.IMAppSecret == "" {
		return "", fmt.Errorf("钉钉 AppKey 或 AppSecret 未配置")
	}

	url := fmt.Sprintf("%s?appkey=%s&appsecret=%s", dtAPIGetToken, c.conn.IMAppID, c.conn.IMAppSecret)
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
		return "", fmt.Errorf("获取access_token失败: %s (code=%d)", result.ErrMsg, result.ErrCode)
	}

	c.accessToken = result.AccessToken
	c.tokenExpire = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	return c.accessToken, nil
}

func (c *DingTalkClient) TestConnection() error {
	_, err := c.getAccessToken()
	return err
}

func (c *DingTalkClient) GetAllDepartments() ([]IMDeptInfo, error) {
	var allDepts []IMDeptInfo
	if err := c.fetchDeptRecursive("1", &allDepts); err != nil {
		return nil, err
	}
	// 将根部门（dept_id=1）加入列表，确保直接挂在根部门下的用户也能被拉取
	// syncDeptToLocalGroup 中会自动跳过根部门，不会创建多余的本地群组
	allDepts = append([]IMDeptInfo{{DeptID: "1", Name: "根部门", ParentID: "0"}}, allDepts...)
	return allDepts, nil
}

func (c *DingTalkClient) fetchDeptRecursive(parentID string, allDepts *[]IMDeptInfo) error {
	token, err := c.getAccessToken()
	if err != nil {
		return err
	}

	pid, _ := strconv.ParseInt(parentID, 10, 64)
	url := fmt.Sprintf("%s?access_token=%s", dtAPIDeptListSub, token)
	reqBody := fmt.Sprintf(`{"dept_id":%d}`, pid)
	resp, err := c.httpClient.Post(url, "application/json", strings.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("请求钉钉部门列表失败: %v", err)
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
		return fmt.Errorf("解析钉钉响应失败: %v", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("获取部门列表失败: %s", result.ErrMsg)
	}

	for _, d := range result.Result {
		dept := IMDeptInfo{
			DeptID:   strconv.FormatInt(d.DeptID, 10),
			Name:     d.Name,
			ParentID: strconv.FormatInt(d.ParentID, 10),
		}
		*allDepts = append(*allDepts, dept)
		if err := c.fetchDeptRecursive(dept.DeptID, allDepts); err != nil {
			return err
		}
	}
	return nil
}

func (c *DingTalkClient) GetDepartmentUsers(deptID string) ([]IMUserInfo, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	did, _ := strconv.ParseInt(deptID, 10, 64)
	var allUsers []IMUserInfo
	cursor := int64(0)

	for {
		url := fmt.Sprintf("%s?access_token=%s", dtAPIDeptUserList, token)
		reqBody := fmt.Sprintf(`{"dept_id":%d,"cursor":%d,"size":100}`, did, cursor)
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
			allUsers = append(allUsers, IMUserInfo{
				UserID:   u.UserID,
				Name:     u.Name,
				Mobile:   u.Mobile,
				Email:    u.Email,
				Avatar:   u.Avatar,
				JobTitle: u.Title,
				DeptID:   deptID,
				Active:   true,
			})
		}

		if !result.Result.HasMore {
			break
		}
		cursor = result.Result.NextCursor
	}

	return allUsers, nil
}

func (c *DingTalkClient) GetUserByAuthCode(authCode string) (*IMUserInfo, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	// 1. 通过 authCode 获取 userID
	url := fmt.Sprintf("%s?access_token=%s", dtAPIGetUserInfo, token)
	reqBody := fmt.Sprintf(`{"code":"%s"}`, authCode)
	resp, err := c.httpClient.Post(url, "application/json", strings.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("请求钉钉用户信息失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var codeResult struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			UserID string `json:"userid"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &codeResult); err != nil {
		return nil, fmt.Errorf("解析钉钉响应失败: %v", err)
	}
	if codeResult.ErrCode != 0 {
		return nil, fmt.Errorf("获取用户ID失败: %s (code=%d)", codeResult.ErrMsg, codeResult.ErrCode)
	}

	// 2. 获取用户详情
	return c.getUserDetail(token, codeResult.Result.UserID)
}

func (c *DingTalkClient) getUserDetail(token, userID string) (*IMUserInfo, error) {
	url := fmt.Sprintf("%s?access_token=%s", dtAPIGetUserDetail, token)
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
			UserID  string  `json:"userid"`
			Name    string  `json:"name"`
			Mobile  string  `json:"mobile"`
			Email   string  `json:"email"`
			Avatar  string  `json:"avatar"`
			Title   string  `json:"title"`
			DeptIDs []int64 `json:"dept_id_list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析钉钉响应失败: %v", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("获取用户详情失败: %s (code=%d)", result.ErrMsg, result.ErrCode)
	}

	deptID := ""
	if len(result.Result.DeptIDs) > 0 {
		deptID = strconv.FormatInt(result.Result.DeptIDs[0], 10)
	}

	return &IMUserInfo{
		UserID:   result.Result.UserID,
		Name:     result.Result.Name,
		Mobile:   result.Result.Mobile,
		Email:    result.Result.Email,
		Avatar:   result.Result.Avatar,
		JobTitle: result.Result.Title,
		DeptID:   deptID,
		Active:   true,
	}, nil
}

// GetUserIDByMobile 通过手机号查询钉钉用户ID
// API: https://oapi.dingtalk.com/topapi/v2/user/getbymobile
func (c *DingTalkClient) GetUserIDByMobile(mobile string) (string, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("https://oapi.dingtalk.com/topapi/v2/user/getbymobile?access_token=%s", token)
	reqBody := fmt.Sprintf(`{"mobile":"%s"}`, mobile)
	resp, err := c.httpClient.Post(apiURL, "application/json", strings.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("请求钉钉API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			UserID string `json:"userid"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析钉钉响应失败: %v", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("通过手机号查询钉钉用户失败: %s (code=%d)", result.ErrMsg, result.ErrCode)
	}
	return result.Result.UserID, nil
}

func (c *DingTalkClient) SendMessage(userID string, content string) error {
	token, err := c.getAccessToken()
	if err != nil {
		return fmt.Errorf("获取access_token失败: %v", err)
	}

	if c.conn.IMAgentID == "" {
		return fmt.Errorf("钉钉 AgentID 未配置")
	}

	agentID, err := strconv.ParseInt(strings.TrimSpace(c.conn.IMAgentID), 10, 64)
	if err != nil {
		return fmt.Errorf("钉钉 AgentID 格式错误: %v", err)
	}

	apiURL := fmt.Sprintf("%s?access_token=%s", dtAPISendWorkMessage, token)
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

	resp, err := c.httpClient.Post(apiURL, "application/json", strings.NewReader(string(bodyJSON)))
	if err != nil {
		return fmt.Errorf("请求钉钉工作消息API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析钉钉响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		if result.ErrCode == 40014 || result.ErrCode == 40001 || result.ErrCode == 42001 {
			c.mu.Lock()
			c.accessToken = ""
			c.tokenExpire = time.Time{}
			c.mu.Unlock()
		}
		return fmt.Errorf("发送工作消息失败: %s (code=%d)", result.ErrMsg, result.ErrCode)
	}

	log.Printf("[钉钉消息] 发送成功 → %s", userID)
	return nil
}
