<template>
  <div class="admin-home">
    <!-- 欢迎区域 -->
    <div class="welcome-section">
      <div class="welcome-text">
        <h1>欢迎回来，{{ userStore.user?.nickname || userStore.user?.username }}</h1>
        <p>{{ greeting }}，祝您工作愉快！</p>
      </div>
      <div class="quick-stats">
        <div class="stat-item">
          <span class="stat-value">{{ systemStatus.runtime?.uptime || '-' }}</span>
          <span class="stat-label">运行时长</span>
        </div>
        <div class="stat-item">
          <span class="stat-value">{{ formatTime(new Date()) }}</span>
          <span class="stat-label">当前时间</span>
        </div>
      </div>
    </div>

    <!-- 系统状态卡片 -->
    <div class="status-cards">
      <div class="status-card" :class="getCpuClass(systemStatus.cpu?.usage || 0)">
        <div class="card-header">
          <el-icon class="card-icon"><Cpu /></el-icon>
          <span class="card-title">CPU使用率</span>
        </div>
        <div class="card-body">
          <div class="progress-ring">
            <svg viewBox="0 0 100 100">
              <circle class="bg" cx="50" cy="50" r="42" />
              <circle 
                class="progress" 
                cx="50" cy="50" r="42" 
                :stroke-dasharray="`${(systemStatus.cpu?.usage || 0) * 2.64} 264`"
              />
            </svg>
            <span class="ring-value">{{ (systemStatus.cpu?.usage || 0).toFixed(1) }}%</span>
          </div>
          <div class="card-detail">
            <span>核心数: {{ systemStatus.cpu?.cores || 0 }}</span>
          </div>
        </div>
      </div>

      <div class="status-card" :class="getMemoryClass(systemStatus.memory?.percent || 0)">
        <div class="card-header">
          <el-icon class="card-icon"><Coin /></el-icon>
          <span class="card-title">内存使用</span>
        </div>
        <div class="card-body">
          <div class="progress-ring">
            <svg viewBox="0 0 100 100">
              <circle class="bg" cx="50" cy="50" r="42" />
              <circle 
                class="progress" 
                cx="50" cy="50" r="42" 
                :stroke-dasharray="`${(systemStatus.memory?.percent || 0) * 2.64} 264`"
              />
            </svg>
            <span class="ring-value">{{ (systemStatus.memory?.percent || 0).toFixed(1) }}%</span>
          </div>
          <div class="card-detail">
            <span>{{ (systemStatus.memory?.used || 0).toFixed(2) }}GB / {{ (systemStatus.memory?.total || 0).toFixed(2) }}GB</span>
          </div>
        </div>
      </div>

      <div class="status-card" :class="getDiskClass(systemStatus.disk?.percent || 0)">
        <div class="card-header">
          <el-icon class="card-icon"><FolderOpened /></el-icon>
          <span class="card-title">磁盘使用</span>
        </div>
        <div class="card-body">
          <div class="progress-ring">
            <svg viewBox="0 0 100 100">
              <circle class="bg" cx="50" cy="50" r="42" />
              <circle 
                class="progress" 
                cx="50" cy="50" r="42" 
                :stroke-dasharray="`${(systemStatus.disk?.percent || 0) * 2.64} 264`"
              />
            </svg>
            <span class="ring-value">{{ (systemStatus.disk?.percent || 0).toFixed(1) }}%</span>
          </div>
          <div class="card-detail">
            <span>{{ (systemStatus.disk?.used || 0).toFixed(1) }}GB / {{ (systemStatus.disk?.total || 0).toFixed(1) }}GB</span>
          </div>
        </div>
      </div>

      <div class="status-card success">
        <div class="card-header">
          <el-icon class="card-icon"><Connection /></el-icon>
          <span class="card-title">网络流量</span>
        </div>
        <div class="card-body">
          <div class="network-stats">
            <div class="net-item">
              <el-icon class="up"><Top /></el-icon>
              <span class="net-value">{{ (systemStatus.network?.outRate || 0).toFixed(2) }} MB/s</span>
            </div>
            <div class="net-item">
              <el-icon class="down"><Bottom /></el-icon>
              <span class="net-value">{{ (systemStatus.network?.inRate || 0).toFixed(2) }} MB/s</span>
            </div>
          </div>
          <div class="card-detail">
            <span>实时网络速率</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 服务状态 & 快捷操作 -->
    <div class="section-row">
      <div class="section-card services-card">
        <div class="section-header">
          <h3><el-icon><SetUp /></el-icon> 服务状态</h3>
          <span class="header-desc">服务状态显示LDAP服务和VPN服务等内置服务的状态</span>
          <el-button type="primary" link @click="loadSystemStatus">
            <el-icon><Refresh /></el-icon> 刷新
          </el-button>
        </div>
        <div class="services-list">
          <div class="service-item" :class="{ online: systemStatus.database?.status === 'ok' }">
            <div class="service-icon">
              <el-icon><DataLine /></el-icon>
            </div>
            <div class="service-info">
              <span class="service-name">数据库服务</span>
              <span class="service-status">{{ systemStatus.database?.type || 'PostgreSQL' }} {{ systemStatus.database?.status === 'ok' ? '运行正常' : '连接异常' }}</span>
            </div>
            <el-tag :type="systemStatus.database?.status === 'ok' ? 'success' : 'danger'" size="small">
              {{ systemStatus.database?.status === 'ok' ? '在线' : '离线' }}
            </el-tag>
          </div>
          <div class="service-item" :class="{ online: systemStatus.ldap?.running }">
            <div class="service-icon ldap">
              <el-icon><Connection /></el-icon>
            </div>
            <div class="service-info">
              <span class="service-name">LDAP 服务</span>
              <span class="service-status">{{ systemStatus.ldap?.running ? '端口 ' + (systemStatus.ldap?.port || 389) : '服务未启动' }}</span>
            </div>
            <el-tag :type="systemStatus.ldap?.running ? 'success' : 'info'" size="small">
              {{ systemStatus.ldap?.running ? '在线' : '已停止' }}
            </el-tag>
          </div>
          <div class="service-item" :class="{ online: systemStatus.vpn?.running }">
            <div class="service-icon vpn">
              <el-icon><Link /></el-icon>
            </div>
            <div class="service-info">
              <span class="service-name">VPN 服务</span>
              <span class="service-status">{{ systemStatus.vpn?.running ? '端口 ' + (systemStatus.vpn?.port || 443) : '服务未启动' }}</span>
            </div>
            <el-tag :type="systemStatus.vpn?.running ? 'success' : 'info'" size="small">
              {{ systemStatus.vpn?.running ? '在线' : '已停止' }}
            </el-tag>
          </div>
        </div>
      </div>

      <div class="section-card quick-actions-card">
        <div class="section-header">
          <h3><el-icon><Grid /></el-icon> 快捷操作</h3>
        </div>
        <div class="quick-actions">
          <div class="action-item" @click="router.push('/admin/users/local')">
            <el-icon><User /></el-icon>
            <span>本地用户</span>
          </div>
          <div class="action-item" @click="router.push('/admin/roles')">
            <el-icon><Stamp /></el-icon>
            <span>角色管理</span>
          </div>
          <div class="action-item" @click="router.push('/admin/vpn')">
            <el-icon><Link /></el-icon>
            <span>VPN 接入</span>
          </div>
          <div class="action-item" @click="router.push('/admin/settings/ldap')">
            <el-icon><Connection /></el-icon>
            <span>LDAP 服务</span>
          </div>
          <div class="action-item" @click="router.push('/admin/settings')">
            <el-icon><Setting /></el-icon>
            <span>系统设置</span>
          </div>
          <div class="action-item" @click="router.push('/admin/security')">
            <el-icon><Lock /></el-icon>
            <span>安全中心</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { 
  Cpu, Coin, FolderOpened, Connection, Top, Bottom, SetUp, Refresh,
  DataLine, Lock, Grid, User, Setting, Monitor, Stamp, Link
} from "@element-plus/icons-vue";
import { useUserStore } from "../../store/user";
import { systemApi } from "../../api";

