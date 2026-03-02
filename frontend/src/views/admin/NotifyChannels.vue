<template>
  <div class="notify-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>通知渠道管理</span>
          <el-button type="primary" size="small" @click="openAddChannel">添加渠道</el-button>
        </div>
      </template>
      
      <el-table :data="channels" v-loading="loading" stripe>
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="channelType" label="类型" width="130">
          <template #default="{ row }">
            <el-tag size="small">{{ getChannelTypeName(row.channelType) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="isActive" label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.isActive ? 'success' : 'info'" size="small">
              {{ row.isActive ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="testResult" label="测试结果" width="120" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.testResult" :type="row.testResult === '成功' ? 'success' : 'danger'" size="small">
              {{ row.testResult }}
            </el-tag>
            <span v-else class="text-muted">未测试</span>
          </template>
        </el-table-column>
        <el-table-column prop="updatedAt" label="更新时间" width="160">
          <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="openTestDialog(row)">测试</el-button>
            <el-button type="primary" link size="small" @click="editChannel(row)">编辑</el-button>
            <el-popconfirm title="确定删除此渠道?" @confirm="deleteChannel(row.id)">
              <template #reference>
                <el-button type="danger" link size="small">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 渠道编辑对话框 -->
    <el-dialog v-model="showDialog" :title="editingId ? '编辑渠道' : '添加渠道'" width="550px" destroy-on-close>
      <el-form :model="form" label-width="100px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="渠道名称" />
        </el-form-item>
        <el-form-item label="类型" required>
          <el-select v-model="form.channelType" style="width: 100%" @change="onTypeChange" :disabled="!!editingId">
            <el-option-group label="通用">
              <el-option label="邮件" value="email" />
              <el-option label="Webhook" value="webhook" />
            </el-option-group>
            <el-option-group label="云厂商短信">
              <el-option label="阿里云短信" value="sms_aliyun" />
              <el-option label="腾讯云短信" value="sms_tencent" />
              <el-option label="华为云短信" value="sms_huawei" />
              <el-option label="百度云短信" value="sms_baidu" />
              <el-option label="天翼云短信" value="sms_ctyun" />
            </el-option-group>
            <el-option-group label="第三方短信">
              <el-option label="云片短信" value="sms_yunpian" />
              <el-option label="创蓝短信" value="sms_chuanglan" />
              <el-option label="融合云信" value="sms_ronghe" />
              <el-option label="移动云MAS" value="sms_cmcc" />
              <el-option label="移动5G消息" value="sms_cmcc_5g" />
            </el-option-group>
            <el-option-group label="IM平台">
              <el-option label="企业微信" value="sms_wecom" />
              <el-option label="钉钉" value="sms_dingtalk" />
              <el-option label="飞书" value="sms_feishu" />
              <el-option label="钉钉工作消息" value="dingtalk_work" />
            </el-option-group>
            <el-option-group label="自定义">
              <el-option label="HTTPS自定义" value="sms_https" />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.isActive" />
        </el-form-item>
        
        <!-- 邮件配置 -->
        <template v-if="form.channelType === 'email'">
          <el-divider content-position="left">SMTP配置</el-divider>
          <el-form-item label="SMTP服务器" required>
            <el-row :gutter="12">
              <el-col :span="16">
                <el-input v-model="form.config.smtp_host" placeholder="smtp.example.com" />
              </el-col>
              <el-col :span="8">
                <el-input-number v-model="form.config.smtp_port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
              </el-col>
            </el-row>
          </el-form-item>
          <el-form-item label="加密方式">
            <el-radio-group v-model="form.config.smtp_tls">
              <el-radio-button value="ssl">SSL/TLS (465)</el-radio-button>
              <el-radio-button value="starttls">STARTTLS (587)</el-radio-button>
              <el-radio-button value="none">无加密 (25)</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="用户名" required>
            <el-input v-model="form.config.smtp_user" placeholder="登录邮箱账号" />
          </el-form-item>
          <el-form-item label="密码" required>
            <el-input v-model="form.config.smtp_password" type="password" show-password placeholder="SMTP密码或授权码" />
          </el-form-item>
          <el-form-item label="发件人地址" required>
            <el-input v-model="form.config.from" placeholder="noreply@example.com" />
          </el-form-item>
          <el-form-item label="发件人名称">
            <el-input v-model="form.config.from_name" placeholder="统一身份认证平台（可选）" />
          </el-form-item>
          <el-form-item label="收件人">
            <el-input v-model="form.config.recipients" placeholder="默认收件人，多个邮箱用逗号分隔（可选，测试用）" />
          </el-form-item>
        </template>
        
        <!-- Webhook配置 -->
        <template v-if="form.channelType === 'webhook'">
          <el-divider content-position="left">Webhook配置</el-divider>
          <el-form-item label="URL" required>
            <el-input v-model="form.config.url" placeholder="https://hooks.example.com/webhook" />
          </el-form-item>
          <el-form-item label="请求方法">
            <el-radio-group v-model="form.config.method">
              <el-radio-button value="POST">POST</el-radio-button>
              <el-radio-button value="GET">GET</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="签名方式">
            <el-radio-group v-model="form.config.signType">
              <el-radio-button value="none">无签名</el-radio-button>
              <el-radio-button value="hmac_sha256">HMAC-SHA256</el-radio-button>
              <el-radio-button value="token">Token令牌</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <template v-if="form.config.signType === 'hmac_sha256'">
            <el-form-item label="签名密钥" required>
              <el-input v-model="form.config.signSecret" type="password" show-password placeholder="HMAC-SHA256 Secret" />
            </el-form-item>
            <el-form-item label="签名Header">
              <el-input v-model="form.config.signHeader" placeholder="X-Signature-256" />
            </el-form-item>
          </template>
          <template v-if="form.config.signType === 'token'">
            <el-form-item label="Token值" required>
              <el-input v-model="form.config.tokenValue" type="password" show-password placeholder="Bearer Token 或自定义令牌" />
            </el-form-item>
            <el-form-item label="Token位置">
              <el-radio-group v-model="form.config.tokenPosition">
                <el-radio-button value="header">请求头</el-radio-button>
                <el-radio-button value="query">URL参数</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="Header名称" v-if="form.config.tokenPosition === 'header'">
              <el-input v-model="form.config.tokenHeader" placeholder="Authorization" />
            </el-form-item>
          </template>
          <el-divider content-position="left">请求体模板（可选）</el-divider>
          <el-form-item label="请求体">
            <el-input v-model="form.config.bodyTemplate" type="textarea" :rows="3" placeholder='留空使用默认JSON格式，可用变量: {{message}}, {{time}}' />
          </el-form-item>
        </template>
        
        <!-- 自定义HTTPS短信 -->
        <template v-if="form.channelType === 'sms_https'">
          <el-divider content-position="left">请求配置</el-divider>
          <el-form-item label="请求地址">
            <el-input v-model="form.config.url" placeholder="https://api.example.com/sms/send" />
          </el-form-item>
          <el-row :gutter="16">
            <el-col :span="8">
              <el-form-item label="请求方法">
                <el-select v-model="form.config.method" style="width: 100%">
                  <el-option label="POST" value="POST" />
                  <el-option label="GET" value="GET" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="16">
              <el-form-item label="Content-Type">
                <el-select v-model="form.config.contentType" style="width: 100%">
                  <el-option label="application/json" value="application/json" />
                  <el-option label="application/x-www-form-urlencoded" value="application/x-www-form-urlencoded" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>
          <el-divider content-position="left">请求头</el-divider>
          <div v-for="(header, index) in form.config.headers" :key="index" class="header-row">
            <el-input v-model="header.key" placeholder="Header名" style="width: 140px" />
            <el-input v-model="header.value" placeholder="Header值" style="flex: 1; margin: 0 8px" />
            <el-button type="danger" :icon="Delete" circle size="small" @click="removeHeader(index)" />
          </div>
          <el-button type="primary" link size="small" @click="addHeader" style="margin-top: 8px">+ 添加请求头</el-button>
          
          <el-divider content-position="left">请求体模板</el-divider>
          <el-form-item label="请求体">
            <el-input 
              v-model="form.config.bodyTemplate" 
              type="textarea" 
              :rows="4"
              placeholder='{"phone": "{{phone}}", "content": "{{message}}"}'
            />
          </el-form-item>
          <el-alert type="info" :closable="false" style="margin-bottom: 16px">
            <template #default>
              可用变量：<code>{{phone}}</code> 手机号, <code>{{message}}</code> 消息内容, <code>{{time}}</code> 时间
            </template>
          </el-alert>
          <el-form-item label="接收手机号">
            <el-input v-model="form.config.phones" placeholder="多个手机号用逗号分隔（同时用于测试发送）" />
          </el-form-item>
        </template>

        <!-- 阿里云短信 -->
        <template v-if="form.channelType === 'sms_aliyun'">
          <el-divider content-position="left">阿里云短信配置</el-divider>
          <el-form-item label="AccessKeyID" required>
            <el-input v-model="form.config.accessKeyId" placeholder="阿里云 AccessKey ID" />
          </el-form-item>
          <el-form-item label="AccessKeySecret" required>
            <el-input v-model="form.config.accessKeySecret" type="password" show-password placeholder="阿里云 AccessKey Secret" />
          </el-form-item>
          <el-form-item label="短信签名" required>
            <el-input v-model="form.config.signName" placeholder="如：阿里云" />
          </el-form-item>
          <el-divider content-position="left">模板编码配置</el-divider>
          <el-form-item label="默认模板" required>
            <el-input v-model="form.config.templateCode" placeholder="默认模板编码（兜底）" />
          </el-form-item>
          <el-form-item label="验证码模板">
            <el-input v-model="form.config.templateCodeMap.verify_code" placeholder="验证码场景模板编码（可选）" />
          </el-form-item>
          <el-form-item label="密码重置模板">
            <el-input v-model="form.config.templateCodeMap.password_reset" placeholder="密码重置场景模板编码（可选）" />
          </el-form-item>
          <el-form-item label="安全告警模板">
            <el-input v-model="form.config.templateCodeMap.security_alert" placeholder="安全告警场景模板编码（可选）" />
          </el-form-item>
          <el-alert type="info" :closable="false" style="margin-bottom: 12px">
            <template #default>不同场景可使用不同的短信模板。未配置的场景将使用默认模板。</template>
          </el-alert>
          <el-form-item label="Region">
            <el-input v-model="form.config.regionId" placeholder="cn-hangzhou（默认）" />
          </el-form-item>
          <el-form-item label="测试手机号">
            <el-input v-model="form.config.phones" placeholder="用于测试发送的手机号" />
          </el-form-item>
        </template>

        <!-- 腾讯云短信 -->
        <template v-if="form.channelType === 'sms_tencent'">
          <el-divider content-position="left">腾讯云短信配置</el-divider>
          <el-form-item label="SecretId" required>
            <el-input v-model="form.config.secretId" placeholder="腾讯云 SecretId" />
          </el-form-item>
          <el-form-item label="SecretKey" required>
            <el-input v-model="form.config.secretKey" type="password" show-password placeholder="腾讯云 SecretKey" />
          </el-form-item>
          <el-form-item label="SdkAppId" required>
            <el-input v-model="form.config.sdkAppId" placeholder="短信应用ID" />
          </el-form-item>
          <el-form-item label="短信签名" required>
            <el-input v-model="form.config.signName" placeholder="如：腾讯科技" />
          </el-form-item>
          <el-divider content-position="left">模板ID配置</el-divider>
          <el-form-item label="默认模板" required>
            <el-input v-model="form.config.templateId" placeholder="默认模板ID（兜底）" />
          </el-form-item>
          <el-form-item label="验证码模板">
            <el-input v-model="form.config.templateIdMap.verify_code" placeholder="验证码场景模板ID（可选）" />
          </el-form-item>
          <el-form-item label="密码重置模板">
            <el-input v-model="form.config.templateIdMap.password_reset" placeholder="密码重置场景模板ID（可选）" />
          </el-form-item>
          <el-form-item label="安全告警模板">
            <el-input v-model="form.config.templateIdMap.security_alert" placeholder="安全告警场景模板ID（可选）" />
          </el-form-item>
          <el-alert type="info" :closable="false" style="margin-bottom: 12px">
            <template #default>不同场景可使用不同的短信模板。未配置的场景将使用默认模板。</template>
          </el-alert>
          <el-form-item label="测试手机号">
            <el-input v-model="form.config.phones" placeholder="用于测试发送的手机号" />
          </el-form-item>
        </template>

        <!-- 华为云短信 -->
        <template v-if="form.channelType === 'sms_huawei'">
          <el-divider content-position="left">华为云短信配置</el-divider>
          <el-form-item label="AppKey" required>
            <el-input v-model="form.config.appKey" placeholder="华为云短信 App Key" />
          </el-form-item>
          <el-form-item label="AppSecret" required>
            <el-input v-model="form.config.appSecret" type="password" show-password placeholder="华为云短信 App Secret" />
          </el-form-item>
          <el-form-item label="通道号" required>
            <el-input v-model="form.config.channel" placeholder="短信签名通道号" />
          </el-form-item>
          <el-form-item label="模板ID" required>
            <el-input v-model="form.config.templateId" placeholder="短信模板ID" />
          </el-form-item>
          <el-form-item label="签名名称">
            <el-input v-model="form.config.signName" placeholder="短信签名（可选）" />
          </el-form-item>
          <el-form-item label="API端点">
            <el-input v-model="form.config.endpoint" placeholder="默认: cn-north-4" />
          </el-form-item>
          <el-form-item label="测试手机号">
            <el-input v-model="form.config.phones" placeholder="用于测试发送的手机号" />
          </el-form-item>
        </template>

        <!-- 百度云短信 -->
        <template v-if="form.channelType === 'sms_baidu'">
          <el-divider content-position="left">百度云短信配置</el-divider>
          <el-form-item label="AccessKeyID" required>
            <el-input v-model="form.config.accessKeyId" placeholder="百度云 AK" />
          </el-form-item>
          <el-form-item label="SecretAccessKey" required>
            <el-input v-model="form.config.accessKeySecret" type="password" show-password placeholder="百度云 SK" />
          </el-form-item>
          <el-form-item label="InvokeId" required>
            <el-input v-model="form.config.invokeId" placeholder="业务ID" />
          </el-form-item>
          <el-form-item label="签名ID" required>
            <el-input v-model="form.config.signName" placeholder="签名ID" />
          </el-form-item>
          <el-form-item label="模板编码" required>
            <el-input v-model="form.config.templateCode" placeholder="模板编码" />
          </el-form-item>
          <el-form-item label="API端点">
            <el-input v-model="form.config.endpoint" placeholder="默认: smsv3.bj.baidubce.com" />
          </el-form-item>
          <el-form-item label="测试手机号">
            <el-input v-model="form.config.phones" placeholder="用于测试发送的手机号" />
          </el-form-item>
        </template>

        <!-- 天翼云短信 -->
        <template v-if="form.channelType === 'sms_ctyun'">
          <el-divider content-position="left">天翼云短信配置</el-divider>
          <el-form-item label="AppID" required>
            <el-input v-model="form.config.appId" placeholder="天翼云 App ID" />
          </el-form-item>
          <el-form-item label="AppSecret" required>
            <el-input v-model="form.config.appSecret" type="password" show-password placeholder="天翼云 App Secret" />
          </el-form-item>
          <el-form-item label="签名名称" required>
            <el-input v-model="form.config.signName" placeholder="短信签名" />
          </el-form-item>
          <el-form-item label="模板编码" required>
            <el-input v-model="form.config.templateCode" placeholder="模板编码" />
          </el-form-item>
          <el-form-item label="API端点">
            <el-input v-model="form.config.endpoint" placeholder="默认: sms-global.ctapi.ctyun.cn" />
          </el-form-item>
          <el-form-item label="测试手机号">
            <el-input v-model="form.config.phones" placeholder="用于测试发送的手机号" />
          </el-form-item>
        </template>

        <!-- 云片短信 -->
        <template v-if="form.channelType === 'sms_yunpian'">
          <el-divider content-position="left">云片短信配置</el-divider>
          <el-alert type="info" :closable="false" style="margin-bottom: 16px">
            <template #default>
              <a href="https://www.yunpian.com/official/document/sms/zh_cn/domestic_single_send" target="_blank">云片短信API文档</a>。
              请在云片后台获取 API Key，并完成签名和模板报备。
            </template>
          </el-alert>
          <el-form-item label="API Key" required>
            <el-input v-model="form.config.apiKey" placeholder="云片 API Key（在账号设置中查看）" />
          </el-form-item>
          <el-form-item label="短信签名" required>
            <el-input v-model="form.config.signName" placeholder="如：云片网（不含【】）" />
          </el-form-item>
          <el-alert type="warning" :closable="false" style="margin-bottom: 12px">
            <template #default>云片短信会在内容前自动添加签名，格式为：【签名】内容</template>
          </el-alert>
          <el-form-item label="测试手机号">
            <el-input v-model="form.config.phones" placeholder="用于测试发送的手机号" />
          </el-form-item>
        </template>

        <!-- 创蓝短信 -->
        <template v-if="form.channelType === 'sms_chuanglan'">
          <el-divider content-position="left">创蓝短信配置</el-divider>
          <el-alert type="info" :closable="false" style="margin-bottom: 16px">
            <template #default>
              <a href="https://doc.chuanglan.com/" target="_blank">创蓝短信API文档</a>。
              API账号和密码请登录 zz.253.com 后台获取，需完成签名和模板报备。
            </template>
          </el-alert>
          <el-form-item label="API账号" required>
            <el-input v-model="form.config.account" placeholder="创蓝 API 账号（如：N1234567）" />
          </el-form-item>
          <el-form-item label="API密码" required>
            <el-input v-model="form.config.password" type="password" show-password placeholder="创蓝 API 密码" />
          </el-form-item>
          <el-form-item label="短信签名">
            <el-input v-model="form.config.signName" placeholder="如：创蓝云智（不含【】，可选）" />
          </el-form-item>
          <el-form-item label="API端点">
            <el-input v-model="form.config.endpoint" placeholder="默认: https://smssh1.253.com/msg/send/json" />
          </el-form-item>
          <el-alert type="warning" :closable="false" style="margin-bottom: 12px">
            <template #default>不同区域可能使用不同端点，请参考创蓝文档配置正确的API地址</template>
          </el-alert>
          <el-form-item label="测试手机号">
            <el-input v-model="form.config.phones" placeholder="用于测试发送的手机号" />
          </el-form-item>
        </template>

        <!-- 融合云信 -->
        <template v-if="form.channelType === 'sms_ronghe'">
          <el-divider content-position="left">融合云信配置</el-divider>
          <el-form-item label="账号" required>
            <el-input v-model="form.config.account" placeholder="平台账号" />
          </el-form-item>
          <el-form-item label="密码" required>
            <el-input v-model="form.config.password" type="password" show-password placeholder="平台密码" />
          </el-form-item>
          <el-form-item label="签名名称">
            <el-input v-model="form.config.signName" placeholder="短信签名（可选）" />
          </el-form-item>
          <el-form-item label="API端点">
            <el-input v-model="form.config.endpoint" placeholder="默认: api.mix2.zthysms.com" />
          </el-form-item>
          <el-form-item label="测试手机号">
            <el-input v-model="form.config.phones" placeholder="用于测试发送的手机号" />
          </el-form-item>
        </template>

        <!-- 移动5G消息（CSP） -->
        <template v-if="form.channelType === 'sms_cmcc_5g'">
          <el-divider content-position="left">移动5G消息配置</el-divider>
          <el-form-item label="服务器地址" required>
            <el-input v-model="form.config.endpoint" placeholder="CSP平台地址，如: https://xxx.chinamobile.com/openapi" />
            <div class="field-hint">CSP北向接口的 serverRoot 地址</div>
          </el-form-item>
          <el-form-item label="Chatbot URI" required>
            <el-input v-model="form.config.chatbotUri" placeholder="如: sip:10086@botplatform.rcs.chinamobile.com" />
            <div class="field-hint">Chatbot 的 SIP URI</div>
          </el-form-item>
          <el-form-item label="AppID" required>
            <el-input v-model="form.config.appId" placeholder="开发者账号" />
          </el-form-item>
          <el-form-item label="开发者密码" required>
            <el-input v-model="form.config.cmcc5gPassword" type="password" show-password placeholder="开发者密码" />
          </el-form-item>
          <el-form-item label="测试手机号">
            <el-input v-model="form.config.phones" placeholder="用于测试发送的手机号" />
          </el-form-item>
        </template>

        <!-- 移动云MAS -->
        <template v-if="form.channelType === 'sms_cmcc'">
          <el-divider content-position="left">移动云MAS配置</el-divider>
          <el-form-item label="企业代码" required>
            <el-input v-model="form.config.ecId" placeholder="企业代码（ECID）" />
          </el-form-item>
          <el-form-item label="API Key" required>
            <el-input v-model="form.config.apiKey" placeholder="API Key" />
          </el-form-item>
          <el-form-item label="SecretKey" required>
            <el-input v-model="form.config.secretKeyM" type="password" show-password placeholder="Secret Key" />
          </el-form-item>
          <el-form-item label="签名名称">
            <el-input v-model="form.config.signName" placeholder="短信签名" />
          </el-form-item>
          <el-form-item label="模板ID">
            <el-input v-model="form.config.templateId" placeholder="模板ID（可选）" />
          </el-form-item>
          <el-form-item label="API端点">
            <el-input v-model="form.config.endpoint" placeholder="默认: mas.10086.cn" />
          </el-form-item>
          <el-form-item label="测试手机号">
            <el-input v-model="form.config.phones" placeholder="用于测试发送的手机号" />
          </el-form-item>
        </template>

        <!-- 企业微信 -->
        <template v-if="form.channelType === 'sms_wecom'">
          <el-divider content-position="left">企业微信配置</el-divider>
          <el-form-item label="CorpID" required>
            <el-input v-model="form.config.corpId" placeholder="企业微信 Corp ID" />
          </el-form-item>
          <el-form-item label="CorpSecret" required>
            <el-input v-model="form.config.corpSecret" type="password" show-password placeholder="应用 Secret" />
          </el-form-item>
          <el-form-item label="AgentID" required>
            <el-input v-model="form.config.agentId" placeholder="应用 Agent ID" />
          </el-form-item>
          <el-form-item label="测试用户">
            <el-input v-model="form.config.phones" placeholder="企业微信用户ID（用于测试）" />
          </el-form-item>
        </template>

        <!-- 钉钉(SMS通道) -->
        <template v-if="form.channelType === 'sms_dingtalk'">
          <el-divider content-position="left">钉钉配置</el-divider>
          <el-form-item label="AppKey" required>
            <el-input v-model="form.config.dingAppKey" placeholder="钉钉应用 AppKey" />
          </el-form-item>
          <el-form-item label="AppSecret" required>
            <el-input v-model="form.config.dingAppSecret" type="password" show-password placeholder="钉钉应用 AppSecret" />
          </el-form-item>
          <el-form-item label="AgentID" required>
            <el-input v-model="form.config.dingAgentId" placeholder="钉钉应用 Agent ID" />
          </el-form-item>
          <el-form-item label="测试用户">
            <el-input v-model="form.config.phones" placeholder="钉钉用户ID（用于测试）" />
          </el-form-item>
        </template>

        <!-- 飞书 -->
        <template v-if="form.channelType === 'sms_feishu'">
          <el-divider content-position="left">飞书配置</el-divider>
          <el-form-item label="App ID" required>
            <el-input v-model="form.config.feishuAppId" placeholder="飞书应用 App ID" />
          </el-form-item>
          <el-form-item label="App Secret" required>
            <el-input v-model="form.config.feishuAppSecret" type="password" show-password placeholder="飞书应用 App Secret" />
          </el-form-item>
          <el-form-item label="测试用户">
            <el-input v-model="form.config.phones" placeholder="飞书用户 open_id（用于测试）" />
          </el-form-item>
        </template>

        <!-- 钉钉工作消息 -->
        <template v-if="form.channelType === 'dingtalk_work'">
          <el-divider content-position="left">钉钉工作消息配置</el-divider>
          <el-alert type="info" :closable="false" style="margin-bottom: 16px">
            <template #default>
              钉钉工作消息将复用系统已配置的钉钉应用参数（AppKey、AppSecret、AgentID），无需额外配置凭证。
              请在「数据源 - 钉钉配置」页面确认钉钉应用信息已正确填写。
            </template>
          </el-alert>
          <el-form-item label="测试用户ID">
            <el-input v-model="form.config.testUserID" placeholder="钉钉用户ID（用于默认测试发送，可选）" />
          </el-form-item>
          <el-alert type="warning" :closable="false" style="margin-bottom: 16px">
            <template #default>
              测试用户ID为钉钉的 <code>userid</code>，可在钉钉管理后台查看。留空将尝试使用管理员的钉钉ID。
            </template>
          </el-alert>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" @click="saveChannel" :loading="saving">保存</el-button>
      </template>
    </el-dialog>

    <!-- 测试消息发送对话框 -->
    <el-dialog v-model="showTestDialog" title="测试消息发送" width="560px" destroy-on-close>
      <el-form :model="testForm" label-width="90px">
        <el-form-item label="通道">
          <el-tag>{{ testingChannel?.name }}</el-tag>
          <el-tag type="info" style="margin-left: 8px">{{ getChannelTypeName(testingChannel?.channelType || '') }}</el-tag>
        </el-form-item>

        <el-form-item label="收件人" required>
          <!-- 钉钉工作通知 -->
          <template v-if="testingChannel?.channelType === 'dingtalk_work'">
            <el-select
              v-model="testForm.recipient"
              filterable
              placeholder="选择或输入钉钉用户"
              style="width: 100%"
              allow-create
              default-first-option
            >
              <el-option
                v-for="u in userList"
                :key="u.id"
                :label="`${u.nickname || u.username} (${u.dingtalkUid || '未绑定'})`"
                :value="u.dingtalkUid || ''"
                :disabled="!u.dingtalkUid"
              />
            </el-select>
            <div class="form-tip">选择已绑定钉钉的用户，或直接输入钉钉UserID</div>
          </template>
          <!-- 邮件渠道 -->
          <template v-else-if="testingChannel?.channelType === 'email'">
            <el-select
              v-model="testForm.recipient"
              filterable
              placeholder="选择或输入邮箱地址"
              style="width: 100%"
              allow-create
              default-first-option
            >
              <el-option
                v-for="u in userList"
                :key="u.id"
                :label="`${u.nickname || u.username} (${u.email || '无邮箱'})`"
                :value="u.email || ''"
                :disabled="!u.email"
              />
            </el-select>
            <div class="form-tip">选择有邮箱的用户，或直接输入邮箱地址</div>
          </template>
          <!-- 企业微信应用消息 -->
          <template v-else-if="testingChannel?.channelType === 'wecom_app'">
            <el-select
              v-model="testForm.recipient"
              filterable
              placeholder="选择或输入企业微信用户ID"
              style="width: 100%"
              allow-create
              default-first-option
            >
              <el-option
                v-for="u in userList"
                :key="u.id"
                :label="`${u.nickname || u.username}`"
                :value="u.username || ''"
              />
            </el-select>
            <div class="form-tip">选择用户或直接输入企业微信UserID</div>
          </template>
          <!-- 飞书应用消息 -->
          <template v-else-if="testingChannel?.channelType === 'feishu_app'">
            <el-select
              v-model="testForm.recipient"
              filterable
              placeholder="选择或输入飞书用户open_id"
              style="width: 100%"
              allow-create
              default-first-option
            >
              <el-option
                v-for="u in userList"
                :key="u.id"
                :label="`${u.nickname || u.username}`"
                :value="u.username || ''"
              />
            </el-select>
            <div class="form-tip">选择用户或直接输入飞书open_id</div>
          </template>
          <!-- 短信及其他渠道 -->
          <template v-else>
            <el-select
              v-model="testForm.recipient"
              filterable
              placeholder="选择或输入手机号"
              style="width: 100%"
              allow-create
              default-first-option
            >
              <el-option
                v-for="u in userList"
                :key="u.id"
                :label="`${u.nickname || u.username} (${u.phone || '无手机号'})`"
                :value="u.phone || ''"
                :disabled="!u.phone"
              />
            </el-select>
            <div class="form-tip">选择有手机号的用户，或直接输入手机号</div>
          </template>
        </el-form-item>

        <el-form-item label="消息模板">
          <el-select v-model="testForm.templateId" style="width: 100%" @change="onTemplateSelect">
            <el-option label="自定义内容" :value="0" />
            <el-option
              v-for="t in msgTemplates"
              :key="t.id"
              :label="t.name"
              :value="t.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="消息内容" required>
          <el-input
            v-model="testForm.message"
            type="textarea"
            :rows="4"
            placeholder="请输入测试消息内容"
          />
        </el-form-item>

        <el-form-item label="预览">
          <div class="preview-box">{{ previewMessage }}</div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showTestDialog = false">取消</el-button>
        <el-button type="primary" @click="doTestSend" :loading="testSending">发送测试</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { Delete } from "@element-plus/icons-vue";
import { securityApi, userApi } from "../../api";

const loading = ref(false);
const saving = ref(false);
const channels = ref<any[]>([]);
const showDialog = ref(false);
const editingId = ref<number | null>(null);
const userList = ref<any[]>([]);
const msgTemplates = ref<any[]>([]);

// ========== 渠道管理 ==========

const defaultConfig = {
  smtp_host: "", smtp_port: 465, smtp_user: "", smtp_password: "", smtp_tls: "ssl",
  from: "", from_name: "", recipients: "",
  url: "", method: "POST", contentType: "application/json", headers: [] as any[], bodyTemplate: "", phones: "",
  signType: "none", signSecret: "", signHeader: "X-Signature-256", tokenValue: "", tokenPosition: "header", tokenHeader: "Authorization",
  templateCodeMap: { verify_code: "", password_reset: "", security_alert: "" },
  templateIdMap: { verify_code: "", password_reset: "", security_alert: "" },
  testUserID: "", sign: ""
};

const form = reactive<any>({
  name: "",
  channelType: "email",
  isActive: true,
  config: { ...defaultConfig }
});

const channelTypeMap: Record<string, string> = {
  email: "邮件",
  webhook: "Webhook",
  sms_aliyun: "阿里云短信",
  sms_tencent: "腾讯云短信",
  sms_huawei: "华为云短信",
  sms_baidu: "百度云短信",
  sms_ctyun: "天翼云短信",
  sms_yunpian: "云片短信",
  sms_chuanglan: "创蓝短信",
  sms_ronghe: "融合云信",
  sms_cmcc: "移动云MAS",
  sms_cmcc_5g: "移动5G消息",
  sms_wecom: "企业微信",
  sms_dingtalk: "钉钉",
  sms_feishu: "飞书",
  sms_https: "HTTPS自定义",
  sms_custom: "HTTPS自定义",
  dingtalk_work: "钉钉工作消息"
};

const getChannelTypeName = (type: string) => channelTypeMap[type] || type;

const formatTime = (time: string) => {
  if (!time) return "-";
  return new Date(time).toLocaleString("zh-CN");
};

const loadChannels = async () => {
  loading.value = true;
  try {
    const res = await securityApi.notifyChannels();
    if (res.data.success) {
      channels.value = res.data.data || [];
    }
  } finally {
    loading.value = false;
  }
};

const loadUsers = async () => {
  try {
    const res = await userApi.list({ pageSize: 500 });
    if (res.data.success) {
      userList.value = res.data.data?.list || res.data.data || [];
    }
  } catch { /* ignore */ }
};

const loadTemplates = async () => {
  try {
    const res = await securityApi.getTemplates();
    if (res.data.success) {
      msgTemplates.value = (res.data.data || []).filter((t: any) => t.isActive);
    }
  } catch { /* ignore */ }
};

const openAddChannel = () => {
  editingId.value = null;
  form.name = "";
  form.channelType = "email";
  form.isActive = true;
  form.config = { ...defaultConfig };
  showDialog.value = true;
};

const editChannel = (row: any) => {
  editingId.value = row.id;
  form.name = row.name;
  form.channelType = row.channelType;
  form.isActive = row.isActive;
  form.config = { ...defaultConfig, ...row.config };
  if (!form.config.headers) form.config.headers = [];
  showDialog.value = true;
};

const onTypeChange = () => {
  form.config = { ...defaultConfig };
};

const addHeader = () => {
  if (!form.config.headers) form.config.headers = [];
  form.config.headers.push({ key: "", value: "" });
};

const removeHeader = (index: number) => {
  form.config.headers.splice(index, 1);
};

const saveChannel = async () => {
  if (!form.name) {
    ElMessage.warning("请输入名称");
    return;
  }
  saving.value = true;
  try {
    if (editingId.value) {
      await securityApi.updateNotifyChannel(editingId.value, form);
    } else {
      await securityApi.createNotifyChannel(form);
    }
    ElMessage.success("保存成功");
    showDialog.value = false;
    loadChannels();
  } finally {
    saving.value = false;
  }
};

const deleteChannel = async (id: number) => {
  try {
    await securityApi.deleteNotifyChannel(id);
    ElMessage.success("删除成功");
    loadChannels();
  } catch (e) {
    ElMessage.error("删除失败");
  }
};

// ========== 测试发送弹窗 ==========

const showTestDialog = ref(false);
const testSending = ref(false);
const testingChannel = ref<any>(null);

const testForm = reactive({
  recipient: "",
  message: "",
  templateId: 0,
});

const openTestDialog = (row: any) => {
  testingChannel.value = row;
  testForm.recipient = "";
  testForm.templateId = 0;

  // 找到 test 模板作为默认内容
  const testTpl = msgTemplates.value.find(t => t.scene === "test");
  testForm.message = testTpl?.content || "【测试】这是一条测试消息，收到请忽略。";
  if (testTpl) testForm.templateId = testTpl.id;

  // 预填默认收件人（根据渠道类型）
  if (row.channelType?.startsWith("sms_")) {
    const phones = row.config?.phones;
    if (phones) testForm.recipient = phones.split(",")[0].trim();
  } else if (row.channelType === "dingtalk_work") {
    testForm.recipient = row.config?.testUserID || "";
  } else if (row.channelType === "email") {
    const recipients = row.config?.recipients;
    if (recipients) testForm.recipient = recipients.split(",")[0].trim();
  } else if (row.channelType === "wecom_app") {
    testForm.recipient = row.config?.phones || "";
  } else if (row.channelType === "feishu_app") {
    testForm.recipient = row.config?.phones || "";
  }

  showTestDialog.value = true;
};

const onTemplateSelect = (id: number) => {
  if (id === 0) {
    testForm.message = "";
    return;
  }
  const tpl = msgTemplates.value.find(t => t.id === id);
  if (tpl) testForm.message = tpl.content;
};

const previewMessage = computed(() => {
  let msg = testForm.message || "";
  const now = new Date().toLocaleString("zh-CN");
  msg = msg.replace(/\{\{username\}\}/g, "testuser");
  msg = msg.replace(/\{\{nickname\}\}/g, "测试用户");
  msg = msg.replace(/\{\{name\}\}/g, "张三");
  msg = msg.replace(/\{\{code\}\}/g, "283746");
  msg = msg.replace(/\{\{time\}\}/g, now);
  msg = msg.replace(/\{\{ip\}\}/g, "192.168.1.100");
  msg = msg.replace(/\{\{app_name\}\}/g, "统一身份认证平台");
  return msg;
});

const doTestSend = async () => {
  if (!testForm.recipient) {
    ElMessage.warning("请选择或输入收件人");
    return;
  }
  if (!testForm.message) {
    ElMessage.warning("请输入消息内容");
    return;
  }

  // 替换变量为预览值用于实际发送
  let msg = testForm.message;
  const now = new Date().toLocaleString("zh-CN");
  msg = msg.replace(/\{\{username\}\}/g, "testuser");
  msg = msg.replace(/\{\{nickname\}\}/g, "测试用户");
  msg = msg.replace(/\{\{name\}\}/g, "张三");
  msg = msg.replace(/\{\{code\}\}/g, String(Math.floor(100000 + Math.random() * 900000)));
  msg = msg.replace(/\{\{time\}\}/g, now);
  msg = msg.replace(/\{\{ip\}\}/g, "127.0.0.1");
  msg = msg.replace(/\{\{app_name\}\}/g, "统一身份认证平台");

  testSending.value = true;
  try {
    const res = await securityApi.testNotifyChannel(testingChannel.value.id, {
      recipient: testForm.recipient,
      message: msg,
    });
    if (res.data.success) {
      ElMessage.success("测试消息发送成功");
    } else {
      ElMessage.error(res.data.message || "发送失败");
    }
    loadChannels();
  } catch (e: any) {
    const errMsg = e?.response?.data?.message || "发送失败";
    ElMessage.error(errMsg);
    loadChannels();
  } finally {
    testSending.value = false;
  }
};

// ========== 初始化 ==========

onMounted(() => {
  loadChannels();
  loadUsers();
  loadTemplates();
});
</script>

<style scoped>
/* ============================================
   Corporate Notify Channels - 企业风格
   ============================================ */
.notify-page {
  max-width: 1100px;
  
}


.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.header-row {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
}

.text-muted {
  color: var(--color-text-tertiary);
  font-size: var(--font-size-sm);
}

.form-tip {
  font-size: var(--font-size-xs);
  color: var(--color-text-tertiary);
  margin-top: 4px;
}

.preview-box {
  background: var(--color-fill-secondary);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 10px 14px;
  font-size: var(--font-size-sm);
  color: var(--color-text-primary);
  line-height: 1.6;
  min-height: 40px;
  word-break: break-all;
}
</style>
