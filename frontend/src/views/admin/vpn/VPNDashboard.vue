<template>
  <div class="vpn-dashboard">
    <el-row :gutter="20" class="mb-4">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" :class="serviceStatus.running ? 'success' : 'danger'">
              <el-icon :size="32"><Connection /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-label">服务状态</div>
              <div class="stat-value">
                <el-tag :type="serviceStatus.running ? 'success' : 'danger'" size="large">
                  {{ serviceStatus.running ? '运行中' : '已停止' }}
                </el-tag>
              </div>
            </div>
          </div>
          <div class="stat-actions" v-if="!serviceStatus.running">
            <el-button type="primary" size="small" @click="startService" :loading="actionLoading">
              启动服务
            </el-button>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon primary">
              <el-icon :size="32"><User /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-label">在线用户</div>
              <div class="stat-value">{{ dashboard.online_count || 0 }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon warning">
              <el-icon :size="32"><Link /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-label">今日连接</div>
              <div class="stat-value">{{ dashboard.today_connect || 0 }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon info">
              <el-icon :size="32"><DataLine /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-label">总连接数</div>
              <div class="stat-value">{{ dashboard.total_connect || 0 }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20">
      <el-col :span="12">
        <el-card shadow="hover" class="info-card">
          <template #header>
            <span>服务信息</span>
          </template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="监听地址">
              {{ serviceStatus.listen_addr || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="DTLS">
              <el-tag :type="serviceStatus.dtls_enabled ? 'success' : 'info'" size="small">
                {{ serviceStatus.dtls_enabled ? '已启用' : '未启用' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="链路模式">
              {{ serviceStatus.link_mode || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="IP 网段">
              {{ serviceStatus.ipv4_cidr || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="启动时间">
              {{ serviceStatus.started_at ? formatTime(serviceStatus.started_at) : '-' }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover" class="info-card">
          <template #header>
            <span>资源统计</span>
          </template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="用户组数">
              {{ dashboard.group_count || 0 }}
            </el-descriptions-item>
            <el-descriptions-item label="VPN 用户数">
              {{ dashboard.vpn_user_count || 0 }}
            </el-descriptions-item>
            <el-descriptions-item label="上传带宽">
              {{ formatBandwidth(dashboard.bandwidth?.upload || 0) }}
            </el-descriptions-item>
            <el-descriptions-item label="下载带宽">
              {{ formatBandwidth(dashboard.bandwidth?.download || 0) }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="hover" class="mt-4">
      <template #header>
        <div class="flex justify-between items-center">
          <span>用户组在线统计</span>
          <el-button type="primary" link @click="loadData">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </template>
      <el-table :data="groupStatsData" stripe>
        <el-table-column prop="name" label="用户组" />
        <el-table-column prop="count" label="在线人数" align="center" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { vpnApi } from '@/api'
import { ElMessage } from 'element-plus'
import { Connection, User, Link, DataLine, Refresh } from '@element-plus/icons-vue'

const loading = ref(false)
const actionLoading = ref(false)
const serviceStatus = ref<any>({})
const dashboard = ref<any>({})
let refreshTimer: any = null

const groupStatsData = computed(() => {
  const stats = dashboard.value.group_stats || {}
  return Object.keys(stats).map(name => ({
    name,
    count: stats[name]
  }))
})

const loadData = async () => {
  loading.value = true
  try {
    const [statusRes, dashRes] = await Promise.all([
      vpnApi.serviceStatus(),
      vpnApi.dashboard()
    ])
    serviceStatus.value = statusRes.data?.data || {}
    dashboard.value = dashRes.data?.data || {}
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const startService = async () => {
  actionLoading.value = true
  try {
    const res = await vpnApi.serviceStart()
    if (res.data?.success) {
      ElMessage.success('VPN 服务已启动')
      await loadData()
    } else {
      ElMessage.error(res.data?.message || '启动失败')
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '启动失败')
  } finally {
    actionLoading.value = false
  }
}

const stopService = async () => {
  actionLoading.value = true
  try {
    const res = await vpnApi.serviceStop()
    if (res.data?.success) {
      ElMessage.success('VPN 服务已停止')
      await loadData()
    } else {
      ElMessage.error(res.data?.message || '停止失败')
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '停止失败')
  } finally {
    actionLoading.value = false
  }
}

const restartService = async () => {
  actionLoading.value = true
  try {
    const res = await vpnApi.serviceRestart()
    if (res.data?.success) {
      ElMessage.success('VPN 服务已重启')
      await loadData()
    } else {
      ElMessage.error(res.data?.message || '重启失败')
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '重启失败')
  } finally {
    actionLoading.value = false
  }
}

const formatTime = (time: string) => {
  if (!time) return '-'
  return new Date(time).toLocaleString()
}

const formatBandwidth = (bytes: number) => {
  if (bytes < 1024) return bytes + ' B/s'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB/s'
  return (bytes / 1024 / 1024).toFixed(2) + ' MB/s'
}

onMounted(() => {
  loadData()
  refreshTimer = setInterval(loadData, 10000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<style scoped>
/* ============================================
   Corporate VPN Dashboard - 企业风格
   ============================================ */
.vpn-dashboard {
  padding: 20px;
  
}


.stat-card {
  height: 160px;
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.stat-icon.success { background: var(--color-success); }
.stat-icon.danger { background: var(--color-error); }
.stat-icon.primary { background: var(--color-primary); }
.stat-icon.warning { background: var(--color-warning); }
.stat-icon.info { background: var(--color-info); }

.stat-info {
  flex: 1;
}

.stat-label {
  font-size: var(--font-size-sm);
  color: var(--color-text-tertiary);
  margin-bottom: 8px;
}

.stat-value {
  font-size: var(--font-size-2xl);
  font-weight: 600;
  color: var(--color-text-primary);
}

.stat-actions {
  display: flex;
  gap: 8px;
}

.info-card {
  height: 100%;
}

.mb-4 {
  margin-bottom: 16px;
}

.mt-4 {
  margin-top: 16px;
}

.flex {
  display: flex;
}

.justify-between {
  justify-content: space-between;
}

.items-center {
  align-items: center;
}
</style>
