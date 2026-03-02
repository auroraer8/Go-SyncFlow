<template>
  <div class="page-container sso-page">
    <div class="page-header sso-page__header">
      <div>
        <h2>单点登录 / 权限管理</h2>
        <p class="page-desc sso-page__desc">按应用统一维护权限项与授权分配，并在 SSO 下发 roles/scopes/permissions</p>
      </div>
    </div>

    <el-row :gutter="16">
      <el-col :xs="24" :sm="24" :md="7" :lg="6" :xl="6">
        <div class="sso-card">
          <div class="sso-aside__toolbar">
            <el-input v-model="appKeyword" placeholder="搜索应用名称 / code" clearable />
            <el-button class="sso-btn" plain :loading="loadingApps" @click="loadApps">刷新</el-button>
          </div>
          <el-scrollbar height="560">
            <div
              v-for="a in filteredApps"
              :key="a.id"
              :class="['sso-app-item', { 'sso-app-item--active': a.id === appId }]"
              @click="selectApp(a)"
            >
              <div style="display: flex; justify-content: space-between; align-items: flex-start; gap: 10px">
                <div style="min-width: 0">
                  <div class="sso-app-item__name sso-ellipsis">{{ a.name }}</div>
                  <div class="sso-app-item__code sso-ellipsis">{{ a.code }}</div>
                </div>
                <span :class="['sso-tag', a.enabled ? 'sso-tag--success' : 'sso-tag--info']">{{ a.enabled ? "启用" : "禁用" }}</span>
              </div>
              <div class="sso-app-item__tags">
                <span v-for="p in enabledProtocols(a)" :key="a.id + '-' + p" class="sso-tag sso-tag--primary">{{ protocolLabel(p) }}</span>
                <span v-if="enabledProtocols(a).length === 0" class="sso-tag sso-tag--info">未配置协议</span>
              </div>
            </div>
          </el-scrollbar>
        </div>
      </el-col>

      <el-col :xs="24" :sm="24" :md="17" :lg="18" :xl="18">
        <div class="sso-card">
          <div class="sso-topbar">
            <div class="sso-topbar__left" style="min-width: 0">
              <div class="sso-topbar__app">
                {{ currentApp ? `${currentApp.name} (${currentApp.code})` : "请选择左侧应用" }}
              </div>
              <div class="sso-topbar__hint" v-if="currentApp">在此配置的权限项/授权分配会在 CAS/OIDC/SAML 下发阶段合并输出</div>
            </div>
            <div class="sso-topbar__right">
              <el-button class="sso-btn" plain :disabled="!appId" @click="loadAll">刷新</el-button>
              <el-button type="primary" :disabled="!appId" :loading="primarySaving" @click="handlePrimarySave">{{ primaryActionLabel }}</el-button>
            </div>
          </div>

          <el-tabs v-model="activeTab" type="border-card">
            <el-tab-pane label="授权分配" name="grants">
              <div class="sso-panel__toolbar">
                <div class="sso-panel__title">授权分配</div>
                <div class="sso-panel__actions">
                  <el-button type="primary" :disabled="!appId" @click="addGrantRow">新增授权</el-button>
                  <el-button class="sso-btn" plain :disabled="!appId" @click="loadGrants">刷新</el-button>
                </div>
              </div>
              <el-table :data="grants" v-loading="loadingGrants" stripe class="modern-table" empty-text="暂无授权">
                <template #empty>
                  <el-empty description="暂无授权">
                    <el-button type="primary" @click="addGrantRow">新增授权</el-button>
                  </el-empty>
                </template>
                <el-table-column label="主体" width="140">
                  <template #default="{ row }">
                    <el-select v-model="row.subjectType" style="width: 120px">
                      <el-option label="角色" value="role" />
                      <el-option label="用户" value="user" />
                    </el-select>
                  </template>
                </el-table-column>
                <el-table-column label="对象" min-width="240">
                  <template #default="{ row }">
                    <el-select v-if="row.subjectType === 'role'" v-model="row.subjectId" filterable style="width: 220px">
                      <el-option v-for="r in roles" :key="'role-' + r.id" :label="`${r.name} (${r.code})`" :value="r.id" />
                    </el-select>
                    <el-select
                      v-else
                      v-model="row.subjectId"
                      filterable
                      remote
                      :remote-method="searchUsers"
                      :loading="usersLoading"
                      style="width: 220px"
                    >
                      <el-option v-for="u in userOptions" :key="'user-' + u.id" :label="`${u.username}${u.nickname ? ' - ' + u.nickname : ''}`" :value="u.id" />
                    </el-select>
                  </template>
                </el-table-column>
                <el-table-column label="权限项" min-width="360">
                  <template #default="{ row }">
                    <el-select v-model="row.entitlementIds" multiple filterable collapse-tags collapse-tags-tooltip style="width: 340px">
                      <el-option v-for="e in entitlementsEnabled" :key="'ent-' + e.id" :label="`${typeLabel(e.type)}:${e.value}`" :value="e.id" />
                    </el-select>
                  </template>
                </el-table-column>
                <el-table-column label="启用" width="90" align="center">
                  <template #default="{ row }">
                    <el-switch v-model="row.enabled" />
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="100">
                  <template #default="{ $index }">
                    <el-button link type="danger" @click="removeGrantRow($index)">删除</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>

            <el-tab-pane label="应用权限项" name="entitlements">
              <div class="sso-panel__toolbar">
                <div class="sso-panel__title">应用权限项</div>
                <div class="sso-panel__actions">
                  <el-button type="primary" :disabled="!appId" @click="addEntitlementRow">新增权限项</el-button>
                  <el-button class="sso-btn" plain :disabled="!appId" @click="loadEntitlements">刷新</el-button>
                </div>
              </div>
              <el-table :data="entitlements" v-loading="loadingEntitlements" stripe class="modern-table" empty-text="暂无权限项">
                <template #empty>
                  <el-empty description="暂无权限项">
                    <el-button type="primary" @click="addEntitlementRow">新增权限项</el-button>
                  </el-empty>
                </template>
                <el-table-column label="类型" width="140">
                  <template #default="{ row }">
                    <el-select v-model="row.type" placeholder="类型" style="width: 120px">
                      <el-option label="角色" value="role" />
                      <el-option label="权限" value="permission" />
                      <el-option label="Scope" value="scope" />
                    </el-select>
                  </template>
                </el-table-column>
                <el-table-column label="Key" min-width="160">
                  <template #default="{ row }">
                    <el-input v-model="row.key" placeholder="可选（用于分组/备注）" />
                  </template>
                </el-table-column>
                <el-table-column label="Value" min-width="240">
                  <template #default="{ row }">
                    <el-input v-model="row.value" placeholder="应用侧权限值，例如：HR、hr_admin、hr:write" />
                  </template>
                </el-table-column>
                <el-table-column label="描述" min-width="220">
                  <template #default="{ row }">
                    <el-input v-model="row.description" placeholder="可选" />
                  </template>
                </el-table-column>
                <el-table-column label="启用" width="90" align="center">
                  <template #default="{ row }">
                    <el-switch v-model="row.enabled" />
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="100">
                  <template #default="{ $index, row }">
                    <el-button link type="danger" @click="removeEntitlementRow($index, row)">删除</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>

            <el-tab-pane label="输出模板" name="templates">
              <div class="sso-panel__toolbar">
                <div class="sso-panel__title">输出模板</div>
                <div class="sso-panel__actions">
                  <el-button type="primary" :disabled="!appId" @click="applyTemplatePreset('aliyun')">阿里云模板</el-button>
                  <el-button type="primary" :disabled="!appId" @click="applyTemplatePreset('tencent')">腾讯云模板</el-button>
                  <el-button class="sso-btn" plain :disabled="!appId" @click="loadTemplates">刷新</el-button>
                </div>
              </div>
              <div class="sso-page__desc" style="margin-bottom: 10px">
                模板目前用于 SAML 追加额外 Attribute（JSON：extraAttributes/attributes）。云厂商角色联合通常需要固定字段名。
              </div>
              <el-table :data="templates" v-loading="loadingTemplates" stripe class="modern-table" empty-text="暂无模板">
                <template #empty>
                  <el-empty description="暂无模板">
                    <el-button type="primary" @click="applyTemplatePreset('aliyun')">使用阿里云模板</el-button>
                  </el-empty>
                </template>
                <el-table-column label="协议" width="140">
                  <template #default="{ row }">
                    <el-select v-model="row.protocol" style="width: 120px">
                      <el-option label="SAML" value="saml" />
                    </el-select>
                  </template>
                </el-table-column>
                <el-table-column label="启用" width="90" align="center">
                  <template #default="{ row }">
                    <el-switch v-model="row.enabled" />
                  </template>
                </el-table-column>
                <el-table-column label="模板(JSON)" min-width="520">
                  <template #default="{ row }">
                    <el-input v-model="row.template" type="textarea" :rows="8" placeholder='示例：{\"extraAttributes\":{\"https://example.com/Attr\":[\"v1\"]}}' />
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="100">
                  <template #default="{ $index }">
                    <el-button link type="danger" @click="removeTemplateRow($index)">删除</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>

            <el-tab-pane label="映射" name="mappings">
              <div class="sso-panel__toolbar">
                <div class="sso-panel__title">映射</div>
                <div class="sso-panel__actions">
                  <el-button type="primary" :disabled="!appId" @click="addMappingRow">新增映射</el-button>
                  <el-button class="sso-btn" plain :disabled="!appId" @click="loadMappings">刷新</el-button>
                </div>
              </div>
              <el-table :data="mappings" v-loading="loadingMappings" stripe class="modern-table" empty-text="暂无映射">
                <template #empty>
                  <el-empty description="暂无映射">
                    <el-button type="primary" @click="addMappingRow">新增映射</el-button>
                  </el-empty>
                </template>
                <el-table-column label="类型" width="140">
                  <template #default="{ row }">
                    <el-select v-model="row.type" placeholder="类型" style="width: 120px">
                      <el-option label="角色" value="role" />
                      <el-option label="权限" value="permission" />
                      <el-option label="属性" value="attr" />
                    </el-select>
                  </template>
                </el-table-column>
                <el-table-column label="源" min-width="220">
                  <template #default="{ row }">
                    <el-input v-model="row.source" placeholder="如：super_admin 或 role:super_admin 或 user:list" />
                  </template>
                </el-table-column>
                <el-table-column label="目标" min-width="240">
                  <template #default="{ row }">
                    <el-input v-model="row.target" placeholder="如：admin 或 hr_admin 或 https://.../Role" />
                  </template>
                </el-table-column>
                <el-table-column label="优先级" width="120">
                  <template #default="{ row }">
                    <el-input-number v-model="row.priority" :min="0" :max="999" controls-position="right" style="width: 100px" />
                  </template>
                </el-table-column>
                <el-table-column label="备注" min-width="160">
                  <template #default="{ row }">
                    <el-input v-model="row.remark" placeholder="可选" />
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="100">
                  <template #default="{ $index }">
                    <el-button link type="danger" @click="removeMappingRow($index)">删除</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
          </el-tabs>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { roleApi, ssoAdminApi, userApi } from "../../api";

