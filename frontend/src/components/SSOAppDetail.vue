<template>
  <el-drawer
    :model-value="visible"
    @update:model-value="$emit('update:visible', $event)"
    :title="app?.name || '应用配置'"
    size="720px"
    direction="rtl"
    :close-on-click-modal="false"
  >
    <div v-loading="loading" class="app-detail">
      <!-- 顶部状态栏 -->
      <div class="app-detail__status" v-if="app">
        <div class="status-info">
          <span class="status-label">应用标识：</span>
          <el-tag>{{ app.code }}</el-tag>
          <el-tag :type="app.enabled ? 'success' : 'info'" style="margin-left: 12px">
            {{ app.enabled ? '已启用' : '已禁用' }}
          </el-tag>
        </div>
        <el-switch
          v-model="app.enabled"
          active-text="启用"
          inactive-text="禁用"
          @change="toggleEnabled"
        />
      </div>

      <!-- Tab 导航 -->
      <el-tabs v-model="activeTab" type="border-card" class="app-detail__tabs">
        <!-- 基本信息 -->
        <el-tab-pane label="基本信息" name="basic">
          <el-form :model="form" label-width="100px" class="detail-form">
            <el-form-item label="应用名称">
              <el-input v-model="form.name" placeholder="应用显示名称" />
            </el-form-item>
            <el-form-item label="描述">
              <el-input v-model="form.description" type="textarea" :rows="3" />
            </el-form-item>
            <el-form-item label="访问控制">
              <el-radio-group v-model="form.accessMode">
                <el-radio value="all">全部用户</el-radio>
                <el-radio value="allowlist">白名单</el-radio>
                <el-radio value="role_based">基于角色</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="saving" @click="saveBasic">保存</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 协议配置 -->
        <el-tab-pane label="协议配置" name="protocols">
          <div class="protocol-cards">
            <!-- CAS -->
            <div class="protocol-card" :class="{ 'protocol-card--active': protocols.cas.enabled }">
              <div class="protocol-card__header">
                <div class="protocol-card__title">
                  <el-checkbox v-model="protocols.cas.enabled" @change="saveProtocols">CAS 协议</el-checkbox>
                </div>
                <el-tag size="small">适用于群晖等</el-tag>
              </div>
              <el-collapse-transition>
                <div v-if="protocols.cas.enabled" class="protocol-card__body">
                  <div class="field-label">服务地址白名单</div>
                  <div v-for="(url, idx) in protocols.cas.services" :key="'cas-' + idx" class="url-row">
                    <el-input v-model="protocols.cas.services[idx]" placeholder="https://..." />
                    <el-button :icon="Delete" :disabled="protocols.cas.services.length <= 1" @click="protocols.cas.services.splice(idx, 1)" />
                  </div>
                  <el-button link type="primary" @click="protocols.cas.services.push('')">
                    <el-icon><Plus /></el-icon> 添加
                  </el-button>

                  <el-divider />
                  <div class="field-label">接入信息（复制到第三方系统）</div>
                  <div class="endpoint-list">
                    <div class="endpoint-item">
                      <span class="endpoint-label">登录地址</span>
                      <el-input :model-value="endpoints?.cas?.login" readonly>
                        <template #append>
                          <el-button @click="copyText(endpoints?.cas?.login)">复制</el-button>
                        </template>
                      </el-input>
                    </div>
                    <div class="endpoint-item">
                      <span class="endpoint-label">验证地址</span>
                      <el-input :model-value="endpoints?.cas?.serviceValidate" readonly>
                        <template #append>
                          <el-button @click="copyText(endpoints?.cas?.serviceValidate)">复制</el-button>
                        </template>
                      </el-input>
                    </div>
                    <div class="endpoint-item">
                      <span class="endpoint-label">登出地址</span>
                      <el-input :model-value="endpoints?.cas?.logout" readonly>
                        <template #append>
                          <el-button @click="copyText(endpoints?.cas?.logout)">复制</el-button>
                        </template>
                      </el-input>
                    </div>
                  </div>

                  <el-button type="primary" style="margin-top: 16px" :loading="saving" @click="saveProtocols">保存 CAS 配置</el-button>
                </div>
              </el-collapse-transition>
            </div>

            <!-- OIDC -->
            <div class="protocol-card" :class="{ 'protocol-card--active': protocols.oidc.enabled }">
              <div class="protocol-card__header">
                <div class="protocol-card__title">
                  <el-checkbox v-model="protocols.oidc.enabled" @change="saveProtocols">OAuth2 / OIDC</el-checkbox>
                </div>
                <el-tag size="small" type="success">推荐</el-tag>
              </div>
              <el-collapse-transition>
                <div v-if="protocols.oidc.enabled" class="protocol-card__body">
                  <el-form label-width="120px">
                    <el-form-item label="Client ID">
                      <el-input v-model="protocols.oidc.clientId" :placeholder="app?.code" />
                    </el-form-item>
                    <el-form-item label="Client Secret">
                      <el-input v-model="protocols.oidc.clientSecret" show-password placeholder="输入新密钥（留空不修改）">
                        <template #append>
                          <el-button @click="generateSecret">生成</el-button>
                        </template>
                      </el-input>
                      <div class="form-hint" v-if="protocols.oidc.clientSecretHash">当前已配置密钥</div>
                    </el-form-item>
                    <el-form-item label="回调地址">
                      <div v-for="(url, idx) in protocols.oidc.redirectUris" :key="'oidc-' + idx" class="url-row">
                        <el-input v-model="protocols.oidc.redirectUris[idx]" placeholder="https://app/callback" />
                        <el-button :icon="Delete" :disabled="protocols.oidc.redirectUris.length <= 1" @click="protocols.oidc.redirectUris.splice(idx, 1)" />
                      </div>
                      <el-button link type="primary" @click="protocols.oidc.redirectUris.push('')">
                        <el-icon><Plus /></el-icon> 添加
                      </el-button>
                    </el-form-item>
                  </el-form>

                  <el-divider />
                  <div class="field-label">接入信息（复制到第三方系统）</div>
                  <div class="endpoint-list">
                    <div class="endpoint-item">
                      <span class="endpoint-label">Issuer</span>
                      <el-input :model-value="endpoints?.issuer" readonly>
                        <template #append>
                          <el-button @click="copyText(endpoints?.issuer)">复制</el-button>
                        </template>
                      </el-input>
                    </div>
                    <div class="endpoint-item">
                      <span class="endpoint-label">Discovery</span>
                      <el-input :model-value="endpoints?.oidc?.discovery" readonly>
                        <template #append>
                          <el-button @click="copyText(endpoints?.oidc?.discovery)">复制</el-button>
                        </template>
                      </el-input>
                    </div>
                    <div class="endpoint-item">
                      <span class="endpoint-label">JWKS</span>
                      <el-input :model-value="endpoints?.oidc?.jwks" readonly>
                        <template #append>
                          <el-button @click="copyText(endpoints?.oidc?.jwks)">复制</el-button>
                        </template>
                      </el-input>
                    </div>
                  </div>

                  <el-button type="primary" style="margin-top: 16px" :loading="saving" @click="saveProtocols">保存 OIDC 配置</el-button>
                </div>
              </el-collapse-transition>
            </div>

            <!-- SAML -->
            <div class="protocol-card" :class="{ 'protocol-card--active': protocols.saml.enabled }">
              <div class="protocol-card__header">
                <div class="protocol-card__title">
                  <el-checkbox v-model="protocols.saml.enabled" @change="saveProtocols">SAML 2.0</el-checkbox>
                </div>
                <el-tag size="small" type="warning">企业级</el-tag>
              </div>
              <el-collapse-transition>
                <div v-if="protocols.saml.enabled" class="protocol-card__body">
                  <div class="quick-templates">
                    <span>快速配置：</span>
                    <el-button size="small" @click="applySamlTemplate('aliyun')">阿里云</el-button>
                    <el-button size="small" @click="applySamlTemplate('tencent')">腾讯云</el-button>
                  </div>

                  <el-form label-width="120px" style="margin-top: 16px">
                    <el-form-item label="SP EntityID">
                      <el-input v-model="protocols.saml.spEntityId" placeholder="https://sp.example.com" />
                    </el-form-item>
                    <el-form-item label="ACS URL">
                      <el-input v-model="protocols.saml.acsUrl" placeholder="https://sp.example.com/saml/acs" />
                    </el-form-item>
                    <el-form-item label="NameID 格式">
                      <el-select v-model="protocols.saml.nameIdFormat" style="width: 100%">
                        <el-option label="Email" value="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" />
                        <el-option label="Unspecified" value="urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified" />
                        <el-option label="Persistent" value="urn:oasis:names:tc:SAML:2.0:nameid-format:persistent" />
                      </el-select>
                    </el-form-item>
                  </el-form>

                  <el-divider />
                  <div class="field-label">IdP 信息（复制到第三方系统）</div>
                  <div class="endpoint-list">
                    <div class="endpoint-item">
                      <span class="endpoint-label">IdP EntityID</span>
                      <el-input :model-value="getSamlEndpoint('entityId')" readonly>
                        <template #append>
                          <el-button @click="copyText(getSamlEndpoint('entityId'))">复制</el-button>
                        </template>
                      </el-input>
                    </div>
                    <div class="endpoint-item">
                      <span class="endpoint-label">SSO URL</span>
                      <el-input :model-value="getSamlEndpoint('sso')" readonly>
                        <template #append>
                          <el-button @click="copyText(getSamlEndpoint('sso'))">复制</el-button>
                        </template>
                      </el-input>
                    </div>
                  </div>
                  <div class="download-buttons">
                    <el-button @click="downloadMetadata">下载 IdP 元数据 XML</el-button>
                    <el-button @click="downloadCertificate">下载 X.509 证书</el-button>
                  </div>

                  <el-button type="primary" style="margin-top: 16px" :loading="saving" @click="saveProtocols">保存 SAML 配置</el-button>
                </div>
              </el-collapse-transition>
            </div>
          </div>
        </el-tab-pane>

        <!-- 访问授权 -->
        <el-tab-pane label="访问授权" name="access">
          <div class="access-section">
            <el-alert
              title="授权说明"
              :description="getAccessModeDesc()"
              type="info"
              show-icon
              :closable="false"
              style="margin-bottom: 20px"
            />

            <div v-if="form.accessMode !== 'all'" class="grant-list">
              <div class="grant-list__header">
                <span>已授权列表</span>
                <el-button type="primary" size="small" @click="showAddGrant = true">
                  <el-icon><Plus /></el-icon> 添加授权
                </el-button>
              </div>
              <el-table :data="grants" stripe empty-text="暂无授权">
                <el-table-column label="类型" width="100">
                  <template #default="{ row }">
                    <el-tag size="small">{{ row.subjectType === 'role' ? '角色' : '用户' }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="名称" prop="subjectName" />
                <el-table-column label="状态" width="100">
                  <template #default="{ row }">
                    <el-switch v-model="row.enabled" @change="saveGrants" />
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="80">
                  <template #default="{ $index }">
                    <el-button link type="danger" @click="grants.splice($index, 1); saveGrants()">移除</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
        </el-tab-pane>

        <!-- 高级设置 -->
        <el-tab-pane label="高级设置" name="advanced">
          <el-collapse>
            <el-collapse-item title="属性映射" name="mappings">
              <div class="mapping-section">
                <p class="section-desc">配置用户属性到 SSO 断言的映射规则</p>
                <el-table :data="mappings" stripe empty-text="使用默认映射">
                  <el-table-column label="类型" width="120">
                    <template #default="{ row }">
                      <el-select v-model="row.type" size="small">
                        <el-option label="角色" value="role" />
                        <el-option label="属性" value="attr" />
                      </el-select>
                    </template>
                  </el-table-column>
                  <el-table-column label="源" min-width="150">
                    <template #default="{ row }">
                      <el-input v-model="row.source" size="small" placeholder="user.role" />
                    </template>
                  </el-table-column>
                  <el-table-column label="目标" min-width="150">
                    <template #default="{ row }">
                      <el-input v-model="row.target" size="small" placeholder="app_role" />
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="80">
                    <template #default="{ $index }">
                      <el-button link type="danger" size="small" @click="mappings.splice($index, 1)">删除</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <el-button link type="primary" style="margin-top: 8px" @click="mappings.push({ type: 'attr', source: '', target: '' })">
                  <el-icon><Plus /></el-icon> 添加映射
                </el-button>
                <div style="margin-top: 16px">
                  <el-button type="primary" :loading="saving" @click="saveMappings">保存映射</el-button>
                </div>
              </div>
            </el-collapse-item>
            <el-collapse-item title="负责人设置" name="owners">
              <div class="owner-section">
                <p class="section-desc">指定可以管理此应用配置的用户或角色</p>
                <el-form label-width="80px">
                  <el-form-item label="角色">
                    <el-select v-model="ownerRoleIds" multiple filterable style="width: 100%">
                      <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
                    </el-select>
                  </el-form-item>
                </el-form>
                <el-button type="primary" :loading="saving" @click="saveOwners">保存</el-button>
              </div>
            </el-collapse-item>
          </el-collapse>
        </el-tab-pane>
      </el-tabs>
    </div>

    <!-- 添加授权对话框 -->
    <el-dialog v-model="showAddGrant" title="添加授权" width="400px" append-to-body>
      <el-form label-width="80px">
        <el-form-item label="类型">
          <el-radio-group v-model="newGrant.subjectType">
            <el-radio value="role">角色</el-radio>
            <el-radio value="user">用户</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="选择">
          <el-select
            v-if="newGrant.subjectType === 'role'"
            v-model="newGrant.subjectId"
            filterable
            style="width: 100%"
          >
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
          <el-select
            v-else
            v-model="newGrant.subjectId"
            filterable
            remote
            :remote-method="searchUsers"
            :loading="usersLoading"
            style="width: 100%"
          >
            <el-option v-for="u in userOptions" :key="u.id" :label="u.username" :value="u.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddGrant = false">取消</el-button>
        <el-button type="primary" @click="addGrant">确定</el-button>
      </template>
    </el-dialog>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { Delete, Plus } from "@element-plus/icons-vue";
import { roleApi, ssoAdminApi, userApi } from "../api";

const props = defineProps<{
  visible: boolean;
  appId: number | null;
}>();

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  (e: "updated"): void;
}>();

