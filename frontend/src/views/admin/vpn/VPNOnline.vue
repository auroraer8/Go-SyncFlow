<template>
  <div class="vpn-online">
    <el-card shadow="hover">
      <template #header>
        <div class="card-header">
          <span>在线用户</span>
          <div class="header-actions">
            <el-select v-model="searchCate" placeholder="搜索类型" style="width: 120px" clearable>
              <el-option label="用户名" value="username" />
              <el-option label="用户组" value="group" />
              <el-option label="MAC地址" value="mac_addr" />
              <el-option label="IP地址" value="ip" />
              <el-option label="远端地址" value="remote_addr" />
            </el-select>
            <el-input v-model="searchText" placeholder="搜索关键字" style="width: 200px" clearable @keyup.enter="loadOnline" />
            <el-checkbox v-model="showSleeper">显示休眠用户</el-checkbox>
            <el-button type="primary" @click="loadOnline">
              <el-icon><Search /></el-icon>
              搜索
            </el-button>
            <el-button @click="loadOnline">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="onlineUsers" v-loading="loading" stripe>
        <el-table-column prop="username" label="用户名" min-width="100" />
        <el-table-column prop="group" label="用户组" min-width="80" />
        <el-table-column prop="ip" label="VPN IP" min-width="120">
          <template #default="{ row }">
            {{ row.ip || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="remote_addr" label="远端地址" min-width="150" show-overflow-tooltip />
        <el-table-column prop="transport_protocol" label="协议" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.transport_protocol === 'UDP' ? 'success' : 'primary'" size="small">
              {{ row.transport_protocol }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="tun_name" label="接口" width="100" />
        <el-table-column prop="client" label="客户端" min-width="100" show-overflow-tooltip />
        <el-table-column label="上传/下载" width="160" align="center">
          <template #default="{ row }">
            <div>↑ {{ row.bandwidth_up }}</div>
            <div>↓ {{ row.bandwidth_down }}</div>
          </template>
        </el-table-column>
        <el-table-column label="累计流量" width="160" align="center">
          <template #default="{ row }">
            <div>↑ {{ row.bandwidth_up_all }}</div>
            <div>↓ {{ row.bandwidth_down_all }}</div>
          </template>
        </el-table-column>
        <el-table-column prop="last_login" label="登录时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.last_login) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button type="danger" link size="small" @click="kickUser(row)">踢下线</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="stats-footer">
        <el-statistic title="在线用户" :value="onlineUsers.length" />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { vpnApi } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh } from '@element-plus/icons-vue'

const loading = ref(false)
const onlineUsers = ref<any[]>([])
const searchCate = ref('')
const searchText = ref('')
const showSleeper = ref(false)
let refreshTimer: any = null

const loadOnline = async () => {
  loading.value = true
  try {
    const res = await vpnApi.listOnline({
      search_cate: searchCate.value,
      search_text: searchText.value,
      show_sleeper: showSleeper.value
    })
    onlineUsers.value = res.data?.data?.list || []
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const kickUser = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确定要将用户「${row.username}」踢下线吗？`, '确认', {
      type: 'warning'
    })
    await vpnApi.kickUser(row.token)
    ElMessage.success('已踢下线')
    await loadOnline()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.message || '操作失败')
    }
  }
}

const formatTime = (time: string) => {
  if (!time) return '-'
  return new Date(time).toLocaleString()
}

onMounted(() => {
  loadOnline()
  refreshTimer = setInterval(loadOnline, 10000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<style scoped>
/* ============================================
   Corporate VPN Online - 企业风格
   ============================================ */
.vpn-online {
  padding: 20px;
  
}


.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.stats-footer {
  display: flex;
  justify-content: flex-end;
  padding: 16px 0 0;
  border-top: 1px solid var(--color-border);
  margin-top: 16px;
}
</style>