const loadingApps = ref(false);
const apps = ref<any[]>([]);
const appKeyword = ref("");
const appId = ref<number | null>(null);
const currentApp = computed(() => apps.value.find((a) => a.id === appId.value) || null);

const activeTab = ref("grants");

const primaryActionLabel = computed(() => {
  if (activeTab.value === "mappings") return "保存映射";
  if (activeTab.value === "entitlements") return "保存权限项";
  if (activeTab.value === "grants") return "保存授权";
  if (activeTab.value === "templates") return "保存模板";
  return "保存";
});

const primarySaving = computed(() => {
  if (activeTab.value === "mappings") return savingMappings.value;
  if (activeTab.value === "entitlements") return savingEntitlements.value;
  if (activeTab.value === "grants") return savingGrants.value;
  if (activeTab.value === "templates") return savingTemplates.value;
  return false;
});

async function handlePrimarySave() {
  if (activeTab.value === "mappings") return saveMappings();
  if (activeTab.value === "entitlements") return saveEntitlements();
  if (activeTab.value === "grants") return saveGrants();
  if (activeTab.value === "templates") return saveTemplates();
}

const loadingMappings = ref(false);
const savingMappings = ref(false);
const mappings = ref<any[]>([]);

const loadingEntitlements = ref(false);
const savingEntitlements = ref(false);
const entitlements = ref<any[]>([]);
const entitlementsEnabled = computed(() => entitlements.value.filter((e: any) => e.enabled));

