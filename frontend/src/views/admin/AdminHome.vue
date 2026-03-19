<template>
  <div class="dashboard">
    <!-- 顶部状态条 -->
    <header class="dashboard-header">
      <div class="header-left">
        <h1>{{ greeting }}，{{ userStore.user?.nickname || userStore.user?.username }}</h1>
        <span class="header-subtitle">系统运行正常，祝您工作愉快</span>
      </div>
      <div class="header-stats">
        <div class="header-stat">
          <span class="stat-value">{{ systemStatus.runtime?.uptime || '-' }}</span>
          <span class="stat-label">运行时长</span>
        </div>
        <div class="header-stat">
          <span class="stat-value">{{ currentTime }}</span>
          <span class="stat-label">当前时间</span>
        </div>
        <div class="header-stat">
          <span class="stat-value">{{ systemStatus.runtime?.goroutines || 0 }}</span>
          <span class="stat-label">协程数</span>
        </div>
      </div>
    </header>

    <!-- 系统资源监控 -->
    <section class="metrics-row">
      <div class="metric-card" :class="getStatusClass(systemStatus.cpu?.usage)">
        <div class="metric-ring">
          <svg viewBox="0 0 100 100">
            <circle class="ring-bg" cx="50" cy="50" r="42" />
            <circle class="ring-progress" cx="50" cy="50" r="42" 
              :style="{ strokeDasharray: `${(systemStatus.cpu?.usage || 0) * 2.64} 264` }" />
          </svg>
          <div class="ring-center">
            <span class="ring-value">{{ (systemStatus.cpu?.usage || 0).toFixed(1) }}</span>
            <span class="ring-unit">%</span>
          </div>
        </div>
        <div class="metric-info">
          <span class="metric-title">CPU 使用率</span>
          <span class="metric-detail">{{ systemStatus.cpu?.cores || 0 }} 核心</span>
        </div>
      </div>

      <div class="metric-card" :class="getStatusClass(systemStatus.memory?.percent)">
        <div class="metric-ring">
          <svg viewBox="0 0 100 100">
            <circle class="ring-bg" cx="50" cy="50" r="42" />
            <circle class="ring-progress" cx="50" cy="50" r="42" 
              :style="{ strokeDasharray: `${(systemStatus.memory?.percent || 0) * 2.64} 264` }" />
          </svg>
          <div class="ring-center">
            <span class="ring-value">{{ (systemStatus.memory?.percent || 0).toFixed(1) }}</span>
            <span class="ring-unit">%</span>
          </div>
        </div>
        <div class="metric-info">
          <span class="metric-title">内存使用</span>
          <span class="metric-detail">{{ (systemStatus.memory?.used || 0).toFixed(1) }} / {{ (systemStatus.memory?.total || 0).toFixed(1) }} GB</span>
        </div>
      </div>

      <div class="metric-card" :class="getStatusClass(systemStatus.disk?.percent, true)">
        <div class="metric-ring">
          <svg viewBox="0 0 100 100">
            <circle class="ring-bg" cx="50" cy="50" r="42" />
            <circle class="ring-progress" cx="50" cy="50" r="42" 
              :style="{ strokeDasharray: `${(systemStatus.disk?.percent || 0) * 2.64} 264` }" />
          </svg>
          <div class="ring-center">
            <span class="ring-value">{{ (systemStatus.disk?.percent || 0).toFixed(1) }}</span>
            <span class="ring-unit">%</span>
          </div>
        </div>
        <div class="metric-info">
          <span class="metric-title">磁盘使用</span>
          <span class="metric-detail">{{ (systemStatus.disk?.used || 0).toFixed(0) }} / {{ (systemStatus.disk?.total || 0).toFixed(0) }} GB</span>
        </div>
      </div>

      <div class="metric-card network">
        <div class="network-display">
          <div class="net-row upload">
            <el-icon><Top /></el-icon>
            <span class="net-value">{{ (systemStatus.network?.outRate || 0).toFixed(2) }}</span>
            <span class="net-unit">MB/s</span>
          </div>
          <div class="net-row download">
            <el-icon><Bottom /></el-icon>
            <span class="net-value">{{ (systemStatus.network?.inRate || 0).toFixed(2) }}</span>
            <span class="net-unit">MB/s</span>
          </div>
        </div>
        <div class="metric-info">
          <span class="metric-title">网络流量</span>
          <span class="metric-detail">实时速率</span>
        </div>
      </div>
    </section>

    <!-- 业务统计 -->
    <section class="stats-row">
      <div class="stat-card" @click="router.push('/admin/users/local')">
        <div class="stat-icon users"><el-icon><User /></el-icon></div>
        <div class="stat-content">
          <span class="stat-number">{{ dashboardStats.users?.total || 0 }}</span>
          <span class="stat-name">用户总数</span>
        </div>
        <div class="stat-footer">
          <span class="stat-tag success">启用 {{ dashboardStats.users?.active || 0 }}</span>
          <span class="stat-tag warning">禁用 {{ dashboardStats.users?.disabled || 0 }}</span>
          <span class="stat-tag info" v-if="dashboardStats.users?.today">今日 +{{ dashboardStats.users?.today }}</span>
        </div>
      </div>

      <div class="stat-card" @click="router.push('/admin/sync/upstream')">
        <div class="stat-icon connectors"><el-icon><Connection /></el-icon></div>
        <div class="stat-content">
          <span class="stat-number">{{ dashboardStats.connectors?.online || 0 }}</span>
          <span class="stat-name">在线连接器</span>
        </div>
        <div class="stat-footer">
          <span class="stat-tag primary">上游 {{ dashboardStats.connectors?.upstream || 0 }}</span>
          <span class="stat-tag">下游 {{ dashboardStats.connectors?.downstream || 0 }}</span>
        </div>
      </div>

      <div class="stat-card" @click="router.push('/admin/roles')">
        <div class="stat-icon roles"><el-icon><Stamp /></el-icon></div>
        <div class="stat-content">
          <span class="stat-number">{{ dashboardStats.roles?.total || 0 }}</span>
          <span class="stat-name">角色数量</span>
        </div>
        <div class="stat-footer">
          <span class="stat-tag">权限 {{ dashboardStats.roles?.permissions || 0 }}</span>
          <span class="stat-tag">分配 {{ dashboardStats.roles?.assignments || 0 }}</span>
        </div>
      </div>

      <div class="stat-card" @click="router.push('/admin/sso')">
        <div class="stat-icon sso"><el-icon><Key /></el-icon></div>
        <div class="stat-content">
          <span class="stat-number">{{ dashboardStats.ssoApps?.total || 0 }}</span>
          <span class="stat-name">SSO 应用</span>
        </div>
        <div class="stat-footer">
          <span class="stat-tag success">启用 {{ dashboardStats.ssoApps?.enabled || 0 }}</span>
          <span class="stat-tag" v-if="dashboardStats.sync?.lastSync !== '-'">同步 {{ dashboardStats.sync?.lastSync }}</span>
        </div>
      </div>
    </section>

    <!-- 底部双列：服务状态 + 快捷入口 -->
    <section class="bottom-row">
      <div class="services-panel">
        <div class="panel-header">
          <h3><el-icon><SetUp /></el-icon> 服务状态</h3>
          <el-button text size="small" @click="loadSystemStatus" :loading="refreshing">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>
        <div class="services-grid">
          <div class="service-item" :class="{ online: systemStatus.database?.status === 'ok' }">
            <div class="service-indicator"></div>
            <div class="service-icon db"><el-icon><DataLine /></el-icon></div>
            <div class="service-info">
              <span class="service-name">数据库</span>
              <span class="service-desc">{{ systemStatus.database?.type || 'PostgreSQL' }}</span>
            </div>
            <span class="service-status">{{ systemStatus.database?.status === 'ok' ? '正常' : '异常' }}</span>
          </div>

          <div class="service-item" :class="{ online: systemStatus.ldap?.running }">
            <div class="service-indicator"></div>
            <div class="service-icon ldap"><el-icon><Connection /></el-icon></div>
            <div class="service-info">
              <span class="service-name">LDAP</span>
              <span class="service-desc">端口 {{ systemStatus.ldap?.port || 389 }}</span>
            </div>
            <span class="service-status">{{ systemStatus.ldap?.running ? '运行中' : '已停止' }}</span>
          </div>

          <div class="service-item" :class="{ online: systemStatus.vpn?.running }">
            <div class="service-indicator"></div>
            <div class="service-icon vpn"><el-icon><Link /></el-icon></div>
            <div class="service-info">
              <span class="service-name">VPN</span>
              <span class="service-desc">端口 {{ systemStatus.vpn?.port || 443 }}</span>
            </div>
            <span class="service-status">{{ systemStatus.vpn?.running ? '运行中' : '已停止' }}</span>
          </div>

          <div class="service-item online">
            <div class="service-indicator"></div>
            <div class="service-icon api"><el-icon><Monitor /></el-icon></div>
            <div class="service-info">
              <span class="service-name">API 服务</span>
              <span class="service-desc">Go {{ systemStatus.runtime?.goVersion?.replace('go', '') || '-' }}</span>
            </div>
            <span class="service-status">运行中</span>
          </div>
        </div>
      </div>

      <div class="shortcuts-panel">
        <div class="panel-header">
          <h3><el-icon><Grid /></el-icon> 快捷入口</h3>
        </div>
        <div class="shortcuts-grid">
          <div class="shortcut-item" @click="router.push('/admin/users/local')">
            <el-icon><User /></el-icon>
            <span>本地用户</span>
          </div>
          <div class="shortcut-item" @click="router.push('/admin/roles')">
            <el-icon><Stamp /></el-icon>
            <span>角色管理</span>
          </div>
          <div class="shortcut-item" @click="router.push('/admin/sso')">
            <el-icon><Key /></el-icon>
            <span>单点登录</span>
          </div>
          <div class="shortcut-item" @click="router.push('/admin/vpn')">
            <el-icon><Link /></el-icon>
            <span>VPN 接入</span>
          </div>
          <div class="shortcut-item" @click="router.push('/admin/sync/upstream')">
            <el-icon><Upload /></el-icon>
            <span>上游同步</span>
          </div>
          <div class="shortcut-item" @click="router.push('/admin/sync/downstream')">
            <el-icon><Download /></el-icon>
            <span>下游同步</span>
          </div>
          <div class="shortcut-item" @click="router.push('/admin/settings/ldap')">
            <el-icon><Connection /></el-icon>
            <span>LDAP 服务</span>
          </div>
          <div class="shortcut-item" @click="router.push('/admin/settings')">
            <el-icon><Setting /></el-icon>
            <span>系统设置</span>
          </div>
        </div>
      </div>
    </section>

    <!-- 实时连接状态指示器 -->
    <div class="refresh-indicator" :class="{ connected: wsConnected }">
      <span v-if="wsConnected && !usePolling">● 实时</span>
      <span v-else-if="wsConnected && usePolling">● 轮询</span>
      <span v-else>○ 离线</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { 
  User, Connection, Stamp, Key, Link, SetUp, Refresh, DataLine, 
  Grid, Monitor, Setting, Top, Bottom, Upload, Download
} from "@element-plus/icons-vue";
import { useUserStore } from "../../store/user";
import { useWebSocketStore } from "../../store/websocket";
import { systemApi } from "../../api";

