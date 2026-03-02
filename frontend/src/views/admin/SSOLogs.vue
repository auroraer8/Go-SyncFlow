<template>
  <div class="page-container sso-page">
    <div class="page-header sso-page__header">
      <div>
        <h2>单点登录 / 登录日志</h2>
        <p class="page-desc sso-page__desc">审计 CAS 校验、SAML 断言、OAuth2/OIDC 授权与令牌签发等事件</p>
      </div>
    </div>

    <div class="filter-bar">
      <el-select v-model="appId" placeholder="应用" clearable filterable style="width: 240px" @change="changePage(1)">
        <el-option v-for="a in apps" :key="a.id" :label="`${a.name} (${a.code})`" :value="a.id" />
      </el-select>
      <el-select v-model="protocol" placeholder="协议" clearable style="width: 140px" @change="load">
        <el-option label="OIDC" value="oidc" />
        <el-option label="CAS" value="cas" />
        <el-option label="SAML" value="saml" />
      </el-select>
      <el-input v-model="keyword" placeholder="搜索用户/动作/消息" clearable style="width: 260px" @clear="load" @keyup.enter="load" />
      <el-button class="sso-btn" plain @click="load">刷新</el-button>
    </div>

    <el-table :data="tableData" v-loading="loading" stripe class="modern-table" empty-text="暂无日志">
      <el-table-column label="时间" width="190" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="sso-ellipsis">{{ formatTime(row.createdAt) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="应用" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="appMap[row.appId]" class="sso-ellipsis">{{ appMap[row.appId] }}</span>
          <span v-else class="sso-muted">—</span>
        </template>
      </el-table-column>
      <el-table-column prop="protocol" label="协议" width="90" />
      <el-table-column prop="action" label="动作" width="120" />
      <el-table-column prop="username" label="用户" width="160" />
      <el-table-column label="结果" width="90">
        <template #default="{ row }">
          <el-tag :type="row.success ? 'success' : 'danger'" size="small">{{ row.success ? '成功' : '失败' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="消息" min-width="420" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.message" class="sso-ellipsis">{{ row.message }}</span>
          <span v-else class="sso-muted">—</span>
        </template>
      </el-table-column>
      <el-table-column prop="clientIp" label="IP" width="140" />
    </el-table>

    <div style="display:flex;justify-content:flex-end;margin-top:12px">
      <el-pagination
        background
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="changePage"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { ssoAdminApi } from "../../api";

const loading = ref(false);
const tableData = ref<any[]>([]);
const apps = ref<any[]>([]);
const appId = ref<number | undefined>(undefined);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const protocol = ref<string | undefined>(undefined);
const keyword = ref("");

const appMap = computed<Record<number, string>>(() => {
  const m: Record<number, string> = {};
  for (const a of apps.value || []) {
    m[a.id] = `${a.name} (${a.code})`;
  }
  return m;
});

function formatTime(value: any) {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return String(value);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

async function loadApps() {
  try {
    const res = await ssoAdminApi.listApps();
    apps.value = res.data?.data?.list || [];
  } catch (e: any) {
    if (e?.response?.status === 403) {
      try {
        const res2 = await ssoAdminApi.listMyApps();
        apps.value = res2.data?.data?.list || [];
      } catch {
      }
    }
  }
}

async function load() {
  loading.value = true;
  try {
    const res = await ssoAdminApi.logs({
      page: page.value,
      pageSize: pageSize.value,
      appId: appId.value || undefined,
      protocol: protocol.value || undefined,
      keyword: keyword.value || undefined,
    });
    tableData.value = res.data?.data?.list || [];
    total.value = res.data?.data?.total || 0;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "加载失败");
  } finally {
    loading.value = false;
  }
}

function changePage(p: number) {
  page.value = p;
  load();
}

onMounted(load);
onMounted(loadApps);
</script>