const loadingGrants = ref(false);
const savingGrants = ref(false);
const grants = ref<any[]>([]);

const loadingTemplates = ref(false);
const savingTemplates = ref(false);
const templates = ref<any[]>([]);

const roles = ref<any[]>([]);
const usersLoading = ref(false);
const userOptions = ref<any[]>([]);

const filteredApps = computed(() => {
  const k = appKeyword.value.trim().toLowerCase();
  if (!k) return apps.value;
  return apps.value.filter((a: any) => (a.name || "").toLowerCase().includes(k) || (a.code || "").toLowerCase().includes(k));
});

function enabledProtocols(app: any) {
  const ps = Array.isArray(app?.protocols) ? app.protocols : [];
  return ps.filter((p: any) => p.enabled).map((p: any) => p.protocol);
}

function protocolLabel(p: string) {
  if (p === "cas") return "CAS";
  if (p === "saml") return "SAML";
  if (p === "oidc") return "OIDC";
  return p;
}

function typeLabel(t: string) {
  if (t === "role") return "角色";
  if (t === "permission") return "权限";
  if (t === "scope") return "Scope";
  return t;
}

async function loadApps() {
  loadingApps.value = true;
  try {
    const res = await ssoAdminApi.listApps();
    apps.value = res.data?.data?.list || [];
    if (!appId.value && apps.value.length > 0) {
      appId.value = apps.value[0].id;
    }
  } catch (e: any) {
    if (e?.response?.status === 403) {
      try {
        const res2 = await ssoAdminApi.listMyApps();
        apps.value = res2.data?.data?.list || [];
        if (!appId.value && apps.value.length > 0) {
          appId.value = apps.value[0].id;
        }
      } catch (e2: any) {
        ElMessage.error(e2?.response?.data?.message || "加载应用失败");
      }
    } else {
      ElMessage.error(e?.response?.data?.message || "加载应用失败");
    }
  } finally {
    loadingApps.value = false;
  }
}

