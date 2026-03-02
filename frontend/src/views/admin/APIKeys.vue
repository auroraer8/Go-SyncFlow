<template>
  <div class="page-container">
    <!-- 页头 -->
    <div class="page-header">
      <div>
        <h2>API 密钥管理</h2>
        <p class="page-desc">管理开放接口的 AppID / AppKey，第三方系统可通过密钥调用 API</p>
      </div>
      <el-button type="primary" @click="showCreate">
        <el-icon><Plus /></el-icon>创建密钥
      </el-button>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-input v-model="searchKeyword" placeholder="搜索名称 / AppID" clearable style="width: 260px" @clear="loadData" @keyup.enter="loadData">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-select v-model="filterStatus" placeholder="状态" clearable style="width: 120px" @change="loadData">
        <el-option label="已启用" value="active" />
        <el-option label="已禁用" value="inactive" />
      </el-select>
      <el-button @click="loadData"><el-icon><Refresh /></el-icon></el-button>
    </div>

    <!-- 列表 -->
    <el-table :data="tableData" v-loading="loading" stripe class="modern-table" empty-text="暂无 API 密钥">
      <el-table-column label="名称" min-width="160">
        <template #default="{ row }">
          <div class="key-name">{{ row.name }}</div>
          <div class="key-desc" v-if="row.description">{{ row.description }}</div>
        </template>
      </el-table-column>
      <el-table-column prop="appId" label="AppID" width="200">
        <template #default="{ row }">
          <code class="app-id">{{ row.appId }}</code>
          <el-button link size="small" @click="copyText(row.appId)" style="margin-left: 4px">
            <el-icon><CopyDocument /></el-icon>
          </el-button>
        </template>
      </el-table-column>
      <el-table-column label="AppKey" width="140">
        <template #default="{ row }">
          <code class="app-key-hint">{{ row.appKeyHint }}</code>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag v-if="row.isExpired" type="info" size="small">已过期</el-tag>
          <el-tag v-else :type="row.isActive ? 'success' : 'danger'" size="small">
            {{ row.isActive ? '已启用' : '已禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="IP限制" width="100" align="center">
        <template #default="{ row }">
          <template v-if="(row.ipWhiteList && row.ipWhiteList.length) || (row.ipBlackList && row.ipBlackList.length)">
            <el-tooltip :content="formatIPInfo(row)" placement="top">
              <el-tag size="small" type="warning">已配置</el-tag>
            </el-tooltip>
          </template>
          <span v-else class="text-muted">无限制</span>
        </template>
      </el-table-column>
      <el-table-column label="调用次数" width="100" align="center">
        <template #default="{ row }">
          <span class="usage-count">{{ formatNumber(row.usageCount) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="最后使用" width="160">
        <template #default="{ row }">
          <template v-if="row.lastUsedAt">
            <div class="last-used">{{ formatTime(row.lastUsedAt) }}</div>
            <div class="last-ip" v-if="row.lastUsedIp">{{ row.lastUsedIp }}</div>
          </template>
          <span v-else class="text-muted">未使用</span>
        </template>
      </el-table-column>
      <el-table-column label="过期时间" width="120">
        <template #default="{ row }">
          <span v-if="row.expiresAt">{{ formatDate(row.expiresAt) }}</span>
          <span v-else class="text-muted">永不过期</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="showEdit(row)">编辑</el-button>
          <el-button link :type="row.isActive ? 'warning' : 'success'" size="small" @click="toggleStatus(row)">
            {{ row.isActive ? '禁用' : '启用' }}
          </el-button>
          <el-button link type="primary" size="small" @click="resetKey(row)">重置</el-button>
          <el-button link type="danger" size="small" @click="deleteKey(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 创建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑 API 密钥' : '创建 API 密钥'" width="640px" :close-on-click-modal="false">
      <el-form :model="form" label-width="100px" label-position="top">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如：ERP 系统集成" maxlength="128" show-word-limit />
        </el-form-item>

        <el-form-item label="备注">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="用途描述、负责人等信息" maxlength="512" />
        </el-form-item>

        <template v-if="!editingId">
          <el-divider content-position="left">密钥配置</el-divider>

          <el-form-item label="AppID">
            <el-input v-model="form.appId" placeholder="留空自动生成（格式：ak_xxxxxx）">
              <template #append>
                <el-button @click="form.appId = ''">自动生成</el-button>
              </template>
            </el-input>
            <div class="form-tip">唯一标识，创建后不可修改</div>
          </el-form-item>

          <el-form-item label="AppKey">
            <el-input v-model="form.appKey" placeholder="留空自动生成（64位随机字符串）" type="password" show-password>
              <template #append>
                <el-button @click="form.appKey = ''">自动生成</el-button>
              </template>
            </el-input>
            <div class="form-tip">密钥仅在创建时显示一次，请妥善保存</div>
          </el-form-item>
        </template>

        <el-divider content-position="left">访问控制</el-divider>

        <el-form-item label="IP 白名单">
          <div class="ip-list-editor">
            <el-tag
              v-for="(ip, i) in form.ipWhitelist"
              :key="'wl-'+i"
              closable
              @close="form.ipWhitelist.splice(i, 1)"
              style="margin: 2px 4px 2px 0"
            >{{ ip }}</el-tag>
            <el-input
              v-if="showWhitelistInput"
              v-model="whitelistInputVal"
              size="small"
              style="width: 180px"
              placeholder="输入 IP 后回车"
              @keyup.enter="addWhitelistIP"
              @blur="addWhitelistIP"
            />
            <el-button v-else size="small" @click="showWhitelistInput = true">+ 添加IP</el-button>
          </div>
          <div class="form-tip">配置后仅允许白名单内 IP 调用，支持 CIDR（如 10.0.0.0/8）和通配符（如 192.168.1.*）</div>
        </el-form-item>

        <el-form-item label="IP 黑名单">
          <div class="ip-list-editor">
            <el-tag
              v-for="(ip, i) in form.ipBlacklist"
              :key="'bl-'+i"
              closable
              type="danger"
              @close="form.ipBlacklist.splice(i, 1)"
              style="margin: 2px 4px 2px 0"
            >{{ ip }}</el-tag>
            <el-input
              v-if="showBlacklistInput"
              v-model="blacklistInputVal"
              size="small"
              style="width: 180px"
              placeholder="输入 IP 后回车"
              @keyup.enter="addBlacklistIP"
              @blur="addBlacklistIP"
            />
            <el-button v-else size="small" @click="showBlacklistInput = true">+ 添加IP</el-button>
          </div>
        </el-form-item>

        <el-form-item label="频率限制">
          <el-input-number v-model="form.rateLimit" :min="1" :max="10000" :step="10" />
          <span style="margin-left: 8px; color: #999">次/分钟</span>
        </el-form-item>

        <el-form-item label="过期时间">
          <el-date-picker v-model="form.expiresAt" type="date" placeholder="留空则永不过期" value-format="YYYY-MM-DD" clearable />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveKey">{{ editingId ? '保存' : '创建' }}</el-button>
      </template>
    </el-dialog>

    <!-- 密钥展示对话框（创建/重置后显示明文密钥） -->
    <el-dialog v-model="keyResultVisible" title="API 密钥信息" width="540px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px">
        请立即复制并安全保存以下密钥信息，关闭后将无法再次查看完整 AppKey。
      </el-alert>

      <div class="key-display">
        <div class="key-row">
          <span class="key-label">AppID</span>
          <code class="key-value">{{ keyResult.appId }}</code>
          <el-button size="small" @click="copyText(keyResult.appId)">复制</el-button>
        </div>
        <div class="key-row">
          <span class="key-label">AppKey</span>
          <code class="key-value key-secret">{{ keyResult.appKey }}</code>
          <el-button size="small" @click="copyText(keyResult.appKey)">复制</el-button>
        </div>
      </div>

      <el-divider />

      <div class="usage-example">
        <h4>调用示例</h4>
        <div class="code-block">
          <pre>curl -H "X-App-ID: {{ keyResult.appId }}" \
     -H "X-App-Key: {{ keyResult.appKey }}" \
     {{ baseURL }}/api/open/users</pre>
        </div>
        <el-button size="small" @click="copyUsageExample" style="margin-top: 8px">复制示例</el-button>
      </div>

      <template #footer>
        <el-button type="primary" @click="keyResultVisible = false">我已安全保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus, Search, Refresh, CopyDocument } from '@element-plus/icons-vue';
import { api } from '../../api';

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const keyResultVisible = ref(false);
const editingId = ref<number | null>(null);
const tableData = ref<any[]>([]);
const searchKeyword = ref('');
const filterStatus = ref('');

const showWhitelistInput = ref(false);
const showBlacklistInput = ref(false);
const whitelistInputVal = ref('');
const blacklistInputVal = ref('');

const form = reactive({
  name: '',
  description: '',
  appId: '',
  appKey: '',
  ipWhitelist: [] as string[],
  ipBlacklist: [] as string[],
  rateLimit: 60,
  expiresAt: '',
});

const keyResult = reactive({
  appId: '',
  appKey: '',
});

const baseURL = computed(() => window.location.origin);

const resetForm = () => {
  form.name = '';
  form.description = '';
  form.appId = '';
  form.appKey = '';
  form.ipWhitelist = [];
  form.ipBlacklist = [];
  form.rateLimit = 60;
  form.expiresAt = '';
  showWhitelistInput.value = false;
  showBlacklistInput.value = false;
  whitelistInputVal.value = '';
  blacklistInputVal.value = '';
};

const loadData = async () => {
  loading.value = true;
  try {
    const params: any = {};
    if (searchKeyword.value) params.keyword = searchKeyword.value;
    if (filterStatus.value) params.status = filterStatus.value;
    const { data } = await api.get('/apikeys', { params });
    tableData.value = data.data || [];
  } catch (e: any) {
    ElMessage.error('加载失败');
  } finally {
    loading.value = false;
  }
};

const showCreate = () => {
  editingId.value = null;
  resetForm();
  dialogVisible.value = true;
};

const showEdit = (row: any) => {
  editingId.value = row.id;
  form.name = row.name;
  form.description = row.description || '';
  form.ipWhitelist = row.ipWhiteList || [];
  form.ipBlacklist = row.ipBlackList || [];
  form.rateLimit = row.rateLimit || 60;
  form.expiresAt = row.expiresAt ? row.expiresAt.substring(0, 10) : '';
  dialogVisible.value = true;
};

const saveKey = async () => {
  if (!form.name.trim()) {
    ElMessage.warning('请输入名称');
    return;
  }
  saving.value = true;
  try {
    if (editingId.value) {
      await api.put(`/apikeys/${editingId.value}`, {
        name: form.name,
        description: form.description,
        ipWhitelist: form.ipWhitelist,
        ipBlacklist: form.ipBlacklist,
        rateLimit: form.rateLimit,
        expiresAt: form.expiresAt || null,
      });
      ElMessage.success('更新成功');
    } else {
      const { data } = await api.post('/apikeys', {
        name: form.name,
        description: form.description,
        appId: form.appId || undefined,
        appKey: form.appKey || undefined,
        ipWhitelist: form.ipWhitelist,
        ipBlacklist: form.ipBlacklist,
        rateLimit: form.rateLimit,
        expiresAt: form.expiresAt || undefined,
      });
      // 显示密钥信息
      keyResult.appId = data.data.appId;
      keyResult.appKey = data.data.appKey;
      keyResultVisible.value = true;
    }
    dialogVisible.value = false;
    loadData();
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '操作失败');
  } finally {
    saving.value = false;
  }
};

const toggleStatus = async (row: any) => {
  const action = row.isActive ? '禁用' : '启用';
  try {
    await ElMessageBox.confirm(`确定${action}密钥「${row.name}」？`, '提示', { type: 'warning' });
    await api.put(`/apikeys/${row.id}/toggle`);
    ElMessage.success(`${action}成功`);
    loadData();
  } catch {}
};

const resetKey = async (row: any) => {
  try {
    await ElMessageBox.confirm(
      `确定重置密钥「${row.name}」的 AppKey？重置后旧密钥将立即失效。`,
      '重置 AppKey',
      { type: 'warning', confirmButtonText: '确定重置', cancelButtonText: '取消' }
    );
    const { data } = await api.post(`/apikeys/${row.id}/reset`);
    keyResult.appId = data.data.appId;
    keyResult.appKey = data.data.appKey;
    keyResultVisible.value = true;
    loadData();
  } catch {}
};

const deleteKey = async (row: any) => {
  try {
    await ElMessageBox.confirm(
      `确定删除密钥「${row.name}」(${row.appId})？删除后无法恢复。`,
      '删除确认',
      { type: 'danger', confirmButtonText: '确定删除' }
    );
    await api.delete(`/apikeys/${row.id}`);
    ElMessage.success('删除成功');
    loadData();
  } catch {}
};

const addWhitelistIP = () => {
  const ip = whitelistInputVal.value.trim();
  if (ip && !form.ipWhitelist.includes(ip)) {
    form.ipWhitelist.push(ip);
  }
  whitelistInputVal.value = '';
  showWhitelistInput.value = false;
};

const addBlacklistIP = () => {
  const ip = blacklistInputVal.value.trim();
  if (ip && !form.ipBlacklist.includes(ip)) {
    form.ipBlacklist.push(ip);
  }
  blacklistInputVal.value = '';
  showBlacklistInput.value = false;
};

const copyText = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text);
    ElMessage.success('已复制');
  } catch {
    ElMessage.error('复制失败');
  }
};