const router = useRouter();
const userStore = useUserStore();
const wsStore = useWebSocketStore();

// 状态数据
const systemStatus = ref<any>({});
const dashboardStats = ref<any>({});
const currentTime = ref('');
const refreshing = ref(false);

// 从全局 WebSocket Store 获取连接状态
const wsConnected = computed(() => wsStore.connected);
const usePolling = computed(() => wsStore.usePolling);

// 时钟定时器
let clockTimer: number | null = null;

// 事件取消订阅函数
let unsubscribe: (() => void) | null = null;

// 问候语
const greeting = computed(() => {
  const hour = new Date().getHours();
  if (hour < 6) return '夜深了';
  if (hour < 9) return '早上好';
  if (hour < 12) return '上午好';
  if (hour < 14) return '中午好';
  if (hour < 18) return '下午好';
  if (hour < 22) return '晚上好';
  return '夜深了';
});

// 状态颜色
const getStatusClass = (value: number, isDisk = false) => {
  const threshold = isDisk ? { warn: 80, danger: 90 } : { warn: 70, danger: 90 };
  if (value >= threshold.danger) return 'danger';
  if (value >= threshold.warn) return 'warning';
  return 'success';
};

// 更新时钟
const updateClock = () => {
  currentTime.value = new Date().toLocaleTimeString('zh-CN', { 
    hour: '2-digit', minute: '2-digit', second: '2-digit' 
  });
};

