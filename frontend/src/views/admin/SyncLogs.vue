<template>
  <div class="page-container">
    <el-card class="table-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">同步日志</span>
          <div class="header-actions">
            <el-button size="small" @click="exportLogs" :loading="exporting">
              <el-icon><Download /></el-icon> 导出Excel
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="logs" v-loading="loading" border size="small" row-key="id" table-layout="auto" style="width: 100%">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="expand-content">
              <div class="detail-grid">
                <div class="detail-item" v-if="row.username">
                  <span class="detail-label">用户</span>
                  <span class="detail-value">{{ row.username }}</span>
                </div>
                <div class="detail-item" v-if="row.triggerEvent">
                  <span class="detail-label">事件</span>
                  <span class="detail-value">{{ eventMap[row.triggerEvent] || row.triggerEvent }}</span>
                </div>
                <div class="detail-item" v-if="row.message">
                  <span class="detail-label">结果</span>
                  <span class="detail-value">{{ row.message }}</span>
                </div>
                <div class="detail-item" v-if="row.duration">
                  <span class="detail-label">耗时</span>
                  <span class="detail-value">{{ row.duration }}ms</span>
                </div>
              </div>
              <template v-if="row.detail">
                <div class="detail-raw-label">详情:</div>
                <div class="detail-raw">{{ row.detail }}</div>
              </template>
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

        <el-table-column label="方向" width="80" align="center">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>方向</span>
              <el-popover placement="bottom" :width="120" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.direction }"><Filter /></el-icon>
                </template>
                <el-select v-model="filters.direction" placeholder="全部" clearable size="small" @change="loadLogs" style="width: 100%">
                  <el-option label="上游" value="upstream" />
                  <el-option label="下游" value="downstream" />
                </el-select>
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">
            <el-tag :type="row.direction === 'upstream' ? 'primary' : 'success'" size="small">
              {{ row.direction === 'upstream' ? '上游' : '下游' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="触发" width="70" align="center">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>触发</span>
              <el-popover placement="bottom" :width="100" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.triggerType }"><Filter /></el-icon>
                </template>
                <el-select v-model="filters.triggerType" placeholder="全部" clearable size="small" @change="loadLogs" style="width: 100%">
                  <el-option label="事件" value="event" />
                  <el-option label="定时" value="schedule" />
                  <el-option label="手动" value="manual" />
                </el-select>
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">
            <el-tag size="small" :type="row.triggerType === 'event' ? 'warning' : (row.triggerType === 'schedule' ? 'primary' : 'info')">
              {{ triggerTypeMap[row.triggerType] || row.triggerType }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="事件" width="90">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>事件</span>
              <el-popover placement="bottom" :width="120" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.event }"><Filter /></el-icon>
                </template>
                <el-select v-model="filters.event" placeholder="全部" clearable size="small" @change="loadLogs" style="width: 100%">
                  <el-option label="全量同步" value="full_sync" />
                  <el-option label="用户创建" value="user_create" />
                  <el-option label="用户更新" value="user_update" />
                  <el-option label="用户删除" value="user_delete" />
                  <el-option label="用户启用" value="user_enable" />
                  <el-option label="用户禁用" value="user_disable" />
                  <el-option label="密码修改" value="password_change" />
                  <el-option label="状态变更" value="user_status_change" />
                </el-select>
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">
            {{ eventMap[row.triggerEvent] || row.triggerEvent || '-' }}
          </template>
        </el-table-column>

        <el-table-column label="状态" width="70" align="center">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>状态</span>
              <el-popover placement="bottom" :width="100" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.status }"><Filter /></el-icon>
                </template>
                <el-select v-model="filters.status" placeholder="全部" clearable size="small" @change="loadLogs" style="width: 100%">
                  <el-option label="成功" value="success" />
                  <el-option label="部分成功" value="partial" />
                  <el-option label="失败" value="failed" />
                </el-select>
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : (row.status === 'partial' ? 'warning' : 'danger')" size="small">
              {{ statusMap[row.status] || row.status }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="数据源" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.connectorName || '-' }}
          </template>
        </el-table-column>

        <el-table-column label="概要" min-width="300" show-overflow-tooltip>
          <template #header>
            <div class="th-filter" @click.stop>
              <span>概要</span>
              <el-popover placement="bottom" :width="200" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filters.keyword }"><Filter /></el-icon>
                </template>
                <el-input v-model="filters.keyword" placeholder="搜索概要内容" clearable size="small" @keyup.enter="loadLogs" @clear="loadLogs" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">
            <span v-if="row.username" class="summary-user">{{ row.username }} </span>
            <span class="summary-msg">{{ row.message }}</span>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-row">
        <span class="total-text">共 {{ pagination.total }} 条</span>
        <el-pagination v-model:current-page="pagination.page" v-model:page-size="pagination.size"
          :total="pagination.total" :page-sizes="[20, 50, 100]"
          layout="sizes, prev, pager, next" @current-change="loadLogs" @size-change="loadLogs" />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { Filter, Download } from "@element-plus/icons-vue";
import { logApi } from "../../api";
import { useWebSocketStore } from "../../store/websocket";
import * as XLSX from 'xlsx';

const wsStore = useWebSocketStore();
let wsUnsubscribe: (() => void) | null = null;

const loading = ref(false);
const exporting = ref(false);
const logs = ref<any[]>([]);
const filters = reactive({ direction: "", event: "", status: "", triggerType: "", keyword: "", dateRange: null as any });
const pagination = reactive({ page: 1, size: 20, total: 0 });

