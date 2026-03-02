<template>
  <div class="vpn-groups">
    <el-card shadow="hover">
      <template #header>
        <div class="card-header">
          <span>VPN 用户组</span>
          <el-button type="primary" @click="showCreateDialog">
            <el-icon><Plus /></el-icon>
            新建用户组
          </el-button>
        </div>
      </template>

      <el-table :data="groups" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="组名" min-width="120" />
        <el-table-column prop="note" label="备注" min-width="150" show-overflow-tooltip />
        <el-table-column label="认证方式" width="120" align="center">
          <template #default="{ row }">
            <el-tag :type="row.auth?.type === 'syncflow' ? 'success' : 'info'" size="small">
              {{ authTypeLabel(row.auth?.type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="使用范围" min-width="150">
          <template #default="{ row }">
            <span v-if="!row.allowed_user_ids?.length && !row.allowed_group_ids?.length" class="text-gray">
              全部用户
            </span>
            <span v-else class="text-info">
              {{ row.allowed_user_ids?.length || 0 }} 人，{{ row.allowed_group_ids?.length || 0 }} 部门
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="allow_lan" label="允许局域网" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="row.allow_lan ? 'success' : 'info'" size="small">
              {{ row.allow_lan ? '是' : '否' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
              {{ row.status === 1 ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="editGroup(row)">编辑</el-button>
            <el-button type="danger" link size="small" @click="deleteGroup(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新建/编辑对话框 -->
    <el-dialog 
      v-model="dialogVisible" 
      :title="isEdit ? '编辑用户组' : '新建用户组'" 
      width="700px" 
      top="5vh"
      :align-center="false"
      class="vpn-group-dialog"
    >
      <el-form :model="form" label-width="100px">
        <el-form-item label="组名" required>
          <el-input v-model="form.name" placeholder="请输入组名" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.note" type="textarea" placeholder="请输入备注" />
        </el-form-item>
        <el-form-item label="允许局域网">
          <el-switch v-model="form.allow_lan" />
        </el-form-item>
        <el-form-item label="带宽限制">
          <el-input-number v-model="form.bandwidth" :min="0" :step="100" />
          <span class="ml-2">Kbps（0 表示不限）</span>
        </el-form-item>
        <el-form-item label="客户端 DNS">
          <div v-for="(dns, index) in form.client_dns" :key="index" class="dns-item">
            <el-input v-model="dns.val" placeholder="DNS 地址" style="width: 200px" />
            <el-input v-model="dns.note" placeholder="备注" style="width: 150px; margin-left: 8px" />
            <el-button type="danger" link @click="removeDns(index)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
          <el-button type="primary" link @click="addDns">
            <el-icon><Plus /></el-icon>
            添加 DNS
          </el-button>
        </el-form-item>
        <el-form-item label="路由包含">
          <el-select
            v-model="routeIncludeSelection"
            multiple
            placeholder="选择 all 或地址组"
            style="width: 100%"
          >
            <el-option label="all（全部流量）" value="all" />
            <el-option
              v-for="ag in addressGroupOptions"
              :key="'ag-' + ag.id"
              :label="ag.name"
              :value="'ag:' + ag.id"
            />
          </el-select>
          <div class="route-select-tip">选择 all 表示所有流量走 VPN；选择地址组则只有对应网段走 VPN</div>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :label="1">启用</el-radio>
            <el-radio :label="0">禁用</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-divider content-position="left">认证与使用范围</el-divider>

        <el-form-item label="认证方式">
          <el-select v-model="authType" style="width: 200px">
            <el-option label="系统用户" value="syncflow" />
            <el-option label="上游连接器" value="connector" />
          </el-select>
        </el-form-item>

        <el-form-item label="认证连接器" v-if="authType === 'connector'">
          <el-select v-model="authConnectorId" placeholder="选择上游认证连接器" style="width: 100%">
            <el-option-group v-for="(group, cat) in connectorGroups" :key="cat" :label="group.label">
              <el-option
                v-for="c in group.items"
                :key="c.id"
                :label="c.name"
                :value="c.id"
              >
                <span>{{ c.name }}</span>
                <span class="connector-type-tag">{{ c.typeName }}</span>
              </el-option>
            </el-option-group>
          </el-select>
        </el-form-item>

        <el-form-item label="密码学习" v-if="authType === 'connector'">
          <el-switch v-model="learnPassword" />
          <span class="tip-text">认证成功后将密码同步到本地用户（如果存在匹配的本地用户）</span>
        </el-form-item>

        <el-form-item label="短信双因子" v-if="authType === 'syncflow' || authType === 'connector'">
          <el-switch v-model="smsEnabled" />
          <span class="tip-text">启用后，用户登录 VPN 需要短信验证码二次认证</span>
        </el-form-item>

        <el-form-item label="允许的用户" v-if="authType === 'syncflow'">
          <el-select
            v-model="form.allowed_user_ids"
            multiple
            filterable
            remote
            reserve-keyword
            placeholder="搜索并选择用户"
            :remote-method="searchUsers"
            :loading="loadingUsers"
            style="width: 100%"
          >
            <el-option
              v-for="user in userOptions"
              :key="user.id"
              :label="`${user.nickname || user.username} (${user.username})`"
              :value="user.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="允许的部门" v-if="authType === 'syncflow'">
          <el-tree-select
            ref="deptTreeRef"
            v-model="form.allowed_group_ids"
            :data="departmentTreeData"
            multiple
            show-checkbox
            node-key="id"
            :check-strictly="false"
            :render-after-expand="false"
            :props="{ label: 'label', children: 'children', value: 'id' }"
            placeholder="选择部门"
            style="width: 100%"
            :popper-options="{ modifiers: [{ name: 'flip', enabled: false }, { name: 'preventOverflow', options: { boundary: 'viewport' } }] }"
          />
          <div class="scope-tip">以上两项为"或"关系：用户在允许列表中，或属于允许的部门，均可使用。都不选则允许所有用户。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveGroup" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { vpnApi } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Delete } from '@element-plus/icons-vue'

interface ConnectorItem {
  id: number;
  name: string;
  type: string;
  typeName: string;
  category: string;
}

const loading = ref(false)
const saving = ref(false)
const groups = ref<any[]>([])
const dialogVisible = ref(false)
const isEdit = ref(false)

// 认证相关
const authType = ref('syncflow')
const smsEnabled = ref(false)
const authConnectorId = ref<number | null>(null)
const learnPassword = ref(false)
const authConnectors = ref<ConnectorItem[]>([])

// 兼容旧版
const ldapConnectors = ref<any[]>([])

// 使用范围选项
const userOptions = ref<any[]>([])
const departmentTreeData = ref<any[]>([])
const loadingUsers = ref(false)
const deptTreeRef = ref<any>(null)

// 地址组选项
const addressGroupOptions = ref<any[]>([])

// 路由包含的统一选择值（'all' 或 'ag:123' 格式）
const routeIncludeSelection = ref<string[]>([])

// 监听 routeIncludeSelection 变化，同步到 form
watch(routeIncludeSelection, (val) => {
  const hasAll = val.includes('all')
  const agIds = val.filter(v => v.startsWith('ag:')).map(v => parseInt(v.replace('ag:', '')))

  if (hasAll) {
    form.value.route_include = [{ val: 'all', note: '' }]
  } else {
    form.value.route_include = []
  }
  form.value.route_include_group_ids = agIds
})


const form = ref<any>({
  name: '',
  note: '',
  allow_lan: true,
  bandwidth: 0,
  client_dns: [],
  route_include: [],
  route_exclude: [],
  route_include_group_ids: [],
  route_exclude_group_ids: [],
  status: 1,
  allowed_user_ids: [],
  allowed_group_ids: [],
  auth: { type: 'syncflow' }
})

// 认证方式标签
const authTypeLabel = (type: string) => {
  const labels: Record<string, string> = {
    local: 'VPN 本地',
    syncflow: '系统用户',
    connector: '连接器认证',
    ldap_connector: 'LDAP 连接器',
    ldap: 'LDAP',
    radius: 'Radius'
  }
  return labels[type] || type || '系统用户'
}

// 连接器分组
const connectorGroups = computed(() => {
  const groups: Record<string, { label: string; items: ConnectorItem[] }> = {}
  const categoryLabels: Record<string, string> = {
    ldap: 'LDAP 目录服务',
    database: '数据库',
    radius: 'RADIUS',
    http: 'HTTP API'
  }

  for (const c of authConnectors.value) {
    const cat = c.category || 'other'
    if (!groups[cat]) {
      groups[cat] = { label: categoryLabels[cat] || '其他', items: [] }
    }
    groups[cat].items.push(c)
  }
  return groups
})

// 加载所有可认证的上游连接器
const loadAuthConnectors = async () => {
  try {
    const res = await vpnApi.getAuthConnectors()
    authConnectors.value = res.data?.data || []
  } catch (e) {
    console.error(e)
  }
}

// 兼容旧版：加载 LDAP 连接器
const loadLdapConnectors = async () => {
  try {
    const res = await vpnApi.getLdapConnectors()
    ldapConnectors.value = res.data?.data || []
  } catch (e) {
    console.error(e)
  }
}

// 监听认证方式变化
watch(authType, (val) => {
  form.value.auth = { type: val }
  if (val === 'syncflow' || val === 'connector') {
    form.value.auth.sms_enabled = smsEnabled.value
  }
  if (val === 'connector' && authConnectorId.value) {
    form.value.auth.connector_id = authConnectorId.value
    form.value.auth.learn_password = learnPassword.value
  }
})

watch(smsEnabled, (val) => {
  if (authType.value === 'syncflow' || authType.value === 'connector') {
    form.value.auth.sms_enabled = val
  }
})

watch(authConnectorId, (val) => {
  if (authType.value === 'connector' && val) {
    form.value.auth.connector_id = val
  }
})

watch(learnPassword, (val) => {
  if (authType.value === 'connector') {
    form.value.auth.learn_password = val
  }
})

const loadGroups = async () => {
  loading.value = true
  try {
    const res = await vpnApi.listGroups()
    groups.value = res.data?.data?.list || []
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

// 加载地址组选项
const loadAddressGroups = async () => {
  try {
    const res = await vpnApi.addressGroupNames()
    addressGroupOptions.value = res.data?.data || []
  } catch (e) {
    console.error(e)
  }
}

// 加载部门列表（树形结构）
const loadDepartments = async () => {
  try {
    const res = await vpnApi.getScopeDepartments()
    departmentTreeData.value = res.data?.data || []
  } catch (e) {
    console.error(e)
  }
}

// 搜索用户
const searchUsers = async (keyword: string) => {
  if (!keyword) {
    userOptions.value = []
    return
  }
  loadingUsers.value = true
  try {
    const res = await vpnApi.getScopeUsers(keyword)
    userOptions.value = res.data?.data || []
  } catch (e) {
    console.error(e)
  } finally {
    loadingUsers.value = false
  }
}

// 加载已选中的用户信息
const loadSelectedUsers = async (userIds: number[]) => {
  if (!userIds || userIds.length === 0) return
  try {
    const res = await vpnApi.getScopeUsers('')
    const allUsers = res.data?.data || []
    // 合并已选中的用户到选项中
    userIds.forEach((id: number) => {
      const exists = userOptions.value.find((u: any) => u.id === id)
      if (!exists) {
        const user = allUsers.find((u: any) => u.id === id)
        if (user) {
          userOptions.value.push(user)
        }
      }
    })
  } catch (e) {
    console.error(e)
  }
}

const showCreateDialog = () => {
  isEdit.value = false
  authType.value = 'syncflow'
  smsEnabled.value = false
  authConnectorId.value = null
  learnPassword.value = false
  form.value = {
    name: '',
    note: '',
    allow_lan: true,
    bandwidth: 0,
    client_dns: [{ val: '114.114.114.114', note: '' }],
    route_include: [{ val: 'all', note: '' }],
    route_exclude: [],
    route_include_group_ids: [],
    route_exclude_group_ids: [],
    status: 1,
    allowed_user_ids: [],
    allowed_group_ids: [],
    auth: { type: 'syncflow' }
  }
  routeIncludeSelection.value = ['all']
  dialogVisible.value = true
}

const editGroup = async (row: any) => {
  isEdit.value = true
  // 兼容旧版 ldap_connector 类型
  const rawAuthType = row.auth?.type || 'syncflow'
  authType.value = rawAuthType === 'ldap_connector' ? 'connector' : rawAuthType
  smsEnabled.value = row.auth?.sms_enabled || false
  authConnectorId.value = row.auth?.connector_id || null
  learnPassword.value = row.auth?.learn_password || false
  form.value = {
    ...row,
    client_dns: row.client_dns || [],
    route_include: row.route_include || [],
    route_exclude: row.route_exclude || [],
    route_include_group_ids: row.route_include_group_ids || [],
    route_exclude_group_ids: row.route_exclude_group_ids || [],
    allowed_user_ids: row.allowed_user_ids || [],
    allowed_group_ids: row.allowed_group_ids || [],
    auth: row.auth || { type: 'syncflow' }
  }
  // 还原路由包含选择状态
  const sel: string[] = []
  const ri = form.value.route_include || []
  if (ri.some((r: any) => r.val === 'all')) {
    sel.push('all')
  }
  for (const agId of (form.value.route_include_group_ids || [])) {
    sel.push('ag:' + agId)
  }
  routeIncludeSelection.value = sel
  // 加载已选中的用户信息
  await loadSelectedUsers(form.value.allowed_user_ids)
  dialogVisible.value = true
}

const saveGroup = async () => {
  if (!form.value.name) {
    ElMessage.warning('请输入组名')
    return
  }
  
  // 更新认证配置
  form.value.auth = { type: authType.value }
  if (authType.value === 'syncflow' || authType.value === 'connector') {
    form.value.auth.sms_enabled = smsEnabled.value
  }
  if (authType.value === 'connector') {
    if (!authConnectorId.value) {
      ElMessage.warning('请选择上游认证连接器')
      return
    }
    form.value.auth.connector_id = authConnectorId.value
    form.value.auth.learn_password = learnPassword.value
  }
  
  saving.value = true
  try {
    if (isEdit.value) {
      await vpnApi.updateGroup(form.value.id, form.value)
      ElMessage.success('更新成功')
    } else {
      await vpnApi.createGroup(form.value)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await loadGroups()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '操作失败')
  } finally {
    saving.value = false
  }
}

const deleteGroup = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确定要删除用户组「${row.name}」吗？`, '删除确认', {
      type: 'warning'
    })
    await vpnApi.deleteGroup(row.id)
    ElMessage.success('删除成功')
    await loadGroups()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.message || '删除失败')
    }
  }
}

const addDns = () => {
  form.value.client_dns.push({ val: '', note: '' })
}

const removeDns = (index: number) => {
  form.value.client_dns.splice(index, 1)
}

const addRouteInclude = () => {
  form.value.route_include.push({ val: '', note: '' })
}

const removeRouteInclude = (index: number) => {
  form.value.route_include.splice(index, 1)
}

const addRouteExclude = () => {
  form.value.route_exclude.push({ val: '', note: '' })
}

const removeRouteExclude = (index: number) => {
  form.value.route_exclude.splice(index, 1)
}

onMounted(() => {
  loadGroups()
  loadDepartments()
  loadLdapConnectors()
  loadAuthConnectors()
  loadAddressGroups()
})
</script>

<style scoped>
/* ============================================
   Corporate VPN Groups Page - 企业风格
   ============================================ */
.vpn-groups {
  padding: 0;
  
}


.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.dns-item,
.route-item {
  display: flex;
  align-items: center;
  margin-bottom: 10px;
  padding: 8px 12px;
  background: var(--color-fill-secondary);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
  transition: background-color var(--transition-fast);
}

.dns-item:hover,
.route-item:hover {
  background: var(--color-fill-quaternary);
}

.ml-2 {
  margin-left: 10px;
}

.tip-text {
  margin-left: 14px;
  color: var(--color-text-tertiary);
  font-size: 12px;
}

.scope-selector {
  width: 100%;
}

.scope-section {
  margin-bottom: 4px;
}

.scope-section-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-regular);
  margin-bottom: 8px;
}

.scope-tip {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
  line-height: 1.6;
  margin-top: 4px;
}

.scope-selector :deep(.el-divider) {
  margin: 12px 0;
}

.text-gray {
  color: var(--color-text-tertiary);
}

.text-info {
  color: var(--color-primary);
}

.connector-type-tag {
  color: var(--color-text-tertiary);
  font-size: 11px;
  margin-left: 8px;
  padding: 2px 6px;
  background: var(--color-fill-secondary);
  border-radius: 4px;
}

:deep(.el-divider__text) {
  font-weight: 600;
  color: var(--color-text-primary);
  background: var(--card-bg);
  padding: 4px 12px;
  border-radius: var(--radius-sm);
}

/* 部门树包装器 */
.dept-tree-wrapper {
  max-height: 250px;
  overflow-y: auto;
  border: 1px solid var(--el-border-color);
  border-radius: var(--el-border-radius-base);
  padding: 8px;
  background: var(--el-fill-color-blank);
}

/* 固定对话框位置，避免内容变化时跳动 */
.vpn-group-dialog :deep(.el-overlay-dialog) {
  display: block !important;
  overflow: auto;
}

.vpn-group-dialog :deep(.el-dialog) {
  margin: 5vh auto !important;
  position: relative !important;
  top: 0 !important;
  transform: none !important;
}

.vpn-group-dialog :deep(.el-dialog__body) {
  max-height: 65vh;
  min-height: 400px;
  overflow-y: auto;
  padding-top: 10px;
  padding-bottom: 10px;
}

.route-select-tip {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
  margin-top: 4px;
  line-height: 1.6;
}
</style>

