<template>
  <div class="sso-center">
    <!-- 顶部区域 -->
    <div class="sso-center__header">
      <div class="sso-center__title">
        <h2>单点登录</h2>
        <p class="sso-center__desc">为第三方系统接入 CAS / SAML 2.0 / OAuth2 / OIDC 单点登录</p>
      </div>
      <div class="sso-center__actions">
        <el-button @click="router.push('/admin/sso/config')">
          <el-icon><Setting /></el-icon>
          系统配置
        </el-button>
        <el-button type="primary" @click="showQuickAccess = true">
          <el-icon><Plus /></el-icon>
          快速接入
        </el-button>
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="sso-center__stats">
      <div class="stat-card">
        <div class="stat-card__icon stat-card__icon--total">
          <el-icon :size="24"><Grid /></el-icon>
        </div>
        <div class="stat-card__content">
          <div class="stat-card__value">{{ stats.total }}</div>
          <div class="stat-card__label">应用总数</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-card__icon stat-card__icon--enabled">
          <el-icon :size="24"><CircleCheck /></el-icon>
        </div>
        <div class="stat-card__content">
          <div class="stat-card__value">{{ stats.enabled }}</div>
          <div class="stat-card__label">已启用</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-card__icon stat-card__icon--cas">
          <el-icon :size="24"><Key /></el-icon>
        </div>
        <div class="stat-card__content">
          <div class="stat-card__value">{{ stats.cas }}</div>
          <div class="stat-card__label">CAS 应用</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-card__icon stat-card__icon--oidc">
          <el-icon :size="24"><Connection /></el-icon>
        </div>
        <div class="stat-card__content">
          <div class="stat-card__value">{{ stats.oidc + stats.saml }}</div>
          <div class="stat-card__label">OIDC / SAML</div>
        </div>
      </div>
    </div>

    <!-- 搜索栏 -->
    <div class="sso-center__toolbar">
      <el-input
        v-model="keyword"
        placeholder="搜索应用名称或标识..."
        clearable
        style="width: 280px"
        @clear="loadApps"
        @keyup.enter="loadApps"
      >
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-select v-model="protocolFilter" placeholder="全部协议" clearable style="width: 140px" @change="loadApps">
        <el-option label="CAS" value="cas" />
        <el-option label="OIDC" value="oidc" />
        <el-option label="SAML" value="saml" />
      </el-select>
      <el-select v-model="statusFilter" placeholder="全部状态" clearable style="width: 120px" @change="loadApps">
        <el-option label="已启用" value="true" />
        <el-option label="已禁用" value="false" />
      </el-select>
      <el-button :icon="Refresh" @click="loadApps" />
    </div>

    <!-- 应用卡片网格 -->
    <div class="sso-center__grid" v-loading="loading">
      <div
        v-for="app in filteredApps"
        :key="app.id"
        class="app-card"
        :class="{ 'app-card--disabled': !app.enabled }"
      >
        <div class="app-card__header">
          <div class="app-card__logo">
            <span class="app-card__logo-text">{{ getLogoText(app.name) }}</span>
          </div>
          <div class="app-card__status">
            <el-tag :type="app.enabled ? 'success' : 'info'" size="small">
              {{ app.enabled ? '启用' : '禁用' }}
            </el-tag>
          </div>
        </div>
        <div class="app-card__body">
          <div class="app-card__name">{{ app.name }}</div>
          <div class="app-card__code">{{ app.code }}</div>
          <div class="app-card__protocols">
            <el-tag
              v-for="p in getEnabledProtocols(app)"
              :key="p"
              size="small"
              :type="getProtocolType(p)"
              effect="plain"
            >
              {{ formatProtocol(p) }}
            </el-tag>
            <span v-if="getEnabledProtocols(app).length === 0" class="app-card__no-protocol">
              未配置协议
            </span>
          </div>
        </div>
        <div class="app-card__footer">
          <el-button link type="primary" @click="openAppDetail(app)">
            <el-icon><Setting /></el-icon>
            配置
          </el-button>
          <el-button link @click="viewLogs(app)">
            <el-icon><Document /></el-icon>
            日志
          </el-button>
          <el-button link type="danger" @click="deleteApp(app)">
            <el-icon><Delete /></el-icon>
            删除
          </el-button>
        </div>
      </div>

      <!-- 添加应用卡片 -->
      <div class="app-card app-card--add" @click="showQuickAccess = true">
        <el-icon :size="32" color="#1677ff"><Plus /></el-icon>
        <span>添加应用</span>
      </div>
    </div>

    <!-- 空状态 -->
    <el-empty v-if="!loading && filteredApps.length === 0" description="暂无应用">
      <el-button type="primary" @click="showQuickAccess = true">
        <el-icon><Plus /></el-icon>
        快速接入
      </el-button>
    </el-empty>

    <!-- 快速接入向导 -->
    <SSOQuickAccess
      v-model:visible="showQuickAccess"
      @success="onAppCreated"
    />

    <!-- 应用详情/配置抽屉 -->
    <SSOAppDetail
      v-model:visible="showAppDetail"
      :app-id="currentAppId"
      @updated="loadApps"
    />

    <!-- 登录日志抽屉 -->
    <el-drawer
      v-model="showLogsDrawer"
      title="登录日志"
      size="800px"
      direction="rtl"
    >
      <SSOLogsPanel :app-id="currentAppId" />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  CircleCheck,
  Connection,
  Delete,
  Document,
  Grid,
  Key,
  Plus,
  Refresh,
  Search,
  Setting,
} from "@element-plus/icons-vue";
import { ssoAdminApi } from "../../api";
import SSOQuickAccess from "../../components/SSOQuickAccess.vue";
import SSOAppDetail from "../../components/SSOAppDetail.vue";
import SSOLogsPanel from "../../components/SSOLogsPanel.vue";

