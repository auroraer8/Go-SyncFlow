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
	wlAPIToken    = "https://open.welink.huaweicloud.com/api/auth/v2/tickets"
	wlAPIDeptList = "https://open.welink.huaweicloud.com/api/contact/v1/departments"
	wlAPIUserList = "https://open.welink.huaweicloud.com/api/contact/v1/users"
	wlAPISendMsg  = "https://open.welink.huaweicloud.com/api/messages/v1/send"
)

// WeLinkClient WeLink 客户端
type WeLinkClient struct {
	conn        models.Connector
	accessToken string
	tokenExpire time.Time
	mu          sync.RWMutex
	httpClient  *http.Client
}

func NewWeLinkClient(conn models.Connector) *WeLinkClient {
	return &WeLinkClient{
		conn:       conn,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *WeLinkClient) PlatformType() string { return "im_welink" }

func (c *WeLinkClient) getAccessToken() (string, error) {
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

	reqBody := fmt.Sprintf(`{"client_id":"%s","client_secret":"%s"}`, c.conn.IMAppID, c.conn.IMAppSecret)
	resp, err := c.httpClient.Post(wlAPIToken, "application/json", strings.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("请求WeLink API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code        string `json:"code"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	json.Unmarshal(body, &result)
	if result.AccessToken == "" {
		return "", fmt.Errorf("获取WeLink access_token失败: code=%s", result.Code)
	}

	c.accessToken = result.AccessToken
	c.tokenExpire = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)
	return c.accessToken, nil
}

func (c *WeLinkClient) TestConnection() error {
	_, err := c.getAccessToken()
	return err
}

func (c *WeLinkClient) GetAllDepartments() ([]IMDeptInfo, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("GET", wlAPIDeptList+"?offset=0&limit=100", nil)
	req.Header.Set("x-wlk-Authorization", token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code string `json:"code"`
		Data []struct {
			DeptCode       string `json:"deptCode"`
			DeptName       string `json:"deptNameCn"`
			ParentDeptCode string `json:"parentDeptCode"`
			OrderNo        int    `json:"orderNo"`
		} `json:"data"`
	}
	json.Unmarshal(body, &result)

	depts := make([]IMDeptInfo, 0, len(result.Data))
	for _, d := range result.Data {
		depts = append(depts, IMDeptInfo{
			DeptID:   d.DeptCode,
			Name:     d.DeptName,
			ParentID: d.ParentDeptCode,
			Order:    d.OrderNo,
		})
	}
	return depts, nil
}

func (c *WeLinkClient) GetDepartmentUsers(deptID string) ([]IMUserInfo, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s?deptCode=%s&offset=0&limit=100", wlAPIUserList, deptID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("x-wlk-Authorization", token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code string `json:"code"`
		Data []struct {
			UserID     string `json:"userId"`
			UserNameCn string `json:"userNameCn"`
			MobileNo   string `json:"mobileNumber"`
			Email      string `json:"corpMailAddress"`
			Avatar     string `json:"userHeadImg"`
			Position   string `json:"position"`
			Status     string `json:"userStatus"`
		} `json:"data"`
	}
	json.Unmarshal(body, &result)

	users := make([]IMUserInfo, 0, len(result.Data))
	for _, u := range result.Data {
		users = append(users, IMUserInfo{
			UserID:   u.UserID,
			Name:     u.UserNameCn,
			Mobile:   u.MobileNo,
			Email:    u.Email,
			Avatar:   u.Avatar,
			JobTitle: u.Position,
			DeptID:   deptID,
			Active:   u.Status == "0", // 0=在职
		})
	}
	return users, nil
}

func (c *WeLinkClient) GetUserByAuthCode(authCode string) (*IMUserInfo, error) {
	// WeLink 不支持 SSO 免登
	return nil, fmt.Errorf("WeLink 不支持免登认证")
}

func (c *WeLinkClient) GetUserIDByMobile(mobile string) (string, error) {
	return "", fmt.Errorf("WeLink 暂不支持通过手机号查询用户")
}

func (c *WeLinkClient) SendMessage(userID string, content string) error {
	token, err := c.getAccessToken()
	if err != nil {
		return err
	}

	msgBody := map[string]interface{}{
		"toUserList": []string{userID},
		"msgType":    "text",
		"msgContent": content,
	}
	bodyJSON, _ := json.Marshal(msgBody)

	req, _ := http.NewRequest("POST", wlAPISendMsg, strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-wlk-Authorization", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code string `json:"code"`
	}
	json.Unmarshal(body, &result)
	if result.Code != "0" {
		return fmt.Errorf("发送WeLink消息失败: code=%s", result.Code)
	}

	log.Printf("[WeLink消息] 发送成功 → %s", userID)
	return nil
}
