<template>
  <div class="api-logs-page">
    <!-- 统计概览 -->
    <div class="stat-row">
      <div class="stat-card">
        <span class="stat-value">{{ stats.totalToday || 0 }}</span>
        <span class="stat-label">今日调用</span>
      </div>
      <div class="stat-card stat-success">
        <span class="stat-value">{{ formatSuccessRate(stats.successRate) }}</span>
        <span class="stat-label">成功率</span>
      </div>
      <div class="stat-card">
        <span class="stat-value">{{ stats.avgDuration || 0 }}ms</span>
        <span class="stat-label">平均耗时</span>
      </div>
      <div class="stat-card stat-error">
        <span class="stat-value">{{ stats.errorCount || 0 }}</span>
        <span class="stat-label">今日异常</span>
      </div>
    </div>

    <!-- 日志列表 -->
    <el-card>
      <template #header>
        <div class="card-header">
          <span class="card-title">API 调用日志</span>
          <div class="header-actions">
            <el-button size="small" @click="exportLogs" :loading="exporting">
              <el-icon><Download /></el-icon> 导出Excel
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="logs" v-loading="loading" stripe size="small" row-key="id" border style="width: 100%">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="log-expand">
              <div class="expand-section" v-if="row.query">
                <strong>Query: </strong><code>{{ row.query }}</code>
              </div>
              <div class="expand-section" v-if="row.requestBody">
                <strong>Request Body: </strong>
                <pre class="code-block">{{ row.requestBody }}</pre>
              </div>
              <div class="expand-section" v-if="row.errorMessage">
                <strong>Error: </strong><span class="text-error">{{ row.errorMessage }}</span>
              </div>
              <div class="expand-section">
                <strong>User-Agent: </strong><span class="text-muted">{{ row.userAgent || '-' }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="时间" width="160">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>时间</span>
              <el-popover placement="bottom" :width="260" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.dateRange?.length }"><Filter /></el-icon>
                </template>
                <el-date-picker v-model="filters.dateRange" type="daterange" range-separator="至"
                  start-placeholder="开始" end-placeholder="结束" value-format="YYYY-MM-DD"
                  size="small" clearable @change="loadLogs" style="width: 100%" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
        </el-table-column>

        <el-table-column label="认证" width="80" align="center">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>认证</span>
              <el-popover placement="bottom" :width="100" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.authType }"><Filter /></el-icon>
                </template>
                <el-select v-model="filters.authType" placeholder="全部" clearable size="small" @change="loadLogs" style="width: 100%">
                  <el-option label="APIKey" value="apikey" />
                  <el-option label="JWT" value="jwt" />
                </el-select>
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">
            <el-tag :type="row.authType === 'apikey' ? 'warning' : 'primary'" size="small" effect="light">
              {{ row.authType === 'apikey' ? 'APIKey' : 'JWT' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="调用者" width="140" show-overflow-tooltip>
          <template #header>
            <div class="th-filter" @click.stop>
              <span>调用者</span>
              <el-popover placement="bottom" :width="160" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.caller }"><Filter /></el-icon>
                </template>
                <el-input v-model="filters.caller" placeholder="搜索调用者" clearable size="small" @keyup.enter="loadLogs" @clear="loadLogs" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">
            <div class="caller-info">
              <span class="caller-name">{{ row.appName || row.username || '-' }}</span>
              <span class="caller-id text-muted" v-if="row.appName && row.appId">({{ row.appId }})</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="概要" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="summary-text">{{ row.summary || '-' }}</span>
          </template>
        </el-table-column>

        <el-table-column label="方法" width="70" align="center">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>方法</span>
              <el-popover placement="bottom" :width="100" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.method }"><Filter /></el-icon>
                </template>
                <el-select v-model="filters.method" placeholder="全部" clearable size="small" @change="loadLogs" style="width: 100%">
                  <el-option label="GET" value="GET" />
                  <el-option label="POST" value="POST" />
                  <el-option label="PUT" value="PUT" />
                  <el-option label="DELETE" value="DELETE" />
                </el-select>
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">
            <el-tag :type="methodTagType(row.method)" size="small" effect="plain">{{ row.method }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column label="路径" min-width="180" show-overflow-tooltip>
          <template #header>
            <div class="th-filter" @click.stop>
              <span>路径</span>
              <el-popover placement="bottom" :width="200" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.path }"><Filter /></el-icon>
                </template>
                <el-input v-model="filters.path" placeholder="输入路径关键词" clearable size="small" @keyup.enter="loadLogs" @clear="loadLogs" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">{{ row.path }}</template>
        </el-table-column>

        <el-table-column label="状态" width="70" align="center">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>状态</span>
              <el-popover placement="bottom" :width="100" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.statusGroup }"><Filter /></el-icon>
                </template>
                <el-select v-model="filters.statusGroup" placeholder="全部" clearable size="small" @change="loadLogs" style="width: 100%">
                  <el-option label="成功(2xx)" value="2xx" />
                  <el-option label="客户端错误(4xx)" value="4xx" />
                  <el-option label="服务端错误(5xx)" value="5xx" />
                </el-select>
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">
            <el-tag :type="row.statusCode < 400 ? 'success' : (row.statusCode < 500 ? 'warning' : 'danger')" size="small" effect="light">
              {{ row.statusCode }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="耗时" width="70" align="center">
          <template #default="{ row }">
            <span :class="{ 'text-error': row.duration > 1000 }">{{ row.duration }}ms</span>
          </template>
        </el-table-column>

        <el-table-column label="来源IP" width="120">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>IP</span>
              <el-popover placement="bottom" :width="150" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.ip }"><Filter /></el-icon>
                </template>
                <el-input v-model="filters.ip" placeholder="输入IP" clearable size="small" @keyup.enter="loadLogs" @clear="loadLogs" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">{{ row.ip }}</template>
        </el-table-column>

        <el-table-column label="响应" width="60" align="center">
          <template #default="{ row }">{{ formatSize(row.responseSize) }}</template>
        </el-table-column>
      </el-table>

      <div class="pagination-bar">
        <span class="total-text">共 {{ total }} 条</span>
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[20, 50, 100]"
          layout="sizes, prev, pager, next"
          @size-change="loadLogs"
          @current-change="loadLogs"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { Download, Filter } from "@element-plus/icons-vue";