const router = useRouter();
const userStore = useUserStore();

const CACHE_KEY = 'admin_home_system_status';
const CACHE_EXPIRY = 5 * 60 * 1000; // 5分钟缓存

const systemStatus = ref<any>({
  cpu: { usage: 0, cores: 0 },
  memory: { used: 0, total: 0, percent: 0 },
  disk: { used: 0, total: 0, percent: 0 },
  network: { inRate: 0, outRate: 0 },
  database: { status: 'ok', type: 'sqlite' },
  runtime: { uptime: '-', goVersion: '', goroutines: 0, startTime: '' },
  ldap: { running: false, port: 389 },
  vpn: { running: false, port: 443 }
});

let refreshTimer: number | null = null;

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

const formatTime = (date: Date) => {
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
};

const getCpuClass = (value: number) => {
  if (value >= 90) return 'danger';
  if (value >= 70) return 'warning';
  return 'success';
};

const getMemoryClass = (value: number) => {
  if (value >= 90) return 'danger';
  if (value >= 70) return 'warning';
  return 'success';
};

const getDiskClass = (value: number) => {
  if (value >= 90) return 'danger';
  if (value >= 80) return 'warning';
  return 'success';
};

const loadFromCache = () => {
  try {
    const cached = localStorage.getItem(CACHE_KEY);
    if (cached) {
      const { data, timestamp } = JSON.parse(cached);
      if (Date.now() - timestamp < CACHE_EXPIRY) {
        systemStatus.value = data;
        return true;
      }
    }
  } catch (e) {
    // ignore
  }
  return false;
};

