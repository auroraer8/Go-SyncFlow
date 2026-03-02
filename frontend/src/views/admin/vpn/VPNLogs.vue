<template>
  <div class="vpn-logs">
    <el-card shadow="hover">
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <span class="card-title">VPN 日志</span>
            <el-radio-group v-model="logType" @change="loadLogs" size="small">
              <el-radio-button label="activity">用户活动</el-radio-button>
              <el-radio-button label="audit">访问审计</el-radio-button>
            </el-radio-group>
          </div>
          <div class="header-actions">
            <el-button size="small" @click="exportLogs" :loading="exporting">
              <el-icon><Download /></el-icon> 导出Excel
            </el-button>
          </div>
        </div>
      </template>

      <!-- 用户活动日志 -->
      <el-table v-if="logType === 'activity'" :data="logs" v-loading="loading" stripe border size="small" style="width: 100%">
        <el-table-column label="时间" width="160">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>时间</span>
              <el-popover placement="bottom" :width="260" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filter.dateRange?.length }"><Filter /></el-icon>
                </template>
                <el-date-picker v-model="filter.dateRange" type="daterange" range-separator="至"
                  start-placeholder="开始" end-placeholder="结束" value-format="YYYY-MM-DD"
                  size="small" clearable @change="loadLogs" style="width: 100%" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="用户名" width="100">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>用户名</span>
              <el-popover placement="bottom" :width="150" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filter.username }"><Filter /></el-icon>
                </template>
                <el-input v-model="filter.username" placeholder="输入用户名" clearable size="small" @keyup.enter="loadLogs" @clear="loadLogs" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">{{ row.username }}</template>
        </el-table-column>
        <el-table-column prop="group_name" label="用户组" width="90" />
        <el-table-column prop="ip_addr" label="VPN IP" width="120" />
        <el-table-column prop="remote_addr" label="远端地址" min-width="140" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="70" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '登录' : '登出' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="auth_type" label="认证源" width="100" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.auth_type" :type="authTypeTag(row.auth_type)" size="small">
              {{ authTypeLabel(row.auth_type) }}
            </el-tag>
            <span v-else style="color: #c0c4cc;">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="device_type" label="设备" width="90" show-overflow-tooltip />
        <el-table-column prop="version" label="版本" width="100" show-overflow-tooltip />
        <el-table-column prop="info" label="信息" min-width="160" show-overflow-tooltip />
      </el-table>

      <!-- 访问审计日志 -->
      <el-table v-else :data="logs" v-loading="loading" stripe border size="small" style="width: 100%">
        <el-table-column label="时间" width="160">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>时间</span>
              <el-popover placement="bottom" :width="260" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filter.dateRange?.length }"><Filter /></el-icon>
                </template>
                <el-date-picker v-model="filter.dateRange" type="daterange" range-separator="至"
                  start-placeholder="开始" end-placeholder="结束" value-format="YYYY-MM-DD"
                  size="small" clearable @change="loadLogs" style="width: 100%" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="用户名" width="100">
          <template #header>
            <div class="th-filter" @click.stop>
              <span>用户名</span>
              <el-popover placement="bottom" :width="150" trigger="click">
                <template #reference>
                  <el-icon class="filter-icon" :class="{ active: filter.username }"><Filter /></el-icon>
                </template>
                <el-input v-model="filter.username" placeholder="输入用户名" clearable size="small" @keyup.enter="loadLogs" @clear="loadLogs" />
              </el-popover>
            </div>
          </template>
          <template #default="{ row }">{{ row.username }}</template>
        </el-table-column>
        <el-table-column prop="src" label="源地址" width="120" />
        <el-table-column prop="src_port" label="源端口" width="70" align="center" />
        <el-table-column prop="dst" label="目标地址" width="120" />
        <el-table-column prop="dst_port" label="目标端口" width="70" align="center" />
        <el-table-column prop="dst_host" label="目标主机" min-width="130" show-overflow-tooltip />
        <el-table-column prop="protocol" label="协议" width="60" align="center">
          <template #default="{ row }">
            {{ row.protocol === 6 ? 'TCP' : row.protocol === 17 ? 'UDP' : row.protocol }}
          </template>
        </el-table-column>
        <el-table-column prop="info" label="信息" min-width="100" show-overflow-tooltip />
      </el-table>

      <div class="pagination-wrapper">
        <span class="total-text">共 {{ pagination.total }} 条</span>
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
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
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { vpnApi } from '@/api'
import { Filter, Download } from '@element-plus/icons-vue'
import * as XLSX from 'xlsx'

const loading = ref(false)
const exporting = ref(false)
const logType = ref('activity')
const logs = ref<any[]>([])
const filter = ref({
  username: '',
  dateRange: [] as string[]
})
const pagination = ref({
  page: 1,
  pageSize: 20,
  total: 0
})

