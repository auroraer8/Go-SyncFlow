<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:visible', $event)"
    title="快速接入"
    width="860px"
    :close-on-click-modal="false"
  >
    <!-- 步骤条 -->
    <el-steps :active="currentStep" finish-status="success" simple style="margin-bottom: 24px">
      <el-step title="选择模板" />
      <el-step title="基本配置" />
      <el-step title="协议设置" />
    </el-steps>

    <!-- 步骤 1: 选择模板 -->
    <div v-if="currentStep === 0" class="template-selector">
      <!-- 分类筛选 -->
      <div class="category-tabs">
        <el-radio-group v-model="selectedCategory" size="small">
          <el-radio-button value="all">全部</el-radio-button>
          <el-radio-button value="devops">DevOps</el-radio-button>
          <el-radio-button value="monitor">监控运维</el-radio-button>
          <el-radio-button value="storage">存储文档</el-radio-button>
          <el-radio-button value="cloud">云平台</el-radio-button>
          <el-radio-button value="hr">HR/CRM</el-radio-button>
          <el-radio-button value="collab">协作办公</el-radio-button>
          <el-radio-button value="custom">自定义</el-radio-button>
        </el-radio-group>
      </div>

      <!-- 搜索框 -->
      <el-input
        v-model="templateSearch"
        placeholder="搜索应用..."
        clearable
        style="margin-bottom: 16px"
        :prefix-icon="Search"
      />

      <!-- 模板网格 -->
      <div class="template-grid">
        <div
          v-for="tpl in filteredTemplates"
          :key="tpl.id"
          :class="['template-card', { 'template-card--selected': selectedTemplate === tpl.id }]"
          @click="selectTemplate(tpl)"
        >
          <div class="template-card__icon" :style="{ background: tpl.color }">
            <el-icon :size="24"><component :is="tpl.icon" /></el-icon>
          </div>
          <div class="template-card__name">{{ tpl.name }}</div>
          <div class="template-card__protocol">{{ tpl.protocol }}</div>
          <div class="template-card__desc">{{ tpl.description }}</div>
        </div>
      </div>

      <div v-if="filteredTemplates.length === 0" class="template-empty">
        没有找到匹配的应用模板
      </div>
    </div>

    <!-- 步骤 2: 基本配置 -->
    <el-form v-if="currentStep === 1" :model="form" label-width="100px" class="config-form">
      <el-form-item label="应用名称" required>
        <el-input v-model="form.name" placeholder="例如：群晖 NAS、GitLab" />
      </el-form-item>
      <el-form-item label="应用标识" required>
        <el-input v-model="form.code" placeholder="例如：synology、gitlab（小写字母、数字、下划线）">
          <template #append>
            <el-button @click="generateCode" :disabled="!form.name">自动生成</el-button>
          </template>
        </el-input>
        <div class="form-hint">应用标识创建后不可修改，用于 API 调用和 URL 路径</div>
      </el-form-item>
      <el-form-item label="描述">
        <el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选，描述该应用用途" />
      </el-form-item>
      <el-form-item label="访问控制">
        <el-radio-group v-model="form.accessMode">
          <el-radio value="all">全部用户</el-radio>
          <el-radio value="allowlist">白名单</el-radio>
          <el-radio value="role_based">基于角色</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>

    <!-- 步骤 3: 协议设置 -->
    <div v-if="currentStep === 2" class="protocol-config">
      <el-alert
        :title="getTemplateHint()"
        type="info"
        show-icon
        :closable="false"
        style="margin-bottom: 20px"
      />

      <!-- CAS 配置 -->
      <div v-if="form.protocols.includes('cas')" class="protocol-section">
        <div class="protocol-section__header">
          <el-tag type="primary">CAS</el-tag>
          <span>服务地址白名单</span>
        </div>
        <div class="protocol-section__body">
          <div v-for="(url, idx) in form.casServices" :key="'cas-' + idx" class="url-input-row">
            <el-input v-model="form.casServices[idx]" placeholder="https://your-app.example.com" />
            <el-button
              :icon="Delete"
              :disabled="form.casServices.length <= 1"
              @click="form.casServices.splice(idx, 1)"
            />
          </div>
          <el-button type="primary" link @click="form.casServices.push('')">
            <el-icon><Plus /></el-icon>
            添加地址
          </el-button>
        </div>
      </div>

      <!-- OIDC 配置 -->
      <div v-if="form.protocols.includes('oidc')" class="protocol-section">
        <div class="protocol-section__header">
          <el-tag type="success">OAuth2 / OIDC</el-tag>
          <span>客户端配置</span>
        </div>
        <div class="protocol-section__body">
          <el-form label-width="120px">
            <el-form-item label="Client ID">
              <el-input v-model="form.oidcClientId" :placeholder="form.code || 'app-code'" />
              <div class="form-hint">留空则使用应用标识</div>
            </el-form-item>
            <el-form-item label="Client Secret">
              <el-input v-model="form.oidcClientSecret" placeholder="输入密钥" show-password>
                <template #append>
                  <el-button @click="generateSecret">生成</el-button>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item label="回调地址">
              <div v-for="(url, idx) in form.oidcRedirectUris" :key="'oidc-' + idx" class="url-input-row">
                <el-input v-model="form.oidcRedirectUris[idx]" placeholder="https://app.example.com/callback" />
                <el-button
                  :icon="Delete"
                  :disabled="form.oidcRedirectUris.length <= 1"
                  @click="form.oidcRedirectUris.splice(idx, 1)"
                />
              </div>
              <el-button type="primary" link @click="form.oidcRedirectUris.push('')">
                <el-icon><Plus /></el-icon>
                添加地址
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- SAML 配置 -->
      <div v-if="form.protocols.includes('saml')" class="protocol-section">
        <div class="protocol-section__header">
          <el-tag type="warning">SAML 2.0</el-tag>
          <span>SP 配置</span>
        </div>
        <div class="protocol-section__body">
          <el-form label-width="120px">
            <el-form-item label="SP EntityID">
              <el-input v-model="form.samlSpEntityId" placeholder="https://sp.example.com" />
            </el-form-item>
            <el-form-item label="ACS URL">
              <el-input v-model="form.samlAcsUrl" placeholder="https://sp.example.com/saml/acs" />
            </el-form-item>
            <el-form-item label="NameID 格式">
              <el-select v-model="form.samlNameIdFormat" style="width: 100%">
                <el-option label="Email" value="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" />
                <el-option label="Unspecified" value="urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified" />
                <el-option label="Persistent" value="urn:oasis:names:tc:SAML:2.0:nameid-format:persistent" />
              </el-select>
            </el-form-item>
          </el-form>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button v-if="currentStep > 0" @click="currentStep--">上一步</el-button>
        <el-button v-if="currentStep < 2" type="primary" @click="nextStep" :disabled="!canNext">
          下一步
        </el-button>
        <el-button v-if="currentStep === 2" type="primary" :loading="saving" @click="createApp">
          创建应用
        </el-button>
        <el-button @click="$emit('update:visible', false)">取消</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import {
  Box,
  Cloudy,
  Connection,
  Delete,
  Document,
  Files,
  Key,
  Link,
  Lock,
  Monitor,
  OfficeBuilding,
  Plus,
  Search,
  Setting,
  Ship,
  Tools,
  User,
} from "@element-plus/icons-vue";
import { ssoAdminApi } from "../api";