import { logManagementApi } from "../../api";
import * as XLSX from 'xlsx';

const logs = ref<any[]>([]);
const loading = ref(false);
const exporting = ref(false);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const stats = ref<any>({});

const filters = reactive({
  authType: '', caller: '', method: '', path: '', ip: '',
  statusGroup: '', dateRange: null as string[] | null
});

const methodTagType = (m: string) => {
  const map: Record<string, string> = { GET: '', POST: 'success', PUT: 'warning', DELETE: 'danger' };
  return map[m] || 'info';
};

const formatTime = (t: string) => t ? t.replace('T', ' ').slice(0, 19) : '-';
const formatSize = (bytes: number) => {
  if (!bytes) return '-';
  if (bytes < 1024) return bytes + 'B';
  return (bytes / 1024).toFixed(1) + 'KB';
};

const formatSuccessRate = (rate: any) => {
  if (rate === undefined || rate === null) return '0%';
  const num = parseFloat(rate);
  if (isNaN(num)) return '0%';
  return num.toFixed(1) + '%';
};

const loadLogs = async () => {
  loading.value = true;
  try {
    const params: any = { page: page.value, size: pageSize.value };
    if (filters.authType) params.authType = filters.authType;
    if (filters.caller) params.caller = filters.caller;
    if (filters.method) params.method = filters.method;
    if (filters.path) params.path = filters.path;
    if (filters.ip) params.ip = filters.ip;
    if (filters.statusGroup) params.statusGroup = filters.statusGroup;
    if (filters.dateRange?.length === 2) {
      params.startDate = filters.dateRange[0];
      params.endDate = filters.dateRange[1];
    }
    const res = await logManagementApi.apiLogs(params);
    const data = (res as any).data?.data;
    logs.value = data?.list || [];
    total.value = data?.total || 0;
  } finally { loading.value = false; }
};

const loadStats = async () => {
  try {
    const res = await logManagementApi.apiLogStats();
    stats.value = (res as any).data?.data || {};
  } catch {}
};

