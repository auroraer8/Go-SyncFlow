<template>
  <div class="vpn-settings">
    <el-card shadow="hover" v-loading="loading">
      <template #header>
        <div class="card-header">
          <span>VPN 服务配置</span>
          <div class="header-actions">
            <el-button 
              v-if="!serviceRunning" 
              type="success" 
              @click="startService" 
              :loading="serviceLoading"
            >启动服务</el-button>
            <el-button 
              v-else 
              type="danger" 
              @click="stopService" 
              :loading="serviceLoading"
            >关闭服务</el-button>
            <el-button type="primary" @click="saveSettings" :loading="saving">保存配置</el-button>
          </div>
        </div>
      </template>

      <el-form :model="form" label-width="140px" style="max-width: 800px">
        <el-divider content-position="left">基础配置</el-divider>
        
        <el-form-item label="启用 VPN 服务">
          <el-switch v-model="form.enabled" />
        </el-form-item>

        <el-form-item label="监听地址">
          <el-input v-model="form.server_addr" placeholder=":443" />
        </el-form-item>

        <el-form-item label="启用 DTLS">
          <el-switch v-model="form.server_dtls" />
          <span class="form-tip">启用 DTLS 可提升 UDP 传输性能</span>
        </el-form-item>

        <el-form-item label="DTLS 地址" v-if="form.server_dtls">
          <el-input v-model="form.server_dtls_addr" placeholder=":443" />
        </el-form-item>

        <el-divider content-position="left">网络配置</el-divider>

        <el-form-item label="链路模式">
          <el-radio-group v-model="form.link_mode">
            <el-radio label="tun">TUN</el-radio>
            <el-radio label="tap">TAP</el-radio>
            <el-radio label="macvtap">MacVTAP</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item label="主网卡">
          <el-select v-model="form.ipv4_master" placeholder="请选择网卡" style="width: 300px">
            <el-option 
              v-for="iface in networkInterfaces" 
              :key="iface.name" 
              :value="iface.name"
              :label="`${iface.name} (${iface.ip_addrs.join(', ')})`"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="IP 网段">
          <el-input v-model="form.ipv4_cidr" placeholder="192.168.90.0/24" />
        </el-form-item>

        <el-form-item label="网关地址">
          <el-input v-model="form.ipv4_gateway" placeholder="192.168.90.1" />
        </el-form-item>

        <el-form-item label="IP 范围">
          <el-input v-model="form.ipv4_start" placeholder="起始 IP" style="width: 180px" />
          <span class="mx-2">-</span>
          <el-input v-model="form.ipv4_end" placeholder="结束 IP" style="width: 180px" />
        </el-form-item>

        <el-divider content-position="left">连接参数</el-divider>

        <el-form-item label="最大客户端数">
          <el-input-number v-model="form.max_client" :min="1" :max="10000" />
        </el-form-item>

        <el-form-item label="单用户最大连接">
          <el-input-number v-model="form.max_user_client" :min="1" :max="100" />
        </el-form-item>

        <el-form-item label="MTU">
          <el-input-number v-model="form.mtu" :min="1200" :max="1500" />
        </el-form-item>

        <el-form-item label="会话超时(秒)">
          <el-input-number v-model="form.session_timeout" :min="0" :step="60" />
          <span class="form-tip">0 表示不超时</span>
        </el-form-item>

        <el-form-item label="空闲超时(秒)">
          <el-input-number v-model="form.idle_timeout" :min="0" :step="60" />
          <span class="form-tip">0 表示不超时</span>
        </el-form-item>

        <el-divider content-position="left">安全配置</el-divider>

        <el-form-item label="启用 NAT">
          <el-switch v-model="form.iptables_nat" />
        </el-form-item>

        <el-form-item label="防暴力破解">
          <el-switch v-model="form.anti_brute_force" />
        </el-form-item>

        <el-form-item label="最大错误分数" v-if="form.anti_brute_force">
          <el-input-number v-model="form.max_ban_score" :min="1" />
        </el-form-item>

        <el-form-item label="锁定时间(秒)" v-if="form.anti_brute_force">
          <el-input-number v-model="form.lock_time" :min="60" :step="60" />
        </el-form-item>

        <el-divider content-position="left">显示配置</el-divider>

        <el-form-item label="发行者名称">
          <el-input v-model="form.issuer" placeholder="Go-SyncFlow VPN" />
        </el-form-item>

        <el-form-item label="登录横幅">
          <el-input v-model="form.banner" type="textarea" :rows="3" placeholder="连接后显示的提示信息" />
        </el-form-item>

        <el-form-item label="默认用户组">
          <el-select v-model="form.default_group" placeholder="请选择用户组" style="width: 300px">
            <el-option 
              v-for="group in groupOptions" 
              :key="group.id" 
              :value="group.name"
              :label="group.name"
            />
          </el-select>
        </el-form-item>

        <el-divider content-position="left">登录页面配置</el-divider>

        <el-form-item label="页面标题">
          <el-input v-model="form.login_page_title" placeholder="输入VPN登录页面的标题（可选）" />
        </el-form-item>

        <el-form-item label="自定义HTML">
          <el-input 
            v-model="form.login_page_html" 
            type="textarea" 
            :rows="6" 
            placeholder="输入自定义HTML内容（可选），支持HTML标签"
          />
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { vpnApi } from '@/api'
import { ElMessage } from 'element-plus'

