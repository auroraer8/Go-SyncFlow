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
	fsAPITenantToken = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	fsAPIDeptList    = "https://open.feishu.cn/open-apis/contact/v3/departments"
	fsAPIUserList    = "https://open.feishu.cn/open-apis/contact/v3/users"
	fsAPIUserToken   = "https://open.feishu.cn/open-apis/authen/v1/oidc/access_token"
	fsAPIUserInfo    = "https://open.feishu.cn/open-apis/authen/v1/user_info"
	fsAPISendMsg     = "https://open.feishu.cn/open-apis/im/v1/messages"
)

// FeishuClient 飞书客户端
type FeishuClient struct {
	conn        models.Connector
	accessToken string
	tokenExpire time.Time
	mu          sync.RWMutex
	httpClient  *http.Client
}

func NewFeishuClient(conn models.Connector) *FeishuClient {
	return &FeishuClient{
		conn:       conn,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *FeishuClient) PlatformType() string { return "im_feishu" }

func (c *FeishuClient) getAccessToken() (string, error) {
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

	reqBody := fmt.Sprintf(`{"app_id":"%s","app_secret":"%s"}`, c.conn.IMAppID, c.conn.IMAppSecret)
	resp, err := c.httpClient.Post(fsAPITenantToken, "application/json", strings.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("请求飞书API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	json.Unmarshal(body, &result)
	if result.Code != 0 {
		return "", fmt.Errorf("获取tenant_access_token失败: %s (code=%d)", result.Msg, result.Code)
	}

	c.accessToken = result.TenantAccessToken
	c.tokenExpire = time.Now().Add(time.Duration(result.Expire-300) * time.Second)
	return c.accessToken, nil
}

func (c *FeishuClient) TestConnection() error {
	_, err := c.getAccessToken()
	return err
}

func (c *FeishuClient) GetAllDepartments() ([]IMDeptInfo, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	var allDepts []IMDeptInfo
	pageToken := ""

	for {
		url := fmt.Sprintf("%s?department_id_type=open_department_id&parent_department_id=0&page_size=50", fsAPIDeptList)
		if pageToken != "" {
			url += "&page_token=" + pageToken
		}

		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Code int `json:"code"`
			Data struct {
				HasMore   bool   `json:"has_more"`
				PageToken string `json:"page_token"`
				Items     []struct {
					DeptID           string `json:"open_department_id"`
					Name             string `json:"name"`
					ParentDeptID     string `json:"parent_department_id"`
					MemberCount      int    `json:"member_count"`
				} `json:"items"`
			} `json:"data"`
		}
		json.Unmarshal(body, &result)
		if result.Code != 0 {
			return nil, fmt.Errorf("获取飞书部门列表失败: code=%d", result.Code)
		}

		for _, d := range result.Data.Items {
			allDepts = append(allDepts, IMDeptInfo{
				DeptID:   d.DeptID,
				Name:     d.Name,
				ParentID: d.ParentDeptID,
			})
		}

		if !result.Data.HasMore {
			break
		}
		pageToken = result.Data.PageToken
	}

	return allDepts, nil
}

func (c *FeishuClient) GetDepartmentUsers(deptID string) ([]IMUserInfo, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	var allUsers []IMUserInfo
	pageToken := ""

	for {
		url := fmt.Sprintf("%s?department_id_type=open_department_id&department_id=%s&page_size=50", fsAPIUserList, deptID)
		if pageToken != "" {
			url += "&page_token=" + pageToken
		}

		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Code int `json:"code"`
			Data struct {
				HasMore   bool   `json:"has_more"`
				PageToken string `json:"page_token"`
				Items     []struct {
					UserID string `json:"user_id"`
					OpenID string `json:"open_id"`
					Name   string `json:"name"`
					Mobile string `json:"mobile"`
					Email  string `json:"email"`
					Avatar struct {
						URL string `json:"avatar_240"`
					} `json:"avatar"`
					JobTitle  string `json:"job_title"`
					Status    struct {
						IsFrozen    bool `json:"is_frozen"`
						IsActivated bool `json:"is_activated"`
					} `json:"status"`
				} `json:"items"`
			} `json:"data"`
		}
		json.Unmarshal(body, &result)
		if result.Code != 0 {
			return nil, fmt.Errorf("获取飞书用户列表失败: code=%d", result.Code)
		}

		for _, u := range result.Data.Items {
			uid := u.OpenID
			if u.UserID != "" {
				uid = u.UserID
			}
			allUsers = append(allUsers, IMUserInfo{
				UserID:   uid,
				Name:     u.Name,
				Mobile:   u.Mobile,
				Email:    u.Email,
				Avatar:   u.Avatar.URL,
				JobTitle: u.JobTitle,
				DeptID:   deptID,
				Active:   u.Status.IsActivated && !u.Status.IsFrozen,
			})
		}

		if !result.Data.HasMore {
			break
		}
		pageToken = result.Data.PageToken
	}

	return allUsers, nil
}

func (c *FeishuClient) GetUserByAuthCode(authCode string) (*IMUserInfo, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	// 用 code 换 user_access_token
	reqBody := fmt.Sprintf(`{"grant_type":"authorization_code","code":"%s"}`, authCode)
	req, _ := http.NewRequest("POST", fsAPIUserToken, strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tokenResult struct {
		Code int `json:"code"`
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	json.Unmarshal(body, &tokenResult)
	if tokenResult.Code != 0 {
		return nil, fmt.Errorf("飞书授权码兑换失败: code=%d", tokenResult.Code)
	}

	// 获取用户信息
	req2, _ := http.NewRequest("GET", fsAPIUserInfo, nil)
	req2.Header.Set("Authorization", "Bearer "+tokenResult.Data.AccessToken)
	resp2, err := c.httpClient.Do(req2)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	var userResult struct {
		Code int `json:"code"`
		Data struct {
			UserID string `json:"user_id"`
			OpenID string `json:"open_id"`
			Name   string `json:"name"`
			Mobile string `json:"mobile"`
			Email  string `json:"email"`
			Avatar string `json:"avatar_url"`
		} `json:"data"`
	}
	json.Unmarshal(body2, &userResult)

	uid := userResult.Data.OpenID
	if userResult.Data.UserID != "" {
		uid = userResult.Data.UserID
	}

	return &IMUserInfo{
		UserID: uid,
		Name:   userResult.Data.Name,
		Mobile: userResult.Data.Mobile,
		Email:  userResult.Data.Email,
		Avatar: userResult.Data.Avatar,
		Active: true,
	}, nil
}

// GetUserIDByMobile 通过手机号查询飞书用户open_id
// API: https://open.feishu.cn/open-apis/contact/v3/users/batch_get_id
func (c *FeishuClient) GetUserIDByMobile(mobile string) (string, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return "", err
	}

	reqBody := fmt.Sprintf(`{"mobiles":["%s"]}`, mobile)
	apiURL := "https://open.feishu.cn/open-apis/contact/v3/users/batch_get_id?user_id_type=open_id"
	req, _ := http.NewRequest("POST", apiURL, strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求飞书API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int `json:"code"`
		Data struct {
			UserList []struct {
				Mobile string `json:"mobile"`
				UserID string `json:"user_id"`
			} `json:"user_list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析飞书响应失败: %v", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("通过手机号查询飞书用户失败: code=%d", result.Code)
	}
	if len(result.Data.UserList) == 0 || result.Data.UserList[0].UserID == "" {
		return "", fmt.Errorf("飞书中未找到手机号 %s 对应的用户", mobile)
	}
	return result.Data.UserList[0].UserID, nil
}

func (c *FeishuClient) SendMessage(userID string, content string) error {
	token, err := c.getAccessToken()
	if err != nil {
		return err
	}

	msgBody := map[string]interface{}{
		"receive_id": userID,
		"msg_type":   "text",
		"content":    fmt.Sprintf(`{"text":"%s"}`, content),
	}
	bodyJSON, _ := json.Marshal(msgBody)

	url := fmt.Sprintf("%s?receive_id_type=open_id", fsAPISendMsg)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int `json:"code"`
	}
	json.Unmarshal(body, &result)
	if result.Code != 0 {
		return fmt.Errorf("发送飞书消息失败: code=%d", result.Code)
	}

	log.Printf("[飞书消息] 发送成功 → %s", userID)
	return nil
}
