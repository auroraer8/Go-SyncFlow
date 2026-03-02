<template>
  <div class="notify-page">
    <el-tabs v-model="activeTab" type="border-card">
      <!-- 消息策略 -->
      <el-tab-pane label="消息策略" name="policies">
        <!-- 默认策略 -->
        <div class="tab-header">
          <span class="tab-desc">配置每种消息场景默认使用的通知通道（适用于所有用户）</span>
          <el-button type="primary" size="small" @click="savePolicies" :loading="savingPolicies">保存默认策略</el-button>
        </div>
        <el-table :data="policies" v-loading="loadingPolicies" stripe size="small">
          <el-table-column prop="sceneName" label="消息场景" width="200">
            <template #default="{ row }">
              <span class="scene-name">{{ row.sceneName }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="scene" label="场景标识" width="180">
            <template #default="{ row }">
              <code class="scene-code">{{ row.scene }}</code>
            </template>
          </el-table-column>
          <el-table-column label="通知通道" min-width="300">
            <template #default="{ row }">
              <el-select
                v-model="row.channelIds"
                multiple
                placeholder="选择通知通道"
                style="width: 100%"
                size="small"
              >
                <el-option
                  v-for="ch in channels"
                  :key="ch.id"
                  :label="ch.name"
                  :value="ch.id"
                />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="启用" width="80" align="center">
            <template #default="{ row }">
              <el-switch v-model="row.isActive" size="small" />
            </template>
          </el-table-column>
        </el-table>

        <!-- 专属策略 -->
        <div class="section-divider"></div>
        <div class="tab-header">
          <div>
            <span class="tab-desc" style="font-weight: 500; font-size: 15px; color: var(--color-text-primary);">专属策略</span>
            <div class="tab-desc" style="margin-top: 4px;">为特定群组或角色设置独立的通知通道，优先级高于默认策略</div>
          </div>
          <el-button type="primary" size="small" @click="openGroupPolicyDialog()">添加专属策略</el-button>
        </div>
        <el-table :data="groupPolicies" v-loading="loadingGroupPolicies" stripe size="small" v-if="groupPolicies.length > 0">
          <el-table-column label="消息场景" width="200">
            <template #default="{ row }">
              <span class="scene-name">{{ row.sceneName || getSceneName(row.scene) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="适用对象" min-width="260">
            <template #default="{ row }">
              <template v-if="(row.groupIdList || []).length > 0">
                <el-tag v-for="gid in row.groupIdList" :key="'g'+gid" size="small" style="margin-right: 4px;">
                  {{ getGroupName(gid) }}
                </el-tag>
              </template>
              <template v-if="(row.roleIdList || []).length > 0">
                <el-tag v-for="rid in row.roleIdList" :key="'r'+rid" type="warning" size="small" style="margin-right: 4px;">
                  {{ getRoleName(rid) }}
                </el-tag>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="通知通道" min-width="200">
            <template #default="{ row }">
              <el-tag v-for="cid in (row.channelIdList || [])" :key="cid" type="success" size="small" style="margin-right: 4px;">
                {{ getChannelName(cid) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="启用" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="row.isActive ? 'success' : 'info'" size="small">{{ row.isActive ? '启用' : '停用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" link size="small" @click="editGroupPolicy(row)">编辑</el-button>
              <el-popconfirm title="确定删除此专属策略?" @confirm="deleteGroupPolicy(row.id)">
                <template #reference>
                  <el-button type="danger" link size="small">删除</el-button>
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-else description="暂无专属策略" :image-size="60" />
      </el-tab-pane>

      <!-- 告警规则 -->
      <el-tab-pane label="告警规则" name="rules">
        <div class="tab-header">
          <span class="tab-desc">配置安全事件触发条件和通知规则</span>
          <el-button type="primary" size="small" @click="openAddRule">添加规则</el-button>
        </div>
        <el-table :data="rules" v-loading="loading" stripe size="small">
          <el-table-column prop="name" label="规则名称" min-width="140" />
          <el-table-column label="告警类型" width="110" align="center">
            <template #default="{ row }">
              <el-tag :type="row.alertType === 'admin' ? 'danger' : 'primary'" size="small">
                {{ row.alertType === 'admin' ? '管理员告警' : '员工告警' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="severityThreshold" label="级别阈值" width="100" align="center">
            <template #default="{ row }">
              <el-tag :type="getSeverityType(row.severityThreshold)" size="small">
                >= {{ getSeverityName(row.severityThreshold) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="eventTypes" label="事件类型" min-width="160">
            <template #default="{ row }">
              <span v-if="!row.eventTypes || row.eventTypes.length === 0" class="text-muted">全部类型</span>
              <el-tag v-else v-for="t in row.eventTypes" :key="t" size="small" style="margin-right: 4px">
                {{ getEventTypeName(t) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="通知方式" width="110">
            <template #default="{ row }">
              <span v-if="row.alertType === 'admin'">渠道直发</span>
              <span v-else class="text-muted">通知本人</span>
            </template>
          </el-table-column>
          <el-table-column prop="cooldownMinutes" label="冷却时间" width="90" align="center">
            <template #default="{ row }">{{ row.cooldownMinutes }}分</template>
          </el-table-column>
          <el-table-column prop="isActive" label="状态" width="70" align="center">
            <template #default="{ row }">
              <el-tag :type="row.isActive ? 'success' : 'info'" size="small">
                {{ row.isActive ? '启用' : '停用' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" link size="small" @click="editRule(row)">编辑</el-button>
              <el-popconfirm title="确定删除此规则?" @confirm="deleteRule(row.id)">
                <template #reference>
                  <el-button type="danger" link size="small">删除</el-button>
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <!-- 规则编辑对话框 -->
    <el-dialog v-model="showDialog" :title="editingId ? '编辑规则' : '添加规则'" width="580px" destroy-on-close>
      <el-form :model="form" label-width="100px">
        <el-form-item label="告警类型" required>
          <el-radio-group v-model="form.alertType">
            <el-radio-button value="admin">
              管理员告警
            </el-radio-button>
            <el-radio-button value="employee">
              员工告警
            </el-radio-button>
          </el-radio-group>
          <div class="form-hint-block" v-if="form.alertType === 'admin'">
            通过选择的通知渠道发送告警（如 Webhook 到钉钉群、邮件到管理员邮箱等）
          </div>
          <div class="form-hint-block" v-else>
            通知触发安全事件的员工本人（通过渠道发送到员工绑定的邮箱/手机）
          </div>
        </el-form-item>
        <el-form-item label="规则名称" required>
          <el-input v-model="form.name" placeholder="如：高危事件告警" />
        </el-form-item>
        <el-form-item label="事件类型">
          <el-select v-model="form.eventTypes" multiple style="width: 100%" placeholder="留空表示全部类型">
            <el-option label="登录失败 [中]" value="login_failed" />
            <el-option label="登录阻止 [高]" value="login_blocked" />
            <el-option label="账户锁定 [高]" value="account_locked" />
            <el-option label="IP封禁 [高]" value="ip_blocked" />
            <el-option label="密码修改 [中]" value="password_changed" />
            <el-option label="配置变更 [中]" value="config_changed" />
            <el-option label="可疑活动 [高]" value="suspicious_activity" />
            <el-option label="会话终止 [低]" value="session_terminated" />
            <el-option label="登录成功 [低]" value="login_success" />
          </el-select>
        </el-form-item>
        <el-form-item label="级别阈值" required>
          <el-select v-model="form.severityThreshold" style="width: 100%">
            <el-option label="低及以上（全部事件）" value="low" />
            <el-option label="中及以上（登录失败/密码修改/配置变更等）" value="medium" />
            <el-option label="高及以上（登录阻止/账户锁定/IP封禁/可疑活动）" value="high" />
            <el-option label="仅严重（暴力破解等极端事件）" value="critical" />
          </el-select>
          <div class="severity-legend">
            <div class="severity-item"><el-tag type="info" size="small">低</el-tag> 登录成功、会话终止、IP解封</div>
            <div class="severity-item"><el-tag type="warning" size="small">中</el-tag> 登录失败、密码修改、配置变更、钉钉登录失败</div>
            <div class="severity-item"><el-tag type="danger" size="small">高</el-tag> 登录阻止、账户锁定、IP封禁、可疑活动</div>
            <div class="severity-item"><el-tag type="danger" size="small" effect="dark">严重</el-tag> 暴力破解检测、异常行为检测</div>
          </div>
        </el-form-item>
        <el-form-item label="通知渠道" required>
          <el-select v-model="form.channelIds" multiple style="width: 100%" placeholder="请选择通知渠道">
            <el-option v-for="ch in channels" :key="ch.id" :label="ch.name" :value="ch.id" />
          </el-select>
        </el-form-item>

        <el-form-item label="消息模板">
          <el-select v-model="form.templateId" style="width: 100%" placeholder="使用默认模板" clearable>
            <el-option label="使用默认模板" :value="0" />
            <el-option v-for="t in templates" :key="t.id" :label="t.name + ' (' + t.scene + ')'" :value="t.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="冷却时间">
          <el-input-number v-model="form.cooldownMinutes" :min="1" :max="1440" />
          <span class="form-hint">分钟（同类事件间隔）</span>
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.isActive" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" @click="saveRule" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
    <!-- 专属策略编辑对话框 -->
    <el-dialog v-model="groupPolicyDialogVisible" :title="editingGroupPolicyId ? '编辑专属策略' : '添加专属策略'" width="560px" destroy-on-close>
      <el-form :model="groupPolicyForm" label-width="100px">
        <el-form-item label="消息场景" required>
          <el-select v-model="groupPolicyForm.scene" style="width: 100%" placeholder="选择消息场景">
            <el-option v-for="ds in defaultScenes" :key="ds.scene" :label="ds.sceneName" :value="ds.scene" />
          </el-select>
        </el-form-item>
        <el-form-item label="适用群组">
          <el-tree-select
            v-model="groupPolicyForm.targetGroupIds"
            :data="groupTree"
            node-key="id"
            :props="{ children: 'children', label: 'name', value: 'id' }"
            multiple
            check-strictly
            filterable
            :default-expand-all="false"
            :render-after-expand="false"
            show-checkbox
            collapse-tags
            collapse-tags-tooltip
            style="width: 100%"
            placeholder="选择适用的用户群组（可选）"
          />
        </el-form-item>
        <el-form-item label="适用角色">
          <el-select v-model="groupPolicyForm.targetRoleIds" multiple style="width: 100%" placeholder="选择适用的角色（可选）" collapse-tags collapse-tags-tooltip>
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
          <div class="form-hint-block">群组和角色至少选一项，符合条件的用户将使用此策略而非默认策略</div>
        </el-form-item>
        <el-form-item label="通知通道" required>
          <el-select v-model="groupPolicyForm.channelIds" multiple style="width: 100%" placeholder="选择通知通道">
            <el-option v-for="ch in channels" :key="ch.id" :label="ch.name" :value="ch.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="groupPolicyForm.isActive" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupPolicyDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveGroupPolicy" :loading="savingGroupPolicy">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { securityApi, groupApi, roleApi, api } from "../../api";

const activeTab = ref("policies");

// ========== 通知通道（共用） ==========
const channels = ref<any[]>([]);
const loadChannels = async () => {
  try {
    const res = await securityApi.notifyChannels();
    if (res.data.success) channels.value = (res.data.data || []).filter((c: any) => c.isActive);
  } catch {}
};

// ========== 消息模板 ==========
const templates = ref<any[]>([]);
const loadTemplates = async () => {
  try {
    const res = await securityApi.getTemplates();
    if (res.data.success) templates.value = res.data.data || [];
  } catch {}
};


// ========== 消息策略 ==========
const loadingPolicies = ref(false);
const savingPolicies = ref(false);

const defaultScenes = [
  { scene: "verify_code", sceneName: "验证码通知" },
  { scene: "password_reset", sceneName: "密码重置验证" },
  { scene: "password_reset_notify", sceneName: "密码被重置通知" },
  { scene: "account_created", sceneName: "账号开通通知" },
  { scene: "test", sceneName: "测试消息" },
];
// 注：安全告警（security_alert / admin_alert）由「告警规则」Tab 统一管理，不在消息策略中配置

const policies = ref<any[]>([]);

const loadPolicies = async () => {
  loadingPolicies.value = true;
  try {
    const res = await securityApi.getPolicies();
    const existing = res.data.success ? (res.data.data || []) : [];
    // 只取默认策略（targetType=all 或空），排除群组策略
    const defaults = existing.filter((p: any) => !p.targetType || p.targetType === 'all');
    policies.value = defaultScenes.map(ds => {
      const found = defaults.find((p: any) => p.scene === ds.scene);
      return {
        scene: ds.scene,
        sceneName: ds.sceneName,
        channelIds: found?.channelIdList || found?.channelIds || [],
        isActive: found ? found.isActive : true,
      };
    });
  } finally {
    loadingPolicies.value = false;
  }
};

const savePolicies = async () => {
  savingPolicies.value = true;
  try {
    await securityApi.batchUpdatePolicies(policies.value);
    ElMessage.success("默认策略已保存");
    loadPolicies();
  } catch {
    ElMessage.error("保存失败");
  } finally {
    savingPolicies.value = false;
  }
};

// ========== 专属策略 ==========
const groupTree = ref<any[]>([]);
const groupFlatMap = ref<Record<number, string>>({});
const roles = ref<any[]>([]);
const roleFlatMap = ref<Record<number, string>>({});
const groupPolicies = ref<any[]>([]);
const loadingGroupPolicies = ref(false);
const groupPolicyDialogVisible = ref(false);
const savingGroupPolicy = ref(false);
const editingGroupPolicyId = ref<number | null>(null);

const groupPolicyForm = reactive({
  scene: "password_reset_notify",
  targetGroupIds: [] as number[],
  targetRoleIds: [] as number[],
  channelIds: [] as number[],
  isActive: true
});

const loadRoles = async () => {
  try {
    const res = await roleApi.list();
    if (res.data.success) {
      roles.value = res.data.data || [];
      const map: Record<number, string> = {};
      for (const r of roles.value) map[r.id] = r.name;
      roleFlatMap.value = map;
    }
  } catch {}
};

const getRoleName = (rid: number): string => {
  return roleFlatMap.value[rid] || `角色#${rid}`;
};

const loadGroups = async () => {
  try {
    const res = await groupApi.list();
    if (res.data.success) {
      const data = res.data.data;
      const rawGroups: any[] = data?.groups || data || [];
      const tree = buildGroupTree(rawGroups);
      // 跳过根部门，直接展示其子节点
      if (tree.length === 1 && tree[0].children?.length > 0) {
        groupTree.value = tree[0].children;
      } else {
        groupTree.value = tree;
      }
      // 构建 id -> name 映射
      const flatMap: Record<number, string> = {};
      const buildMap = (nodes: any[]) => {
        for (const n of nodes) {
          flatMap[n.id] = n.name;
          if (n.children?.length) buildMap(n.children);
        }
      };
      buildMap(rawGroups);
      groupFlatMap.value = flatMap;
    }
  } catch {}
};

const buildGroupTree = (list: any[]): any[] => {
  const map: Record<number, any> = {};
  const roots: any[] = [];
  for (const g of list) {
    map[g.id] = { ...g, children: [] };
  }
  for (const g of list) {
    const node = map[g.id];
    if (g.parentId && map[g.parentId]) {
      map[g.parentId].children.push(node);
    } else {
      roots.push(node);
    }
  }
  return roots;
};

const loadGroupPolicies = async () => {
  loadingGroupPolicies.value = true;
  try {
    const res = await securityApi.getPolicies();
    const all = res.data.success ? (res.data.data || []) : [];
    groupPolicies.value = all.filter((p: any) => p.targetType === "group");
  } finally {
    loadingGroupPolicies.value = false;
  }
};

const getGroupName = (gid: number): string => {
  return groupFlatMap.value[gid] || `群组#${gid}`;
};

const getChannelName = (cid: number): string => {
  const ch = channels.value.find((x: any) => x.id === cid);
  return ch ? ch.name : `渠道#${cid}`;
};

const getSceneName = (scene: string): string => {
  const ds = defaultScenes.find(s => s.scene === scene);
  return ds ? ds.sceneName : scene;
};

const openGroupPolicyDialog = (existing?: any) => {
  if (existing) {
    editingGroupPolicyId.value = existing.id;
    Object.assign(groupPolicyForm, {
      scene: existing.scene,
      targetGroupIds: existing.groupIdList || [],
      targetRoleIds: existing.roleIdList || [],
      channelIds: existing.channelIdList || [],
      isActive: existing.isActive
    });
  } else {
    editingGroupPolicyId.value = null;
    Object.assign(groupPolicyForm, {
      scene: "password_reset_notify",
      targetGroupIds: [],
      targetRoleIds: [],
      channelIds: [],
      isActive: true
    });
  }
  groupPolicyDialogVisible.value = true;
};

const editGroupPolicy = (row: any) => openGroupPolicyDialog(row);

const saveGroupPolicy = async () => {
  if (groupPolicyForm.targetGroupIds.length === 0 && groupPolicyForm.targetRoleIds.length === 0) {
    ElMessage.warning("请至少选择一个适用群组或角色"); return;
  }
  if (groupPolicyForm.channelIds.length === 0) {
    ElMessage.warning("请选择通知通道"); return;
  }
  savingGroupPolicy.value = true;
  try {
    const sceneName = getSceneName(groupPolicyForm.scene);
    const payload = {
      scene: groupPolicyForm.scene,
      sceneName,
      channelIds: groupPolicyForm.channelIds,
      targetGroupIds: groupPolicyForm.targetGroupIds,
      targetRoleIds: groupPolicyForm.targetRoleIds,
      isActive: groupPolicyForm.isActive
    };
    if (editingGroupPolicyId.value) {
      await api.put(`/notify/policies/group/${editingGroupPolicyId.value}`, payload);
    } else {
      await api.post("/notify/policies/group", payload);
    }
    ElMessage.success("专属策略已保存");
    groupPolicyDialogVisible.value = false;
    loadGroupPolicies();
  } catch {
    ElMessage.error("保存失败");
  } finally {
    savingGroupPolicy.value = false;
  }
};

const deleteGroupPolicy = async (id: number) => {
  try {
    await api.delete(`/notify/policies/group/${id}`);
    ElMessage.success("已删除");
    loadGroupPolicies();
  } catch {
    ElMessage.error("删除失败");
  }
};

// ========== 告警规则 ==========
const loading = ref(false);
const saving = ref(false);
const rules = ref<any[]>([]);
const showDialog = ref(false);
const editingId = ref<number | null>(null);

const form = reactive({
  alertType: "admin" as string,
  name: "",
  eventTypes: [] as string[],
  severityThreshold: "high",
  channelIds: [] as number[],
  templateId: 0,
  cooldownMinutes: 30,
  isActive: true
});

const severityMap: Record<string, { name: string; type: string }> = {
  low: { name: "低", type: "info" },
  medium: { name: "中", type: "warning" },
  high: { name: "高", type: "danger" },
  critical: { name: "严重", type: "danger" }
};

const eventTypeMap: Record<string, string> = {
  login_success: "登录成功",
  login_failed: "登录失败",
  login_blocked: "登录阻止",
  account_locked: "账户锁定",
  ip_blocked: "IP封禁",
  password_changed: "密码修改",
  config_changed: "配置变更",
  suspicious_activity: "可疑活动",
  session_terminated: "会话终止"
};

const getSeverityName = (level: string) => severityMap[level]?.name || level;
const getSeverityType = (level: string) => severityMap[level]?.type || "info";
const getEventTypeName = (type: string) => eventTypeMap[type] || type;
const getTemplateName = (id: number) => {
  const t = templates.value.find((tpl: any) => tpl.id === id);
  return t ? t.name : `模板#${id}`;
};

const loadRules = async () => {
  loading.value = true;
  try {
    const res = await securityApi.alertRules();
    if (res.data.success) rules.value = res.data.data || [];
  } finally {
    loading.value = false;
  }
};

const openAddRule = () => {
  editingId.value = null;
  Object.assign(form, {
    alertType: "admin", name: "", eventTypes: [], severityThreshold: "high",
    channelIds: [], templateId: 0, cooldownMinutes: 30, isActive: true
  });
  showDialog.value = true;
};

const editRule = (row: any) => {
  editingId.value = row.id;
  Object.assign(form, {
    alertType: row.alertType || "admin",
    name: row.name,
    eventTypes: row.eventTypes || [],
    severityThreshold: row.severityThreshold,
    channelIds: row.channelIds || [],
    templateId: row.templateId || 0,
    cooldownMinutes: row.cooldownMinutes,
    isActive: row.isActive
  });
  showDialog.value = true;
};

const saveRule = async () => {
  if (!form.name) { ElMessage.warning("请输入规则名称"); return; }
  if (form.channelIds.length === 0) { ElMessage.warning("请选择通知渠道"); return; }
  saving.value = true;
  try {
    const payload = {
      alertType: form.alertType,
      name: form.name,
      eventTypes: form.eventTypes,
      severityThreshold: form.severityThreshold,
      notifyChannels: form.channelIds,
      notifyTarget: form.alertType === "employee" ? "event_user" : "channel",
      templateId: form.templateId,
      cooldownMinutes: form.cooldownMinutes,
      isActive: form.isActive
    };
    if (editingId.value) {
      await securityApi.updateAlertRule(editingId.value, payload);
    } else {
      await securityApi.createAlertRule(payload);
    }
    ElMessage.success("保存成功");
    showDialog.value = false;
    loadRules();
  } finally {
    saving.value = false;
  }
};

const deleteRule = async (id: number) => {
  try {
    await securityApi.deleteAlertRule(id);
    ElMessage.success("删除成功");
    loadRules();
  } catch { ElMessage.error("删除失败"); }
};

// ========== 初始化 ==========
onMounted(() => {
  loadChannels();
  loadTemplates();
  loadPolicies();
  loadGroupPolicies();
  loadGroups();
  loadRoles();
  loadRules();
});
</script>

<style scoped>
.notify-page { max-width: 1100px; }
.tab-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.tab-desc { color: var(--color-text-secondary); font-size: 14px; }
.scene-name { font-weight: 500; color: var(--color-text-primary); }
.scene-code { font-size: 12px; color: var(--color-text-tertiary); background: var(--color-fill-secondary); padding: 2px 6px; border-radius: 3px; }
.form-hint { margin-left: 8px; color: var(--color-text-tertiary); font-size: 13px; }
.form-hint-block { color: var(--color-text-tertiary); font-size: 12px; margin-top: 4px; line-height: 1.4; }
.text-muted { color: var(--color-text-tertiary); font-size: 13px; }
.severity-legend {
  margin-top: 8px;
  padding: 10px 12px;
  background: var(--color-fill-secondary, #f5f7fa);
  border-radius: 6px;
  font-size: 12px;
  line-height: 2;
  color: var(--color-text-secondary);
}
.severity-item {
  display: flex;
  align-items: center;
  gap: 8px;
}
.severity-item .el-tag {
  min-width: 36px;
  text-align: center;
}
.section-divider {
  margin: 24px 0 16px;
  border-top: 1px solid var(--color-border-lighter, #f0f0f0);
}
</style>