const router = useRouter();

const loading = ref(false);
const apps = ref<any[]>([]);
const keyword = ref("");
const protocolFilter = ref<string | undefined>(undefined);
const statusFilter = ref<string | undefined>(undefined);

const showQuickAccess = ref(false);
const showAppDetail = ref(false);
const showLogsDrawer = ref(false);
const currentAppId = ref<number | null>(null);

const stats = computed(() => {
  const list = apps.value || [];
  const enabled = list.filter((a: any) => a.enabled).length;
  const cas = list.filter((a: any) =>
    (a.protocols || []).some((p: any) => p.protocol === "cas" && p.enabled)
  ).length;
  const saml = list.filter((a: any) =>
    (a.protocols || []).some((p: any) => p.protocol === "saml" && p.enabled)
  ).length;
  const oidc = list.filter((a: any) =>
    (a.protocols || []).some((p: any) => p.protocol === "oidc" && p.enabled)
  ).length;
  return { total: list.length, enabled, cas, saml, oidc };
});

const filteredApps = computed(() => {
  let list = apps.value || [];
  const k = keyword.value.trim().toLowerCase();
  if (k) {
    list = list.filter(
      (a: any) =>
        (a.name || "").toLowerCase().includes(k) ||
        (a.code || "").toLowerCase().includes(k)
    );
  }
  if (protocolFilter.value) {
    list = list.filter((a: any) =>
      (a.protocols || []).some(
        (p: any) => p.protocol === protocolFilter.value && p.enabled
      )
    );
  }
  if (statusFilter.value !== undefined && statusFilter.value !== "") {
    const enabled = statusFilter.value === "true";
    list = list.filter((a: any) => a.enabled === enabled);
  }
  return list;
});

function getLogoText(name: string): string {
  if (!name) return "?";
  const chars = name.replace(/[^a-zA-Z\u4e00-\u9fa5]/g, "").slice(0, 2);
  return chars || name.slice(0, 2);
}

function getEnabledProtocols(app: any): string[] {
  return (app.protocols || [])
    .filter((p: any) => p.enabled)
    .map((p: any) => p.protocol);
}

function formatProtocol(p: string): string {
  if (p === "cas") return "CAS";
  if (p === "saml") return "SAML";
  if (p === "oidc") return "OIDC";
  return p.toUpperCase();
}