// 手动刷新（兼容旧逻辑）
const loadSystemStatus = async () => {
  refreshing.value = true;
  try {
    const res = await systemApi.status();
    if (res.data.success) {
      systemStatus.value = res.data.data;
    }
  } catch (e) { /* ignore */ }
  refreshing.value = false;
};

onMounted(() => {
  // 初始化时钟
  updateClock();
  clockTimer = window.setInterval(updateClock, 1000);
  
  // 先通过 HTTP 加载初始数据
  systemApi.status().then(res => {
    if (res.data.success) {
      systemStatus.value = res.data.data;
    }
  }).catch(() => {});
  
  systemApi.dashboardStats().then(res => {
    if (res.data.success) {
      dashboardStats.value = res.data.data;
    }
  }).catch(() => {});
  
  // 连接全局 WebSocket（如果还未连接）
  wsStore.connect();
  
  // 订阅系统指标和业务统计事件
  const unsub1 = wsStore.subscribe('system_metrics', (data) => {
    systemStatus.value = {
      ...systemStatus.value,
      cpu: data.cpu,
      memory: data.memory,
      disk: data.disk,
      network: data.network
    };
  });
  
  const unsub2 = wsStore.subscribe('business_stats', (data) => {
    dashboardStats.value = data;
  });
  
  // 合并取消订阅函数
  unsubscribe = () => {
    unsub1();
    unsub2();
  };
});