const loading = ref(false);
const saving = ref(false);
const activeTab = ref("basic");

const app = ref<any>(null);
const endpoints = ref<any>(null);

const form = reactive({
  name: "",
  description: "",
  accessMode: "all",
});

const protocols = reactive({
  cas: {
    enabled: false,
    services: [""] as string[],
  },
  oidc: {
    enabled: false,
    clientId: "",
    clientSecret: "",
    clientSecretHash: "",
    redirectUris: [""] as string[],
  },
  saml: {
    enabled: false,
    spEntityId: "",
    acsUrl: "",
    nameIdFormat: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
  },
});

const grants = ref<any[]>([]);
const mappings = ref<any[]>([]);
const roles = ref<any[]>([]);
const ownerRoleIds = ref<number[]>([]);

const showAddGrant = ref(false);
const newGrant = reactive({ subjectType: "role", subjectId: 0 });
const usersLoading = ref(false);
const userOptions = ref<any[]>([]);

async function loadApp() {
  if (!props.appId) return;
  loading.value = true;
  try {
    const [appRes, overviewRes] = await Promise.all([
      ssoAdminApi.listApps({ keyword: "" }),
      ssoAdminApi.overview(),
    ]);
    const list = appRes.data?.data?.list || [];
    app.value = list.find((a: any) => a.id === props.appId) || null;
    endpoints.value = overviewRes.data?.data || null;

    if (app.value) {
      form.name = app.value.name;
      form.description = app.value.description || "";
      form.accessMode = app.value.accessMode || "all";
      parseProtocols(app.value.protocols || []);
    }

    await Promise.all([loadGrants(), loadMappings(), loadOwners()]);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "加载失败");
  } finally {
    loading.value = false;
  }
}