const exportLogs = async () => {
  exporting.value = true;
  try {
    const allLogs: any[] = [];
    let p = 1;
    const size = 500;
    while (true) {
      const params: any = { page: p, size };
      if (filters.authType) params.authType = filters.authType;
      if (filters.caller) params.caller = filters.caller;
      if (filters.method) params.method = filters.method;
      if (filters.path) params.path = filters.path;
      if (filters.ip) params.ip = filters.ip;
      if (filters.statusGroup) params.statusGroup = filters.statusGroup;
      if (filters.dateRange?.length === 2) {
        params.startDate = filters.dateRange[0];
        params.endDate = filters.dateRange[1];
      }
      const res = await logManagementApi.apiLogs(params);
      const list = (res as any).data?.data?.list || [];
      allLogs.push(...list);
      if (list.length < size) break;
      p++;
    }
    if (!allLogs.length) {
      ElMessage.warning('没有可导出的数据');
      return;
    }
    downloadExcel(allLogs);
    ElMessage.success(`成功导出 ${allLogs.length} 条记录`);
  } catch (e) {
    ElMessage.error('导出失败');
  } finally {
    exporting.value = false;
  }
};

const downloadExcel = (data: any[]) => {
  const rows = data.map(r => ({
    '时间': formatTime(r.createdAt),
    '认证类型': r.authType === 'apikey' ? 'APIKey' : 'JWT',
    '调用者': r.appId || r.username || '',
    '方法': r.method,
    '路径': r.path,
    '状态码': r.statusCode,
    '耗时(ms)': r.duration,
    '来源IP': r.ip,
    '响应大小': r.responseSize || 0
  }));
  const ws = XLSX.utils.json_to_sheet(rows);
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, 'Sheet1');
  XLSX.writeFile(wb, `API调用日志_${new Date().toISOString().slice(0, 10)}.xlsx`);
};

onMounted(() => { loadLogs(); loadStats(); });
</script>

<style scoped>
.api-logs-page { display: flex; flex-direction: column; gap: var(--spacing-lg); }

.stat-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--spacing-md);
}
.stat-card {
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  padding: var(--spacing-lg);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  transition: box-shadow var(--transition-fast), border-color var(--transition-fast);
}
.stat-card:hover { 
  box-shadow: var(--card-shadow-hover);
  border-color: var(--color-primary-border);
}
.stat-value { 
  font-size: var(--font-size-2xl); 
  font-weight: 700; 
  color: var(--color-primary); 
}
.stat-label { 
  font-size: var(--font-size-sm); 
  color: var(--color-text-secondary); 
}
.stat-success .stat-value { color: var(--color-success); }
.stat-error .stat-value { color: var(--color-error); }

.card-header { display: flex; justify-content: space-between; align-items: center; }
.card-title { font-size: 16px; font-weight: 600; color: var(--color-text-primary); }
.header-actions { display: flex; gap: 8px; }

.pagination-bar { margin-top: var(--spacing-md); display: flex; justify-content: space-between; align-items: center; }
.total-text { font-size: var(--font-size-sm); color: var(--color-text-tertiary); }

.th-filter { display: inline-flex; align-items: center; gap: 4px; white-space: nowrap; }
.filter-icon { font-size: 12px; color: var(--color-text-quaternary); cursor: pointer; }
.filter-icon:hover { color: var(--color-primary); }
.filter-icon.active { color: var(--color-primary); }

.log-expand { padding: var(--spacing-md) var(--spacing-xl); }
.expand-section { margin-bottom: var(--spacing-sm); font-size: var(--font-size-sm); }
.code-block {
  background: var(--color-fill-secondary); padding: var(--spacing-sm);
  border-radius: var(--radius-sm); font-size: var(--font-size-xs);
  max-height: 200px; overflow: auto; white-space: pre-wrap; word-break: break-all;
}
.text-muted { color: var(--color-text-tertiary); }
.text-error { color: var(--color-error); }

.caller-info { display: flex; flex-direction: column; gap: 2px; }
.caller-name { font-weight: 500; color: var(--color-text-primary); }
.caller-id { font-size: var(--font-size-xs); }
.summary-text { color: var(--color-text-secondary); }

@media (max-width: 768px) {
  .stat-row { grid-template-columns: repeat(2, 1fr); }
}
</style>
