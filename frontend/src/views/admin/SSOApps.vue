<template>
  <div class="page-container sso-page">
    <div class="sso-card sso-hero">
      <div class="sso-hero__row">
        <div class="sso-page__header">
          <div>
            <h2>单点登录 / 应用管理</h2>
            <p class="sso-page__desc">为第三方系统注册应用，选择 CAS / SAML2.0 / OAuth2 / OIDC 接入方式</p>
          </div>
        </div>
        <div class="sso-hero__actions">
          <div class="sso-actions">
            <el-button plain @click="router.push('/admin/sso/perms')">
              <el-icon><Key /></el-icon>
              权限管理
            </el-button>
            <el-button plain @click="router.push('/admin/sso/mappings')">
              <el-icon><List /></el-icon>
              授权策略
            </el-button>
            <el-button plain @click="router.push('/admin/sso/integration')">
              <el-icon><Link /></el-icon>
              接入配置
            </el-button>
            <el-button plain @click="router.push('/admin/sso/logs')">
              <el-icon><Document /></el-icon>
              登录日志
            </el-button>
          </div>
          <el-button type="primary" @click="openCreate">
            <el-icon><Plus /></el-icon>
            创建应用
          </el-button>
        </div>
      </div>

      <div class="sso-stat-grid">
        <div class="sso-stat-card">
          <div class="sso-stat-title">应用总数</div>
          <div class="sso-stat-value">{{ stats.total }}</div>
        </div>
        <div class="sso-stat-card">
          <div class="sso-stat-title">启用应用</div>
          <div class="sso-stat-value">{{ stats.enabled }}</div>
        </div>
        <div class="sso-stat-card">
          <div class="sso-stat-title">CAS 应用</div>
          <div class="sso-stat-value">{{ stats.cas }}</div>
          <div class="sso-stat-hint">兼容群晖等客户端</div>
        </div>
        <div class="sso-stat-card">
          <div class="sso-stat-title">OIDC / SAML 应用</div>
          <div class="sso-stat-value">{{ stats.oidc + stats.saml }}</div>
          <div class="sso-stat-hint">推荐用于 SaaS/云厂商</div>
        </div>
      </div>
    </div>

    <div class="filter-bar">
      <el-input v-model="keyword" placeholder="搜索名称 / code" clearable style="width: 260px" @clear="loadData" @keyup.enter="loadData">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-select v-model="enabledFilter" placeholder="状态" clearable style="width: 120px" @change="loadData">
        <el-option label="启用" value="true" />
        <el-option label="禁用" value="false" />
      </el-select>
      <el-button @click="loadData"><el-icon><Refresh /></el-icon></el-button>
    </div>

    <el-table :data="tableData" v-loading="loading" stripe class="modern-table" empty-text="暂无应用">
      <el-table-column label="应用" min-width="220">
        <template #default="{ row }">
          <div style="font-weight: 600">{{ row.name }}</div>
          <div class="text-muted" style="font-size: 12px">{{ row.code }}</div>
          <div v-if="row.description" class="text-muted" style="font-size: 12px">{{ row.description }}</div>
        </template>
      </el-table-column>
      <el-table-column label="协议" width="200">
        <template #default="{ row }">
          <el-tag v-for="p in (row.protocols || [])" :key="p.id" size="small" style="margin-right: 6px" :type="p.enabled ? 'success' : 'info'">
            {{ formatProtocol(p.protocol) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="访问控制" width="140">
        <template #default="{ row }">
          <el-tag size="small">{{ formatAccessMode(row.accessMode) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'danger'" size="small">
            {{ row.enabled ? '启用' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button link @click="openEdit(row)">编辑</el-button>
          <el-button link @click="goMappings(row)">授权策略</el-button>
          <el-button link type="danger" @click="removeApp(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="560px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="名称">
          <el-input v-model="form.name" placeholder="例如：GitLab" />
        </el-form-item>
        <el-form-item label="Code">
          <el-input v-model="form.code" placeholder="例如：gitlab" :disabled="!!form.id" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" placeholder="可选" />
        </el-form-item>
        <el-form-item label="访问控制">
          <el-select v-model="form.accessMode" style="width: 100%">
            <el-option label="全部用户" value="all" />
            <el-option label="白名单" value="allowlist" />
            <el-option label="基于角色" value="role_based" />
          </el-select>
        </el-form-item>
        <el-form-item label="权限负责人">
          <div style="width: 100%; display: grid; grid-template-columns: 1fr; gap: 10px">
            <div>
              <div class="text-muted" style="font-size: 12px; margin-bottom: 6px">负责人角色（拥有该应用权限配置的维护权限）</div>
              <el-select v-model="form.ownerRoleIds" multiple filterable collapse-tags collapse-tags-tooltip style="width: 100%" placeholder="选择角色">
                <el-option v-for="r in roles" :key="'role-' + r.id" :label="`${r.name} (${r.code})`" :value="r.id" />
              </el-select>
            </div>
            <div>
              <div class="text-muted" style="font-size: 12px; margin-bottom: 6px">负责人用户（支持搜索）</div>
              <el-select
                v-model="form.ownerUserIds"
                multiple
                filterable
                remote
                :remote-method="searchUsers"
                :loading="usersLoading"
                collapse-tags
                collapse-tags-tooltip
                style="width: 100%"
                placeholder="搜索用户账号/昵称"
              >
                <el-option v-for="u in userOptions" :key="'user-' + u.id" :label="`${u.username}${u.nickname ? ' - ' + u.nickname : ''}`" :value="u.id" />
              </el-select>
            </div>
          </div>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="协议">
          <el-checkbox-group v-model="form.protocols">
            <el-checkbox label="cas">CAS</el-checkbox>
            <el-checkbox label="saml">SAML2</el-checkbox>
            <el-checkbox label="oidc">OAuth2/OIDC</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item v-if="form.protocols.includes('cas')" label="CAS 配置">
          <div style="width: 100%">
            <div v-for="(s, idx) in form.protocolConfig.casServices" :key="'cas-' + idx" style="display:flex;gap:8px;margin-bottom:8px">
              <el-input v-model="form.protocolConfig.casServices[idx]" placeholder="例如：https://172.16.220.188:5001" />
              <el-button :disabled="form.protocolConfig.casServices.length <= 1" @click="removeCasService(idx)">删除</el-button>
            </div>
            <el-button @click="addCasService">添加一条</el-button>
          </div>
        </el-form-item>
        <el-form-item v-if="form.protocols.includes('oidc')" label="OIDC 配置">
          <div style="width: 100%">
            <div style="margin-bottom: 10px">
              <div class="text-muted" style="font-size: 12px; margin-bottom: 6px">Client ID（留空默认使用应用 code）</div>
              <el-input v-model="form.protocolConfig.oidcClientId" placeholder="例如：synology" />
            </div>
            <div style="margin-bottom: 10px">
              <div class="text-muted" style="font-size: 12px; margin-bottom: 6px">Client Secret（留空则不修改，当前：{{ form.protocolConfig.oidcClientSecretHash ? '已配置' : '未配置' }}）</div>
              <el-input v-model="form.protocolConfig.oidcClientSecret" placeholder="输入后将自动写入哈希" show-password />
            </div>
            <div>
              <div class="text-muted" style="font-size: 12px; margin-bottom: 6px">Redirect URIs</div>
              <div v-for="(u, idx) in form.protocolConfig.oidcRedirectUris" :key="'oidc-redirect-' + idx" style="display:flex;gap:8px;margin-bottom:8px">
                <el-input v-model="form.protocolConfig.oidcRedirectUris[idx]" placeholder="例如：https://app.example.com/callback" />
                <el-button :disabled="form.protocolConfig.oidcRedirectUris.length <= 1" @click="removeOidcRedirect(idx)">删除</el-button>
              </div>
              <el-button @click="addOidcRedirect">添加一条</el-button>
            </div>
          </div>
        </el-form-item>
        <el-form-item v-if="form.protocols.includes('saml')" label="SAML 配置">
          <div style="width: 100%">
            <div style="margin-bottom: 10px">
              <div class="text-muted" style="font-size: 12px; margin-bottom: 6px">SP EntityID</div>
              <el-input v-model="form.protocolConfig.samlSpEntityId" placeholder="例如：https://sp.example.com" />
            </div>
            <div style="margin-bottom: 10px">
              <div class="text-muted" style="font-size: 12px; margin-bottom: 6px">ACS URL</div>
              <el-input v-model="form.protocolConfig.samlAcsUrl" placeholder="例如：https://sp.example.com/saml/acs" />
            </div>
            <div style="margin-bottom: 10px">
              <div class="text-muted" style="font-size: 12px; margin-bottom: 6px">NameID 格式（留空=unspecified）</div>
              <el-input v-model="form.protocolConfig.samlNameIdFormat" placeholder="例如：urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified" />
            </div>
            <div>
              <div class="text-muted" style="font-size: 12px; margin-bottom: 6px">IdP EntityID（可选，留空自动生成）</div>
              <el-input v-model="form.protocolConfig.samlIdpEntityId" placeholder="例如：https://idp.example.com/saml/idp" />
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveApp">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Document, Key, Link, List, Plus, Refresh, Search } from "@element-plus/icons-vue";
import { roleApi, ssoAdminApi, userApi } from "../../api";

const router = useRouter();

const loading = ref(false);
const saving = ref(false);
const tableData = ref<any[]>([]);
const keyword = ref("");
const enabledFilter = ref<string | undefined>(undefined);

const stats = computed(() => {
  const list = tableData.value || [];
  const enabled = list.filter((a: any) => a.enabled).length;
  const cas = list.filter((a: any) => (a.protocols || []).some((p: any) => p.protocol === "cas" && p.enabled)).length;
  const saml = list.filter((a: any) => (a.protocols || []).some((p: any) => p.protocol === "saml" && p.enabled)).length;
  const oidc = list.filter((a: any) => (a.protocols || []).some((p: any) => p.protocol === "oidc" && p.enabled)).length;
  return { total: list.length, enabled, cas, saml, oidc };
});

const dialogVisible = ref(false);
const dialogTitle = ref("创建应用");

const form = reactive({
  id: 0,
  name: "",
  code: "",
  description: "",
  enabled: true,
  accessMode: "all",
  ownerRoleIds: [] as number[],
  ownerUserIds: [] as number[],
  protocols: [] as string[],
  protocolConfig: {
    casServices: [""] as string[],
    oidcClientId: "",
    oidcClientSecret: "",
    oidcClientSecretHash: "",
    oidcRedirectUris: [""] as string[],
    samlSpEntityId: "",
    samlAcsUrl: "",
    samlNameIdFormat: "",
    samlIdpEntityId: "",
  },
});

const roles = ref<any[]>([]);
const usersLoading = ref(false);
const userOptions = ref<any[]>([]);

function resetForm() {
  form.id = 0;
  form.name = "";
  form.code = "";
  form.description = "";
  form.enabled = true;
  form.accessMode = "all";
  form.ownerRoleIds = [];
  form.ownerUserIds = [];
  form.protocols = [];
  form.protocolConfig.casServices = [""];
  form.protocolConfig.oidcClientId = "";
  form.protocolConfig.oidcClientSecret = "";
  form.protocolConfig.oidcClientSecretHash = "";
  form.protocolConfig.oidcRedirectUris = [""];
  form.protocolConfig.samlSpEntityId = "";
  form.protocolConfig.samlAcsUrl = "";
  form.protocolConfig.samlNameIdFormat = "";
  form.protocolConfig.samlIdpEntityId = "";
}

function formatProtocol(p: string) {
  if (p === "cas") return "CAS";
  if (p === "saml") return "SAML2";
  if (p === "oidc") return "OIDC";
  return p || "-";
}

function formatAccessMode(v: string) {
  if (v === "all") return "全部用户";
  if (v === "allowlist") return "白名单";
  if (v === "role_based") return "基于角色";
  return v || "-";
}

async function loadData() {
  loading.value = true;
  try {
    const res = await ssoAdminApi.listApps({
      keyword: keyword.value || undefined,
      enabled: enabledFilter.value || undefined,
    });
    tableData.value = res.data?.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "加载失败");
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  resetForm();
  dialogTitle.value = "创建应用";
  dialogVisible.value = true;
}

function openEdit(row: any) {
  resetForm();
  form.id = row.id;
  form.name = row.name;
  form.code = row.code;
  form.description = row.description || "";
  form.enabled = !!row.enabled;
  form.accessMode = row.accessMode || "all";
  form.protocols = (row.protocols || []).filter((p: any) => p.enabled).map((p: any) => p.protocol);
  for (const p of row.protocols || []) {
    if (!p.enabled) continue;
    if (p.protocol === "cas") {
      try {
        const cfg = JSON.parse(p.config || "{}");
        const services = Array.isArray(cfg.services) ? cfg.services : [];
        form.protocolConfig.casServices = services.length > 0 ? services : [""];
      } catch {
        form.protocolConfig.casServices = [""];
      }
    }
    if (p.protocol === "oidc") {
      try {
        const cfg = JSON.parse(p.config || "{}");
        form.protocolConfig.oidcClientId = cfg.clientId || row.code || "";
        form.protocolConfig.oidcClientSecretHash = cfg.clientSecretHash || "";
        const uris = Array.isArray(cfg.redirectUris) ? cfg.redirectUris : [];
        form.protocolConfig.oidcRedirectUris = uris.length > 0 ? uris : [""];
      } catch {
        form.protocolConfig.oidcClientId = row.code || "";
        form.protocolConfig.oidcClientSecretHash = "";
        form.protocolConfig.oidcRedirectUris = [""];
      }
      form.protocolConfig.oidcClientSecret = "";
    }
    if (p.protocol === "saml") {
      try {
        const cfg = JSON.parse(p.config || "{}");
        form.protocolConfig.samlSpEntityId = cfg.spEntityId || "";
        form.protocolConfig.samlAcsUrl = cfg.acsUrl || "";
        form.protocolConfig.samlNameIdFormat = cfg.nameIdFormat || "";
        form.protocolConfig.samlIdpEntityId = cfg.idpEntityId || "";
      } catch {
        form.protocolConfig.samlSpEntityId = "";
        form.protocolConfig.samlAcsUrl = "";
        form.protocolConfig.samlNameIdFormat = "";
        form.protocolConfig.samlIdpEntityId = "";
      }
    }
  }
  dialogTitle.value = "编辑应用";
  dialogVisible.value = true;
  loadOwners(row.id);
}

function addCasService() {
  form.protocolConfig.casServices.push("");
}

function removeCasService(index: number) {
  form.protocolConfig.casServices.splice(index, 1);
}

function addOidcRedirect() {
  form.protocolConfig.oidcRedirectUris.push("");
}

function removeOidcRedirect(index: number) {
  form.protocolConfig.oidcRedirectUris.splice(index, 1);
}

function goMappings(row: any) {
  router.push("/admin/sso/mappings?appId=" + row.id);
}

async function removeApp(row: any) {
  try {
    await ElMessageBox.confirm(`确认删除应用「${row.name}」？`, "提示", { type: "warning" });
    await ssoAdminApi.deleteApp(row.id);
    ElMessage.success("已删除");
    loadData();
  } catch {
  }
}

async function saveApp() {
  if (!form.name.trim() || !form.code.trim()) {
    ElMessage.warning("请输入名称与 code");
    return;
  }
  saving.value = true;
  try {
    let appId = form.id;
    if (!form.id) {
      const created = await ssoAdminApi.createApp({
        name: form.name.trim(),
        code: form.code.trim(),
        description: form.description,
        enabled: form.enabled,
        accessMode: form.accessMode,
        claimPolicy: "",
      });
      appId = created.data?.data?.id;
    } else {
      await ssoAdminApi.updateApp(form.id, {
        name: form.name.trim(),
        description: form.description,
        enabled: form.enabled,
        accessMode: form.accessMode,
        claimPolicy: "",
      });
    }
    await ssoAdminApi.putProtocols(appId, [
      {
        protocol: "cas",
        enabled: form.protocols.includes("cas"),
        config: JSON.stringify({
          services: form.protocolConfig.casServices.map((x) => x.trim()).filter(Boolean),
        }),
      },
      {
        protocol: "oidc",
        enabled: form.protocols.includes("oidc"),
        config: JSON.stringify({
          clientId: (form.protocolConfig.oidcClientId || form.code).trim(),
          clientSecret: form.protocolConfig.oidcClientSecret.trim() || undefined,
          clientSecretHash: form.protocolConfig.oidcClientSecretHash || undefined,
          redirectUris: form.protocolConfig.oidcRedirectUris.map((x) => x.trim()).filter(Boolean),
        }),
      },
      {
        protocol: "saml",
        enabled: form.protocols.includes("saml"),
        config: JSON.stringify({
          spEntityId: form.protocolConfig.samlSpEntityId.trim(),
          acsUrl: form.protocolConfig.samlAcsUrl.trim(),
          nameIdFormat: form.protocolConfig.samlNameIdFormat.trim(),
          idpEntityId: form.protocolConfig.samlIdpEntityId.trim(),
        }),
      },
    ]);
    await ssoAdminApi.putOwners(appId, [
      ...form.ownerRoleIds.map((id) => ({ subjectType: "role", subjectId: id, capability: "all" })),
      ...form.ownerUserIds.map((id) => ({ subjectType: "user", subjectId: id, capability: "all" })),
    ]);
    ElMessage.success("已保存");
    dialogVisible.value = false;
    loadData();
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
  } catch {
  }
}

async function searchUsers(keyword: string) {
  const k = (keyword || "").trim();
  if (!k) return;
  usersLoading.value = true;
  try {
    const res = await userApi.list({ pageIndex: 0, pageSize: 20, keyword: k });
    userOptions.value = res.data?.data?.list || [];
  } catch {
  } finally {
    usersLoading.value = false;
  }
}

async function loadOwners(appId: number) {
  try {
    const res = await ssoAdminApi.getOwners(appId);
    const rows = res.data?.data?.list || [];
    form.ownerRoleIds = rows.filter((x: any) => x.subjectType === "role").map((x: any) => x.subjectId);
    form.ownerUserIds = rows.filter((x: any) => x.subjectType === "user").map((x: any) => x.subjectId);
  } catch {
  }
}

onMounted(() => {
  loadData();
  loadRoles();
});
</script>