const copyUsageExample = () => {
  const cmd = `curl -H "X-App-ID: ${keyResult.appId}" -H "X-App-Key: ${keyResult.appKey}" ${baseURL.value}/api/open/users`;
  copyText(cmd);
};

const formatTime = (t: string) => {
  if (!t) return '';
  return new Date(t).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
};

const formatDate = (t: string) => {
  if (!t) return '';
  return new Date(t).toLocaleDateString('zh-CN');
};

const formatNumber = (n: number) => {
  if (!n) return '0';
  if (n > 10000) return (n / 10000).toFixed(1) + 'w';
  if (n > 1000) return (n / 1000).toFixed(1) + 'k';
  return String(n);
};

const formatIPInfo = (row: any) => {
  const parts: string[] = [];
  if (row.ipWhiteList?.length) parts.push(`白名单: ${row.ipWhiteList.join(', ')}`);
  if (row.ipBlackList?.length) parts.push(`黑名单: ${row.ipBlackList.join(', ')}`);
  return parts.join(' | ');
};

onMounted(loadData);
</script>

<style scoped>
/* ============================================
   Corporate API Keys - 企业风格
   ============================================ */
.page-container {
  padding: 20px 24px;
  max-width: 1400px;
  
}


.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: var(--font-size-xl);
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 4px;
}