async function loadMappings() {
  if (!appId.value) return;
  loadingMappings.value = true;
  try {
    const res = await ssoAdminApi.getMappings(appId.value);
    mappings.value = res.data?.data?.mappings || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "加载映射失败");
  } finally {
    loadingMappings.value = false;
  }
}

function addMappingRow() {
  mappings.value.unshift({ type: "role", source: "", target: "", priority: 0, remark: "" });
}

function removeMappingRow(index: number) {
  mappings.value.splice(index, 1);
}

async function saveMappings() {
  if (!appId.value) return;
  savingMappings.value = true;
  try {
    await ssoAdminApi.putMappings(appId.value, mappings.value);
    ElMessage.success("已保存");
    loadMappings();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "保存失败");
  } finally {
    savingMappings.value = false;
  }
}

async function loadEntitlements() {
  if (!appId.value) return;
  loadingEntitlements.value = true;
  try {
    const res = await ssoAdminApi.getEntitlements(appId.value);
    entitlements.value = res.data?.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "加载权限项失败");
  } finally {
    loadingEntitlements.value = false;
  }
}

function addEntitlementRow() {
  entitlements.value.unshift({ id: 0, type: "role", key: "", value: "", description: "", enabled: true });
}

async function removeEntitlementRow(index: number, row: any) {
  if (row.id) {
    try {
      await ssoAdminApi.deleteEntitlement(row.id);
    } catch (e: any) {
      ElMessage.error(e?.response?.data?.message || "删除失败");
      return;
    }
  }
  entitlements.value.splice(index, 1);
}

