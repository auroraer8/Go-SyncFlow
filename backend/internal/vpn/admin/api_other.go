package admin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

	"go-syncflow/internal/vpn/base"
	"go-syncflow/internal/vpn/dbdata"
	"go-syncflow/internal/vpn/pkg/sms"
)

func setOtherGet(data interface{}, w http.ResponseWriter) {
	err := dbdata.SettingGet(data)
	if err != nil && !dbdata.CheckErrNotFound(err) {
		RespError(w, RespInternalErr, err)
		return
	}
	// 不明文输出SMTP的密码
	switch dbdata.StructName(data) {
	case "SettingSmtp":
		data.(*dbdata.SettingSmtp).Password = ""
	}
	RespSucess(w, data)
}

func setOtherEdit(data interface{}, w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	defer r.Body.Close()

	err = json.Unmarshal(body, data)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}

	// fmt.Println(data)
	switch dbdata.StructName(data) {
	case "SettingSmtp":
		// 密码为空时则不修改
		smtp := &dbdata.SettingSmtp{}
		err := dbdata.SettingGet(smtp)
		if err == nil && data.(*dbdata.SettingSmtp).Password == "" {
			data.(*dbdata.SettingSmtp).Password = smtp.Password
		}
	}
	err = dbdata.SettingSet(data)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	RespSucess(w, data)
}

func SetOtherSmtp(w http.ResponseWriter, r *http.Request) {
	data := &dbdata.SettingSmtp{}
	setOtherGet(data, w)
}

func SetOtherSmtpEdit(w http.ResponseWriter, r *http.Request) {
	data := &dbdata.SettingSmtp{}
	setOtherEdit(data, w, r)
}

func SetOther(w http.ResponseWriter, r *http.Request) {
	data := &dbdata.SettingOther{}
	setOtherGet(data, w)
}

func SetOtherEdit(w http.ResponseWriter, r *http.Request) {
	data := &dbdata.SettingOther{}
	setOtherEdit(data, w, r)
}

func SetOtherAuditLog(w http.ResponseWriter, r *http.Request) {
	data, err := dbdata.SettingGetAuditLog()
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	data.AuditInterval = base.Cfg.AuditInterval
	RespSucess(w, data)
}

func SetOtherAuditLogEdit(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	defer r.Body.Close()
	data := &dbdata.SettingAuditLog{}
	err = json.Unmarshal(body, data)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	if data.LifeDay < 0 || data.LifeDay > 365 {
		RespError(w, RespParamErr, errors.New("日志存储时长范围在 0 ~ 365"))
		return
	}
	ok, _ := regexp.Match("^([0-9]|0[0-9]|1[0-9]|2[0-3]):([0][0])$", []byte(data.ClearTime))
	if !ok {
		RespError(w, RespParamErr, errors.New("每天清理时间填写有误"))
		return
	}
	err = dbdata.SettingSet(data)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	RespSucess(w, data)
}

// SetOtherSms 获取短信配置
func SetOtherSms(w http.ResponseWriter, r *http.Request) {
	data, err := dbdata.SettingGetSms()
	if err != nil && !dbdata.CheckErrNotFound(err) {
		RespError(w, RespInternalErr, err)
		return
	}
	data.AliyunAccessKeySecret = ""
	data.TencentSecretKey = ""
	RespSucess(w, data)
}

// SetOtherSmsEdit 修改短信配置
func SetOtherSmsEdit(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	defer r.Body.Close()
	data := &dbdata.SettingSms{}
	err = json.Unmarshal(body, data)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	oldData, _ := dbdata.SettingGetSms()
	if data.AliyunAccessKeySecret == "" {
		data.AliyunAccessKeySecret = oldData.AliyunAccessKeySecret
	}
	if data.TencentSecretKey == "" {
		data.TencentSecretKey = oldData.TencentSecretKey
	}
	err = dbdata.SettingSet(data)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	RespSucess(w, data)
}

// SetOtherSmsTest 测试发送短信
func SetOtherSmsTest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	defer r.Body.Close()
	var req struct {
		Phone string `json:"phone"`
	}
	err = json.Unmarshal(body, &req)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	if req.Phone == "" {
		RespError(w, RespParamErr, errors.New("请输入手机号"))
		return
	}
	config, err := dbdata.SettingGetSms()
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	_, err = sms.SendCode(req.Phone, &config)
	if err != nil {
		RespError(w, RespInternalErr, err)
		return
	}
	RespSucess(w, nil)
}