const triggerTypeMap: Record<string, string> = { event: '事件', schedule: '定时', manual: '手动' };
const eventMap: Record<string, string> = {
  password_change: '密码修改', full_sync: '全量同步', user_create: '用户创建',
  user_update: '用户更新', user_delete: '用户删除', user_status_change: '状态变更',
  user_enable: '用户启用', user_disable: '用户禁用',
  role_change: '角色变更', dingtalk_sync: '钉钉同步',
  group_create: '分组创建', group_update: '分组更新', group_delete: '分组删除',
  role_create: '角色创建', role_update: '角色更新', role_delete: '角色删除',
};
const statusMap: Record<string, string> = { success: '成功', partial: '部分', failed: '失败' };

const formatTime = (time: string) => {
  if (!time) return '-';
  return time.replace('T', ' ').slice(0, 19);
};

const loadLogs = async () => {
  loading.value = true;
  try {
    const params: any = {
      page: pagination.page, size: pagination.size,
      event: filters.event || undefined,
      status: filters.status || undefined,
      direction: filters.direction || undefined,
      triggerType: filters.triggerType || undefined,
      keyword: filters.keyword || undefined
    };
    if (filters.dateRange && filters.dateRange.length === 2) {
      params.startDate = filters.dateRange[0];
      params.endDate = filters.dateRange[1];
    }
    const res = await logApi.syncLogs(params);
    const data = (res as any).data?.data;
    logs.value = data?.list || [];
    pagination.total = data?.total || 0;
  } finally { loading.value = false; }
};

const exportLogs = async () => {
  exporting.value = true;
  try {
    const allLogs: any[] = [];
    let page = 1;
    const size = 500;
    while (true) {
      const params: any = { page, size };
      if (filters.direction) params.direction = filters.direction;
      if (filters.event) params.event = filters.event;
      if (filters.status) params.status = filters.status;
      if (filters.triggerType) params.triggerType = filters.triggerType;
      if (filters.keyword) params.keyword = filters.keyword;
      if (filters.dateRange?.length === 2) {
        params.startDate = filters.dateRange[0];
        params.endDate = filters.dateRange[1];
      }
      const res = await logApi.syncLogs(params);
      const data = (res as any).data?.data;
      const list = data?.list || [];
      allLogs.push(...list);
      if (list.length < size) break;
      page++;
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
    '方向': r.direction === 'upstream' ? '上游同步' : '下游同步',
    '触发': triggerTypeMap[r.triggerType] || r.triggerType,
    '事件': eventMap[r.triggerEvent] || r.triggerEvent || '',
    '用户': r.username || '',
    '数据源': r.connectorName || '',
    '状态': statusMap[r.status] || r.status,
    '概要': r.message || '',
    '耗时(ms)': r.duration || 0
  }));
  const ws = XLSX.utils.json_to_sheet(rows);
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, 'Sheet1');
  XLSX.writeFile(wb, `同步日志_${new Date().toISOString().slice(0, 10)}.xlsx`);
};

onMounted(() => {
  loadLogs();
  
  // 订阅同步日志事件（实时刷新）
  wsStore.connect();
  wsUnsubscribe = wsStore.subscribe('sync_log', (data) => {
    console.log('[同步日志] 收到新同步日志:', data);
    // 如果在第一页且没有筛选条件，直接将新日志添加到列表顶部
    if (pagination.page === 1 && !filters.direction && !filters.event && !filters.status && !filters.keyword) {
      logs.value.unshift(data);
      pagination.total++;
      // 保持列表长度不超过 pageSize
      if (logs.value.length > pagination.size) {
        logs.value.pop();
      }
    }
  });
});

onUnmounted(() => {
  if (wsUnsubscribe) wsUnsubscribe();
});
</script>

<style scoped>
.page-container { display: flex; flex-direction: column; gap: 16px; }
.table-card :deep(.el-card__body) { padding: 16px; }
.card-header { display: flex; justify-content: space-between; align-items: center; }
.card-title { font-size: 16px; font-weight: 600; color: var(--color-text-primary); }
.header-actions { display: flex; gap: 8px; }
.pagination-row { display: flex; justify-content: space-between; align-items: center; margin-top: 16px; }
.total-text { font-size: 14px; color: var(--color-text-tertiary); }

.th-filter { display: inline-flex; align-items: center; gap: 4px; white-space: nowrap; }
.filter-icon { font-size: 12px; color: var(--color-text-quaternary); cursor: pointer; }
.filter-icon:hover { color: var(--color-primary); }
.filter-icon.active { color: var(--color-primary); }

.expand-content { padding: 12px 20px; background: var(--color-fill-light); }
.detail-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 12px; }
.detail-item { display: flex; flex-direction: column; gap: 2px; }
.detail-label { color: var(--color-text-tertiary); font-size: 12px; }
.detail-value { color: var(--color-text-primary); font-size: 13px; font-weight: 500; }
.detail-raw-label { color: var(--color-text-tertiary); font-size: 12px; margin-bottom: 4px; }
.detail-raw { background: var(--color-fill-secondary); border: 1px solid var(--color-border); border-radius: 4px; padding: 10px; font-size: 12px; line-height: 1.6; white-space: pre-wrap; word-break: break-all; max-height: 200px; overflow-y: auto; }

.summary-user { color: var(--color-primary); font-weight: 500; }
.summary-msg { color: var(--color-text-secondary); }

@media (max-width: 1200px) {
  .detail-grid { grid-template-columns: repeat(2, 1fr); }
}
</style>