const saveToCache = (data: any) => {
  try {
    localStorage.setItem(CACHE_KEY, JSON.stringify({
      data,
      timestamp: Date.now()
    }));
  } catch (e) {
    // ignore
  }
};

const loadSystemStatus = async () => {
  try {
    const res = await systemApi.status();
    if (res.data.success) {
      systemStatus.value = res.data.data;
      saveToCache(res.data.data);
    }
  } catch (e) {
    // ignore
  }
};

onMounted(() => {
  // 先从缓存加载，避免每次打开都等待
  const hasCache = loadFromCache();
  
  // 异步加载最新数据
  loadSystemStatus();
  
  // 定时刷新（30秒）
  refreshTimer = window.setInterval(loadSystemStatus, 30000);
});

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer);
});
</script>

<style scoped>
/* ============================================
   Corporate Admin Home - 企业风格
   ============================================ */
.admin-home {
  max-width: 1400px;
  margin: 0 auto;
  
}


/* ============================================
   欢迎区域 - 企业蓝渐变
   ============================================ */
.welcome-section {
  background: var(--gradient-primary);
  border-radius: var(--radius-xl);
  padding: 28px 36px;
  color: white;
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-xl);
  box-shadow: var(--shadow-primary);
}

.welcome-text {
  position: relative;
  z-index: 1;
}

.welcome-text h1 {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 6px 0;
}

.welcome-text p {
  margin: 0;
  opacity: 0.9;
  font-size: 14px;
}

.quick-stats {
  display: flex;
  gap: 24px;
}

.stat-item {
  text-align: center;
  padding: 12px 20px;
  background: rgba(255, 255, 255, 0.15);
  border-radius: var(--radius-lg);
  min-width: 100px;
}

.stat-value {
  display: block;
  font-size: 22px;
  font-weight: 700;
}

.stat-label {
  font-size: 12px;
  opacity: 0.85;
  margin-top: 4px;
}

/* ============================================
   状态卡片 - 企业风格
   ============================================ */
.status-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: var(--spacing-xl);
}

.status-card {
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  padding: 20px;
  box-shadow: var(--card-shadow);
  transition: box-shadow var(--transition-fast);
}

.status-card:hover {
  box-shadow: var(--card-shadow-hover);
}

.card-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.card-icon {
  font-size: 20px;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
  background: var(--color-primary-bg);
  color: var(--color-primary);
}

.status-card.warning .card-icon {
  background: var(--color-warning-bg);
  color: var(--color-warning);
}

.status-card.danger .card-icon {
  background: var(--color-error-bg);
  color: var(--color-error);
}

.status-card.success .card-icon {
  background: var(--color-success-bg);
  color: var(--color-success);
}

.card-title {
  font-size: 13px;
  color: var(--color-text-tertiary);
  font-weight: 500;
}