const loading = ref(false)
const saving = ref(false)
const serviceRunning = ref(false)
const serviceLoading = ref(false)
const networkInterfaces = ref<any[]>([])
const groupOptions = ref<any[]>([])

const form = ref<any>({
  enabled: false,
  server_addr: '443',
  server_dtls: false,
  server_dtls_addr: '443',
  link_mode: 'tun',
  ipv4_master: 'eth0',
  ipv4_cidr: '192.168.90.0/24',
  ipv4_gateway: '192.168.90.1',
  ipv4_start: '192.168.90.100',
  ipv4_end: '192.168.90.200',
  max_client: 200,
  max_user_client: 3,
  mtu: 1460,
  session_timeout: 3600,
  idle_timeout: 0,
  iptables_nat: true,
  anti_brute_force: true,
  max_ban_score: 5,
  lock_time: 300,
  issuer: 'Go-SyncFlow VPN',
  banner: '',
  default_group: 'all',
  login_page_title: '',
  login_page_html: ''
})

const loadNetworkInterfaces = async () => {
  try {
    const res = await vpnApi.getNetworkInterfaces()
    if (res.data?.success) {
      networkInterfaces.value = res.data.data || []
      // 如果当前未设置网卡或网卡不存在，自动选择默认网卡
      const defaultIface = res.data.default_interface
      if (defaultIface) {
        const currentMaster = form.value.ipv4_master
        const exists = networkInterfaces.value.some((i: any) => i.name === currentMaster)
        if (!currentMaster || currentMaster === 'eth0' && !exists) {
          form.value.ipv4_master = defaultIface
        }
      }
    }
  } catch (e) {
    console.error('加载网卡列表失败', e)
  }
}

const loadGroupOptions = async () => {
  try {
    const res = await vpnApi.getGroupOptions()
    if (res.data?.success) {
      groupOptions.value = res.data.data || []
    }
  } catch (e) {
    console.error('加载用户组列表失败', e)
  }
}

const loadSettings = async () => {
  loading.value = true
  try {
    const res = await vpnApi.getSettings()
    if (res.data?.data) {
      const data = res.data.data
      // 去除地址中的前导冒号
      if (data.server_addr?.startsWith(':')) {
        data.server_addr = data.server_addr.slice(1)
      }
      if (data.server_dtls_addr?.startsWith(':')) {
        data.server_dtls_addr = data.server_dtls_addr.slice(1)
      }
      form.value = { ...form.value, ...data }
    }
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  saving.value = true
  try {
    // 保存时添加回冒号前缀
    const payload = { ...form.value }
    if (payload.server_addr && !payload.server_addr.startsWith(':')) {
      payload.server_addr = ':' + payload.server_addr
    }
    if (payload.server_dtls_addr && !payload.server_dtls_addr.startsWith(':')) {
      payload.server_dtls_addr = ':' + payload.server_dtls_addr
    }
    const res = await vpnApi.updateSettings(payload)
    if (res.data?.success) {
      ElMessage.success('配置已保存')
    } else {
      ElMessage.error(res.data?.message || '保存失败')
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// 加载服务状态
const loadServiceStatus = async () => {
  try {
    const res = await vpnApi.serviceStatus()
    if (res.data?.success) {
      serviceRunning.value = res.data.data?.running || false
    }
  } catch (e) {
    console.error('加载服务状态失败', e)
  }
}

// 启动服务
const startService = async () => {
  serviceLoading.value = true
  try {
    const res = await vpnApi.serviceStart()
    if (res.data?.success) {
      ElMessage.success('VPN 服务已启动')
      serviceRunning.value = true
    } else {
      ElMessage.error(res.data?.message || '启动失败')
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '启动失败')
  } finally {
    serviceLoading.value = false
  }
}

// 关闭服务
const stopService = async () => {
  serviceLoading.value = true
  try {
    const res = await vpnApi.serviceStop()
    if (res.data?.success) {
      ElMessage.success('VPN 服务已关闭')
      serviceRunning.value = false
    } else {
      ElMessage.error(res.data?.message || '关闭失败')
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '关闭失败')
  } finally {
    serviceLoading.value = false
  }
}

onMounted(() => {
  loadSettings()
  loadNetworkInterfaces()
  loadGroupOptions()
  loadServiceStatus()
})
</script>

<style scoped>
/* ============================================
   Corporate VPN Settings - 企业风格
   ============================================ */
.vpn-settings {
  padding: 20px;
  
}


.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.form-tip {
  margin-left: 12px;
  color: var(--color-text-tertiary);
  font-size: var(--font-size-xs);
}

.mx-2 {
  margin: 0 8px;
}
</style>
