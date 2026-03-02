<template>
  <div class="dingtalk-page">
    <!-- 顶部操作栏 -->
    <el-card class="top-bar-card">
      <div class="top-actions">
        <div class="sync-info">
          <el-tag v-if="syncStatus.lastSyncStatus === 'success'" type="success" effect="light">
            上次同步成功
          </el-tag>
          <el-tag v-else-if="syncStatus.lastSyncStatus === 'failed'" type="danger" effect="light">
            上次同步失败
          </el-tag>
          <el-tag v-else type="info" effect="light">未同步</el-tag>
          <span class="sync-time" v-if="syncStatus.lastSyncAt">{{ syncStatus.lastSyncAt }}</span>
          <span class="sync-interval" v-if="syncStatus.syncInterval > 0">
            · 自动同步间隔 {{ syncStatus.syncInterval }} 分钟
          </span>
        </div>
        <div class="action-btns">
          <el-button @click="showSettingsDialog">
            <el-icon><Setting /></el-icon> 同步配置
          </el-button>
          <el-button type="primary" @click="triggerSync" :loading="syncing">
            <el-icon><Refresh /></el-icon> 手动同步
          </el-button>
        </div>
      </div>
    </el-card>

    <!-- 主要内容区 -->
    <div class="main-content">
      <!-- 左侧部门树 -->
      <el-card class="dept-tree-card">
        <template #header>
          <div class="card-header-row">
            <span>组织架构</span>
            <el-tag size="small" type="info">{{ departments.length }} 个部门</el-tag>
          </div>
        </template>
        <el-input
          v-model="deptSearch"
          placeholder="搜索部门"
          clearable
          size="small"
          style="margin-bottom: 12px"
        />
        <el-tree
          :data="deptTree"
          :props="{ children: 'children', label: 'name' }"
          node-key="deptId"
          :filter-node-method="filterDeptNode"
          ref="deptTreeRef"
          highlight-current
          :default-expanded-keys="defaultExpandedKeys"
          @node-click="handleDeptClick"
          :expand-on-click-node="false"
        >
          <template #default="{ data }">
            <div class="dept-node">
              <span>{{ data.name }}</span>
              <el-tag size="small" type="info" v-if="data.memberCount > 0">{{ data.memberCount }}</el-tag>
            </div>
          </template>
        </el-tree>
        <el-empty v-if="departments.length === 0" description="暂无部门数据，请先同步" :image-size="80" />
      </el-card>

      <!-- 右侧用户列表 -->
      <el-card class="user-list-card">
        <template #header>
          <div class="card-header-row">
            <span>{{ selectedDeptName || '全部钉钉用户' }}</span>
            <el-input
              v-model="userKeyword"
              placeholder="搜索用户"
              clearable
              size="small"
              style="width: 200px"
              @keyup.enter="loadUsers"
            />
          </div>
        </template>

        <el-table :data="users" v-loading="usersLoading" stripe border size="default">
          <el-table-column prop="dingtalkUid" label="钉钉UID" width="130" />
          <el-table-column prop="name" label="姓名" width="100">
            <template #default="{ row }">{{ row.name || '-' }}</template>
          </el-table-column>
          <el-table-column prop="departmentName" label="部门" width="130">
            <template #default="{ row }">{{ row.departmentName || '-' }}</template>
          </el-table-column>
          <el-table-column prop="jobTitle" label="职位" width="100">
            <template #default="{ row }">{{ row.jobTitle || '-' }}</template>
          </el-table-column>
          <el-table-column prop="mobile" label="手机号" width="130">
            <template #default="{ row }">{{ row.mobile || '-' }}</template>
          </el-table-column>
          <el-table-column prop="email" label="邮箱" min-width="200">
            <template #default="{ row }">{{ row.email || '-' }}</template>
          </el-table-column>
          <el-table-column label="已同步" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="row.localUserId > 0 ? 'success' : 'info'" size="small" effect="plain">
                {{ row.localUserId > 0 ? '是' : '否' }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination-row">
          <span class="total-text">共 {{ pagination.total }} 条</span>
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.size"
            :total="pagination.total"
            :page-sizes="[20, 50, 100]"
            layout="sizes, prev, pager, next"
            @current-change="loadUsers"
            @size-change="loadUsers"
          />
        </div>
      </el-card>
    </div>

    <!-- 同步配置对话框 -->
    <el-dialog v-model="settingsDialogVisible" title="钉钉同步配置" width="520px" destroy-on-close>
      <el-form :model="settingsForm" label-width="120px">
        <el-form-item label="用户名生成策略">
          <el-select v-model="settingsForm.usernameField" style="width: 100%">
            <el-option label="邮箱前缀（默认）" value="email_prefix" />
            <el-option label="完整邮箱" value="email" />
            <el-option label="钉钉UserID" value="dingtalk_userid" />
            <el-option label="手机号" value="mobile" />
            <el-option label="姓名拼音" value="pinyin" />
          </el-select>
          <div class="form-tip">
            <template v-if="settingsForm.usernameField === 'email_prefix'">
              例：yanze@duiba.com.cn → yanze
            </template>
            <template v-else-if="settingsForm.usernameField === 'pinyin'">
              例：闫泽 → yanze（重名自动加数字：yanze2）
            </template>
            <template v-else-if="settingsForm.usernameField === 'mobile'">
              使用钉钉绑定手机号作为用户名
            </template>
            <template v-else-if="settingsForm.usernameField === 'dingtalk_userid'">
              使用钉钉UserID作为用户名
            </template>
            <template v-else-if="settingsForm.usernameField === 'email'">
              使用完整邮箱地址作为用户名
            </template>
          </div>
          <div class="form-tip fallback-tip">
            回退规则：当首选字段为空时，系统自动按 邮箱前缀 → 手机号 → 姓名拼音 的顺序生成用户名，确保不会出现无意义的长数字 ID。
          </div>
        </el-form-item>
        <el-form-item label="默认角色">
          <el-select v-model="settingsForm.defaultRoleId" placeholder="选择默认角色" style="width: 100%">
            <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="自动注册">
          <el-switch v-model="settingsForm.autoRegister" active-text="是" inactive-text="否" />
          <div class="form-tip">钉钉免登时自动创建本地用户</div>
        </el-form-item>
        <el-divider v-if="hasSynced" content-position="left">定时同步</el-divider>
        <el-form-item v-if="hasSynced" label="同步间隔（分钟）">
          <el-input-number v-model="settingsForm.syncInterval" :min="0" :max="1440" :step="30" />
          <div class="form-tip">设为 0 则不自动同步，仅手动触发</div>
        </el-form-item>
        <el-alert
          v-if="!hasSynced"
          type="info"
          :closable="false"
          show-icon
          style="margin-top: 8px;"
        >
          <template #title>
            请先手动执行一次同步后，再设置定时同步计划。
          </template>
        </el-alert>
      </el-form>
      <template #footer>
        <el-button @click="settingsDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveSettings" :loading="savingSettings">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { Setting, Refresh } from "@element-plus/icons-vue";
import { dingtalkApi, roleApi } from "../../api";

// 部门
const departments = ref<any[]>([]);
const deptTree = ref<any[]>([]);
const deptSearch = ref("");
const deptTreeRef = ref();
const selectedDeptId = ref<number | null>(null);
const selectedDeptName = ref("");
const defaultExpandedKeys = ref<number[]>([1]); // 只展开根部门

// 用户
const users = ref<any[]>([]);
const usersLoading = ref(false);
const userKeyword = ref("");
const pagination = reactive({ page: 1, size: 20, total: 0 });

// 同步
const syncing = ref(false);
const syncStatus = ref<any>({});

// 配置
const settingsDialogVisible = ref(false);
const savingSettings = ref(false);
const settingsForm = reactive({
  usernameField: "email_prefix",
  syncInterval: 0,
  defaultRoleId: 2,
  autoRegister: true
});
const roles = ref<any[]>([]);

// 是否已完成过首次同步
const hasSynced = computed(() => !!syncStatus.value.lastSyncAt);

// 头像颜色
const avatarColors = [
  '#409eff', '#67c23a', '#e6a23c', '#f56c6c', '#909399',
  '#6366f1', '#8b5cf6', '#ec4899', '#14b8a6', '#f97316'
];
const getAvatarColor = (name: string) => {
  if (!name) return avatarColors[0];
  return avatarColors[name.charCodeAt(0) % avatarColors.length];
};

// 构建部门树
const buildDeptTree = (depts: any[]) => {
  const map: Record<number, any> = {};
  const roots: any[] = [];

  depts.forEach(d => {
    map[d.deptId] = { ...d, children: [] };
  });

  depts.forEach(d => {
    const node = map[d.deptId];
    if (d.parentId === 0 || !map[d.parentId]) {
      roots.push(node);
    } else {
      map[d.parentId].children.push(node);
    }
  });

  return roots;
};

// 过滤部门节点
const filterDeptNode = (value: string, data: any) => {
  if (!value) return true;
  return data.name.includes(value);
};

// 监听搜索
watch(deptSearch, (val) => {
  deptTreeRef.value?.filter(val);
});

// 点击部门
const handleDeptClick = (data: any) => {
  if (selectedDeptId.value === data.deptId) {
    selectedDeptId.value = null;
    selectedDeptName.value = "";
  } else {
    selectedDeptId.value = data.deptId;
    selectedDeptName.value = data.name;
  }
  pagination.page = 1;
  loadUsers();
};

// 加载部门
const loadDepartments = async () => {
  try {
    const res = await dingtalkApi.departments();
    if (res.data.success) {
      departments.value = res.data.data || [];
      deptTree.value = buildDeptTree(departments.value);
    }
  } catch (e) {
    // handled
  }
};

// 加载用户
const loadUsers = async () => {
  usersLoading.value = true;
  try {
    const params: any = {
      page: pagination.page,
      pageSize: pagination.size
    };
    if (selectedDeptId.value) {
      params.deptId = selectedDeptId.value;
    }
    if (userKeyword.value) {
      params.keyword = userKeyword.value;
    }
    const res = await dingtalkApi.users(params);
    if (res.data.success) {
      users.value = res.data.data.list || [];
      pagination.total = res.data.data.total || 0;
    }
  } catch (e) {
    // handled
  } finally {
    usersLoading.value = false;
  }
};

// 加载同步状态
const loadSyncStatus = async () => {
  try {
    const res = await dingtalkApi.syncStatus();
    if (res.data.success) {
      syncStatus.value = res.data.data;
    }
  } catch (e) {
    // handled
  }
};

// 手动触发同步
const triggerSync = async () => {
  syncing.value = true;
  try {
    const res = await dingtalkApi.sync();
    if (res.data.success) {
      const data = res.data.data;
      ElMessage.success(`同步完成：${data.departmentsSynced} 个部门，新增 ${data.usersCreated} 用户，更新 ${data.usersUpdated} 用户`);
      loadDepartments();
      loadUsers();
      loadSyncStatus();
    }
  } catch (e) {
    // handled
  } finally {
    syncing.value = false;
  }
};

// 配置对话框
const showSettingsDialog = async () => {
  try {
    const res = await dingtalkApi.getSettings();
    if (res.data.success) {
      const data = res.data.data;
      settingsForm.usernameField = data.usernameField || "email_prefix";
      settingsForm.syncInterval = data.syncInterval || 0;
      settingsForm.defaultRoleId = data.defaultRoleId || 2;
      settingsForm.autoRegister = data.autoRegister ?? true;
      // 同步 lastSyncAt 到 syncStatus（用于判断是否允许设置定时同步）
      if (data.lastSyncAt) {
        syncStatus.value.lastSyncAt = data.lastSyncAt;
      }
    }
  } catch (e) {
    // use defaults
  }
  settingsDialogVisible.value = true;
};

const saveSettings = async () => {
  savingSettings.value = true;
  try {
    const res = await dingtalkApi.updateSettings(settingsForm);
    if (res.data.success) {
      ElMessage.success("配置保存成功");
      settingsDialogVisible.value = false;
      loadSyncStatus();
    }
  } catch (e) {
    // handled
  } finally {
    savingSettings.value = false;
  }
};

// 加载角色
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

onMounted(() => {
  loadDepartments();
  loadUsers();
  loadSyncStatus();
  loadRoles();
});
</script>

<style scoped>
.dingtalk-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.top-bar-card :deep(.el-card__body) {
  padding: 12px 20px;
}

.top-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.sync-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sync-time {
  font-size: 13px;
  color: #909399;
}

.sync-interval {
  font-size: 13px;
  color: #909399;
}

.action-btns {
  display: flex;
  gap: 8px;
}

.main-content {
  display: flex;
  gap: 16px;
}

.dept-tree-card {
  width: 280px;
  flex-shrink: 0;
}

.user-list-card {
  flex: 1;
  min-width: 0;
}

.card-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 500;
}

.dept-node {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  justify-content: space-between;
  padding-right: 8px;
}

.user-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.user-cell .el-avatar {
  font-size: 12px;
  flex-shrink: 0;
}

.text-muted {
  color: #909399;
}

.pagination-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #ebeef5;
}

.total-text {
  font-size: 14px;
  color: #606266;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
  line-height: 1.4;
}

.fallback-tip {
  margin-top: 8px;
  padding: 8px 10px;
  background: #f4f4f5;
  border-radius: 4px;
  color: #606266;
  line-height: 1.6;
}

@media (max-width: 900px) {
  .main-content {
    flex-direction: column;
  }
  .dept-tree-card {
    width: 100%;
  }
}
</style>
