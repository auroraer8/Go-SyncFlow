package sms

import (
	"go-syncflow/internal/vpn/dbdata"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tencentSms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

// TencentProvider 腾讯云短信发送器
type TencentProvider struct {
	config *dbdata.SettingSms
}

// NewTencentProvider 创建腾讯云短信发送器
func NewTencentProvider(config *dbdata.SettingSms) *TencentProvider {
	return &TencentProvider{config: config}
}

// Send 发送短信
func (p *TencentProvider) Send(phone, code string) error {
	credential := common.NewCredential(p.config.TencentSecretId, p.config.TencentSecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "sms.tencentcloudapi.com"
	client, err := tencentSms.NewClient(credential, "ap-guangzhou", cpf)
	if err != nil {
		return err
	}
	request := tencentSms.NewSendSmsRequest()
	request.SmsSdkAppId = common.StringPtr(p.config.TencentAppId)
	request.SignName = common.StringPtr(p.config.TencentSignName)
	request.TemplateId = common.StringPtr(p.config.TencentTemplateId)
	request.TemplateParamSet = common.StringPtrs([]string{code})
	request.PhoneNumberSet = common.StringPtrs([]string{"+86" + phone})
	_, err = client.SendSms(request)
	return err
}