.card-body {
  display: flex;
  flex-direction: column;
  align-items: center;
}

/* ============================================
   进度环 - 企业风格
   ============================================ */
.progress-ring {
  width: 100px;
  height: 100px;
  position: relative;
}

.progress-ring svg {
  transform: rotate(-90deg);
}

.progress-ring circle {
  fill: none;
  stroke-width: 8;
  stroke-linecap: round;
}

.progress-ring .bg {
  stroke: var(--color-fill-secondary);
}

.progress-ring .progress {
  stroke: var(--color-primary);
  transition: stroke-dasharray 0.4s linear;
}

.status-card.success .progress-ring .progress {
  stroke: var(--color-success);
}

.status-card.warning .progress-ring .progress {
  stroke: var(--color-warning);
}

.status-card.danger .progress-ring .progress {
  stroke: var(--color-error);
}

.ring-value {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: 18px;
  font-weight: 700;
  color: var(--color-text-primary);
}

.card-detail {
  margin-top: 12px;
  font-size: 12px;
  color: var(--color-text-tertiary);
  background: var(--color-fill-secondary);
  padding: 4px 10px;
  border-radius: var(--radius-sm);
}

/* ============================================
   网络卡片
   ============================================ */
.network-stats {
  display: flex;
  gap: 20px;
  margin: 16px 0;
}

.net-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--color-fill-secondary);
  border-radius: var(--radius-md);
}

.net-item .up {
  color: var(--color-success);
  font-size: 16px;
}

.net-item .down {
  color: var(--color-primary);
  font-size: 16px;
}

.net-value {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
}

/* ============================================
   服务状态 & 快捷操作
   ============================================ */
.section-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
  margin-bottom: var(--spacing-xl);
}

.section-card {
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  padding: 20px;
  box-shadow: var(--card-shadow);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 10px;
}

.section-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--color-text-primary);
}

.section-header h3 .el-icon {
  color: var(--color-primary);
}

.header-desc {
  flex: 1;
  font-size: 12px;
  color: var(--color-text-quaternary);
  margin-left: 8px;
}

/* ============================================
   服务列表
   ============================================ */
.services-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.service-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--color-fill-secondary);
  border: 1px solid transparent;
  transition: background-color var(--transition-fast);
}

.service-item:hover {
  background: var(--color-fill-quaternary);
}

.service-item.online {
  background: var(--color-success-bg);
  border-color: var(--color-success-border);
}

.service-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  background: var(--color-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  color: white;
}

.service-item.online .service-icon {
  background: var(--color-success);
}

.service-icon.ldap {
  background: var(--color-info);
}

.service-icon.vpn {
  background: var(--color-warning);
}

.service-item.online .service-icon.ldap,
.service-item.online .service-icon.vpn {
  background: var(--color-success);
}

.service-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.service-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.service-status {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

/* ============================================
   快捷操作
   ============================================ */
.quick-actions {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.action-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 16px 12px;
  border-radius: var(--radius-md);
  background: var(--color-fill-secondary);
  border: 1px solid transparent;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.action-item:hover {
  background: var(--color-primary-bg);
  border-color: var(--color-primary-border);
}

.action-item:hover .el-icon {
  color: var(--color-primary);
}

.action-item .el-icon {
  font-size: 24px;
  color: var(--color-text-secondary);
  transition: color var(--transition-fast);
}

.action-item span {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-secondary);
  transition: color var(--transition-fast);
}

.action-item:hover span {
  color: var(--color-primary);
}

/* ============================================
   响应式
   ============================================ */
@media (max-width: 1200px) {
  .status-cards {
    grid-template-columns: repeat(2, 1fr);
  }
  .section-row {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .welcome-section {
    flex-direction: column;
    gap: 20px;
    text-align: center;
    padding: 20px;
  }
  
  .quick-stats {
    gap: 12px;
  }
  
  .stat-item {
    padding: 8px 14px;
  }
  
  .status-cards {
    grid-template-columns: 1fr;
  }
  
  .quick-actions {
    grid-template-columns: repeat(2, 1fr);
  }
  
  .section-header {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .header-desc {
    margin-left: 0;
    margin-top: 4px;
  }
}
</style>
