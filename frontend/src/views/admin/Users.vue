<template>
  <div class="users-page">
    <!-- 主要内容区 -->
    <div class="main-content">
      <!-- 左侧面板 -->
      <el-card class="group-tree-card">
        <!-- 统一树：本地用户 + 钉钉 作为两个根节点 -->
        <el-tree
          :data="unifiedTree"
          :props="{ children: 'children', label: 'label' }"
          node-key="_uid"
          ref="unifiedTreeRef"
          highlight-current
          :default-expanded-keys="unifiedDefaultExpanded"
          @node-click="handleUnifiedNodeClick"
          :expand-on-click-node="false"
        >
          <template #default="{ node, data }">
            <div class="group-node">
              <span :class="{ 'source-root-label': data._isRoot }">{{ data.label }}</span>
              <div class="group-node-actions">
                <el-tag v-if="data._isRoot && data._source === 'local'" size="small" type="primary" effect="dark">本地/{{ totalUserCount }}</el-tag>
                <template v-if="data._isRoot && data._source === 'dingtalk'">
                  <el-tag size="small" type="warning" effect="dark">外部/dingTalk/{{ dingtalkUserCount }}</el-tag>
                  <el-dropdown trigger="click" @command="(cmd: string) => handleDtRootCommand(cmd)" @click.stop>
                    <el-icon class="group-more-btn" style="opacity: 1; margin-left: 4px;" @click.stop><MoreFilled /></el-icon>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="settings">同步配置</el-dropdown-item>
                        <el-dropdown-item command="sync">手动同步</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </template>
                <template v-if="!data._isRoot && data._source === 'local'">
                  <el-dropdown trigger="click" @command="(cmd: string) => handleGroupCommand(cmd, data)" @click.stop>
                    <el-icon class="group-more-btn" @click.stop><MoreFilled /></el-icon>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="edit">编辑</el-dropdown-item>
                        <el-dropdown-item command="addChild">添加子分组</el-dropdown-item>
                        <el-dropdown-item command="delete" divided>
                          <span class="text-error">删除</span>
                        </el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </template>
              </div>
            </div>
          </template>
        </el-tree>
      </el-card>

      <!-- 右侧用户列表 -->
      <el-card class="user-list-card">
        <template #header>
          <div class="card-header-row">
            <div class="filter-item">
              <el-input v-model="keyword" placeholder="用户名/姓名/手机号" clearable class="keyword-input" @keyup.enter="handleSearch" />
              <el-button type="primary" @click="handleSearch">搜索</el-button>
            </div>
            <div class="action-btns" v-if="activeSource === 'local'">
              <el-button v-if="canExport" type="warning" @click="exportUsers" :loading="exporting">
                <el-icon><Download /></el-icon> 导出用户
              </el-button>
              <el-button v-if="canCreateGroup" @click="showCreateGroupDialog">
                <el-icon><FolderAdd /></el-icon> 新增群组
              </el-button>
              <el-button v-if="canCreate" type="success" @click="showCreateDialog">
                <el-icon><Plus /></el-icon> 新增用户
              </el-button>
              <el-button v-if="canUpdate || canResetPassword" type="primary" @click="triggerAllSync" :loading="allSyncing">
                <el-icon><Refresh /></el-icon> 同步全部
              </el-button>
            </div>
            <div class="action-btns" v-if="activeSource === 'dingtalk'">
              <el-button v-if="canSyncDingtalk" type="primary" @click="triggerDtSync" :loading="dtSyncing">
                <el-icon><Refresh /></el-icon> 同步
              </el-button>
            </div>
          </div>
        </template>

        <!-- 批量操作栏 -->
        <div v-if="activeSource === 'local' && selectedUsers.length > 0" class="batch-bar">
          <span class="batch-count-text">已选 <b>{{ selectedUsers.length }}</b> 名用户</span>
          <el-button v-if="canResetPassword" type="warning" size="small" @click="showBatchResetDialog">
            <el-icon><RefreshRight /></el-icon> 批量重置密码
          </el-button>
          <el-button v-if="canDelete" type="danger" size="small" @click="confirmBatchDelete">
            <el-icon><Delete /></el-icon> 批量删除
          </el-button>
          <el-button size="small" @click="clearSelection">取消选择</el-button>
        </div>

        <!-- 本地用户表格 -->
        <el-table ref="userTableRef" v-if="activeSource === 'local'" :data="users" v-loading="loading" stripe size="small"
          @selection-change="handleSelectionChange" :header-cell-style="{ padding: '8px 0' }" :cell-style="{ padding: '6px 0' }">
          <el-table-column type="selection" width="40" align="center" />
          <el-table-column prop="id" label="ID" width="55" align="center" />
          <el-table-column prop="username" label="用户名" min-width="110" show-overflow-tooltip />
          <el-table-column prop="nickname" label="姓名" min-width="80" show-overflow-tooltip>
            <template #default="{ row }">{{ row.nickname || '-' }}</template>
          </el-table-column>
          <el-table-column prop="phone" label="手机号" min-width="115" show-overflow-tooltip>
            <template #default="{ row }">{{ row.phone || '-' }}</template>
          </el-table-column>
          <el-table-column prop="email" label="邮箱" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ row.email || '-' }}</template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="65" align="center">
            <template #default="{ row }">
              <el-switch
                :model-value="row.status === 1"
                @change="(val: boolean) => toggleUserStatus(row, val)"
                inline-prompt
                size="small"
                :disabled="!canToggleStatus"
              />
            </template>
          </el-table-column>
          <el-table-column label="操作" :width="canDelete ? 180 : 130" fixed="right" align="center">
            <template #default="{ row }">
              <el-button v-if="canUpdate" type="primary" link size="small" @click="showEditDialog(row)">编辑</el-button>
              <el-button v-if="canResetPassword" type="warning" link size="small" @click="showResetPasswordDialog(row)">重置密码</el-button>
              <el-button v-if="canDelete && row.username !== 'admin'" type="danger" link size="small" @click="confirmDeleteUser(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <!-- 钉钉用户表格 -->
        <el-table v-if="activeSource === 'dingtalk'" :data="dtUsers" v-loading="dtLoading" stripe size="small"
          :header-cell-style="{ padding: '8px 0' }" :cell-style="{ padding: '6px 0' }">
          <el-table-column prop="name" label="姓名" min-width="80" show-overflow-tooltip>
            <template #default="{ row }">{{ row.name || '-' }}</template>
          </el-table-column>
          <el-table-column prop="departmentName" label="部门" min-width="110" show-overflow-tooltip>
            <template #default="{ row }">{{ row.departmentName || '-' }}</template>
          </el-table-column>
          <el-table-column prop="jobTitle" label="职位" min-width="120" show-overflow-tooltip>
            <template #default="{ row }">{{ row.jobTitle || '-' }}</template>
          </el-table-column>
          <el-table-column prop="mobile" label="手机号" min-width="115" show-overflow-tooltip>
            <template #default="{ row }">{{ row.mobile || '-' }}</template>
          </el-table-column>
          <el-table-column prop="email" label="邮箱" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ row.email || '-' }}</template>
          </el-table-column>
          <el-table-column label="已同步" width="70" align="center">
            <template #default="{ row }">
              <el-tag :type="row.localUserId > 0 ? 'success' : 'info'" size="small" effect="plain">
                {{ row.localUserId > 0 ? '是' : '否' }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination-row">
          <span class="total-text">共 {{ activeSource === 'local' ? pagination.total : dtPagination.total }} 条</span>
          <el-pagination
            v-if="activeSource === 'local'"
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.size"
            :total="pagination.total"
            :page-sizes="[20, 50, 100]"
            layout="sizes, prev, pager, next"
            @current-change="loadUsers"
            @size-change="loadUsers"
          />
          <el-pagination
            v-if="activeSource === 'dingtalk'"
            v-model:current-page="dtPagination.page"
            v-model:page-size="dtPagination.size"
            :total="dtPagination.total"
            :page-sizes="[20, 50, 100]"
            layout="sizes, prev, pager, next"
            @current-change="loadDtUsers"
            @size-change="loadDtUsers"
          />
        </div>
      </el-card>
    </div>

    <!-- 新增/编辑用户对话框 -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑用户' : '新增用户'" width="520px" destroy-on-close>
      <el-alert v-if="isIMSyncUser" type="info" :closable="false" show-icon style="margin-bottom: 16px">
        该用户由IM平台同步，基本信息不可手动修改，如需变更请在对应平台管理后台修改后重新同步。
      </el-alert>
      <el-form :model="form" label-width="80px">
        <el-form-item label="用户名" v-if="!isEdit" required>
          <el-input v-model="form.username" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="用户名" v-else>
          <el-input v-model="form.username" disabled />
        </el-form-item>
        <el-form-item label="密码" v-if="!isEdit" required>
          <el-input v-model="form.password" type="password" show-password placeholder="请输入密码" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="姓名">
              <el-input v-model="form.nickname" placeholder="请输入姓名" :disabled="isIMSyncUser" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="手机号">
              <el-input v-model="form.phone" placeholder="请输入手机号" :disabled="isIMSyncUser" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="邮箱">
          <el-input v-model="form.email" placeholder="请输入邮箱" :disabled="isIMSyncUser" />
        </el-form-item>
        <el-form-item label="所属分组">
          <el-tree-select
            v-model="form.groupId"
            :data="groupSelectTree"
            :props="{ children: 'children', label: 'name', value: 'id' }"
            placeholder="选择分组（可选）"
            clearable
            check-strictly
            :disabled="isIMSyncUser"
            style="width: 100%"
          />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="状态">
              <el-switch v-model="form.status" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="禁用" />
            </el-form-item>
          </el-col>
          <el-col :span="12" v-if="canAssignRole">
            <el-form-item label="角色">
              <el-select v-model="form.roleIds" multiple placeholder="请选择角色" style="width: 100%">
                <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="永久有效">
              <el-switch v-model="form.neverExpire" active-text="是" inactive-text="否" />
            </el-form-item>
          </el-col>
          <el-col :span="12" v-if="!form.neverExpire">
            <el-form-item label="到期时间">
              <el-date-picker
                v-model="form.expiresAt"
                type="datetime"
                placeholder="选择到期时间"
                format="YYYY-MM-DD HH:mm"
                value-format="YYYY-MM-DDTHH:mm:ssZ"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveUser" :loading="saving">保存</el-button>
      </template>
    </el-dialog>

    <!-- 重置密码对话框（单个用户） -->
    <el-dialog v-model="passwordDialogVisible" title="重置密码" width="480px" destroy-on-close>
      <el-alert type="info" :closable="false" show-icon class="mb-lg">
        系统将自动生成符合密码策略的随机密码，并通过消息策略配置的通知渠道发送给员工。管理员无法查看生成的密码。
      </el-alert>
      <el-form label-width="100px">
        <el-form-item label="用户">
          <span style="font-weight: 500;">{{ passwordForm.username }}</span>
          <el-tag v-if="passwordForm.nickname" size="small" style="margin-left: 8px;">{{ passwordForm.nickname }}</el-tag>
        </el-form-item>
        <el-form-item label="手机号">
          <span>{{ passwordForm.phone || '未设置' }}</span>
        </el-form-item>
        <el-form-item label="通知渠道">
          <div v-if="policyChannelNames.length > 0" style="display: flex; gap: 8px; flex-wrap: wrap;">
            <el-tag v-for="name in policyChannelNames" :key="name" size="small" type="success">{{ name }}</el-tag>
          </div>
          <span v-else style="color: var(--color-text-tertiary); font-size: 13px;">未配置消息策略，请在「规则与策略」中设置</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="resetPassword" :loading="resetting">确认重置</el-button>
      </template>
    </el-dialog>

    <!-- 批量重置密码对话框 -->
    <el-dialog v-model="batchResetVisible" title="批量重置密码" width="600px" destroy-on-close>
      <el-alert type="warning" :closable="false" show-icon class="mb-lg">
        将为选中的 <b>{{ batchResetUsers.length }}</b> 名用户自动生成随机密码，并通过消息策略配置的渠道通知。此操作不可撤销。
      </el-alert>

      <div style="margin-bottom: 16px;">
        <div style="font-weight: 500; margin-bottom: 8px; color: #303133;">选中用户</div>
        <el-table :data="batchResetUsers" max-height="240" border size="small">
          <el-table-column prop="username" label="用户名" width="120" />
          <el-table-column prop="nickname" label="姓名" width="100">
            <template #default="{ row }">{{ row.nickname || '-' }}</template>
          </el-table-column>
          <el-table-column prop="phone" label="手机号" width="130">
            <template #default="{ row }">
              <span :class="{ 'text-error': !row.phone }">{{ row.phone || '未设置' }}</span>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <el-form label-width="100px">
        <el-form-item label="通知渠道">
          <div v-if="policyChannelNames.length > 0" style="display: flex; gap: 8px; flex-wrap: wrap;">
            <el-tag v-for="name in policyChannelNames" :key="name" size="small" type="success">{{ name }}</el-tag>
          </div>
          <span v-else style="color: var(--color-text-tertiary); font-size: 13px;">未配置消息策略</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchResetVisible = false">取消</el-button>
        <el-button type="primary" @click="doBatchReset" :loading="batchResetting">确认批量重置</el-button>
      </template>
    </el-dialog>

    <!-- 批量重置结果对话框 -->
    <el-dialog v-model="batchResultVisible" title="批量重置结果" width="600px">
      <el-alert :type="batchResultData.success === batchResultData.total ? 'success' : 'warning'" :closable="false" show-icon class="mb-lg">
        共 {{ batchResultData.total }} 人，成功 {{ batchResultData.success }} 人
      </el-alert>
      <el-table :data="batchResultData.results" max-height="360" border size="small">
        <el-table-column prop="username" label="用户名" width="120" />
        <el-table-column prop="nickname" label="姓名" width="100" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.success ? 'success' : 'danger'" size="small">{{ row.success ? '成功' : '失败' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="notifyResult" label="通知结果" min-width="200" />
      </el-table>
      <template #footer>
        <el-button type="primary" @click="batchResultVisible = false">确定</el-button>
      </template>
    </el-dialog>

    <!-- 新增/编辑分组对话框 -->
    <el-dialog v-model="groupDialogVisible" :title="isEditGroup ? '编辑分组' : '新增分组'" width="420px" destroy-on-close>
      <el-form :model="groupForm" label-width="80px">
        <el-form-item label="分组名称" required>
          <el-input v-model="groupForm.name" placeholder="请输入分组名称" />
        </el-form-item>
        <el-form-item label="上级分组">
          <el-tree-select
            v-model="groupForm.parentId"
            :data="groupSelectTree"
            :props="{ children: 'children', label: 'name', value: 'id' }"
            placeholder="无（顶级分组）"
            clearable
            check-strictly
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveGroup" :loading="savingGroup">保存</el-button>
      </template>
    </el-dialog>

    <!-- 钉钉同步配置对话框 -->
    <el-dialog v-model="dtSettingsVisible" title="钉钉同步配置" width="520px" destroy-on-close>
      <el-form :model="dtSettingsForm" label-width="130px">
        <el-form-item label="用户名生成策略">
          <el-select v-model="dtSettingsForm.usernameField" style="width: 100%">
            <el-option label="邮箱前缀（默认）" value="email_prefix" />
            <el-option label="完整邮箱" value="email" />
            <el-option label="钉钉UserID" value="dingtalk_userid" />
            <el-option label="手机号" value="mobile" />
            <el-option label="姓名拼音" value="pinyin" />
          </el-select>
          <div class="form-tip">
            <template v-if="dtSettingsForm.usernameField === 'email_prefix'">
              例：yanze@duiba.com.cn → yanze
            </template>
            <template v-else-if="dtSettingsForm.usernameField === 'pinyin'">
              例：闫泽 → yanze（重名自动加数字：yanze2）
            </template>
            <template v-else-if="dtSettingsForm.usernameField === 'mobile'">
              使用钉钉绑定手机号作为用户名
            </template>
            <template v-else-if="dtSettingsForm.usernameField === 'dingtalk_userid'">
              使用钉钉UserID作为用户名
            </template>
            <template v-else-if="dtSettingsForm.usernameField === 'email'">
              使用完整邮箱地址作为用户名
            </template>
          </div>
        </el-form-item>
        <el-form-item label="自动同步间隔(分钟)">
          <el-input-number v-model="dtSettingsForm.syncInterval" :min="0" :max="1440" :step="30" />
          <div class="form-tip">设为 0 则不自动同步</div>
        </el-form-item>
        <el-form-item label="默认角色">
          <el-select v-model="dtSettingsForm.defaultRoleId" placeholder="选择默认角色" style="width: 100%">
            <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="自动注册">
          <el-switch v-model="dtSettingsForm.autoRegister" active-text="是" inactive-text="否" />
          <div class="form-tip">钉钉免登时自动创建本地用户</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dtSettingsVisible = false">取消</el-button>
        <el-button type="primary" @click="saveDtSettings" :loading="dtSettingsSaving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Plus, FolderAdd, MoreFilled, Refresh, RefreshRight, Iphone, ChatDotRound, Download, Delete } from "@element-plus/icons-vue";
import { userApi, roleApi, groupApi, dingtalkApi, settingsApi, securityApi, syncApi, api } from "../../api";
import { useUserStore } from "../../store/user";
import { useWebSocketStore } from "../../store/websocket";

const userStore = useUserStore();
const wsStore = useWebSocketStore();

// WebSocket 取消订阅函数
let wsUnsubscribe: (() => void) | null = null;
const canCreate = computed(() => userStore.hasPermission('user:create'));
const canUpdate = computed(() => userStore.hasPermission('user:update'));
const canDelete = computed(() => userStore.hasPermission('user:delete'));
const canResetPassword = computed(() => userStore.hasPermission('user:reset_password') || userStore.hasPermission('user:update'));
const canToggleStatus = computed(() => userStore.hasPermission('user:toggle_status') || userStore.hasPermission('user:update'));
const canCreateGroup = computed(() => userStore.hasPermission('user:create_group') || userStore.hasPermission('user:create'));
const canAssignRole = computed(() => userStore.hasPermission('user:assign_role'));
const canExport = computed(() => userStore.hasPermission('user:export') || userStore.hasPermission('settings:system'));
const canViewDingtalk = computed(() => userStore.hasPermission('dingtalk:list'));
const canSyncDingtalk = computed(() => userStore.hasPermission('dingtalk:sync'));

// ===== 用户导出 =====
const exporting = ref(false);
const exportUsers = async () => {
  exporting.value = true;
  try {
    const res = await fetch('/api/users/export', {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    if (!res.ok) throw new Error('导出失败');
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `users_${new Date().toISOString().slice(0,10)}.xlsx`;
    a.click();
    URL.revokeObjectURL(url);
    ElMessage.success('导出成功');
  } catch (e: any) {
    ElMessage.error(e?.message || '导出失败');
  } finally { exporting.value = false; }
};

// ===== 钉钉对接状态 =====
const dingtalkEnabled = ref(false);

// ===== 用户源切换 =====
const activeSource = ref<'local' | 'dingtalk'>('local');
const localUserCount = ref(0);
const dingtalkUserCount = ref(0);

const switchSource = (source: 'local' | 'dingtalk') => {
  if (activeSource.value === source) return;
  activeSource.value = source;
  keyword.value = '';
  if (source === 'local') {
    selectedGroupId.value = 0;
    pagination.page = 1;
    loadUsers();
  } else {
    dtSelectedDeptId.value = null;
    dtPagination.page = 1;
    loadDtUsers();
  }
};

const handleSearch = () => {
  if (activeSource.value === 'local') {
    pagination.page = 1;
    loadUsers();
  } else {
    dtPagination.page = 1;
    loadDtUsers();
  }
};

// ===== 统一树 =====
const unifiedTreeRef = ref();
const unifiedDefaultExpanded = ref<string[]>(['root-local']);

const unifiedTree = computed(() => {
  // 本地用户根节点
  const localChildren = buildLocalSubtree(groups.value);
  const localRoot = {
    _uid: 'root-local',
    _isRoot: true,
    _source: 'local' as const,
    label: '本地用户',
    children: localChildren
  };

  const tree: any[] = [localRoot];

  // 仅当钉钉对接启用且有权限时显示钉钉根节点
  if (dingtalkEnabled.value && canViewDingtalk.value) {
    const dtChildren = buildDtSubtree(dtDepts.value);
    const dtRoot = {
      _uid: 'root-dingtalk',
      _isRoot: true,
      _source: 'dingtalk' as const,
      label: '钉钉',
      children: dtChildren
    };
    tree.push(dtRoot);
  }

  return tree;
});

// 构建本地分组子树（跳过根部门，子部门直接作为顶级）
const buildLocalSubtree = (items: any[]) => {
  const map: Record<number, any> = {};
  const topRoots: any[] = [];

  items.forEach(g => {
    map[g.id] = {
      _uid: `local-${g.id}`,
      _source: 'local',
      _groupId: g.id,
      label: g.name,
      id: g.id,
      parentId: g.parentId,
      children: []
    };
  });

  items.forEach(g => {
    const node = map[g.id];
    if (!g.parentId || g.parentId === 0 || !map[g.parentId]) {
      topRoots.push(node);
    } else {
      map[g.parentId].children.push(node);
    }
  });

  // 如果只有一个根节点（根部门），跳过它，直接返回其子节点
  if (topRoots.length === 1 && topRoots[0].children.length > 0) {
    return topRoots[0].children;
  }

  return topRoots;
};

// 构建钉钉部门子树（跳过根部门，子部门直接作为顶级）
const buildDtSubtree = (depts: any[]) => {
  const map: Record<number, any> = {};
  const topRoots: any[] = [];

  depts.forEach(d => {
    map[d.deptId] = {
      _uid: `dt-${d.deptId}`,
      _source: 'dingtalk',
      _deptId: d.deptId,
      label: d.name,
      deptId: d.deptId,
      parentId: d.parentId,
      children: []
    };
  });

  depts.forEach(d => {
    const node = map[d.deptId];
    if (!d.parentId || d.parentId === 0 || !map[d.parentId]) {
      topRoots.push(node);
    } else {
      map[d.parentId].children.push(node);
    }
  });

  // 如果只有一个根节点（根部门），跳过它，直接返回其子节点
  if (topRoots.length === 1 && topRoots[0].children.length > 0) {
    return topRoots[0].children;
  }

  return topRoots;
};

// 统一树节点点击
const handleUnifiedNodeClick = (data: any) => {
  if (data._isRoot && data._source === 'local') {
    switchSource('local');
    selectedGroupId.value = 0;
    pagination.page = 1;
    loadUsers();
  } else if (data._isRoot && data._source === 'dingtalk') {
    switchSource('dingtalk');
    dtSelectedDeptId.value = null;
    dtPagination.page = 1;
    loadDtUsers();
  } else if (data._source === 'local') {
    activeSource.value = 'local';
    selectGroup(data._groupId, data.label);
  } else if (data._source === 'dingtalk') {
    activeSource.value = 'dingtalk';
    dtSelectedDeptId.value = data._deptId;
    dtPagination.page = 1;
    loadDtUsers();
  }
};

// ===== 本地用户：分组 =====
const groups = ref<any[]>([]);
const groupTree = ref<any[]>([]);
const groupSearch = ref("");
const groupTreeRef = ref();
const selectedGroupId = ref<number | null>(0);
const selectedGroupName = ref("全部用户");
const ungroupedCount = ref(0);
const defaultExpandedKeys = ref<number[]>([]);

// ===== 钉钉用户源 =====
const dtDepts = ref<any[]>([]);
const dtSelectedDeptId = ref<number | null>(null);
const dtUsers = ref<any[]>([]);
const dtLoading = ref(false);
const dtSyncing = ref(false);
const dtPagination = reactive({ page: 1, size: 20, total: 0 });

// 钉钉同步配置
const dtSettingsVisible = ref(false);
const dtSettingsSaving = ref(false);
const dtSettingsForm = reactive({
  usernameField: "email_prefix",
  syncInterval: 0,
  defaultRoleId: 2,
  autoRegister: true
});

// 用户
const keyword = ref("");
const loading = ref(false);
const saving = ref(false);
const resetting = ref(false);
const users = ref<any[]>([]);
const roles = ref<any[]>([]);
const pagination = reactive({ page: 1, size: 20, total: 0 });

// 用户表单
const dialogVisible = ref(false);
const isEdit = ref(false);
const editingId = ref(0);
const editingSource = ref(''); // 记录正在编辑的用户来源（local/dingtalk）
// 判断是否为IM平台同步的用户（钉钉/企微/飞书等），这些用户的基本信息不可手动修改
const isIMSyncUser = computed(() => isEdit.value && (
  editingSource.value === 'dingtalk' ||
  editingSource.value === 'im_dingtalk' ||
  editingSource.value === 'im_wechatwork' ||
  editingSource.value === 'im_feishu' ||
  editingSource.value === 'im_welink'
));
const form = reactive({
  username: "",
  password: "",
  nickname: "",
  phone: "",
  email: "",
  status: 1,
  roleIds: [] as number[],
  groupId: null as number | null,
  neverExpire: true,
  expiresAt: null as string | null
});

// 重置密码
const passwordDialogVisible = ref(false);
const passwordForm = reactive({
  userId: 0,
  username: "",
  nickname: "",
  phone: "",
  dingtalkUid: "",
  notifyChannels: [] as string[]
});

// 消息策略渠道展示
const policyChannelNames = ref<string[]>([]);
const loadPolicyChannels = async (groupId?: number, userId?: number) => {
  try {
    const params: any = { scene: 'password_reset_notify' };
    if (groupId && groupId > 0) params.groupId = groupId;
    if (userId && userId > 0) params.userId = userId;
    let res = await api.get('/notify/policies/scene', { params });
    let data = res.data?.data;
    if (!data?.isActive || !(data.channelNames?.length > 0)) {
      const params2: any = { scene: 'password_reset' };
      if (groupId && groupId > 0) params2.groupId = groupId;
      if (userId && userId > 0) params2.userId = userId;
      res = await api.get('/notify/policies/scene', { params: params2 });
      data = res.data?.data;
    }
    if (data?.isActive && data.channelNames?.length > 0) {
      policyChannelNames.value = data.channelNames;
      passwordForm.notifyChannels = data.channelTypes || [];
    } else {
      policyChannelNames.value = [];
      passwordForm.notifyChannels = [];
    }
  } catch {
    policyChannelNames.value = [];
    passwordForm.notifyChannels = [];
  }
};

// 批量操作
const selectedUsers = ref<any[]>([]);
const userTableRef = ref();
const batchResetVisible = ref(false);
const batchResetUsers = ref<any[]>([]);
const batchNotifyChannels = ref<string[]>([]);
const batchResetting = ref(false);
const batchResultVisible = ref(false);
const batchResultData = reactive({ total: 0, success: 0, results: [] as any[] });

// 分组表单
const groupDialogVisible = ref(false);
const isEditGroup = ref(false);
const editingGroupId = ref(0);
const savingGroup = ref(false);
const groupForm = reactive({
  name: "",
  parentId: null as number | null
});

// 总用户数
const totalUserCount = computed(() => {
  const groupedSum = groups.value.reduce((sum: number, g: any) => sum + (g.memberCount || 0), 0);
  return groupedSum + ungroupedCount.value;
});

// 头像颜色
const avatarColors = [
  '#409eff', '#67c23a', '#e6a23c', '#f56c6c', '#909399',
  '#6366f1', '#8b5cf6', '#ec4899', '#14b8a6', '#f97316'
];
const getAvatarColor = (name: string) => {
  if (!name) return avatarColors[0];
  return avatarColors[name.charCodeAt(0) % avatarColors.length];
};

// 分组选择树（用于 tree-select，跳过根部门）
const groupSelectTree = computed(() => {
  const map: Record<number, any> = {};
  const roots: any[] = [];
  groups.value.forEach(g => { map[g.id] = { ...g, children: [] }; });
  groups.value.forEach(g => {
    const node = map[g.id];
    if (!g.parentId || g.parentId === 0 || !map[g.parentId]) {
      roots.push(node);
    } else {
      map[g.parentId].children.push(node);
    }
  });
  // 跳过根部门，直接返回其子节点
  if (roots.length === 1 && roots[0].children.length > 0) {
    return roots[0].children;
  }
  return roots;
});

// 选择分组
const selectGroup = (id: number | null, name: string) => {
  selectedGroupId.value = id;
  selectedGroupName.value = name;
  pagination.page = 1;
  loadUsers();
};

// 分组操作菜单
const handleGroupCommand = (cmd: string, data: any) => {
  if (cmd === 'edit') {
    isEditGroup.value = true;
    editingGroupId.value = data.id;
    groupForm.name = data.name;
    groupForm.parentId = data.parentId || null;
    groupDialogVisible.value = true;
  } else if (cmd === 'addChild') {
    isEditGroup.value = false;
    editingGroupId.value = 0;
    groupForm.name = "";
    groupForm.parentId = data.id;
    groupDialogVisible.value = true;
  } else if (cmd === 'delete') {
    const hasChildren = groups.value.some((g: any) => g.parentId === data.id);
    const msg = hasChildren
      ? `确定删除分组「${data.name}」及其所有下级分组？\n\n该操作将递归删除所有子分组，分组下的用户将变为未分组。此操作不可撤销。`
      : `确定删除分组「${data.name}」？该分组下的用户将变为未分组。`;
    ElMessageBox.confirm(msg, '删除分组', {
      type: 'warning',
      confirmButtonText: hasChildren ? '确认全部删除' : '确认删除',
      cancelButtonText: '取消',
      confirmButtonClass: hasChildren ? 'el-button--danger' : '',
    }).then(async () => {
      try {
        const res = await groupApi.delete(data.id, hasChildren);
        if (res.data.success) {
          ElMessage.success(hasChildren ? '分组及所有下级已删除' : '删除成功');
          loadGroups();
          loadUsers();
        }
      } catch (e: any) {
        const errMsg = e?.response?.data?.message;
        if (errMsg) ElMessage.error(errMsg);
      }
    }).catch(() => {});
  }
};

// 加载分组
const loadGroups = async () => {
  try {
    const res = await groupApi.list();
    if (res.data.success) {
      groups.value = res.data.data.groups || [];
      ungroupedCount.value = res.data.data.ungroupedCount || 0;
    }
  } catch (e) {
    // handled
  }
};

// 加载用户
const loadUsers = async () => {
  loading.value = true;
  try {
    const params: any = {
      pageIndex: pagination.page - 1,
      pageSize: pagination.size,
      keyword: keyword.value
    };
    if (!keyword.value && selectedGroupId.value !== null && selectedGroupId.value !== undefined) {
      params.groupId = selectedGroupId.value;
    }
    const res = await userApi.list(params);
    if (res.data.success) {
      users.value = res.data.data.list || [];
      pagination.total = res.data.data.total || 0;
    }
  } catch (e) {
    // handled
  } finally {
    loading.value = false;
  }
};

const loadRoles = async () => {
  try {
    const res = await roleApi.list();
    if (res.data.success) {
      roles.value = res.data.data || [];
    }
  } catch (e) {
    // handled
  }
};

// 新增分组对话框
const showCreateGroupDialog = () => {
  isEditGroup.value = false;
  editingGroupId.value = 0;
  groupForm.name = "";
  groupForm.parentId = null;
  groupDialogVisible.value = true;
};

// 保存分组
const saveGroup = async () => {
  if (!groupForm.name.trim()) {
    ElMessage.warning("请输入分组名称");
    return;
  }
  savingGroup.value = true;
  try {
    if (isEditGroup.value) {
      const res = await groupApi.update(editingGroupId.value, {
        name: groupForm.name,
        parentId: groupForm.parentId || 0
      });
      if (res.data.success) {
        ElMessage.success("保存成功");
        groupDialogVisible.value = false;
        loadGroups();
      }
    } else {
      const res = await groupApi.create({
        name: groupForm.name,
        parentId: groupForm.parentId || 0
      });
      if (res.data.success) {
        ElMessage.success("创建成功");
        groupDialogVisible.value = false;
        loadGroups();
      }
    }
  } catch (e) {
    // handled
  } finally {
    savingGroup.value = false;
  }
};

// 用户对话框
const showCreateDialog = () => {
  isEdit.value = false;
  editingId.value = 0;
  editingSource.value = 'local';
  Object.assign(form, {
    username: "",
    password: "",
    nickname: "",
    phone: "",
    email: "",
    status: 1,
    roleIds: [],
    groupId: selectedGroupId.value !== null && selectedGroupId.value > 0 ? selectedGroupId.value : null,
    neverExpire: true,
    expiresAt: null
  });
  loadRoles(); // 每次打开对话框刷新角色列表
  dialogVisible.value = true;
};

const showEditDialog = (user: any) => {
  isEdit.value = true;
  editingId.value = user.id;
  editingSource.value = user.source || 'local';
  Object.assign(form, {
    username: user.username,
    password: "",
    nickname: user.nickname,
    phone: user.phone,
    email: user.email,
    status: user.status,
    roleIds: user.roles?.map((r: any) => r.id) || [],
    groupId: user.groupId || null,
    neverExpire: !user.expiresAt,
    expiresAt: user.expiresAt || null
  });
  loadRoles(); // 每次打开对话框刷新角色列表
  dialogVisible.value = true;
};

const saveUser = async () => {
  saving.value = true;
  try {
    const payload: any = { ...form, groupId: form.groupId || 0 };
    // 处理到期时间
    if (form.neverExpire) {
      payload.expiresAt = null;
      payload.clearExpiresAt = true;
    } else {
      payload.expiresAt = form.expiresAt;
      payload.clearExpiresAt = false;
    }
    delete payload.neverExpire;

    if (isEdit.value) {
      await userApi.update(editingId.value, payload);
      ElMessage.success("保存成功");
    } else {
      if (!form.username || !form.password) {
        ElMessage.warning("请填写用户名和密码");
        saving.value = false;
        return;
      }
      await userApi.create(payload);
      ElMessage.success("创建成功");
    }
    dialogVisible.value = false;
    loadUsers();
    loadGroups(); // 刷新分组计数
  } catch (e) {
    // handled
  } finally {
    saving.value = false;
  }
};

const showResetPasswordDialog = (user: any) => {
  passwordForm.userId = user.id;
  passwordForm.username = user.username;
  passwordForm.nickname = user.nickname || "";
  passwordForm.phone = user.phone || "";
  passwordForm.dingtalkUid = user.dingtalkUid || "";
  passwordForm.notifyChannels = [];
  loadPolicyChannels(user.groupId, user.id);
  passwordDialogVisible.value = true;
};

const resetPassword = async () => {
  if (passwordForm.notifyChannels.length === 0) {
    try {
      await ElMessageBox.confirm(
        "未选择任何通知方式，员工将无法收到新密码。是否继续？",
        "提示", { type: "warning", confirmButtonText: "继续重置", cancelButtonText: "返回" }
      );
    } catch { return; }
  }
  resetting.value = true;
  try {
    const res = await userApi.resetPassword(passwordForm.userId, passwordForm.notifyChannels);
    const data = (res as any).data;
    const notifyInfo = data?.notifyResult || "";
    ElMessage.success(`密码重置成功。${notifyInfo}`);
    passwordDialogVisible.value = false;
    loadUsers();
  } catch (e) {
    // handled
  } finally {
    resetting.value = false;
  }
};

// 批量选择
const handleSelectionChange = (val: any[]) => {
  selectedUsers.value = val;
};

const clearSelection = () => {
  userTableRef.value?.clearSelection();
  selectedUsers.value = [];
};

const showBatchResetDialog = () => {
  batchResetUsers.value = [...selectedUsers.value];
  batchNotifyChannels.value = [];
  loadPolicyChannels();
  batchResetVisible.value = true;
};

const doBatchReset = async () => {
  if (batchNotifyChannels.value.length === 0) {
    try {
      await ElMessageBox.confirm(
        "未选择任何通知方式，员工将无法收到新密码。是否继续？",
        "提示", { type: "warning", confirmButtonText: "继续重置", cancelButtonText: "返回" }
      );
    } catch { return; }
  }

  batchResetting.value = true;
  try {
    const ids = batchResetUsers.value.map((u: any) => u.id);
    const res = await userApi.batchResetPassword(ids, batchNotifyChannels.value);
    const data = (res as any).data;
    batchResultData.total = data.total || 0;
    batchResultData.success = data.success || 0;
    batchResultData.results = data.results || [];
    batchResetVisible.value = false;
    batchResultVisible.value = true;
    clearSelection();
    loadUsers();
  } catch (e) {
    // handled
  } finally {
    batchResetting.value = false;
  }
};

const deleteUser = async (user: any) => {
  try {
    await userApi.delete(user.id);
    ElMessage.success("删除成功");
    loadUsers();
    loadGroups();
  } catch (e) {
    // handled
  }
};

const confirmDeleteUser = (user: any) => {
  ElMessageBox.confirm(
    `确定删除用户「${user.nickname || user.username}」(${user.username}) 吗？删除后该用户将无法登录。`,
    '删除确认',
    { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' }
  ).then(() => deleteUser(user)).catch(() => {});
};

const confirmBatchDelete = () => {
  const users = selectedUsers.value.filter((u: any) => u.username !== 'admin');
  if (users.length === 0) {
    ElMessage.warning('没有可删除的用户（admin 不可删除）');
    return;
  }
  ElMessageBox.confirm(
    `确定删除选中的 ${users.length} 名用户吗？删除后这些用户将无法登录。`,
    '批量删除确认',
    { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' }
  ).then(async () => {
    let success = 0;
    for (const u of users) {
      try {
        await userApi.delete(u.id);
        success++;
      } catch (e) { /* skip failed */ }
    }
    ElMessage.success(`成功删除 ${success} 名用户`);
    clearSelection();
    loadUsers();
    loadGroups();
  }).catch(() => {});
};

const toggleUserStatus = async (user: any, enabled: boolean) => {
  try {
    await userApi.updateStatus(user.id, enabled ? 1 : 0);
    ElMessage.success(enabled ? "已启用" : "已禁用");
    loadUsers();
  } catch (e) {
    // handled
  }
};

// ===== 钉钉相关方法 =====

const loadDtDepts = async () => {
  try {
    const res = await dingtalkApi.departments();
    if (res.data.success) {
      dtDepts.value = res.data.data || [];
    }
  } catch (e) {
    // handled
  }
};

const loadDtUsers = async () => {
  dtLoading.value = true;
  try {
    const params: any = {
      page: dtPagination.page,
      pageSize: dtPagination.size,
      keyword: keyword.value
    };
    if (dtSelectedDeptId.value !== null) {
      params.deptId = dtSelectedDeptId.value;
    }
    const res = await dingtalkApi.users(params);
    if (res.data.success) {
      dtUsers.value = res.data.data.list || [];
      dtPagination.total = res.data.data.total || 0;
      dingtalkUserCount.value = res.data.data.total || 0;
    }
  } catch (e) {
    // handled
  } finally {
    dtLoading.value = false;
  }
};

// 钉钉根节点操作菜单
const handleDtRootCommand = (cmd: string) => {
  if (cmd === 'settings') {
    showDtSettingsDialog();
  } else if (cmd === 'sync') {
    triggerDtSync();
  }
};

const showDtSettingsDialog = async () => {
  try {
    const res = await dingtalkApi.getSettings();
    if (res.data.success) {
      const data = res.data.data;
      dtSettingsForm.usernameField = data.usernameField || "email_prefix";
      dtSettingsForm.syncInterval = data.syncInterval || 0;
      dtSettingsForm.defaultRoleId = data.defaultRoleId || 2;
      dtSettingsForm.autoRegister = data.autoRegister ?? true;
    }
  } catch (e) {
    // use defaults
  }
  dtSettingsVisible.value = true;
};

const saveDtSettings = async () => {
  dtSettingsSaving.value = true;
  try {
    const res = await dingtalkApi.updateSettings(dtSettingsForm);
    if (res.data.success) {
      ElMessage.success("配置保存成功");
      dtSettingsVisible.value = false;
    }
  } catch (e) {
    // handled
  } finally {
    dtSettingsSaving.value = false;
  }
};

// ===== 全局同步（钉钉 + 所有同步器） =====
const allSyncing = ref(false);
const triggerAllSync = async () => {
  allSyncing.value = true;
  try {
    const res = await syncApi.triggerAll();
    if (res.data.success) {
      const data = res.data.data;
      ElMessage.success(data.message || '同步任务已触发');
      // 延迟刷新用户列表，等待同步完成
      setTimeout(() => {
        loadUsers();
        loadGroups();
      }, 2000);
      // 再次延迟刷新，确保同步完成后数据最新
      setTimeout(() => {
        loadUsers();
        loadGroups();
      }, 5000);
    } else {
      ElMessage.error(res.data.message || '触发同步失败');
    }
  } catch (e) {
    ElMessage.error('触发同步失败');
  } finally {
    allSyncing.value = false;
  }
};

const triggerDtSync = async () => {
  dtSyncing.value = true;
  try {
    const res = await dingtalkApi.sync();
    if (res.data.success) {
      const d = res.data.data;
      ElMessage.success(`同步完成：部门${d.departmentsSynced} 新增${d.usersCreated} 更新${d.usersUpdated}`);
      loadDtDepts();
      loadDtUsers();
      // 同步后也刷新本地用户计数
      loadUsers();
      loadGroups();
    }
  } catch (e) {
    ElMessage.error('同步失败');
  } finally {
    dtSyncing.value = false;
  }
};

// 加载钉钉对接状态
const loadDingtalkStatus = async () => {
  try {
    const res = await settingsApi.getDingtalkStatus();
    if (res.data.success) {
      dingtalkEnabled.value = res.data.data.enabled || false;
    }
  } catch (e) {
    dingtalkEnabled.value = false;
  }
};

onMounted(async () => {
  loadGroups();
  loadUsers();
  loadRoles();

  // 先检查钉钉是否启用，启用后且有权限才加载钉钉数据
  await loadDingtalkStatus();
  if (dingtalkEnabled.value && canViewDingtalk.value) {
    loadDtDepts();
    dingtalkApi.users({ page: 1, pageSize: 1 }).then(res => {
      if (res.data.success) {
        dingtalkUserCount.value = res.data.data.total || 0;
      }
    }).catch(() => {});
  }
  
  // 订阅用户和群组变更事件（实时刷新）
  wsStore.connect();
  const unsub1 = wsStore.subscribe('user_change', (data) => {
    console.log('[用户管理] 收到用户变更事件:', data);
    // 刷新用户列表和分组计数
    loadUsers();
    loadGroups();
  });
  const unsub2 = wsStore.subscribe('group_change', (data) => {
    console.log('[用户管理] 收到群组变更事件:', data);
    loadGroups();
    loadUsers();
  });
  wsUnsubscribe = () => {
    unsub1();
    unsub2();
  };
});

onUnmounted(() => {
  if (wsUnsubscribe) wsUnsubscribe();
});
</script>

<style scoped>
/* ============================================
   Corporate Users Page - 企业风格
   ============================================ */
.users-page {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-lg);
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  
}


.batch-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  background: var(--color-warning-bg);
  border: 1px solid var(--color-warning-border);
  border-radius: var(--radius-md);
  margin-bottom: 12px;
}

.source-root-label {
  font-weight: 600;
  font-size: 14px;
}

.form-tip {
  font-size: 12px;
  color: var(--color-text-tertiary);
  line-height: 1.4;
  margin-top: 4px;
}

.filter-item {
  display: flex;
  align-items: center;
  gap: 10px;
}

.action-btns {
  display: flex;
  align-items: center;
  gap: 10px;
}

.keyword-input {
  width: 200px;
}

.batch-count-text {
  color: var(--color-text-secondary);
  font-size: 13px;
  font-weight: 500;
}

.text-error {
  color: var(--color-error);
}

.mb-lg {
  margin-bottom: 16px;
}

.main-content {
  display: flex;
  gap: var(--spacing-xl);
  min-width: 0;
  max-width: 100%;
}

.group-tree-card {
  width: 240px;
  flex-shrink: 0;
}

.group-tree-card :deep(.el-card__body) {
  padding: var(--spacing-md);
}

.user-list-card {
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.user-list-card :deep(.el-card__body) {
  overflow-x: auto;
  padding: var(--spacing-lg);
}

.card-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 500;
  flex-wrap: wrap;
  gap: 12px;
}

.group-node-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  cursor: pointer;
  border-radius: var(--radius-lg);
  transition: all var(--transition-normal) var(--ease-out);
  font-size: 14px;
  color: var(--color-text-secondary);
}

.group-node-item:hover {
  background: var(--color-fill-secondary);
}

.group-node-item.active {
  background: var(--color-primary-bg);
  color: var(--color-primary);
  font-weight: 500;
}

.group-node {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding-right: 4px;
}

.group-node-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.group-more-btn {
  font-size: 14px;
  color: var(--color-text-tertiary);
  cursor: pointer;
  opacity: 0;
  transition: all var(--transition-fast);
  padding: 4px;
  border-radius: var(--radius-sm);
}

.el-tree-node:hover .group-more-btn {
  opacity: 1;
}

.group-more-btn:hover {
  color: var(--color-primary);
  background: var(--color-primary-bg);
}

.user-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.user-cell .el-avatar {
  font-size: 12px;
  flex-shrink: 0;
}

.text-muted {
  color: var(--color-text-tertiary);
}

.pagination-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: var(--spacing-lg);
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--color-border);
}

.total-text {
  font-size: 14px;
  color: var(--color-text-secondary);
  font-weight: 500;
}

@media (max-width: 900px) {
  .main-content {
    flex-direction: column;
  }
  .group-tree-card {
    width: 100%;
  }
  .card-header-row {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