function getProtocolType(p: string): string {
  if (p === "cas") return "";
  if (p === "saml") return "warning";
  if (p === "oidc") return "success";
  return "info";
}

async function loadApps() {
  loading.value = true;
  try {
    const res = await ssoAdminApi.listApps({
      keyword: keyword.value || undefined,
      enabled: statusFilter.value || undefined,
    });
    apps.value = res.data?.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "加载失败");
  } finally {
    loading.value = false;
  }
}

function openAppDetail(app: any) {
  currentAppId.value = app.id;
  showAppDetail.value = true;
}

function viewLogs(app: any) {
  currentAppId.value = app.id;
  showLogsDrawer.value = true;
}

async function deleteApp(app: any) {
  try {
    await ElMessageBox.confirm(
      `确认删除应用「${app.name}」？删除后无法恢复。`,
      "删除确认",
      { type: "warning" }
    );
    await ssoAdminApi.deleteApp(app.id);
    ElMessage.success("已删除");
    loadApps();
  } catch {
  }
}

function onAppCreated() {
  showQuickAccess.value = false;
  loadApps();
}

onMounted(() => {
  loadApps();
});
</script>

<style scoped>
/* ============================================
   Corporate SSO Center - 企业风格
   ============================================ */
.sso-center {
  padding: 0;
  
}


.sso-center__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.sso-center__title h2 {
  margin: 0 0 6px 0;
  font-size: var(--font-size-xl);
  font-weight: 600;
  color: var(--color-text-primary);
}

.sso-center__desc {
  margin: 0;
  font-size: var(--font-size-base);
  color: var(--color-text-tertiary);
}

.sso-center__actions {
  display: flex;
  gap: 12px;
}

.sso-center__stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}

@media (max-width: 1200px) {
  .sso-center__stats {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  .sso-center__stats {
    grid-template-columns: 1fr;
  }
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  transition: box-shadow var(--transition-fast);
  box-shadow: var(--card-shadow);
}

.stat-card:hover {
  box-shadow: var(--card-shadow-hover);
}

.stat-card__icon {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.stat-card__icon--total {
  background: var(--color-primary);
}

.stat-card__icon--enabled {
  background: var(--color-success);
}

.stat-card__icon--cas {
  background: var(--color-warning);
}

.stat-card__icon--oidc {
  background: var(--color-info);
}

.stat-card__value {
  font-size: var(--font-size-2xl);
  font-weight: 700;
  color: var(--color-text-primary);
  line-height: 1.2;
}

.stat-card__label {
  font-size: var(--font-size-base);
  color: var(--color-text-tertiary);
  margin-top: 2px;
}

.sso-center__toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.sso-center__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
}

.app-card {
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  padding: 20px;
  transition: box-shadow var(--transition-fast), border-color var(--transition-fast);
  display: flex;
  flex-direction: column;
  box-shadow: var(--card-shadow);
}

.app-card:hover {
  border-color: var(--color-primary-border);
  box-shadow: var(--card-shadow-hover);
}

.app-card--disabled {
  opacity: 0.7;
}

.app-card--disabled:hover {
  border-color: var(--color-border);
  box-shadow: var(--card-shadow);
}

.app-card--add {
  border: 2px dashed var(--color-border);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  min-height: 200px;
  cursor: pointer;
  color: var(--color-text-tertiary);
  font-size: var(--font-size-base);
}

.app-card--add:hover {
  border-color: var(--color-primary);
  color: #1677ff;
  background: #f0f7ff;
}

.app-card__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
}

.app-card__logo {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  background: linear-gradient(135deg, #1677ff, #69b1ff);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 600;
  font-size: 16px;
}

.app-card__body {
  flex: 1;
}

.app-card__name {
  font-size: var(--font-size-lg);
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: 4px;
}

.app-card__code {
  font-size: var(--font-size-sm);
  color: var(--color-text-quaternary);
  margin-bottom: 12px;
}

.app-card__protocols {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.app-card__no-protocol {
  font-size: var(--font-size-xs);
  color: var(--color-text-quaternary);
}

.app-card__footer {
  display: flex;
  gap: 8px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--color-border);
}
</style>