async function saveEntitlements() {
  if (!appId.value) return;
  savingEntitlements.value = true;
  try {
    for (let i = 0; i < entitlements.value.length; i++) {
      const r = entitlements.value[i];
      const payload = { type: r.type, key: r.key, value: r.value, description: r.description, enabled: !!r.enabled };
      if (!payload.type || !payload.value) continue;
      if (!r.id) {
        const res = await ssoAdminApi.createEntitlement(appId.value, payload);
        entitlements.value[i] = res.data?.data || entitlements.value[i];
      } else {
        const res = await ssoAdminApi.updateEntitlement(r.id, payload);
        entitlements.value[i] = res.data?.data || entitlements.value[i];
      }
    }
    ElMessage.success("已保存");
    loadEntitlements();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "保存失败");
  } finally {
    savingEntitlements.value = false;
  }
}

async function loadGrants() {
  if (!appId.value) return;
  loadingGrants.value = true;
  try {
    const res = await ssoAdminApi.getGrants(appId.value);
    grants.value = res.data?.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "加载授权失败");
  } finally {
    loadingGrants.value = false;
  }
}

function addGrantRow() {
  grants.value.unshift({ subjectType: "role", subjectId: 0, enabled: true, entitlementIds: [] as number[] });
}

function removeGrantRow(index: number) {
  grants.value.splice(index, 1);
}

async function saveGrants() {
  if (!appId.value) return;
  savingGrants.value = true;
  try {
    await ssoAdminApi.putGrants(
      appId.value,
      grants.value.map((g: any) => ({
        subjectType: g.subjectType,
        subjectId: Number(g.subjectId) || 0,
        enabled: !!g.enabled,
        entitlementIds: Array.isArray(g.entitlementIds) ? g.entitlementIds : [],
      }))
    );
    ElMessage.success("已保存");
    loadGrants();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "保存失败");
  } finally {
    savingGrants.value = false;
  }
}

async function loadTemplates() {
  if (!appId.value) return;
  loadingTemplates.value = true;
  try {
    const res = await ssoAdminApi.getTemplates(appId.value);
    templates.value = res.data?.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "加载模板失败");
  } finally {
    loadingTemplates.value = false;
  }
}

function removeTemplateRow(index: number) {
  templates.value.splice(index, 1);
}

function applyTemplatePreset(kind: "aliyun" | "tencent") {
  const preset =
    kind === "aliyun"
      ? {
          extraAttributes: {
            "https://www.aliyun.com/SAML-Role/Attributes/Role": ["acs:ram::$account_id:role/role1,acs:ram::$account_id:saml-provider/provider1"],
            "https://www.aliyun.com/SAML-Role/Attributes/RoleSessionName": ["user_id"],
          },
        }
      : {
          extraAttributes: {
            "https://cloud.tencent.com/SAML/Attributes/Role": ["qcs::cam::uin/$account_id:roleName/role1"],
            "https://cloud.tencent.com/SAML/Attributes/RoleSessionName": ["user_id"],
          },
        };
  templates.value = [{ protocol: "saml", enabled: true, template: JSON.stringify(preset, null, 2) }];
}

async function saveTemplates() {
  if (!appId.value) return;
  savingTemplates.value = true;
  try {
    await ssoAdminApi.putTemplates(
      appId.value,
      templates.value.map((t: any) => ({ protocol: t.protocol, enabled: !!t.enabled, template: t.template || "" }))
    );
    ElMessage.success("已保存");
    loadTemplates();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "保存失败");
  } finally {
    savingTemplates.value = false;
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

async function loadAll() {
  await Promise.all([loadMappings(), loadEntitlements(), loadGrants(), loadTemplates()]);
}

function selectApp(a: any) {
  appId.value = a.id;
}

watch(
  () => appId.value,
  (v) => {
    if (v) loadAll();
  }
);

onMounted(async () => {
  await Promise.all([loadApps(), loadRoles()]);
  if (appId.value) {
    loadAll();
  }
});
</script>