onUnmounted(() => {
  if (clockTimer) clearInterval(clockTimer);
  if (unsubscribe) unsubscribe();
});
</script>

<style scoped>
/* ============================================
   全屏自适应仪表盘
   ============================================ */
.dashboard {
  height: calc(100vh - 112px);
  display: grid;
  grid-template-rows: auto auto auto 1fr;
  gap: 16px;
  padding: 0;
  position: relative;
}

/* ============================================
   顶部状态条
   ============================================ */
.dashboard-header {
  background: var(--gradient-primary);
  border-radius: var(--radius-xl);
  padding: 20px 28px;
  color: white;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: var(--shadow-primary);
}

.header-left h1 {
  font-size: 20px;
  font-weight: 600;
  margin: 0 0 4px 0;
}

.header-subtitle {
  font-size: 13px;
  opacity: 0.85;
}

.header-stats {
  display: flex;
  gap: 16px;
}

.header-stat {
  text-align: center;
  padding: 10px 18px;
  background: rgba(255, 255, 255, 0.15);
  border-radius: var(--radius-lg);
  min-width: 80px;
  backdrop-filter: blur(10px);
}

.header-stat .stat-value {
  display: block;
  font-size: 18px;
  font-weight: 700;
}

.header-stat .stat-label {
  font-size: 11px;
  opacity: 0.8;
}

/* ============================================
   系统资源监控行
   ============================================ */
.metrics-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

.metric-card {
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  padding: 16px;
  display: flex;
  align-items: center;
  gap: 16px;
  box-shadow: var(--card-shadow);
  transition: all var(--transition-fast);
}

.metric-card:hover {
  box-shadow: var(--card-shadow-hover);
  transform: translateY(-2px);
}

/* 圆环进度 */
.metric-ring {
  width: 72px;
  height: 72px;
  position: relative;
  flex-shrink: 0;
}

.metric-ring svg {
  transform: rotate(-90deg);
  width: 100%;
  height: 100%;
}

.metric-ring circle {
  fill: none;
  stroke-width: 6;
  stroke-linecap: round;
}

.ring-bg {
  stroke: var(--color-fill-secondary);
}

.ring-progress {
  stroke: var(--color-primary);
  transition: stroke-dasharray 0.6s ease;
}

.metric-card.success .ring-progress { stroke: var(--color-success); }
.metric-card.warning .ring-progress { stroke: var(--color-warning); }
.metric-card.danger .ring-progress { stroke: var(--color-error); }

.ring-center {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
}

.ring-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-text-primary);
  line-height: 1;
}

.ring-unit {
  font-size: 10px;
  color: var(--color-text-tertiary);
}

.metric-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.metric-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.metric-detail {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

/* 网络流量卡片 */
.metric-card.network {
  background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
  border: none;
  color: white;
}

.network-display {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex-shrink: 0;
}

.net-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  background: rgba(255, 255, 255, 0.2);
  border-radius: var(--radius-md);
}

