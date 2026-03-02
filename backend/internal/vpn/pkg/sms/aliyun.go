package sms

import (
	"errors"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
	"go-syncflow/internal/vpn/dbdata"
)

// AliyunProvider 阿里云短信发送器
type AliyunProvider struct {
	config *dbdata.SettingSms
}

// NewAliyunProvider 创建阿里云短信发送器
func NewAliyunProvider(config *dbdata.SettingSms) *AliyunProvider {
	return &AliyunProvider{config: config}
}

// Send 发送短信
func (p *AliyunProvider) Send(phone, code string) error {
	client, err := dysmsapi.NewClientWithAccessKey("cn-hangzhou", p.config.AliyunAccessKeyId, p.config.AliyunAccessKeySecret)
	if err != nil {
		return err
	}
	request := dysmsapi.CreateSendSmsRequest()
	request.Scheme = "https"
	request.PhoneNumbers = phone
	request.SignName = p.config.AliyunSignName
	request.TemplateCode = p.config.AliyunTemplateCode
	request.TemplateParam = `{"code":"` + code + `"}`
	response, err := client.SendSms(request)
	if err != nil {
		return err
	}
	if response.Code != "OK" {
		return errors.New(response.Message)
	}
	return nil
}
