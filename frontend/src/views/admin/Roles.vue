<template>
  <div class="roles-page">
    <el-card>
      <template #header>
        <div class="toolbar">
          <el-button type="success" @click="showCreateDialog">新增角色</el-button>
          <el-button @click="showAutoAssignDialog">设置</el-button>
        </div>
      </template>

      <el-table :data="roles" v-loading="loading">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="角色名称" width="150" />
        <el-table-column prop="code" label="角色编码" width="150" />
        <el-table-column prop="description" label="描述" />
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">
              {{ row.status === 1 ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280">
          <template #default="{ row }">
            <el-button type="primary" link @click="showEditDialog(row)">编辑</el-button>
            <el-button type="success" link @click="showPermissionDialog(row)">权限</el-button>
            <el-button type="warning" link @click="showLayoutDialog(row)">布局</el-button>
            <el-button v-if="row.code !== 'super_admin' && row.code !== 'user'" type="danger" link @click="deleteRole(row)">删除</el-button>
            <el-tag v-else size="small" type="info">内置</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新增/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑角色' : '新增角色'" width="500px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="角色名称">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="角色编码" v-if="!isEdit">
          <el-input v-model="form.code" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveRole">保存</el-button>
      </template>
    </el-dialog>

    <!-- 布局配置对话框 -->
    <el-dialog v-model="layoutDialogVisible" title="页面布局配置" width="500px" destroy-on-close>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px;">
        配置该角色用户登录后的页面布局和默认首页。当用户拥有多个角色时，取最严格的配置。
      </el-alert>
      <el-form :model="layoutForm" label-width="100px">
        <el-form-item label="侧边栏">
          <el-radio-group v-model="layoutForm.sidebarMode">
            <el-radio-button value="auto">自动</el-radio-button>
            <el-radio-button value="visible">始终显示</el-radio-button>
            <el-radio-button value="hidden">始终隐藏</el-radio-button>
          </el-radio-group>
          <div class="layout-hint">
            <template v-if="layoutForm.sidebarMode === 'auto'">根据可访问菜单数量自动判断：仅1个菜单时隐藏</template>
            <template v-else-if="layoutForm.sidebarMode === 'hidden'">隐藏侧边栏，用户仅看到内容区域</template>
            <template v-else>始终显示侧边栏导航</template>
          </div>
        </el-form-item>
        <el-form-item label="默认首页">
          <el-select v-model="layoutForm.landingPage" style="width: 100%" placeholder="自动检测（按权限跳转）" clearable>
            <el-option label="自动检测" value="" />
            <el-option label="用户管理" value="/admin/users/local" />
            <el-option label="角色管理" value="/admin/roles" />
            <el-option label="登录日志" value="/admin/logs/login" />
            <el-option label="操作日志" value="/admin/logs/operation" />
            <el-option label="个人中心" value="/admin/profile" />
            <el-option label="系统首页" value="/admin" />
          </el-select>
          <div class="layout-hint">该角色用户登录后默认打开的页面，留空则根据权限自动跳转</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="layoutDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveLayoutConfig" :loading="savingLayout">保存</el-button>
      </template>
    </el-dialog>

    <!-- 分配权限对话框 -->
    <el-dialog v-model="permissionDialogVisible" title="分配权限" width="500px">
      <el-tree
        ref="treeRef"
        :data="permissionTree"
        :props="{ label: 'name', children: 'children' }"
        show-checkbox
        node-key="id"
        default-expand-all
      />
      <template #footer>
        <el-button @click="permissionDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="savePermissions">保存</el-button>
      </template>
    </el-dialog>

    <!-- 自动分配规则对话框 -->
    <el-dialog v-model="autoAssignVisible" title="角色自动分配规则" width="650px" destroy-on-close>
      <div class="auto-assign-tip" style="margin-bottom: 16px; color: var(--color-text-tertiary); font-size: 13px;">
        配置规则后点击"保存"仅保存规则；点击"立即执行"可将已保存的规则应用到所有匹配的已有用户。用户同步时也会自动应用。
      </div>

      <div v-for="(role, rIdx) in autoAssignRoles" :key="role.id" class="role-rule-section">
        <div class="role-rule-header">
          <el-tag type="primary" effect="dark">{{ role.name }}</el-tag>
          <el-button type="primary" link size="small" @click="addRule(rIdx)">+ 添加规则</el-button>
        </div>
        <div v-if="role.rules.length === 0" style="color: #c0c4cc; font-size: 13px; padding: 8px 0;">
          暂无规则
        </div>
        <div v-for="(rule, idx) in role.rules" :key="idx" class="rule-row">
          <el-select v-model="rule.ruleType" placeholder="条件类型" style="width: 120px" size="small">
            <el-option label="群组" value="group" />
            <el-option label="岗位" value="job_title" />
          </el-select>
          <template v-if="rule.ruleType === 'group'">
            <el-tree-select
              v-model="rule.ruleValue"
              :data="groupSelectTree"
              :props="{ children: 'children', label: 'name', value: 'id' }"
              placeholder="选择群组"
              clearable
              check-strictly
              size="small"
              style="flex: 1"
            />
          </template>
          <template v-else>
            <el-input v-model="rule.ruleValue" placeholder="输入岗位名称（精确匹配）" size="small" style="flex: 1" />
          </template>
          <el-button type="danger" link size="small" @click="removeRule(rIdx, idx)">删除</el-button>
        </div>
      </div>

      <!-- 执行结果展示 -->
      <div v-if="applyResult" class="apply-result" style="margin-top: 16px;">
        <el-alert
          :title="`执行完成：匹配 ${applyResult.totalMatched} 人，新分配 ${applyResult.totalAssigned} 人，已有角色跳过 ${applyResult.totalSkipped} 人`"
          :type="applyResult.totalAssigned > 0 ? 'success' : 'info'"
          show-icon
          :closable="false"
        />
        <div v-if="applyResult.details && applyResult.details.length > 0" style="margin-top: 8px;">
          <div v-for="d in applyResult.details" :key="d.roleId" style="font-size: 13px; color: var(--color-text-secondary); padding: 4px 0;">
            <el-tag size="small" type="primary">{{ d.roleName }}</el-tag>
            <span style="margin-left: 8px;">匹配 {{ d.matched }} 人，新分配 {{ d.assigned }} 人，跳过 {{ d.skipped }} 人</span>
          </div>
        </div>
      </div>

      <template #footer>
        <div style="display: flex; justify-content: space-between; width: 100%;">
          <el-button type="warning" @click="applyRulesNow" :loading="applyingRules">立即执行</el-button>
          <div>
            <el-button @click="autoAssignVisible = false">取消</el-button>
            <el-button type="primary" @click="saveAutoAssignRules" :loading="savingAutoAssign">保存</el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import type { ElTree } from "element-plus";
import { roleApi, permissionApi, groupApi } from "../../api";

const loading = ref(false);
const roles = ref<any[]>([]);
const permissionTree = ref<any[]>([]);
const groups = ref<any[]>([]);

const dialogVisible = ref(false);
const isEdit = ref(false);
const editingId = ref(0);
const form = reactive({ name: "", code: "", description: "" });

const permissionDialogVisible = ref(false);
const permissionRoleId = ref(0);
const treeRef = ref<InstanceType<typeof ElTree>>();

const loadRoles = async () => {
  loading.value = true;
  try {
    const res = await roleApi.list();
    if (res.data.success) {
      roles.value = res.data.data || [];
    }
  } catch (e) {
    // handled
  } finally {
    loading.value = false;
  }
};

const loadPermissionTree = async () => {
  try {
    const res = await permissionApi.tree();
    if (res.data.success) {
      permissionTree.value = res.data.data || [];
    }
  } catch (e) {
    // handled
  }
};

const showCreateDialog = () => {
  isEdit.value = false;
  editingId.value = 0;
  Object.assign(form, { name: "", code: "", description: "" });
  dialogVisible.value = true;
};

const showEditDialog = (role: any) => {
  isEdit.value = true;
  editingId.value = role.id;
  Object.assign(form, {
    name: role.name,
    code: role.code,
    description: role.description
  });
  dialogVisible.value = true;
};

const saveRole = async () => {
  if (!form.name) {
    ElMessage.warning("请输入角色名称");
    return;
  }
  try {
    if (isEdit.value) {
      await roleApi.update(editingId.value, form);
      ElMessage.success("保存成功");
    } else {
      if (!form.code) {
        ElMessage.warning("请输入角色编码");
        return;
      }
      await roleApi.create(form);
      ElMessage.success("创建成功");
    }
    dialogVisible.value = false;
    loadRoles();
  } catch (e) {
    // handled
  }
};

const deleteRole = async (role: any) => {
  try {
    await ElMessageBox.confirm(`确定删除角色 ${role.name}？`, "提示");
    await roleApi.delete(role.id);
    ElMessage.success("删除成功");
    loadRoles();
  } catch (e) {
    // cancelled or error
  }
};

// 从权限树中提取所有叶子节点ID（无 children 的节点）
const getLeafIds = (ids: number[], tree: any[]): number[] => {
  const parentIds = new Set<number>();
  const collectParents = (nodes: any[]) => {
    for (const n of nodes) {
      if (n.children && n.children.length > 0) {
        parentIds.add(n.id);
        collectParents(n.children);
      }
    }
  };
  collectParents(tree);
  return ids.filter(id => !parentIds.has(id));
};

const showPermissionDialog = async (role: any) => {
  permissionRoleId.value = role.id;
  try {
    const res = await roleApi.getPermissions(role.id);
    if (res.data.success) {
      const allIds: number[] = res.data.data || [];
      // 只传叶子节点给 setCheckedKeys，父节点由树组件自动计算（半选/全选）
      const leafIds = getLeafIds(allIds, permissionTree.value);
      permissionDialogVisible.value = true;
      setTimeout(() => {
        treeRef.value?.setCheckedKeys(leafIds, false);
      }, 100);
    }
  } catch (e) {
    // handled
  }
};

const savePermissions = async () => {
  const checkedIds = treeRef.value?.getCheckedKeys(false) as number[];
  const halfCheckedIds = treeRef.value?.getHalfCheckedKeys() as number[];
  const allIds = [...checkedIds, ...halfCheckedIds];
  try {
    await roleApi.updatePermissions(permissionRoleId.value, allIds);
    ElMessage.success("权限分配成功");
    permissionDialogVisible.value = false;
  } catch (e) {
    // handled
  }
};

// ========== 布局配置 ==========
const layoutDialogVisible = ref(false);
const savingLayout = ref(false);
const layoutRoleId = ref(0);
const layoutForm = reactive({ sidebarMode: 'auto', landingPage: '' });

const showLayoutDialog = (role: any) => {
  layoutRoleId.value = role.id;
  layoutForm.sidebarMode = role.sidebarMode || 'auto';
  layoutForm.landingPage = role.landingPage || '';
  layoutDialogVisible.value = true;
};

const saveLayoutConfig = async () => {
  savingLayout.value = true;
  try {
    const role = roles.value.find((r: any) => r.id === layoutRoleId.value);
    await roleApi.update(layoutRoleId.value, {
      name: role?.name || '',
      description: role?.description || '',
      sidebarMode: layoutForm.sidebarMode,
      landingPage: layoutForm.landingPage
    });
    ElMessage.success("布局配置保存成功");
    layoutDialogVisible.value = false;
    loadRoles();
  } catch (e) {
    ElMessage.error("保存失败");
  } finally {
    savingLayout.value = false;
  }
};

// ========== 自动分配规则 ==========
const autoAssignVisible = ref(false);
const savingAutoAssign = ref(false);
const autoAssignRoles = ref<any[]>([]);

// 构建群组选择树（跳过根部门）
const groupSelectTree = ref<any[]>([]);

const buildGroupTree = (items: any[]) => {
  const map: Record<number, any> = {};
  const roots: any[] = [];
  items.forEach((g: any) => { map[g.id] = { ...g, children: [] }; });
  items.forEach((g: any) => {
    const node = map[g.id];
    if (!g.parentId || g.parentId === 0 || !map[g.parentId]) {
      roots.push(node);
    } else {
      map[g.parentId].children.push(node);
    }
  });
  if (roots.length === 1 && roots[0].children.length > 0) {
    return roots[0].children;
  }
  return roots;
};

const loadGroups = async () => {
  try {
    const res = await groupApi.list();
    if (res.data.success) {
      groups.value = res.data.data.groups || [];
      const tree = buildGroupTree(groups.value);
      groupSelectTree.value = [
        { id: 0, name: '本地用户（全部）', children: tree }
      ];
    }
  } catch (e) {
    // handled
  }
};

const showAutoAssignDialog = async () => {
  await loadGroups();
  // 为每个角色加载规则
  const rolesWithRules: any[] = [];
  for (const role of roles.value) {
    try {
      const res = await roleApi.getAutoAssignRules(role.id);
      const rules = (res.data.success ? res.data.data : []) || [];
      rolesWithRules.push({
        id: role.id,
        name: role.name,
        rules: rules.map((r: any) => ({
          ruleType: r.ruleType,
          ruleValue: r.ruleType === 'group' ? Number(r.ruleValue) : r.ruleValue
        }))
      });
    } catch (e) {
      rolesWithRules.push({ id: role.id, name: role.name, rules: [] });
    }
  }
  autoAssignRoles.value = rolesWithRules;
  applyResult.value = null;
  autoAssignVisible.value = true;
};

const addRule = (roleIdx: number) => {
  autoAssignRoles.value[roleIdx].rules.push({ ruleType: 'group', ruleValue: '' });
};

const removeRule = (roleIdx: number, ruleIdx: number) => {
  autoAssignRoles.value[roleIdx].rules.splice(ruleIdx, 1);
};

const applyingRules = ref(false);
const applyResult = ref<any>(null);

const saveAutoAssignRules = async () => {
  savingAutoAssign.value = true;
  try {
    for (const role of autoAssignRoles.value) {
      const rules = role.rules
        .filter((r: any) => r.ruleType && (r.ruleValue !== '' && r.ruleValue !== null && r.ruleValue !== undefined))
        .map((r: any) => ({
          ruleType: r.ruleType,
          ruleValue: String(r.ruleValue)
        }));
      await roleApi.updateAutoAssignRules(role.id, rules);
    }
    ElMessage.success("规则保存成功");
    autoAssignVisible.value = false;
    applyResult.value = null;
  } catch (e) {
    ElMessage.error("保存失败");
  } finally {
    savingAutoAssign.value = false;
  }
};

const applyRulesNow = async () => {
  try {
    await ElMessageBox.confirm(
      "将根据已保存的规则，立即为所有匹配的已有用户分配对应角色。是否继续？",
      "立即执行自动分配",
      { confirmButtonText: "执行", cancelButtonText: "取消", type: "warning" }
    );
  } catch {
    return;
  }

  applyingRules.value = true;
  applyResult.value = null;
  try {
    const res = await roleApi.applyAutoAssignRules();
    if (res.data.success) {
      applyResult.value = res.data.data;
      if (res.data.data.totalAssigned > 0) {
        ElMessage.success(`成功为 ${res.data.data.totalAssigned} 名用户分配了角色`);
      } else {
        ElMessage.info("所有匹配用户已拥有对应角色，无需新分配");
      }
    }
  } catch (e) {
    ElMessage.error("执行失败");
  } finally {
    applyingRules.value = false;
  }
};

onMounted(() => {
  loadRoles();
  loadPermissionTree();
});
</script>

<style scoped>
/* ============================================
   Corporate Roles Page - 企业风格
   ============================================ */
.roles-page {
  padding: 0;
  
}


.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.layout-hint {
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-top: 6px;
  line-height: 1.5;
  padding: 6px 10px;
  background: var(--color-fill-secondary);
  border-radius: var(--radius-sm);
}

.role-rule-section {
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-lg);
  background: var(--color-fill-secondary);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
  transition: background-color var(--transition-fast);
}

.role-rule-section:hover {
  background: var(--color-fill-quaternary);
}

.role-rule-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--color-border);
}

.rule-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
  padding: 8px 12px;
  background: var(--card-bg);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
}
</style>
