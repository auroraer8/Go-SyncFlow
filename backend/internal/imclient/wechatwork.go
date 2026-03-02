package imclient

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"go-syncflow/internal/models"
)

const (
	wxAPIGetToken   = "https://qyapi.weixin.qq.com/cgi-bin/gettoken"
	wxAPIDeptList   = "https://qyapi.weixin.qq.com/cgi-bin/department/list"
	wxAPIUserList   = "https://qyapi.weixin.qq.com/cgi-bin/user/list"
	wxAPIGetUser    = "https://qyapi.weixin.qq.com/cgi-bin/auth/getuserinfo"
	wxAPIUserDetail = "https://qyapi.weixin.qq.com/cgi-bin/user/get"
	wxAPISendMsg    = "https://qyapi.weixin.qq.com/cgi-bin/message/send"
)

// WeChatWorkClient 企业微信客户端
type WeChatWorkClient struct {
	conn        models.Connector
	accessToken string
	tokenExpire time.Time
	mu          sync.RWMutex
	httpClient  *http.Client
}

func NewWeChatWorkClient(conn models.Connector) *WeChatWorkClient {
	return &WeChatWorkClient{
		conn:       conn,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *WeChatWorkClient) PlatformType() string { return "im_wechatwork" }

func (c *WeChatWorkClient) getAccessToken() (string, error) {
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

	url := fmt.Sprintf("%s?corpid=%s&corpsecret=%s", wxAPIGetToken, c.conn.IMCorpID, c.conn.IMAppSecret)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("请求企业微信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取access_token失败: %s (code=%d)", result.ErrMsg, result.ErrCode)
	}

	c.accessToken = result.AccessToken
	c.tokenExpire = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)
	return c.accessToken, nil
}

func (c *WeChatWorkClient) TestConnection() error {
	_, err := c.getAccessToken()
	return err
}

func (c *WeChatWorkClient) GetAllDepartments() ([]IMDeptInfo, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s?access_token=%s", wxAPIDeptList, token)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求企业微信部门列表失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode    int `json:"errcode"`
		Department []struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			ParentID int    `json:"parentid"`
			Order    int    `json:"order"`
		} `json:"department"`
	}
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("获取部门列表失败: errcode=%d", result.ErrCode)
	}

	depts := make([]IMDeptInfo, 0, len(result.Department))
	for _, d := range result.Department {
		depts = append(depts, IMDeptInfo{
			DeptID:   fmt.Sprintf("%d", d.ID),
			Name:     d.Name,
			ParentID: fmt.Sprintf("%d", d.ParentID),
			Order:    d.Order,
		})
	}
	return depts, nil
}

func (c *WeChatWorkClient) GetDepartmentUsers(deptID string) ([]IMUserInfo, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s?access_token=%s&department_id=%s&fetch_child=0", wxAPIUserList, token, deptID)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求企业微信用户列表失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode  int `json:"errcode"`
		UserList []struct {
			UserID   string `json:"userid"`
			Name     string `json:"name"`
			Mobile   string `json:"mobile"`
			Email    string `json:"email"`
			Avatar   string `json:"avatar"`
			Position string `json:"position"`
			Status   int    `json:"status"`
		} `json:"userlist"`
	}
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("获取用户列表失败: errcode=%d", result.ErrCode)
	}

	users := make([]IMUserInfo, 0, len(result.UserList))
	for _, u := range result.UserList {
		users = append(users, IMUserInfo{
			UserID:   u.UserID,
			Name:     u.Name,
			Mobile:   u.Mobile,
			Email:    u.Email,
			Avatar:   u.Avatar,
			JobTitle: u.Position,
			DeptID:   deptID,
			Active:   u.Status == 1,
		})
	}
	return users, nil
}

func (c *WeChatWorkClient) GetUserByAuthCode(authCode string) (*IMUserInfo, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	// 通过 code 获取 userid
	url := fmt.Sprintf("%s?access_token=%s&code=%s", wxAPIGetUser, token, authCode)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var codeResult struct {
		ErrCode int    `json:"errcode"`
		UserID  string `json:"userid"`
	}
	json.Unmarshal(body, &codeResult)
	if codeResult.ErrCode != 0 || codeResult.UserID == "" {
		return nil, fmt.Errorf("企业微信授权码无效: errcode=%d", codeResult.ErrCode)
	}

	// 获取用户详情
	detailURL := fmt.Sprintf("%s?access_token=%s&userid=%s", wxAPIUserDetail, token, codeResult.UserID)
	resp2, err := c.httpClient.Get(detailURL)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	var detail struct {
		ErrCode  int    `json:"errcode"`
		UserID   string `json:"userid"`
		Name     string `json:"name"`
		Mobile   string `json:"mobile"`
		Email    string `json:"email"`
		Avatar   string `json:"avatar"`
		Position string `json:"position"`
		Status   int    `json:"status"`
	}
	json.Unmarshal(body2, &detail)

	return &IMUserInfo{
		UserID:   detail.UserID,
		Name:     detail.Name,
		Mobile:   detail.Mobile,
		Email:    detail.Email,
		Avatar:   detail.Avatar,
		JobTitle: detail.Position,
		Active:   detail.Status == 1,
	}, nil
}

// GetUserIDByMobile 通过手机号查询企业微信用户ID
// API: https://qyapi.weixin.qq.com/cgi-bin/user/getuserid
func (c *WeChatWorkClient) GetUserIDByMobile(mobile string) (string, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/user/getuserid?access_token=%s", token)
	reqBody := fmt.Sprintf(`{"mobile":"%s"}`, mobile)
	resp, err := c.httpClient.Post(apiURL, "application/json", strings.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("请求企业微信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		UserID  string `json:"userid"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析企业微信响应失败: %v", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("通过手机号查询企微用户失败: %s (code=%d)", result.ErrMsg, result.ErrCode)
	}
	if result.UserID == "" {
		return "", fmt.Errorf("企微中未找到手机号 %s 对应的用户", mobile)
	}
	return result.UserID, nil
}

func (c *WeChatWorkClient) SendMessage(userID string, content string) error {
	token, err := c.getAccessToken()
	if err != nil {
		return err
	}

	msgBody := map[string]interface{}{
		"touser":  userID,
		"msgtype": "text",
		"agentid": c.conn.IMAppID,
		"text": map[string]string{
			"content": content,
		},
	}
	bodyJSON, _ := json.Marshal(msgBody)

	url := fmt.Sprintf("%s?access_token=%s", wxAPISendMsg, token)
	resp, err := c.httpClient.Post(url, "application/json", strings.NewReader(string(bodyJSON)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int `json:"errcode"`
	}
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		return fmt.Errorf("发送消息失败: errcode=%d", result.ErrCode)
	}

	log.Printf("[企业微信消息] 发送成功 → %s", userID)
	return nil
}
