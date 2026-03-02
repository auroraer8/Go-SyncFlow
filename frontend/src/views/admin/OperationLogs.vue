<template>
  <div class="page-container">
    <!-- 统计概览 -->
    <div class="stat-row">
      <div class="stat-card">
        <span class="stat-value">{{ stats.todayTotal || 0 }}</span>
        <span class="stat-label">今日操作</span>
      </div>
      <div class="stat-card">
        <span class="stat-value">{{ stats.createCount || 0 }}</span>
        <span class="stat-label">新增</span>
      </div>
      <div class="stat-card">
        <span class="stat-value">{{ stats.updateCount || 0 }}</span>
        <span class="stat-label">修改</span>
      </div>
      <div class="stat-card stat-error">
        <span class="stat-value">{{ stats.deleteCount || 0 }}</span>
        <span class="stat-label">删除</span>
      </div>
    </div>

    <el-card class="table-card">
      <template #header>
        <div class="filter-row">
          <el-input v-model="filters.username" placeholder="用户名" clearable style="width: 120px" size="small" @keyup.enter="loadLogs" />
          <el-select v-model="filters.module" placeholder="全部模块" clearable style="width: 120px" size="small" @change="loadLogs">
            <el-option label="用户管理" value="用户管理" />
            <el-option label="角色管理" value="角色管理" />
            <el-option label="系统设置" value="系统设置" />
            <el-option label="安全中心" value="安全中心" />
          </el-select>
          <el-date-picker
            v-model="filters.dateRange"
            type="daterange"
            range-separator="-"
            start-placeholder="开始"
            end-placeholder="结束"
            value-format="YYYY-MM-DD"
            style="width: 220px"
            size="small"
            clearable
            @change="loadLogs"
          />
          <el-button type="primary" size="small" @click="loadLogs">搜索</el-button>
          <div style="flex: 1" />
          <el-button size="small" :disabled="selectedRows.length === 0" @click="exportSelected">
            <el-icon style="margin-right: 4px;"><Download /></el-icon>导出选中 ({{ selectedRows.length }})
          </el-button>
          <el-button size="small" @click="exportAll">
            <el-icon style="margin-right: 4px;"><Download /></el-icon>导出全部
          </el-button>
        </div>
      </template>
      <el-table :data="logs" v-loading="loading" stripe border size="default" @selection-change="onSelectionChange">
        <el-table-column type="selection" width="40" />
        <el-table-column prop="id" label="ID" width="70" align="center" />
        <el-table-column prop="username" label="操作人" width="100" />
        <el-table-column prop="module" label="模块" width="120">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" type="info">{{ row.module }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="action" label="操作" width="120">
          <template #default="{ row }">
            <el-tag size="small" :type="getActionType(row.action)" effect="light">{{ row.action }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="target" label="操作目标" min-width="160" show-overflow-tooltip />
        <el-table-column prop="content" label="详情" min-width="200" show-overflow-tooltip />
        <el-table-column prop="ip" label="IP地址" width="140" />
        <el-table-column prop="createdAt" label="操作时间" width="170">
          <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
        </el-table-column>
      </el-table>

      <div class="pagination-row">
        <span class="total-text">共 {{ pagination.total }} 条</span>
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.size"
          :total="pagination.total"
          :page-sizes="[20, 50, 100]"
          layout="sizes, prev, pager, next"
          @current-change="loadLogs"
          @size-change="loadLogs"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { Download } from "@element-plus/icons-vue";
import { logApi } from "../../api";

const loading = ref(false);
const logs = ref<any[]>([]);
const selectedRows = ref<any[]>([]);
const filters = reactive({ username: "", module: "", dateRange: null as any });
const pagination = reactive({ page: 1, size: 20, total: 0 });
const stats = ref<any>({});

const loadStats = async () => {
  try {
    const res = await logApi.operationLogStats();
    stats.value = (res as any).data?.data || {};
  } catch {}
};

const getActionType = (action: string) => {
  if (!action) return 'info';
  if (action.includes('新增') || action.includes('创建')) return 'success';
  if (action.includes('删除')) return 'danger';
  if (action.includes('更新') || action.includes('编辑') || action.includes('修改')) return 'warning';
  if (action.includes('分配')) return 'primary';
  return 'info';
};

const formatTime = (time: string) => {
  if (!time) return '-';
  return new Date(time).toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' });
};

const onSelectionChange = (rows: any[]) => { selectedRows.value = rows; };

const loadLogs = async () => {
  loading.value = true;
  try {
    const params: any = { pageIndex: pagination.page - 1, pageSize: pagination.size, username: filters.username, module: filters.module };
    if (filters.dateRange && filters.dateRange.length === 2) { params.startDate = filters.dateRange[0]; params.endDate = filters.dateRange[1]; }
    const res = await logApi.operationLogs(params);
    if (res.data.success) { logs.value = res.data.data.list || []; pagination.total = res.data.data.total || 0; }
  } catch { /* handled */ } finally { loading.value = false; }
};

function esc(s: string): string { if (!s) return ''; s = s.replace(/"/g, '""'); return (s.includes(',') || s.includes('"') || s.includes('\n')) ? '"' + s + '"' : s; }

function buildCSV(rows: any[]): string {
  const lines = rows.map(r => [r.id, esc(r.username||''), esc(r.module||''), esc(r.action||''), esc(r.target||''), esc(r.content||''), esc(r.ip||''), esc(formatTime(r.createdAt))].join(','));
  return '\uFEFF' + 'ID,操作人,模块,操作,操作目标,详情,IP地址,操作时间\n' + lines.join('\n');
}

function download(csv: string, name: string) { const b = new Blob([csv], {type:'text/csv;charset=utf-8;'}); const u = URL.createObjectURL(b); const a = document.createElement('a'); a.href = u; a.download = name; a.click(); URL.revokeObjectURL(u); }

const exportSelected = () => {
  if (!selectedRows.value.length) { ElMessage.warning('请先选择要导出的日志'); return; }
  download(buildCSV(selectedRows.value), `operation_logs_selected_${selectedRows.value.length}.csv`);
  ElMessage.success(`已导出 ${selectedRows.value.length} 条`);
};

const exportAll = async () => {
  ElMessage.info('正在获取全部日志...');
  try {
    const res = await logApi.operationLogs({ pageIndex: 0, pageSize: 10000, username: filters.username, module: filters.module });
    const all = res.data?.data?.list || [];
    if (!all.length) { ElMessage.warning('没有可导出的日志'); return; }
    download(buildCSV(all), `operation_logs_all_${all.length}.csv`);
    ElMessage.success(`已导出 ${all.length} 条`);
  } catch { ElMessage.error('导出失败'); }
};

onMounted(() => { loadLogs(); loadStats(); });
</script>

<style scoped>
/* ============================================
   Corporate Operation Logs - 企业风格
   ============================================ */
.page-container { display: flex; flex-direction: column; gap: 16px; }

/* 统计卡片行 */
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
.stat-error .stat-value { color: var(--color-error); }

.filter-row { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
.pagination-row { display: flex; justify-content: space-between; align-items: center; margin-top: 16px; }
.total-text { font-size: var(--font-size-sm); color: var(--color-text-tertiary); }

@media (max-width: 768px) {
  .stat-row { grid-template-columns: repeat(2, 1fr); }
}
</style>