function parseProtocols(prots: any[]) {
  protocols.cas.enabled = false;
  protocols.cas.services = [""];
  protocols.oidc.enabled = false;
  protocols.oidc.clientId = "";
  protocols.oidc.clientSecret = "";
  protocols.oidc.clientSecretHash = "";
  protocols.oidc.redirectUris = [""];
  protocols.saml.enabled = false;
  protocols.saml.spEntityId = "";
  protocols.saml.acsUrl = "";
  protocols.saml.nameIdFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress";

  for (const p of prots) {
    if (p.protocol === "cas" && p.enabled) {
      protocols.cas.enabled = true;
      try {
        const cfg = JSON.parse(p.config || "{}");
        protocols.cas.services = cfg.services?.length ? cfg.services : [""];
      } catch {}
    }
    if (p.protocol === "oidc" && p.enabled) {
      protocols.oidc.enabled = true;
      try {
        const cfg = JSON.parse(p.config || "{}");
        protocols.oidc.clientId = cfg.clientId || "";
        protocols.oidc.clientSecretHash = cfg.clientSecretHash || "";
        protocols.oidc.redirectUris = cfg.redirectUris?.length ? cfg.redirectUris : [""];
      } catch {}
    }
    if (p.protocol === "saml" && p.enabled) {
      protocols.saml.enabled = true;
      try {
        const cfg = JSON.parse(p.config || "{}");
        protocols.saml.spEntityId = cfg.spEntityId || "";
        protocols.saml.acsUrl = cfg.acsUrl || "";
        protocols.saml.nameIdFormat = cfg.nameIdFormat || "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress";
      } catch {}
    }
  }
}