const props = defineProps<{
  visible: boolean;
}>();

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  (e: "success"): void;
}>();

const currentStep = ref(0);
const selectedTemplate = ref<string | null>(null);
const saving = ref(false);
const selectedCategory = ref("all");
const templateSearch = ref("");

const templates = [
  // ========== DevOps ==========
  {
    id: "gitlab",
    name: "GitLab",
    protocol: "OIDC",
    protocols: ["oidc"],
    icon: Connection,
    color: "linear-gradient(135deg, #fc6767, #ec008c)",
    description: "代码仓库与 CI/CD 平台",
    category: "devops",
    hint: "在 GitLab Admin > Settings > General > Sign-in restrictions 中配置 OIDC",
  },
  {
    id: "jenkins",
    name: "Jenkins",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Setting,
    color: "linear-gradient(135deg, #d53369, #daae51)",
    description: "CI/CD 自动化服务",
    category: "devops",
    hint: "需安装 SAML 插件，在 Manage Jenkins > Configure Global Security 中配置",
  },
  {
    id: "harbor",
    name: "Harbor",
    protocol: "OIDC",
    protocols: ["oidc"],
    icon: Ship,
    color: "linear-gradient(135deg, #667eea, #764ba2)",
    description: "Docker 镜像仓库",
    category: "devops",
    hint: "在 Harbor 管理界面 > Configuration > Authentication 中选择 OIDC 模式",
  },
  {
    id: "sonarqube",
    name: "SonarQube",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Tools,
    color: "linear-gradient(135deg, #4facfe, #00f2fe)",
    description: "代码质量管理平台",
    category: "devops",
    hint: "在 Administration > Configuration > Security > SAML 中配置",
  },
  {
    id: "nexus",
    name: "Nexus",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Box,
    color: "linear-gradient(135deg, #0ba360, #3cba92)",
    description: "制品仓库管理",
    category: "devops",
    hint: "需安装 nexus-sso 插件支持 SAML/OIDC",
  },
  {
    id: "gitea",
    name: "Gitea",
    protocol: "OIDC",
    protocols: ["oidc"],
    icon: Connection,
    color: "linear-gradient(135deg, #56ab2f, #a8e063)",
    description: "轻量级 Git 服务",
    category: "devops",
    hint: "在 Site Administration > Authentication Sources 中添加 OAuth2 源",
  },
  {
    id: "rancher",
    name: "Rancher",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Ship,
    color: "linear-gradient(135deg, #2193b0, #6dd5ed)",
    description: "Kubernetes 管理平台",
    category: "devops",
    hint: "在 Global Settings > Authentication 中配置 Keycloak SAML",
  },

  // ========== 监控运维 ==========
  {
    id: "grafana",
    name: "Grafana",
    protocol: "OIDC",
    protocols: ["oidc"],
    icon: Monitor,
    color: "linear-gradient(135deg, #f7971e, #ffd200)",
    description: "监控与可视化平台",
    category: "monitor",
    hint: "在 grafana.ini 的 [auth.generic_oauth] 部分配置",
  },
  {
    id: "zabbix",
    name: "Zabbix",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Monitor,
    color: "linear-gradient(135deg, #c33764, #1d2671)",
    description: "企业级监控系统",
    category: "monitor",
    hint: "在 User > Authentication > SAML settings 中配置证书和端点",
  },
  {
    id: "jumpserver",
    name: "JumpServer",
    protocol: "CAS",
    protocols: ["cas"],
    icon: Lock,
    color: "linear-gradient(135deg, #11998e, #38ef7d)",
    description: "开源堡垒机",
    category: "monitor",
    hint: "在系统设置 > 认证设置 > CAS 认证中配置服务地址",
  },
  {
    id: "prometheus",
    name: "Prometheus",
    protocol: "OIDC",
    protocols: ["oidc"],
    icon: Monitor,
    color: "linear-gradient(135deg, #e53935, #ff8a65)",
    description: "监控告警系统",
    category: "monitor",
    hint: "需配合 OAuth2 Proxy 反向代理实现认证",
  },

  // ========== 存储文档 ==========
  {
    id: "synology",
    name: "群晖 NAS",
    protocol: "CAS",
    protocols: ["cas"],
    icon: Box,
    color: "linear-gradient(135deg, #ff7e5f, #feb47b)",
    description: "群晖 DSM 存储系统",
    category: "storage",
    hint: "在控制面板 > 域/LDAP > SSO 客户端中启用 CAS",
  },
  {
    id: "qnap",
    name: "威联通 QNAP",
    protocol: "LDAP",
    protocols: ["cas"],
    icon: Box,
    color: "linear-gradient(135deg, #36d1dc, #5b86e5)",
    description: "威联通 NAS 存储",
    category: "storage",
    hint: "在控制台 > 权限 > 域安全性 > LDAP 中配置",
  },
  {
    id: "nextcloud",
    name: "Nextcloud",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Cloudy,
    color: "linear-gradient(135deg, #0083B0, #00B4DB)",
    description: "开源企业网盘",
    category: "storage",
    hint: "安装 SSO & SAML 应用后在管理设置中配置",
  },
  {
    id: "seafile",
    name: "Seafile",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Files,
    color: "linear-gradient(135deg, #f5af19, #f12711)",
    description: "企业文件同步共享",
    category: "storage",
    hint: "在 seahub_settings.py 中配置 SAML 相关参数",
  },
  {
    id: "minio",
    name: "MinIO",
    protocol: "OIDC",
    protocols: ["oidc"],
    icon: Box,
    color: "linear-gradient(135deg, #c31432, #240b36)",
    description: "S3 兼容对象存储",
    category: "storage",
    hint: "设置 MINIO_IDENTITY_OPENID_* 环境变量",
  },
  {
    id: "wikijs",
    name: "Wiki.js",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Document,
    color: "linear-gradient(135deg, #1a2980, #26d0ce)",
    description: "现代知识库系统",
    category: "storage",
    hint: "在 Administration > Authentication 中添加 SAML 策略",
  },
  {
    id: "outline",
    name: "Outline",
    protocol: "OIDC",
    protocols: ["oidc"],
    icon: Document,
    color: "linear-gradient(135deg, #8360c3, #2ebf91)",
    description: "团队知识库",
    category: "storage",
    hint: "在环境变量中配置 OIDC 相关参数",
  },

  // ========== 云平台 ==========
  {
    id: "aliyun",
    name: "阿里云",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Cloudy,
    color: "linear-gradient(135deg, #ff6a00, #ee0979)",
    description: "阿里云角色 SSO 联合",
    category: "cloud",
    hint: "在 RAM 控制台 > SSO 管理 > 角色 SSO 中配置身份提供商",
    samlDefaults: { spEntityId: "urn:alibaba:cloudcomputing" },
  },
  {
    id: "tencent",
    name: "腾讯云",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Cloudy,
    color: "linear-gradient(135deg, #00c6ff, #0072ff)",
    description: "腾讯云角色 SSO 联合",
    category: "cloud",
    hint: "在 CAM 控制台 > 身份提供商中创建 SAML 提供商",
    samlDefaults: { spEntityId: "cloud.tencent.com" },
  },
  {
    id: "huawei",
    name: "华为云",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Cloudy,
    color: "linear-gradient(135deg, #e52d27, #b31217)",
    description: "华为云身份联邦",
    category: "cloud",
    hint: "在 IAM > 身份提供商中创建 SAML 提供商",
  },
  {
    id: "aws",
    name: "AWS",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Cloudy,
    color: "linear-gradient(135deg, #ff9966, #ff5e62)",
    description: "Amazon Web Services",
    category: "cloud",
    hint: "在 IAM > Identity providers 中创建 SAML provider",
    samlDefaults: { spEntityId: "urn:amazon:webservices" },
  },

  // ========== HR/CRM ==========
  {
    id: "salesforce",
    name: "Salesforce",
    protocol: "SAML",
    protocols: ["saml"],
    icon: User,
    color: "linear-gradient(135deg, #00b4db, #0083b0)",
    description: "CRM 客户关系管理",
    category: "hr",
    hint: "在 Setup > Identity > Single Sign-On Settings 中配置",
  },
  {
    id: "workday",
    name: "Workday",
    protocol: "SAML",
    protocols: ["saml"],
    icon: User,
    color: "linear-gradient(135deg, #f2709c, #ff9472)",
    description: "HR 人力资源管理",
    category: "hr",
    hint: "需通过 Workday Professional Services 配置 SAML SSO",
  },
  {
    id: "fxiaoke",
    name: "纷享销客",
    protocol: "SAML",
    protocols: ["saml"],
    icon: User,
    color: "linear-gradient(135deg, #4776e6, #8e54e9)",
    description: "CRM 销售管理",
    category: "hr",
    hint: "在企业管理后台 > 安全设置 > 单点登录中配置",
  },
  {
    id: "beisen",
    name: "北森",
    protocol: "SAML",
    protocols: ["saml"],
    icon: User,
    color: "linear-gradient(135deg, #6a3093, #a044ff)",
    description: "HR SaaS 平台",
    category: "hr",
    hint: "联系北森技术支持获取 SAML 配置参数",
  },

  // ========== 协作办公 ==========
  {
    id: "jira",
    name: "Jira",
    protocol: "SAML",
    protocols: ["saml"],
    icon: OfficeBuilding,
    color: "linear-gradient(135deg, #0052cc, #2684ff)",
    description: "项目管理与问题追踪",
    category: "collab",
    hint: "Data Center 版在 Settings > System > SAML Authentication 中配置",
  },
  {
    id: "confluence",
    name: "Confluence",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Document,
    color: "linear-gradient(135deg, #0052cc, #36b37e)",
    description: "团队协作与文档",
    category: "collab",
    hint: "Data Center 版在 General Configuration > SAML Authentication 中配置",
  },
  {
    id: "zentao",
    name: "禅道",
    protocol: "LDAP",
    protocols: ["cas"],
    icon: OfficeBuilding,
    color: "linear-gradient(135deg, #59c173, #5d26c1)",
    description: "项目管理系统",
    category: "collab",
    hint: "在后台 > 人员 > LDAP 中配置目录服务",
  },
  {
    id: "dingtalk",
    name: "钉钉",
    protocol: "OAuth",
    protocols: ["oidc"],
    icon: OfficeBuilding,
    color: "linear-gradient(135deg, #007aff, #00c6ff)",
    description: "钉钉工作台免登",
    category: "collab",
    hint: "在钉钉开放平台创建企业内部应用并配置 OAuth",
  },
  {
    id: "feishu",
    name: "飞书",
    protocol: "OIDC",
    protocols: ["oidc"],
    icon: OfficeBuilding,
    color: "linear-gradient(135deg, #3d5afe, #00e5ff)",
    description: "飞书应用免登",
    category: "collab",
    hint: "在飞书开放平台配置应用的 OIDC 登录",
  },
  {
    id: "wecom",
    name: "企业微信",
    protocol: "OAuth",
    protocols: ["oidc"],
    icon: OfficeBuilding,
    color: "linear-gradient(135deg, #07c160, #10aeff)",
    description: "企业微信扫码登录",
    category: "collab",
    hint: "在企业微信管理后台创建自建应用",
  },

  // ========== 自定义 ==========
  {
    id: "custom-cas",
    name: "自定义 CAS",
    protocol: "CAS",
    protocols: ["cas"],
    icon: Key,
    color: "linear-gradient(135deg, #667eea, #764ba2)",
    description: "其他支持 CAS 协议的应用",
    category: "custom",
    hint: "请根据目标应用的 CAS 文档配置服务地址",
  },
  {
    id: "custom-oidc",
    name: "自定义 OIDC",
    protocol: "OIDC",
    protocols: ["oidc"],
    icon: Link,
    color: "linear-gradient(135deg, #11998e, #38ef7d)",
    description: "其他支持 OAuth2/OIDC 的应用",
    category: "custom",
    hint: "请根据目标应用的 OIDC 文档配置客户端参数",
  },
  {
    id: "custom-saml",
    name: "自定义 SAML",
    protocol: "SAML",
    protocols: ["saml"],
    icon: Link,
    color: "linear-gradient(135deg, #f093fb, #f5576c)",
    description: "其他支持 SAML 2.0 的应用",
    category: "custom",
    hint: "请根据目标应用的 SAML 文档配置 SP 参数",
  },
];

