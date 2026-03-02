<template>
  <div class="page-container">
    <el-card class="table-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">系统日志</span>
          <div class="header-actions">
            <el-button size="small" @click="exportLogs" :loading="exporting">
              <el-icon><Download /></el-icon> 导出Excel
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="logs" v-loading="loading" stripe size="small" border style="width: 100%">
        <el-table-column label="时间" width="170">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>时间</span>
              <el-popover placement="bottom" :width="260" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: dateRange?.length }"><Filter /></el-icon>
                </template>
                <el-date-picker v-model="dateRange" type="daterange" range-separator="至"
                  start-placeholder="开始" end-placeholder="结束" value-format="YYYY-MM-DD"
                  size="small" clearable @change="loadLogs" style="width: 100%" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
        </el-table-column>

        <el-table-column label="用户" width="120">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>用户</span>
              <el-popover placement="bottom" :width="150" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: keyword }"><Filter /></el-icon>
                </template>
                <el-input v-model="keyword" placeholder="输入用户名" clearable size="small" @keyup.enter="loadLogs" @clear="loadLogs" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">{{ row.username }}</template>
        </el-table-column>

        <el-table-column label="类型" width="80" align="center">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>类型</span>
              <el-popover placement="bottom" :width="100" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filterType }"><Filter /></el-icon>
                </template>
                <el-select v-model="filterType" placeholder="全部" clearable size="small" @change="loadLogs" style="width: 100%">
                  <el-option label="登录" value="login" />
                  <el-option label="操作" value="operation" />
                </el-select>
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">
            <el-tag :type="row.logType === 'login' ? 'primary' : 'warning'" size="small">
              {{ row.logType === 'login' ? '登录' : '操作' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="描述" min-width="280" show-overflow-tooltip>
          <template #header>
            <div class="th-filter" @click.stop>
              <span>描述</span>
              <el-popover placement="bottom" :width="200" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: summaryKeyword }"><Filter /></el-icon>
                </template>
                <el-input v-model="summaryKeyword" placeholder="搜索描述内容" clearable size="small" @keyup.enter="loadLogs" @clear="loadLogs" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">{{ row.summary }}</template>
        </el-table-column>

        <el-table-column label="状态" width="80" align="center">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>状态</span>
              <el-popover placement="bottom" :width="100" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filterStatus }"><Filter /></el-icon>
                </template>
                <el-select v-model="filterStatus" placeholder="全部" clearable size="small" @change="loadLogs" style="width: 100%">
                  <el-option label="成功" value="success" />
                  <el-option label="失败" value="failed" />
                </el-select>
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small">
              {{ row.status === 'success' ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="IP" width="130">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>IP</span>
              <el-popover placement="bottom" :width="150" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: ipKeyword }"><Filter /></el-icon>
                </template>
                <el-input v-model="ipKeyword" placeholder="搜索IP" clearable size="small" @keyup.enter="loadLogs" @clear="loadLogs" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">{{ row.ip }}</template>
        </el-table-column>
      </el-table>

      <div class="pagination-row">
        <span class="total-text">共 {{ total }} 条</span>
        <el-pagination background layout="sizes, prev, pager, next" :total="total"
          v-model:current-page="page" v-model:page-size="pageSize"
          :page-sizes="[20, 50, 100]" @current-change="loadLogs" @size-change="loadLogs" />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { Filter, Download } from "@element-plus/icons-vue";
import { api } from "../../api";
import * as XLSX from 'xlsx';

const loading = ref(false);
const exporting = ref(false);
const logs = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const filterType = ref("");
const filterStatus = ref("");
const keyword = ref("");
const summaryKeyword = ref("");
const ipKeyword = ref("");
const dateRange = ref<string[]>([]);

const formatTime = (t: string) => t ? t.replace("T", " ").slice(0, 19) : "";

const loadLogs = async () => {
  loading.value = true;
  try {
    const params: any = { page: page.value, size: pageSize.value };
    if (filterType.value) params.type = filterType.value;
    if (filterStatus.value) params.status = filterStatus.value;
    if (keyword.value) params.keyword = keyword.value;
    if (summaryKeyword.value) params.summary = summaryKeyword.value;
    if (ipKeyword.value) params.ip = ipKeyword.value;
    if (dateRange.value?.length === 2) {
      params.startDate = dateRange.value[0];
      params.endDate = dateRange.value[1];
    }
    const res = await api.get("/logs/system", { params });
    if (res.data.success) {
      logs.value = res.data.data?.list || [];
      total.value = res.data.data?.total || 0;
    }
  } finally { loading.value = false; }
};

const exportLogs = async () => {
  exporting.value = true;
  try {
    const allLogs: any[] = [];
    let p = 1;
    const size = 500;
    while (true) {
      const params: any = { page: p, size };
      if (filterType.value) params.type = filterType.value;
      if (filterStatus.value) params.status = filterStatus.value;
      if (keyword.value) params.keyword = keyword.value;
      if (summaryKeyword.value) params.summary = summaryKeyword.value;
      if (ipKeyword.value) params.ip = ipKeyword.value;
      if (dateRange.value?.length === 2) {
        params.startDate = dateRange.value[0];
        params.endDate = dateRange.value[1];
      }
      const res = await api.get("/logs/system", { params });
      const list = res.data.data?.list || [];
      allLogs.push(...list);
      if (list.length < size) break;
      p++;
    }
    if (!allLogs.length) {
      ElMessage.warning('没有可导出的数据');
      return;
    }
    downloadExcel(allLogs, '系统日志');
    ElMessage.success(`成功导出 ${allLogs.length} 条记录`);
  } catch (e) {
    ElMessage.error('导出失败');
  } finally {
    exporting.value = false;
  }
};

const downloadExcel = (data: any[], filename: string) => {
  const rows = data.map(r => ({
    '时间': formatTime(r.createdAt),
    '用户': r.username || '',
    '类型': r.logType === 'login' ? '登录' : '操作',
    '描述': r.summary || '',
    '状态': r.status === 'success' ? '成功' : '失败',
    'IP': r.ip || ''
  }));
  const ws = XLSX.utils.json_to_sheet(rows);
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, 'Sheet1');
  XLSX.writeFile(wb, `${filename}_${new Date().toISOString().slice(0, 10)}.xlsx`);
};

onMounted(() => { loadLogs(); });
</script>

<style scoped>
.page-container { display: flex; flex-direction: column; gap: 16px; }
.table-card :deep(.el-card__body) { padding: 16px; }
.card-header { display: flex; justify-content: space-between; align-items: center; }
.card-title { font-size: 16px; font-weight: 600; color: var(--color-text-primary); }
.header-actions { display: flex; gap: 8px; }
.pagination-row { display: flex; justify-content: space-between; align-items: center; margin-top: 16px; }
.total-text { font-size: var(--font-size-sm); color: var(--color-text-tertiary); }

.th-filter { display: inline-flex; align-items: center; gap: 4px; white-space: nowrap; }
.filter-icon { font-size: 12px; color: var(--color-text-quaternary); cursor: pointer; }
.filter-icon:hover { color: var(--color-primary); }
.filter-icon.active { color: var(--color-primary); }
</style>