async function saveBasic() {
  if (!props.appId) return;
  saving.value = true;
  try {
    await ssoAdminApi.updateApp(props.appId, {
      name: form.name.trim(),
      description: form.description,
      enabled: app.value?.enabled ?? true,
      accessMode: form.accessMode,
      claimPolicy: "",
    });
    ElMessage.success("已保存");
    emit("updated");
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "保存失败");
  } finally {
    saving.value = false;
  }
}

async function toggleEnabled() {
  if (!props.appId || !app.value) return;
  try {
    await ssoAdminApi.updateApp(props.appId, {
      name: app.value.name,
      description: app.value.description,
      enabled: app.value.enabled,
      accessMode: app.value.accessMode,
      claimPolicy: "",
    });
    ElMessage.success(app.value.enabled ? "已启用" : "已禁用");
    emit("updated");
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "操作失败");
  }
}

async function saveProtocols() {
  if (!props.appId) return;
  saving.value = true;
  try {
    await ssoAdminApi.putProtocols(props.appId, [
      {
        protocol: "cas",
        enabled: protocols.cas.enabled,
        config: JSON.stringify({ services: protocols.cas.services.filter(Boolean) }),
      },
      {
        protocol: "oidc",
        enabled: protocols.oidc.enabled,
        config: JSON.stringify({
          clientId: protocols.oidc.clientId || app.value?.code,
          clientSecret: protocols.oidc.clientSecret || undefined,
          clientSecretHash: protocols.oidc.clientSecretHash || undefined,
          redirectUris: protocols.oidc.redirectUris.filter(Boolean),
        }),
      },
      {
        protocol: "saml",
        enabled: protocols.saml.enabled,
        config: JSON.stringify({
          spEntityId: protocols.saml.spEntityId,
          acsUrl: protocols.saml.acsUrl,
          nameIdFormat: protocols.saml.nameIdFormat,
        }),
      },
    ]);
    ElMessage.success("已保存");
    emit("updated");
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "保存失败");
  } finally {
    saving.value = false;
  }
}