const loadLogs = async () => {
  loading.value = true
  try {
    const params: any = {
      page: pagination.value.page,
      page_size: pagination.value.pageSize
    }
    if (filter.value.username) {
      params.username = filter.value.username
    }
    if (filter.value.dateRange?.length === 2) {
      params.start_time = filter.value.dateRange[0] + ' 00:00:00'
      params.end_time = filter.value.dateRange[1] + ' 23:59:59'
    }

    let res
    if (logType.value === 'activity') {
      res = await vpnApi.userActLogs(params)
    } else {
      res = await vpnApi.auditLogs(params)
    }

    logs.value = res.data?.data?.list || []
    pagination.value.total = res.data?.data?.total || 0
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const authTypeLabel = (type_: string) => {
  if (type_.includes(':')) {
    const [connType, connName] = type_.split(':', 2)
    const typeLabels: Record<string, string> = {
      ldap: 'LDAP', ldap_ad: 'AD', database: '数据库',
      radius: 'RADIUS', http_api: 'HTTP API',
    }
    return `${typeLabels[connType] || connType} (${connName})`
  }
  const map: Record<string, string> = {
    local: '本地认证',
    syncflow: '系统用户',
    ldap_connector: 'LDAP',
    connector: '连接器',
    ldap: 'LDAP',
    radius: 'RADIUS',
    api: 'API',
  }
  return map[type_] || type_
}

const authTypeTag = (type_: string) => {
  const base = type_.includes(':') ? type_.split(':')[0] : type_
  const map: Record<string, string> = {
    local: 'info',
    syncflow: 'primary',
    ldap_connector: 'warning',
    ldap: 'warning',
    ldap_ad: 'warning',
    connector: 'warning',
    radius: 'success',
    api: 'danger',
    database: '',
    http_api: 'danger',
  }
  return map[base] || 'info'
}

const formatTime = (time: string) => {
  if (!time) return '-'
  return time.replace('T', ' ').slice(0, 19)
}

const exportLogs = async () => {
  exporting.value = true
  try {
    const allLogs: any[] = []
    let page = 1
    const size = 500
    while (true) {
      const params: any = { page, page_size: size }
      if (filter.value.username) params.username = filter.value.username
      if (filter.value.dateRange?.length === 2) {
        params.start_time = filter.value.dateRange[0] + ' 00:00:00'
        params.end_time = filter.value.dateRange[1] + ' 23:59:59'
      }
      let res
      if (logType.value === 'activity') {
        res = await vpnApi.userActLogs(params)
      } else {
        res = await vpnApi.auditLogs(params)
      }
      const list = res.data?.data?.list || []
      allLogs.push(...list)
      if (list.length < size) break
      page++
    }
    if (!allLogs.length) {
      ElMessage.warning('没有可导出的数据')
      return
    }
    downloadExcel(allLogs)
    ElMessage.success(`成功导出 ${allLogs.length} 条记录`)
  } catch (e) {
    ElMessage.error('导出失败')
  } finally {
    exporting.value = false
  }
}

const downloadExcel = (data: any[]) => {
  let rows: any[]
  
  if (logType.value === 'activity') {
    rows = data.map(r => ({
      '时间': formatTime(r.created_at),
      '用户名': r.username || '',
      '用户组': r.group_name || '',
      'VPN IP': r.ip_addr || '',
      '远端地址': r.remote_addr || '',
      '状态': r.status === 1 ? '登录' : '登出',
      '认证源': r.auth_type ? authTypeLabel(r.auth_type) : '',
      '设备类型': r.device_type || '',
      '版本': r.version || '',
      '信息': r.info || ''
    }))
  } else {
    rows = data.map(r => ({
      '时间': formatTime(r.created_at),
      '用户名': r.username || '',
      '源地址': r.src || '',
      '源端口': r.src_port || '',
      '目标地址': r.dst || '',
      '目标端口': r.dst_port || '',
      '目标主机': r.dst_host || '',
      '协议': r.protocol === 6 ? 'TCP' : r.protocol === 17 ? 'UDP' : String(r.protocol || ''),
      '信息': r.info || ''
    }))
  }
  
  const ws = XLSX.utils.json_to_sheet(rows)
  const wb = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(wb, ws, 'Sheet1')
  XLSX.writeFile(wb, `VPN${logType.value === 'activity' ? '用户活动' : '访问审计'}日志_${new Date().toISOString().slice(0, 10)}.xlsx`)
}

onMounted(() => {
  loadLogs()
})
</script>

<style scoped>
.vpn-logs { display: flex; flex-direction: column; gap: 16px; }

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.header-left { display: flex; align-items: center; gap: 16px; }
.card-title { font-size: 16px; font-weight: 600; color: var(--color-text-primary); }
.header-actions { display: flex; gap: 8px; }

.pagination-wrapper {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 16px;
}
.total-text { font-size: 14px; color: var(--color-text-tertiary); }

.th-filter { display: inline-flex; align-items: center; gap: 4px; white-space: nowrap; }
.filter-icon { font-size: 12px; color: var(--color-text-quaternary); cursor: pointer; }
.filter-icon:hover { color: var(--color-primary); }
.filter-icon.active { color: var(--color-primary); }
</style>