.page-desc {
  font-size: var(--font-size-sm);
  color: var(--color-text-tertiary);
  margin: 0;
}

.filter-bar {
  display: flex;
  gap: 10px;
  margin-bottom: 16px;
  align-items: center;
}

.modern-table {
  border-radius: var(--card-radius);
  overflow: hidden;
}

.key-name {
  font-weight: 500;
  color: var(--color-text-primary);
}

.key-desc {
  font-size: 12px;
  color: #86909c;
  margin-top: 2px;
}

.app-id {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  font-size: 12px;
  background: #f2f3f5;
  padding: 2px 6px;
  border-radius: 4px;
  color: #1d2129;
}

.app-key-hint {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  font-size: 12px;
  color: #86909c;
}

.usage-count {
  font-weight: 500;
  color: #165dff;
}

.last-used {
  font-size: 13px;
  color: #4e5969;
}

.last-ip {
  font-size: 11px;
  color: #86909c;
}

.text-muted {
  color: #c9cdd4;
  font-size: 12px;
}

.form-tip {
  font-size: 12px;
  color: #86909c;
  margin-top: 4px;
}

.ip-list-editor {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}

.key-display {
  background: #f7f8fa;
  border-radius: 8px;
  padding: 16px;
}

.key-row {
  display: flex;
  align-items: center;
  padding: 8px 0;
  gap: 12px;
}

.key-row + .key-row {
  border-top: 1px solid #e5e6eb;
}

.key-label {
  flex: 0 0 60px;
  font-size: 13px;
  color: #86909c;
  font-weight: 500;
}

.key-value {
  flex: 1;
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  font-size: 13px;
  background: #fff;
  padding: 4px 8px;
  border-radius: 4px;
  border: 1px solid #e5e6eb;
  word-break: break-all;
}

.key-secret {
  color: #0fc6c2;
}

.usage-example h4 {
  font-size: var(--font-size-base);
  color: var(--color-text-primary);
  margin: 0 0 8px;
}

.code-block {
  background: #1e293b;
  border-radius: var(--radius-md);
  padding: 12px 16px;
  overflow-x: auto;
}

.code-block pre {
  margin: 0;
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  font-size: 12px;
  color: #e8e8e8;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