const filteredTemplates = computed(() => {
  let list = templates;
  if (selectedCategory.value !== "all") {
    list = list.filter((t) => t.category === selectedCategory.value);
  }
  const keyword = templateSearch.value.trim().toLowerCase();
  if (keyword) {
    list = list.filter(
      (t) =>
        t.name.toLowerCase().includes(keyword) ||
        t.description.toLowerCase().includes(keyword) ||
        t.protocol.toLowerCase().includes(keyword)
    );
  }
  return list;
});

const form = reactive({
  name: "",
  code: "",
  description: "",
  accessMode: "all",
  protocols: [] as string[],
  casServices: [""] as string[],
  oidcClientId: "",
  oidcClientSecret: "",
  oidcRedirectUris: [""] as string[],
  samlSpEntityId: "",
  samlAcsUrl: "",
  samlNameIdFormat: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
});

const canNext = computed(() => {
  if (currentStep.value === 0) {
    return !!selectedTemplate.value;
  }
  if (currentStep.value === 1) {
    return !!form.name.trim() && !!form.code.trim();
  }
  return true;
});

function selectTemplate(tpl: any) {
  selectedTemplate.value = tpl.id;
  form.name = tpl.name.startsWith("自定义") ? "" : tpl.name;
  form.code = tpl.id.startsWith("custom") ? "" : tpl.id.replace(/-/g, "_");
  form.protocols = [...tpl.protocols];
  if (tpl.samlDefaults) {
    form.samlSpEntityId = tpl.samlDefaults.spEntityId || "";
    form.samlAcsUrl = tpl.samlDefaults.acsUrl || "";
  }
}