function generateSecret() {
  const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  let secret = "";
  for (let i = 0; i < 32; i++) {
    secret += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  protocols.oidc.clientSecret = secret;
}

function applySamlTemplate(vendor: string) {
  if (vendor === "aliyun") {
    protocols.saml.spEntityId = "urn:alibaba:cloudcomputing";
    protocols.saml.nameIdFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress";
  } else if (vendor === "tencent") {
    protocols.saml.spEntityId = "cloud.tencent.com";
    protocols.saml.nameIdFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress";
  }
}

function getSamlEndpoint(type: string): string {
  const base = endpoints.value?.issuer || "";
  const code = app.value?.code || "";
  if (type === "entityId") return `${base}/saml/${code}/metadata`;
  if (type === "sso") return `${base}/saml/${code}/sso`;
  return "";
}

function downloadMetadata() {
  window.open(getSamlEndpoint("entityId"), "_blank");
}

function downloadCertificate() {
  ElMessage.info("证书下载功能开发中");
}

async function loadGrants() {
  if (!props.appId) return;
  try {
    const res = await ssoAdminApi.getGrants(props.appId);
    grants.value = res.data?.data?.list || [];
  } catch {}
}

async function saveGrants() {
  if (!props.appId) return;
  try {
    await ssoAdminApi.putGrants(
      props.appId,
      grants.value.map((g: any) => ({
        subjectType: g.subjectType,
        subjectId: g.subjectId,
        enabled: g.enabled,
        entitlementIds: g.entitlementIds || [],
      }))
    );
    ElMessage.success("已保存");
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "保存失败");
  }
}