.net-row .el-icon { font-size: 14px; }
.net-row.upload .el-icon { color: #4ade80; }
.net-row.download .el-icon { color: #38bdf8; }

.net-value {
  font-size: 14px;
  font-weight: 700;
}

.net-unit {
  font-size: 10px;
  opacity: 0.8;
}

.metric-card.network .metric-info { color: white; }
.metric-card.network .metric-title { color: white; }
.metric-card.network .metric-detail { color: rgba(255, 255, 255, 0.8); }

/* ============================================
   业务统计行
   ============================================ */
.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

.stat-card {
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: var(--card-shadow);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.stat-card:hover {
  box-shadow: var(--card-shadow-hover);
  transform: translateY(-2px);
  border-color: var(--color-primary-border);
}

.stat-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  color: white;
}

.stat-icon.users { background: linear-gradient(135deg, #3b82f6, #1d4ed8); }
.stat-icon.connectors { background: linear-gradient(135deg, #10b981, #059669); }
.stat-icon.roles { background: linear-gradient(135deg, #f59e0b, #d97706); }
.stat-icon.sso { background: linear-gradient(135deg, #8b5cf6, #7c3aed); }

.stat-content {
  display: flex;
  flex-direction: column;
}

.stat-number {
  font-size: 28px;
  font-weight: 700;
  color: var(--color-text-primary);
  line-height: 1;
}

.stat-name {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin-top: 4px;
}

.stat-footer {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.stat-tag {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  background: var(--color-fill-secondary);
  color: var(--color-text-secondary);
}

.stat-tag.success { background: var(--color-success-bg); color: var(--color-success); }
.stat-tag.warning { background: var(--color-warning-bg); color: var(--color-warning); }
.stat-tag.primary { background: var(--color-primary-bg); color: var(--color-primary); }
.stat-tag.info { background: var(--color-info-bg); color: var(--color-info); }

/* ============================================
   底部双列
   ============================================ */
.bottom-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  min-height: 0;
}

.services-panel,
.shortcuts-panel {
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  padding: 16px;
  display: flex;
  flex-direction: column;
  box-shadow: var(--card-shadow);
  overflow: hidden;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  flex-shrink: 0;
}

.panel-header h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--color-text-primary);
}

.panel-header h3 .el-icon {
  color: var(--color-primary);
}

/* 服务状态网格 */
.services-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
  flex: 1;
  overflow: auto;
}

.service-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  border-radius: var(--radius-md);
  background: var(--color-fill-secondary);
  position: relative;
  transition: all var(--transition-fast);
}

.service-item:hover {
  background: var(--color-fill-quaternary);
}

.service-item.online {
  background: var(--color-success-bg);
}

.service-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-text-quaternary);
  flex-shrink: 0;
}

.service-item.online .service-indicator {
  background: var(--color-success);
  box-shadow: 0 0 6px var(--color-success);
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.service-icon {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  color: white;
  flex-shrink: 0;
}

.service-icon.db { background: #3b82f6; }
.service-icon.ldap { background: #10b981; }
.service-icon.vpn { background: #f59e0b; }
.service-icon.api { background: #8b5cf6; }

.service-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.service-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.service-desc {
  font-size: 11px;
  color: var(--color-text-tertiary);
}

.service-status {
  font-size: 11px;
  color: var(--color-text-secondary);
  flex-shrink: 0;
}

.service-item.online .service-status {
  color: var(--color-success);
  font-weight: 500;
}

/* 快捷入口网格 */
.shortcuts-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
  flex: 1;
  overflow: auto;
}

.shortcut-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 16px 8px;
  border-radius: var(--radius-md);
  background: var(--color-fill-secondary);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.shortcut-item:hover {
  background: var(--color-primary-bg);
  transform: translateY(-2px);
}

.shortcut-item:hover .el-icon {
  color: var(--color-primary);
}

.shortcut-item .el-icon {
  font-size: 22px;
  color: var(--color-text-secondary);
  transition: color var(--transition-fast);
}

.shortcut-item span {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-secondary);
  transition: color var(--transition-fast);
}

.shortcut-item:hover span {
  color: var(--color-primary);
}

/* 实时连接状态指示器 */
.refresh-indicator {
  position: absolute;
  bottom: 8px;
  right: 8px;
  font-size: 11px;
  color: var(--color-text-quaternary);
  background: var(--color-fill-secondary);
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  transition: all 0.3s ease;
}

.refresh-indicator.connected {
  color: var(--color-success);
  background: var(--color-success-bg);
}

.refresh-indicator.connected span::before {
  animation: pulse-dot 2s infinite;
}

@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

/* ============================================
   响应式适配
   ============================================ */
@media (max-width: 1400px) {
  .metrics-row,
  .stats-row {
    grid-template-columns: repeat(2, 1fr);
  }
  .shortcuts-grid {
    grid-template-columns: repeat(4, 1fr);
  }
}

@media (max-width: 1024px) {
  .dashboard {
    height: auto;
    min-height: calc(100vh - 112px);
  }
  .bottom-row {
    grid-template-columns: 1fr;
  }
  .services-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .dashboard-header {
    flex-direction: column;
    gap: 16px;
    text-align: center;
  }
  .header-stats {
    flex-wrap: wrap;
    justify-content: center;
  }
  .metrics-row,
  .stats-row {
    grid-template-columns: 1fr;
  }
  .shortcuts-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .services-grid {
    grid-template-columns: 1fr;
  }
}
</style>
