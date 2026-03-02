<template>
  <div class="log-settings-page">
    <!-- 存储统计 -->
    <div class="stat-row" v-if="retentionStats">
      <div class="stat-card" v-for="item in statItems" :key="item.key">
        <span class="stat-value">{{ item.count }}</span>
        <span class="stat-label">{{ item.label }}</span>
        <span class="stat-sub" v-if="item.oldest">最早: {{ item.oldest }}</span>
      </div>
    </div>

    <!-- 保留策略配置 -->
    <el-card>
      <template #header>
        <div class="card-header-row">
          <span class="card-title">日志保留策略</span>
          <div class="header-actions">
            <el-button type="danger" @click="cleanNow" :loading="cleaning">
              <el-icon><Delete /></el-icon> 立即清理
            </el-button>
            <el-button type="primary" @click="saveRetention" :loading="saving">
              保存设置
            </el-button>
          </div>
        </div>
      </template>

      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 20px">
        设置各类日志的保留天数。超过保留期限的日志将被自动清理。设为 0 表示永不清理。
      </el-alert>

      <div class="retention-grid">
        <div class="retention-item" v-for="item in retentionItems" :key="item.key">
          <div class="retention-header">
            <el-icon :class="item.iconClass"><component :is="item.icon" /></el-icon>
            <span class="retention-label">{{ item.label }}</span>
          </div>
          <div class="retention-body">
            <el-input-number
              v-model="retentionForm[item.key as keyof typeof retentionForm]"
              :min="0" :max="3650" :step="30"
              class="retention-input"
            />
            <span class="retention-unit">天</span>
          </div>
          <div class="retention-hint">{{ item.hint }}</div>
        </div>
      </div>

      <el-divider />

      <div class="auto-clean-section">
        <div class="section-title">自动清理</div>
        <el-form label-width="120px" style="max-width: 500px">
          <el-form-item label="启用自动清理">
            <el-switch v-model="retentionForm.autoCleanEnabled" />
          </el-form-item>
          <el-form-item label="清理时间" v-if="retentionForm.autoCleanEnabled">
            <el-time-picker v-model="retentionForm.autoCleanTime" format="HH:mm" value-format="HH:mm" placeholder="每天执行清理的时间" style="width: 140px" />
            <span class="field-hint">建议选择业务低峰时段</span>
          </el-form-item>
        </el-form>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Delete, Tickets, Key, Connection, DataLine, Warning, Link } from "@element-plus/icons-vue";
import { logManagementApi } from "../../api";

const saving = ref(false);
const cleaning = ref(false);
const retentionStats = ref<any>(null);

const retentionForm = reactive({
  loginLogDays: 7,
  operationLogDays: 15,
  syncLogDays: 7,
  apiAccessLogDays: 7,
  securityEventDays: 30,
  alertLogDays: 7,
  vpnLogDays: 30,
  vpnAuditLogDays: 30,
  autoCleanEnabled: true,
  autoCleanTime: '03:00'
});

const retentionItems = [
  { key: 'loginLogDays', label: '登录日志', icon: Key, iconClass: 'icon-primary', hint: '用户登录/退出记录' },
  { key: 'operationLogDays', label: '操作日志', icon: Tickets, iconClass: 'icon-success', hint: '用户和管理员操作记录' },
  { key: 'syncLogDays', label: '同步日志', icon: Connection, iconClass: 'icon-warning', hint: '上下游数据同步记录' },
  { key: 'apiAccessLogDays', label: 'API调用日志', icon: DataLine, iconClass: 'icon-info', hint: 'OpenAPI 调用明细' },
  { key: 'securityEventDays', label: '安全事件', icon: Warning, iconClass: 'icon-error', hint: '异常登录、暴力破解等安全事件' },
  { key: 'alertLogDays', label: '告警日志', icon: Delete, iconClass: 'icon-error', hint: '告警通知发送记录' },
  { key: 'vpnLogDays', label: 'VPN活动日志', icon: Link, iconClass: 'icon-vpn', hint: 'VPN用户登录/登出活动记录' },
  { key: 'vpnAuditLogDays', label: 'VPN审计日志', icon: Link, iconClass: 'icon-vpn', hint: 'VPN访问审计记录' }
];