function nextStep() {
  if (currentStep.value === 0 && selectedTemplate.value) {
    currentStep.value = 1;
  } else if (currentStep.value === 1 && form.name && form.code) {
    currentStep.value = 2;
  }
}

function generateCode() {
  if (!form.name) return;
  const pinyin = form.name
    .toLowerCase()
    .replace(/[\u4e00-\u9fa5]/g, (char) => {
      return char;
    })
    .replace(/[^a-z0-9\u4e00-\u9fa5]/g, "_")
    .replace(/_+/g, "_")
    .replace(/^_|_$/g, "");
  form.code = pinyin || "app_" + Date.now();
}

function generateSecret() {
  const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  let secret = "";
  for (let i = 0; i < 32; i++) {
    secret += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  form.oidcClientSecret = secret;
}

function getTemplateHint(): string {
  const tpl = templates.find((t) => t.id === selectedTemplate.value) as any;
  if (!tpl) return "请配置协议参数";
  return tpl.hint || `请根据 ${tpl.name} 的文档配置以下参数`;
}

async function createApp() {
  if (!form.name.trim() || !form.code.trim()) {
    ElMessage.warning("请填写应用名称和标识");
    return;
  }

  saving.value = true;
  try {
    const created = await ssoAdminApi.createApp({
      name: form.name.trim(),
      code: form.code.trim(),
      description: form.description,
      enabled: true,
      accessMode: form.accessMode,
      claimPolicy: "",
    });
    const appId = created.data?.data?.id;

    await ssoAdminApi.putProtocols(appId, [
      {
        protocol: "cas",
        enabled: form.protocols.includes("cas"),
        config: JSON.stringify({
          services: form.casServices.map((x) => x.trim()).filter(Boolean),
        }),
      },
      {
        protocol: "oidc",
        enabled: form.protocols.includes("oidc"),
        config: JSON.stringify({
          clientId: (form.oidcClientId || form.code).trim(),
          clientSecret: form.oidcClientSecret.trim() || undefined,
          redirectUris: form.oidcRedirectUris.map((x) => x.trim()).filter(Boolean),
        }),
      },
      {
        protocol: "saml",
        enabled: form.protocols.includes("saml"),
        config: JSON.stringify({
          spEntityId: form.samlSpEntityId.trim(),
          acsUrl: form.samlAcsUrl.trim(),
          nameIdFormat: form.samlNameIdFormat.trim(),
        }),
      },
    ]);

    ElMessage.success("应用创建成功");
    emit("success");
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "创建失败");
  } finally {
    saving.value = false;
  }
}

