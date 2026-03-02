package handler

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"text/template"

	"go-syncflow/internal/vpn/base"
	"go-syncflow/internal/vpn/dbdata"
	"go-syncflow/internal/vpn/sessdata"
)

var (
	profileHash = ""
	certHash    = ""
)

func LinkAuth(w http.ResponseWriter, r *http.Request) {
	// TODO 调试信息输出
	if base.GetLogLevel() == base.LogLevelTrace {
		hd, _ := httputil.DumpRequest(r, true)
		base.Trace("LinkAuth: ", string(hd))
	}
	// 判断anyconnect客户端
	userAgent := strings.ToLower(r.UserAgent())
	xAggregateAuth := r.Header.Get("X-Aggregate-Auth")
	xTranscendVersion := r.Header.Get("X-Transcend-Version")
	if !((strings.Contains(userAgent, "anyconnect") || strings.Contains(userAgent, "openconnect") || strings.Contains(userAgent, "syncflow")) &&
		xAggregateAuth == "1" && xTranscendVersion == "1") {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "error request")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	cr := &ClientRequest{
		RemoteAddr: r.RemoteAddr,
		UserAgent:  userAgent,
	}
	err = xml.Unmarshal(body, &cr)
	if err != nil {
		base.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	base.Trace(fmt.Sprintf("%+v \n", cr))
	// setCommonHeader(w)
	if cr.Type == "logout" {
		// 退出删除session信息
		if cr.SessionToken != "" {
			sessdata.DelSessByStoken(cr.SessionToken)
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	if cr.Type == "init" {
		w.WriteHeader(http.StatusOK)
		data := RequestData{Group: cr.GroupSelect, Groups: dbdata.GetGroupNamesNormal()}
		tplRequest(tpl_request, w, data)
		return
	}

	// 登陆参数判断
	if cr.Type != "auth-reply" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 锁定状态判断
	if !lockManager.CheckLocked(cr.Auth.Username, r.RemoteAddr) {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	// 获取认证类型（带连接器名称，用于日志展示）
	authTypeDisplay := dbdata.GetGroupAuthType(cr.GroupSelect)
	// 获取原始认证类型（connector/syncflow/ldap 等，用于逻辑判断）
	rawAuthType := dbdata.GetGroupRawAuthType(cr.GroupSelect)

	// 用户活动日志
	ua := &dbdata.UserActLog{
		Username:        cr.Auth.Username,
		GroupName:       cr.GroupSelect,
		RemoteAddr:      r.RemoteAddr,
		Status:          dbdata.UserAuthSuccess,
		DeviceType:      cr.DeviceId.DeviceType,
		PlatformVersion: cr.DeviceId.PlatformVersion,
		AuthType:        authTypeDisplay,
	}

	sessionData := &AuthSession{
		ClientRequest: cr,
		UserActLog:    ua,
	}
	sessionData.AuthType = rawAuthType

	// 用户密码校验
	ext := map[string]interface{}{"mac_addr": cr.MacAddressList.MacAddress}
	err = dbdata.CheckUser(cr.Auth.Username, cr.Auth.Password, cr.GroupSelect, ext)
	if err != nil {
		lockManager.UpdateLoginStatus(cr.Auth.Username, r.RemoteAddr, false)

		base.Warn(err, r.RemoteAddr)
		ua.Info = err.Error()
		ua.Status = dbdata.UserAuthFail
		dbdata.UserActLogIns.Add(*ua, userAgent)

		w.WriteHeader(http.StatusOK)
		data := RequestData{Group: cr.GroupSelect, Groups: dbdata.GetGroupNamesNormal(), Error: "用户名或密码错误"}
		if base.Cfg.DisplayError {
			data.Error = err.Error()
		}
		tplRequest(tpl_request, w, data)
		return
	}

	dbdata.UserActLogIns.Add(*ua, userAgent)

	v := &dbdata.User{}
	err = dbdata.One("Username", cr.Auth.Username, v)
	log.Printf("[VPN-AUTH] 用户 %s 检查VPN本地用户表, err=%v", cr.Auth.Username, err)
	if err != nil {
		log.Printf("[VPN-AUTH] 用户 %s 不在VPN本地用户表，使用第三方认证", cr.Auth.Username)
		// 第三方认证也检查短信验证
		if needSmsAuth(cr.Auth.Username, cr.GroupSelect) {
			phone := getUserPhone(cr.Auth.Username, cr.GroupSelect, rawAuthType)
			if phone != "" {
				sessionData.Phone = phone
				smsErr := sendVpnSmsCode(phone, cr.Auth.Username, rawAuthType)
				if smsErr == nil {
					sessionID, _ := GenerateSessionID()
					SessStore.SaveAuthSession(sessionID, sessionData)
					SetCookie(w, "auth-session-id", sessionID, 0)
					w.WriteHeader(http.StatusOK)
					tplRequest(tpl_sms, w, RequestData{})
					return
				}
				base.Warn("发送短信验证码失败", smsErr)
			}
		}
		CreateSession(w, r, sessionData)
		return
	}

	// 检查是否需要短信验证
	if needSmsAuth(cr.Auth.Username, cr.GroupSelect) {
		phone := getUserPhone(cr.Auth.Username, cr.GroupSelect, rawAuthType)
		if phone != "" {
			sessionData.Phone = phone
			smsErr := sendVpnSmsCode(phone, cr.Auth.Username, rawAuthType)
			if smsErr == nil {
				lockManager.UpdateLoginStatus(cr.Auth.Username, r.RemoteAddr, true)
				sessionID, _ := GenerateSessionID()
				SessStore.SaveAuthSession(sessionID, sessionData)
				SetCookie(w, "auth-session-id", sessionID, 0)
				w.WriteHeader(http.StatusOK)
				tplRequest(tpl_sms, w, RequestData{})
				return
			}
			base.Warn("发送短信验证码失败", smsErr)
		}
	}

	// 用户otp验证
	if base.Cfg.AuthAloneOtp && !v.DisableOtp {
		lockManager.UpdateLoginStatus(cr.Auth.Username, r.RemoteAddr, true) // 重置OTP验证计数

		sessionID, err := GenerateSessionID()
		if err != nil {
			base.Error("Failed to generate session ID: ", err)
			http.Error(w, "Failed to generate session ID", http.StatusInternalServerError)
			return
		}

		sessionData.ClientRequest.Auth.OtpSecret = v.OtpSecret
		SessStore.SaveAuthSession(sessionID, sessionData)

		SetCookie(w, "auth-session-id", sessionID, 0)

		data := RequestData{}
		w.WriteHeader(http.StatusOK)
		tplRequest(tpl_otp, w, data)
		return
	}

	CreateSession(w, r, sessionData)
}

const (
	tpl_request = iota
	tpl_complete
	tpl_otp
	tpl_sms
)

func tplRequest(typ int, w io.Writer, data RequestData) {
	switch typ {
	case tpl_request:
		t, _ := template.New("auth_request").Parse(auth_request)
		_ = t.Execute(w, data)
	case tpl_complete:
		if data.Banner != "" {
			buf := new(bytes.Buffer)
			_ = xml.EscapeText(buf, []byte(data.Banner))
			data.Banner = buf.String()
		}
		t, _ := template.New("auth_complete").Parse(auth_complete)
		_ = t.Execute(w, data)
	case tpl_otp:
		t, _ := template.New("auth_otp").Parse(auth_otp)
		_ = t.Execute(w, data)
	case tpl_sms:
		t, _ := template.New("auth_sms").Parse(auth_sms)
		_ = t.Execute(w, data)
	}
}

// 设置输出信息
type RequestData struct {
	Groups []string
	Group  string
	Error  string

	// complete
	SessionId    string
	SessionToken string
	Banner       string
	ProfileName  string
	ProfileHash  string
	CertHash     string
}

var auth_request = `<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="auth-request" aggregate-auth-version="2">
    <opaque is-for="sg">
        <tunnel-group>{{.Group}}</tunnel-group>
        <group-alias>{{.Group}}</group-alias>
        <aggauth-handle>168179266</aggauth-handle>
        <config-hash>1595829378234</config-hash>
        <auth-method>multiple-cert</auth-method>
        <auth-method>single-sign-on-v2</auth-method>
    </opaque>
    <auth id="main">
        <title>Login</title>
        <message>请输入你的用户名和密码</message>
        <banner></banner>
        {{if .Error}}
        <error id="88" param1="{{.Error}}" param2="">登陆失败:  %s</error>
        {{end}}
        <form>
            <input type="text" name="username" label="Username:"></input>
            <input type="password" name="password" label="Password:"></input>
            <select name="group_list" label="GROUP:">
                {{range $v := .Groups}}
                <option {{if eq $v $.Group}} selected="true"{{end}}>{{$v}}</option>
                {{end}}
            </select>
        </form>
    </auth>
</config-auth>
`

var auth_complete = `<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="complete" aggregate-auth-version="2">
    <session-id>{{.SessionId}}</session-id>
    <session-token>{{.SessionToken}}</session-token>
    <auth id="success">
        <banner>{{.Banner}}</banner>
        <message id="0" param1="" param2=""></message>
    </auth>
    <capabilities>
        <crypto-supported>ssl-dhe</crypto-supported>
    </capabilities>
    <config client="vpn" type="private">
        <vpn-base-config>
            <server-cert-hash>{{.CertHash}}</server-cert-hash>
        </vpn-base-config>
        <opaque is-for="vpn-client"></opaque>
        <vpn-profile-manifest>
            <vpn rev="1.0">
                <file type="profile" service-type="user">
                    <uri>/profile_{{.ProfileName}}.xml</uri>
                    <hash type="sha1">{{.ProfileHash}}</hash>
                </file>
            </vpn>
        </vpn-profile-manifest>
    </config>
</config-auth>
`

// var auth_profile = `<?xml version="1.0" encoding="UTF-8"?>
// <AnyConnectProfile xmlns="http://schemas.xmlsoap.org/encoding/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://schemas.xmlsoap.org/encoding/ AnyConnectProfile.xsd">

// 	<ClientInitialization>
// 		<UseStartBeforeLogon UserControllable="false">false</UseStartBeforeLogon>
// 		<StrictCertificateTrust>false</StrictCertificateTrust>
// 		<RestrictPreferenceCaching>false</RestrictPreferenceCaching>
// 		<RestrictTunnelProtocols>IPSec</RestrictTunnelProtocols>
// 		<BypassDownloader>true</BypassDownloader>
// 		<WindowsVPNEstablishment>AllowRemoteUsers</WindowsVPNEstablishment>
// 		<CertEnrollmentPin>pinAllowed</CertEnrollmentPin>
// 		<CertificateMatch>
// 			<KeyUsage>
// 				<MatchKey>Digital_Signature</MatchKey>
// 			</KeyUsage>
// 			<ExtendedKeyUsage>
// 				<ExtendedMatchKey>ClientAuth</ExtendedMatchKey>
// 			</ExtendedKeyUsage>
// 		</CertificateMatch>

// 		<BackupServerList>
// 	            <HostAddress>localhost</HostAddress>
// 		</BackupServerList>
// 	</ClientInitialization>

//	<ServerList>
//		<HostEntry>
//	            <HostName>VPN Server</HostName>
//	            <HostAddress>localhost</HostAddress>
//		</HostEntry>
//	</ServerList>
//
// </AnyConnectProfile>
// `
var ds_domains_xml = `
<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="complete" aggregate-auth-version="2">
    <config client="vpn" type="private">
        <opaque is-for="vpn-client">
            <custom-attr>
            {{if .DsExcludeDomains}}
               <dynamic-split-exclude-domains><![CDATA[{{.DsExcludeDomains}},]]></dynamic-split-exclude-domains>
            {{else if .DsIncludeDomains}}
               <dynamic-split-include-domains><![CDATA[{{.DsIncludeDomains}}]]></dynamic-split-include-domains>
            {{end}}
            </custom-attr>
        </opaque>
    </config>
</config-auth>
`
