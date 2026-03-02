<template>
  <div class="vpn-address-groups">
    <el-card shadow="hover">
      <template #header>
        <div class="card-header">
          <span>地址组管理</span>
          <el-button type="primary" @click="showCreateDialog">
            <el-icon><Plus /></el-icon>
            新建地址组
          </el-button>
        </div>
      </template>

      <el-table :data="addressGroups" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="名称" min-width="150" />
        <el-table-column prop="note" label="备注" min-width="180" show-overflow-tooltip />
        <el-table-column label="地址数量" width="100" align="center">
          <template #default="{ row }">
            <el-tag size="small">{{ row.addresses?.length || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="地址列表" min-width="250">
          <template #default="{ row }">
            <div class="addr-preview">
              <el-tag
                v-for="(addr, i) in (row.addresses || []).slice(0, 5)"
                :key="i"
                size="small"
                :type="isDomainAddr(addr.val) ? 'warning' : 'info'"
                class="addr-tag"
              >
                {{ addr.val }}
              </el-tag>
              <el-tag v-if="(row.addresses || []).length > 5" size="small" type="warning">
                +{{ row.addresses.length - 5 }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="editItem(row)">编辑</el-button>
            <el-button type="danger" link size="small" @click="deleteItem(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑地址组' : '新建地址组'"
      width="650px"
      top="5vh"
      :align-center="false"
      class="addr-group-dialog"
    >
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如：办公网络、生产环境" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.note" type="textarea" placeholder="地址组用途说明" />
        </el-form-item>
        <el-form-item label="地址列表" required>
          <div class="addr-input-section">
            <div class="addr-input-mode">
              <el-radio-group v-model="inputMode" size="small">
                <el-radio-button label="single">逐条输入</el-radio-button>
                <el-radio-button label="batch">批量输入</el-radio-button>
              </el-radio-group>
            </div>

            <template v-if="inputMode === 'single'">
              <div v-for="(addr, index) in form.addresses" :key="index" class="addr-item">
                <el-input v-model="addr.val" placeholder="CIDR 或域名，如 10.0.0.0/8 或 *.example.com" style="width: 220px" />
                <el-input v-model="addr.note" placeholder="备注" style="width: 160px; margin-left: 8px" />
                <el-button type="danger" link @click="removeAddr(index)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
              <el-button type="primary" link @click="addAddr">
                <el-icon><Plus /></el-icon>
                添加地址
              </el-button>
            </template>

            <template v-else>
              <el-input
                v-model="batchText"
                type="textarea"
                :rows="10"
                placeholder="每行一个 CIDR 或域名，支持备注（用空格或制表符分隔），例如：
10.0.0.0/8 办公网络
172.16.0.0/12 测试环境
*.example.com 域名路由（仅 PC 端生效）
app.internal.com"
              />
              <div class="batch-actions">
                <el-button type="primary" size="small" @click="parseBatch">解析</el-button>
                <el-button size="small" @click="exportBatch">导出为文本</el-button>
                <span class="batch-hint">解析后将替换当前地址列表</span>
              </div>
            </template>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveItem" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { vpnApi } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Delete } from '@element-plus/icons-vue'

interface AddrItem {
  val: string
  note: string
}

const isDomainAddr = (val: string) => {
  if (!val) return false
  return val.includes('.') && !val.includes('/') && !/^\d+\.\d+\.\d+\.\d+$/.test(val)
}

const loading = ref(false)
const saving = ref(false)
const addressGroups = ref<any[]>([])
const dialogVisible = ref(false)
const isEdit = ref(false)
const inputMode = ref<'single' | 'batch'>('single')
const batchText = ref('')

const form = ref<any>({
  name: '',
  note: '',
  addresses: [] as AddrItem[]
})

const loadList = async () => {
  loading.value = true
  try {
    const res = await vpnApi.listAddressGroups()
    addressGroups.value = res.data?.data?.list || []
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const showCreateDialog = () => {
  isEdit.value = false
  inputMode.value = 'single'
  batchText.value = ''
  form.value = {
    name: '',
    note: '',
    addresses: [{ val: '', note: '' }]
  }
  dialogVisible.value = true
}

const editItem = (row: any) => {
  isEdit.value = true
  inputMode.value = 'single'
  form.value = {
    ...row,
    addresses: (row.addresses || []).map((a: any) => ({ ...a }))
  }
  batchText.value = form.value.addresses.map((a: AddrItem) => {
    return a.note ? `${a.val} ${a.note}` : a.val
  }).join('\n')
  dialogVisible.value = true
}

const saveItem = async () => {
  if (!form.value.name) {
    ElMessage.warning('请输入地址组名称')
    return
  }
  // 批量模式下自动解析文本到地址列表
  if (inputMode.value === 'batch' && batchText.value.trim()) {
    const lines = batchText.value.split('\n').filter((l: string) => l.trim())
    const parsed: AddrItem[] = []
    for (const line of lines) {
      const parts = line.trim().split(/[\s\t]+/, 2)
      parsed.push({ val: parts[0] || '', note: parts[1] || '' })
    }
    form.value.addresses = parsed
  }
  const validAddrs = form.value.addresses.filter((a: AddrItem) => a.val.trim())
  if (validAddrs.length === 0) {
    ElMessage.warning('至少需要一个地址')
    return
  }
  form.value.addresses = validAddrs

  saving.value = true
  try {
    if (isEdit.value) {
      await vpnApi.updateAddressGroup(form.value.id, form.value)
      ElMessage.success('更新成功')
    } else {
      await vpnApi.createAddressGroup(form.value)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await loadList()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '操作失败')
  } finally {
    saving.value = false
  }
}

const deleteItem = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确定删除地址组「${row.name}」吗？`, '删除确认', { type: 'warning' })
    await vpnApi.deleteAddressGroup(row.id)
    ElMessage.success('删除成功')
    await loadList()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.message || '删除失败')
    }
  }
}

const addAddr = () => {
  form.value.addresses.push({ val: '', note: '' })
}

const removeAddr = (index: number) => {
  form.value.addresses.splice(index, 1)
}

const parseBatch = () => {
  const lines = batchText.value.split('\n').filter((l: string) => l.trim())
  const addresses: AddrItem[] = []
  for (const line of lines) {
    const parts = line.trim().split(/[\s\t]+/, 2)
    addresses.push({
      val: parts[0] || '',
      note: parts[1] || ''
    })
  }
  if (addresses.length === 0) {
    ElMessage.warning('未解析到有效地址')
    return
  }
  form.value.addresses = addresses
  inputMode.value = 'single'
  ElMessage.success(`已解析 ${addresses.length} 条地址`)
}

const exportBatch = () => {
  batchText.value = form.value.addresses
    .filter((a: AddrItem) => a.val.trim())
    .map((a: AddrItem) => a.note ? `${a.val} ${a.note}` : a.val)
    .join('\n')
  inputMode.value = 'batch'
}

onMounted(() => {
  loadList()
})
</script>

<style scoped>
.vpn-address-groups {
  padding: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.addr-preview {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.addr-tag {
  font-family: 'Cascadia Code', 'Fira Code', monospace;
  font-size: 12px;
}

.addr-input-section {
  width: 100%;
}

.addr-input-mode {
  margin-bottom: 12px;
}

.addr-item {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
  padding: 8px 12px;
  background: var(--color-fill-secondary, #f5f7fa);
  border-radius: 6px;
  border: 1px solid var(--color-border, #e4e7ed);
}

.addr-item:hover {
  background: var(--color-fill-quaternary, #f0f2f5);
}

.batch-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}

.batch-hint {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}

.addr-group-dialog :deep(.el-overlay-dialog) {
  display: block !important;
  overflow: auto;
}

.addr-group-dialog :deep(.el-dialog) {
  margin: 5vh auto !important;
  position: relative !important;
  top: 0 !important;
  transform: none !important;
}

.addr-group-dialog :deep(.el-dialog__body) {
  max-height: 65vh;
  overflow-y: auto;
  padding-top: 10px;
  padding-bottom: 10px;
}
</style>