function resetForm() {
  currentStep.value = 0;
  selectedTemplate.value = null;
  form.name = "";
  form.code = "";
  form.description = "";
  form.accessMode = "all";
  form.protocols = [];
  form.casServices = [""];
  form.oidcClientId = "";
  form.oidcClientSecret = "";
  form.oidcRedirectUris = [""];
  form.samlSpEntityId = "";
  form.samlAcsUrl = "";
  form.samlNameIdFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress";
}

watch(
  () => props.visible,
  (val) => {
    if (val) resetForm();
  }
);
</script>

<style scoped>
.template-selector {
  min-height: 400px;
}

.category-tabs {
  margin-bottom: 16px;
}

.template-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  max-height: 420px;
  overflow-y: auto;
  padding-right: 4px;
}

@media (max-width: 860px) {
  .template-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 640px) {
  .template-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.template-card {
  border: 2px solid #e5e7eb;
  border-radius: 10px;
  padding: 14px 12px;
  cursor: pointer;
  transition: all 0.2s;
  text-align: center;
  background: #fff;
}

.template-card:hover {
  border-color: #1677ff;
  box-shadow: 0 4px 12px rgba(22, 119, 255, 0.12);
  transform: translateY(-2px);
}

.template-card--selected {
  border-color: #1677ff;
  background: #f0f7ff;
}

.template-card__icon {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  margin: 0 auto 10px;
}

.template-card__name {
  font-size: 13px;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.template-card__protocol {
  font-size: 11px;
  color: #1677ff;
  margin-bottom: 6px;
}

.template-card__desc {
  font-size: 11px;
  color: #9ca3af;
  line-height: 1.3;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.template-empty {
  text-align: center;
  color: #9ca3af;
  padding: 40px 20px;
}

.config-form {
  max-width: 500px;
  margin: 0 auto;
}

.form-hint {
  font-size: 12px;
  color: #9ca3af;
  margin-top: 4px;
}

.protocol-section {
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  margin-bottom: 16px;
  overflow: hidden;
}

.protocol-section__header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  background: #f9fafb;
  border-bottom: 1px solid #e5e7eb;
  font-weight: 500;
}

.protocol-section__body {
  padding: 16px;
}

.url-input-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