function addGrant() {
  if (!newGrant.subjectId) {
    ElMessage.warning("请选择授权对象");
    return;
  }
  const name =
    newGrant.subjectType === "role"
      ? roles.value.find((r) => r.id === newGrant.subjectId)?.name
      : userOptions.value.find((u) => u.id === newGrant.subjectId)?.username;
  grants.value.push({
    subjectType: newGrant.subjectType,
    subjectId: newGrant.subjectId,
    subjectName: name || "",
    enabled: true,
    entitlementIds: [],
  });
  showAddGrant.value = false;
  saveGrants();
}

async function loadMappings() {
  if (!props.appId) return;
  try {
    const res = await ssoAdminApi.getMappings(props.appId);
    mappings.value = res.data?.data?.mappings || [];
  } catch {}
}

async function saveMappings() {
  if (!props.appId) return;
  saving.value = true;
  try {
    await ssoAdminApi.putMappings(props.appId, mappings.value);
    ElMessage.success("已保存");
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "保存失败");
  } finally {
    saving.value = false;
  }
}

async function loadOwners() {
  if (!props.appId) return;
  try {
    const res = await ssoAdminApi.getOwners(props.appId);
    const list = res.data?.data?.list || [];
    ownerRoleIds.value = list.filter((o: any) => o.subjectType === "role").map((o: any) => o.subjectId);
  } catch {}
}

async function saveOwners() {
  if (!props.appId) return;
  saving.value = true;
  try {
    await ssoAdminApi.putOwners(
      props.appId,
      ownerRoleIds.value.map((id) => ({ subjectType: "role", subjectId: id, capability: "all" }))
    );
    ElMessage.success("已保存");
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "保存失败");
  } finally {
    saving.value = false;
  }
}

async function loadRoles() {
  try {
    const res = await roleApi.list();
    roles.value = res.data?.data || [];
  } catch {}
}

async function searchUsers(keyword: string) {
  if (!keyword?.trim()) return;
  usersLoading.value = true;
  try {
    const res = await userApi.list({ pageIndex: 0, pageSize: 20, keyword });
    userOptions.value = res.data?.data?.list || [];
  } catch {} finally {
    usersLoading.value = false;
  }
}

function getAccessModeDesc(): string {
  if (form.accessMode === "all") return "所有用户都可以使用此应用进行单点登录";
  if (form.accessMode === "allowlist") return "仅白名单中的用户可以使用此应用";
  if (form.accessMode === "role_based") return "仅拥有指定角色的用户可以使用此应用";
  return "";
}

function copyText(text: string) {
  if (!text) return;
  navigator.clipboard.writeText(text);
  ElMessage.success("已复制");
}

watch(
  () => props.visible,
  (val) => {
    if (val && props.appId) {
      activeTab.value = "basic";
      loadApp();
      loadRoles();
    }
  }
);

onMounted(() => {
  loadRoles();
});
</script>

<style scoped>
.app-detail {
  padding: 0 4px;
}

.app-detail__status {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background: #f9fafb;
  border-radius: 8px;
  margin-bottom: 20px;
}

.status-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-label {
  color: #6b7280;
  font-size: 14px;
}

.app-detail__tabs {
  border: none;
}

.detail-form {
  max-width: 500px;
}

.protocol-cards {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.protocol-card {
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  overflow: hidden;
}

.protocol-card--active {
  border-color: #1677ff;
}

.protocol-card__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background: #f9fafb;
}

.protocol-card__title {
  font-weight: 500;
}

.protocol-card__body {
  padding: 16px;
}

.field-label {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
  margin-bottom: 8px;
}

.url-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.endpoint-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.endpoint-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.endpoint-label {
  font-size: 12px;
  color: #6b7280;
}

.form-hint {
  font-size: 12px;
  color: #9ca3af;
  margin-top: 4px;
}

.quick-templates {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #6b7280;
  font-size: 14px;
}

.download-buttons {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}

.access-section {
  padding: 8px 0;
}

.grant-list__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  font-weight: 500;
}

.section-desc {
  color: #6b7280;
  font-size: 14px;
  margin-bottom: 16px;
}

.mapping-section,
.owner-section {
  padding: 8px 0;
}
</style>
