package handler

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"text/template"

	"go-syncflow/internal/services"
	"go-syncflow/internal/vpn/base"
	"go-syncflow/internal/vpn/dbdata"
	"go-syncflow/internal/vpn/pkg/sms"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// LinkAuth_sms 短信验证码验证
func LinkAuth_sms(w http.ResponseWriter, r *http.Request) {
	sessionID, err := GetCookie(r, "auth-session-id")
	if err != nil {
		http.Error(w, "Invalid session, please login again", http.StatusUnauthorized)
		return
	}

	sessionData, err := SessStore.GetAuthSession(sessionID)
	if err != nil {
		http.Error(w, "Invalid session, please login again", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		base.Error(err)
		SessStore.DeleteAuthSession(sessionID)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	cr := ClientRequest{}
	err = xml.Unmarshal(body, &cr)
	if err != nil {
		base.Error(err)
		SessStore.DeleteAuthSession(sessionID)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ua := sessionData.UserActLog
	username := sessionData.ClientRequest.Auth.Username
	phone := sessionData.Phone
	smsCode := cr.Auth.SecondaryPassword

	if !lockManager.CheckLocked(username, r.RemoteAddr) {
		w.WriteHeader(http.StatusTooManyRequests)
		SessStore.DeleteAuthSession(sessionID)
		return
	}

	if !sms.VerifyCode(phone, smsCode) {
		lockManager.UpdateLoginStatus(username, r.RemoteAddr, false)

		base.Warn("短信验证码错误", username, r.RemoteAddr)
		ua.Info = "短信验证码错误"
		ua.Status = dbdata.UserAuthFail
		dbdata.UserActLogIns.Add(*ua, sessionData.ClientRequest.UserAgent)

		w.WriteHeader(http.StatusOK)
		data := RequestData{Error: "请求错误"}
		if base.Cfg.DisplayError {
			data.Error = "短信验证码错误"
		}
		tplRequestSms(w, data)
		return
	}
	CreateSession(w, r, sessionData)

	SessStore.DeleteAuthSession(sessionID)
}

// ResendSmsCode 重新发送短信验证码
func ResendSmsCode(w http.ResponseWriter, r *http.Request) {
	sessionID, err := GetCookie(r, "auth-session-id")
	if err != nil {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	sessionData, err := SessStore.GetAuthSession(sessionID)
	if err != nil {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	phone := sessionData.Phone
	if phone == "" {
		http.Error(w, "Phone not found", http.StatusBadRequest)
		return
	}

	username := sessionData.ClientRequest.Auth.Username
	authType := sessionData.AuthType

	err = sendVpnSmsCode(phone, username, authType)
	if err != nil {
		base.Error("重新发送短信失败", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	data := RequestData{}
	tplRequestSms(w, data)
}

// isConnectorOrSyncflowAuth 判断是否为连接器或 SyncFlow 认证类型
func isConnectorOrSyncflowAuth(rawAuthType string) bool {
	return rawAuthType == "syncflow" || rawAuthType == "connector" || rawAuthType == "ldap_connector"
}

// needSmsAuth 检查是否需要短信验证
func needSmsAuth(username, group string) bool {
	groupSmsEnabled := dbdata.IsGroupSmsEnabled(group)
	log.Printf("[VPN-SMS] needSmsAuth called - user: %s, group: %s, groupSmsEnabled: %v", username, group, groupSmsEnabled)
	if !groupSmsEnabled {
		log.Printf("[VPN-SMS] Group SMS not enabled, skip SMS auth")
		return false
	}

	rawAuthType := dbdata.GetGroupRawAuthType(group)
	log.Printf("[VPN-SMS] rawAuthType: %s", rawAuthType)

	if isConnectorOrSyncflowAuth(rawAuthType) {
		if dbdata.IsSyncFlowUserSmsDisabled(username) {
			log.Printf("[VPN-SMS] SyncFlow user SMS disabled")
			return false
		}
		log.Printf("[VPN-SMS] %s auth, SMS required!", rawAuthType)
		return true
	}

	config, err := dbdata.SettingGetSms()
	if err != nil || !config.Enabled {
		log.Printf("[VPN-SMS] VPN SMS service disabled or error: %v", err)
		return false
	}
	if dbdata.IsUserSmsDisabled(username) {
		return false
	}
	return true
}

// getUserPhone 获取用户手机号
func getUserPhone(username, group, authType string) string {
	rawAuthType := dbdata.GetGroupRawAuthType(group)

	if authType == "ldap" {
		phone, err := dbdata.GetLdapUserPhone(username, group)
		if err == nil && phone != "" {
			return phone
		}
	}
	if rawAuthType == "connector" || rawAuthType == "ldap_connector" {
		connectorID := dbdata.GetGroupLdapConnectorID(group)
		if connectorID > 0 {
			phone, err := dbdata.GetLdapConnectorUserPhone(username, connectorID)
			if err == nil && phone != "" {
				return phone
			}
		}
		return dbdata.GetSyncFlowUserPhone(username)
	}
	if rawAuthType == "syncflow" {
		return dbdata.GetSyncFlowUserPhone(username)
	}
	return dbdata.GetUserPhone(username)
}

// sendVpnSmsCode 发送 VPN 短信验证码
// 对于 syncflow/connector/ldap_connector 认证使用 Go-SyncFlow 短信服务，其他使用 VPN 内置短信
func sendVpnSmsCode(phone, username, authType string) error {
	if isConnectorOrSyncflowAuth(authType) {
		nickname := dbdata.GetSyncFlowUserNickname(username)
		if nickname == "" {
			nickname = username
		}
		code := sms.GenerateCode(6)
		sms.SaveCode(phone, code, 300)
		result, err := services.SendVerifyCodeSMS(phone, code, username, nickname)
		if err != nil {
			return fmt.Errorf("发送短信失败: %v", err)
		}
		base.Info("VPN 短信验证码已通过 Go-SyncFlow 发送", phone, result.ChannelType)
		return nil
	}

	config, err := dbdata.SettingGetSms()
	if err != nil {
		return fmt.Errorf("短信配置错误: %v", err)
	}
	_, err = sms.SendCode(phone, &config)
	return err
}

func tplRequestSms(w io.Writer, data RequestData) {
	t, _ := template.New("auth_sms").Parse(auth_sms)
	_ = t.Execute(w, data)
}

var auth_sms = `<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="auth-request" aggregate-auth-version="2">
    <auth id="sms-verification">
        <title>短信验证码验证</title>
        <message>请输入您收到的短信验证码</message>
        {{if .Error}}
        <error id="sms-verification" param1="{{.Error}}" param2="">验证失败:  %s</error>
        {{end}}		
        <form method="post" action="/sms-verification">
            <input type="password" name="secondary_password" label="验证码:"/>
        </form>
    </auth>
</config-auth>`
