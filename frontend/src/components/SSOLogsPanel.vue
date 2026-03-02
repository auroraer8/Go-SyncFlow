<template>
  <div class="sso-logs-panel">
    <div class="logs-toolbar">
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        value-format="YYYY-MM-DD"
        style="width: 260px"
        @change="loadLogs"
      />
      <el-select v-model="protocolFilter" placeholder="全部协议" clearable style="width: 120px" @change="loadLogs">
        <el-option label="CAS" value="cas" />
        <el-option label="OIDC" value="oidc" />
        <el-option label="SAML" value="saml" />
      </el-select>
      <el-select v-model="statusFilter" placeholder="全部状态" clearable style="width: 120px" @change="loadLogs">
        <el-option label="成功" value="success" />
        <el-option label="失败" value="failed" />
      </el-select>
      <el-button :icon="Refresh" @click="loadLogs" />
    </div>

    <el-table :data="logs" v-loading="loading" stripe style="width: 100%" empty-text="暂无日志">
      <el-table-column label="时间" width="180">
        <template #default="{ row }">
          {{ formatTime(row.createdAt) }}
        </template>
      </el-table-column>
      <el-table-column label="用户" prop="username" width="140" />
      <el-table-column label="协议" width="100">
        <template #default="{ row }">
          <el-tag size="small" :type="getProtocolType(row.protocol)">
            {{ formatProtocol(row.protocol) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag size="small" :type="row.success ? 'success' : 'danger'">
            {{ row.success ? '成功' : '失败' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="IP" prop="ip" width="140" />
      <el-table-column label="详情" prop="detail" show-overflow-tooltip />
    </el-table>

    <div class="logs-pagination" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="pageIndex"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="loadLogs"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { Refresh } from "@element-plus/icons-vue";
import { ssoAdminApi } from "../api";

const props = defineProps<{
  appId: number | null;
}>();

const loading = ref(false);
const logs = ref<any[]>([]);
const total = ref(0);
const pageIndex = ref(1);
const pageSize = ref(20);
const dateRange = ref<string[]>([]);
const protocolFilter = ref<string | undefined>(undefined);
const statusFilter = ref<string | undefined>(undefined);

async function loadLogs() {
  if (!props.appId) return;
  loading.value = true;
  try {
    const res = await ssoAdminApi.logs({
      appId: props.appId,
      pageIndex: pageIndex.value - 1,
      pageSize: pageSize.value,
      startDate: dateRange.value?.[0] || undefined,
      endDate: dateRange.value?.[1] || undefined,
      protocol: protocolFilter.value || undefined,
      success: statusFilter.value === "success" ? true : statusFilter.value === "failed" ? false : undefined,
    });
    logs.value = res.data?.data?.list || [];
    total.value = res.data?.data?.total || 0;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "加载失败");
  } finally {
    loading.value = false;
  }
}

function formatTime(t: string): string {
  if (!t) return "-";
  return new Date(t).toLocaleString("zh-CN");
}

function formatProtocol(p: string): string {
  if (p === "cas") return "CAS";
  if (p === "saml") return "SAML";
  if (p === "oidc") return "OIDC";
  return p?.toUpperCase() || "-";
}

function getProtocolType(p: string): string {
  if (p === "cas") return "";
  if (p === "saml") return "warning";
  if (p === "oidc") return "success";
  return "info";
}

watch(
  () => props.appId,
  (val) => {
    if (val) {
      pageIndex.value = 1;
      loadLogs();
    }
  },
  { immediate: true }
);

onMounted(() => {
  if (props.appId) loadLogs();
});
</script>

<style scoped>
.sso-logs-panel {
  padding: 8px 0;
}

.logs-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.logs-pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
</style>