const statItems = computed(() => {
  if (!retentionStats.value) return [];
  const data = retentionStats.value;
  return [
    { key: 'login', label: '登录日志', count: data.loginLogCount || 0, oldest: data.loginLogOldest },
    { key: 'operation', label: '操作日志', count: data.operationLogCount || 0, oldest: data.operationLogOldest },
    { key: 'sync', label: '同步日志', count: data.syncLogCount || 0, oldest: data.syncLogOldest },
    { key: 'api', label: 'API日志', count: data.apiAccessLogCount || 0, oldest: data.apiAccessLogOldest },
    { key: 'security', label: '安全事件', count: data.securityEventCount || 0, oldest: data.securityEventOldest },
    { key: 'alert', label: '告警日志', count: data.alertLogCount || 0, oldest: data.alertLogOldest },
    { key: 'vpn', label: 'VPN活动', count: data.vpnLogCount || 0, oldest: data.vpnLogOldest },
    { key: 'vpnAudit', label: 'VPN审计', count: data.vpnAuditLogCount || 0, oldest: data.vpnAuditLogOldest }
  ];
});

const loadRetention = async () => {
  try {
    const res = await logManagementApi.getRetention();
    const data = (res as any).data?.data;
    if (data) {
      Object.assign(retentionForm, data);
    }
  } catch {}
};

const loadStats = async () => {
  try {
    const res = await logManagementApi.retentionStats();
    retentionStats.value = (res as any).data?.data || null;
  } catch {}
};

const saveRetention = async () => {
  saving.value = true;
  try {
    await logManagementApi.updateRetention(retentionForm);
    ElMessage.success('保存成功');
  } finally { saving.value = false; }
};

const cleanNow = async () => {
  try {
    await ElMessageBox.confirm('确定立即清理超过保留期限的日志？此操作不可恢复。', '确认清理', { type: 'warning' });
    cleaning.value = true;
    const res = await logManagementApi.cleanNow();
    const data = (res as any).data?.data;
    ElMessage.success(data?.message || '清理完成');
    loadStats();
  } catch {} finally { cleaning.value = false; }
};

onMounted(() => { loadRetention(); loadStats(); });
</script>

<style scoped>
/* ============================================
   Corporate Log Settings - 企业风格
   ============================================ */
.log-settings-page { 
  display: flex; 
  flex-direction: column; 
  gap: var(--spacing-lg); 
}

/* 统计卡片行 */
.stat-row {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: var(--spacing-md);
}
.stat-card {
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  padding: var(--spacing-md);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  transition: box-shadow var(--transition-fast), border-color var(--transition-fast);
}
.stat-card:hover { 
  box-shadow: var(--card-shadow-hover); 
  border-color: var(--color-primary-border);
}
.stat-value { 
  font-size: var(--font-size-xl); 
  font-weight: 700; 
  color: var(--color-primary); 
}
.stat-label { 
  font-size: var(--font-size-xs); 
  color: var(--color-text-secondary);
  white-space: nowrap;
}
.stat-sub { 
  font-size: 10px; 
  color: var(--color-text-quaternary); 
  white-space: nowrap;
}

.card-header-row { display: flex; justify-content: space-between; align-items: center; }
.card-title { font-size: var(--font-size-lg); font-weight: 600; color: var(--color-text-primary); }
.header-actions { display: flex; gap: var(--spacing-sm); }

.retention-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: var(--spacing-lg); }
.retention-item {
  border: 1px solid var(--card-border); 
  border-radius: var(--card-radius);
  padding: var(--spacing-lg); 
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  background: var(--card-bg);
}
.retention-item:hover { 
  border-color: var(--color-primary-border); 
  box-shadow: var(--card-shadow-hover);
}
.retention-header { display: flex; align-items: center; gap: var(--spacing-sm); margin-bottom: var(--spacing-md); }
.retention-label { font-weight: 500; color: var(--color-text-primary); }
.retention-body { display: flex; align-items: center; gap: var(--spacing-sm); margin-bottom: var(--spacing-sm); }
.retention-input { width: 140px; }
.retention-unit { color: var(--color-text-tertiary); font-size: var(--font-size-sm); }
.retention-hint { font-size: var(--font-size-xs); color: var(--color-text-quaternary); }

.icon-primary { color: var(--color-primary); font-size: 18px; }
.icon-success { color: var(--color-success); font-size: 18px; }
.icon-warning { color: var(--color-warning); font-size: 18px; }
.icon-info { color: #909399; font-size: 18px; }
.icon-error { color: var(--color-error); font-size: 18px; }
.icon-vpn { color: #67c23a; font-size: 18px; }

.section-title { font-size: var(--font-size-base); font-weight: 600; margin-bottom: var(--spacing-lg); color: var(--color-text-primary); }
.field-hint { margin-left: var(--spacing-sm); color: var(--color-text-tertiary); font-size: var(--font-size-xs); }

@media (max-width: 1400px) {
  .stat-row { grid-template-columns: repeat(4, 1fr); }
}
@media (max-width: 1200px) {
  .retention-grid { grid-template-columns: repeat(2, 1fr); }
}
@media (max-width: 768px) {
  .stat-row { grid-template-columns: repeat(2, 1fr); }
  .retention-grid { grid-template-columns: 1fr; }
}
</style>
