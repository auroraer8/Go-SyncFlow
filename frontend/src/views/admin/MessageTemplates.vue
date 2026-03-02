<template>
  <div class="template-page">
    <!-- 模板列表 -->
    <el-card>
      <template #header>
        <div class="card-header">
          <span>消息模板管理</span>
          <el-button type="primary" size="small" @click="openAdd">新增模板</el-button>
        </div>
      </template>

      <el-alert type="info" :closable="false" style="margin-bottom: 16px">
        消息模板用于定义各类通知消息的内容格式。内置模板不可删除但可编辑内容，自定义模板可自由管理。模板存在即生效，删除后对应场景的通知将无法发送。
      </el-alert>

      <el-table :data="templates" v-loading="loading" stripe>
        <el-table-column prop="name" label="模板名称" min-width="140" />
        <el-table-column prop="scene" label="场景标识" width="180">
          <template #default="{ row }">
            <div>
              <code class="scene-code">{{ row.scene }}</code>
              <div v-if="sceneDescMap[row.scene]" class="scene-desc">{{ sceneDescMap[row.scene] }}</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="content" label="模板内容" min-width="300">
          <template #default="{ row }">
            <span class="content-preview">{{ row.content }}</span>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.isBuiltin ? '' : 'success'" size="small">
              {{ row.isBuiltin ? '内置' : '自定义' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="openEdit(row)">编辑</el-button>
            <el-popconfirm
              v-if="!row.isBuiltin"
              title="删除后对应场景的通知将无法发送，确定删除？"
              @confirm="handleDelete(row.id)"
            >
              <template #reference>
                <el-button type="danger" link size="small">删除</el-button>
              </template>
            </el-popconfirm>
            <el-tag v-else type="info" size="small" style="margin-left: 8px; cursor: default">不可删除</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 可用变量说明 -->
    <el-card style="margin-top: 20px">
      <template #header>
        <div class="card-header">
          <span>可用变量说明</span>
        </div>
      </template>
      <el-table :data="builtinVars" stripe size="small">
        <el-table-column prop="key" label="变量" width="180">
          <template #default="{ row }">
            <code class="var-code" v-text="wrapVar(row.key)"></code>
          </template>
        </el-table-column>
        <el-table-column prop="desc" label="说明" min-width="200" />
        <el-table-column prop="example" label="示例值" width="220" />
      </el-table>
    </el-card>

    <!-- 新增/编辑对话框 -->
    <el-dialog
      v-model="showDialog"
      :title="editingId ? '编辑消息模板' : '新增消息模板'"
      width="620px"
      destroy-on-close
    >
      <el-form :model="form" label-width="100px">
        <el-form-item label="模板名称" required>
          <el-input v-model="form.name" placeholder="如：用户注册通知" />
        </el-form-item>
        <el-form-item label="场景标识" required>
          <el-input
            v-model="form.scene"
            placeholder="如：user_register"
            :disabled="!!editingId"
          />
          <div class="form-tip" v-if="!editingId">
            唯一标识，创建后不可修改。建议使用英文小写+下划线，如 <code>order_notify</code>
          </div>
          <el-alert
            v-if="editingId && sceneDescMap[form.scene]"
            :title="'用途：' + sceneDescMap[form.scene]"
            type="info"
            :closable="false"
            show-icon
            style="margin-top: 6px"
          />
        </el-form-item>
        <el-form-item label="模板内容" required>
          <el-input
            v-model="form.content"
            type="textarea"
            :rows="5"
            placeholder="请输入消息模板内容，支持变量替换"
          />
        </el-form-item>
        <el-form-item label="插入变量">
          <div class="var-insert-area">
            <code
              v-for="v in allVars"
              :key="v.key"
              class="var-tag clickable"
              @click="insertVar(v.key)"
              v-text="wrapVar(v.key)"
            ></code>
          </div>
        </el-form-item>
        <el-form-item label="预览">
          <div class="preview-box">{{ previewContent }}</div>
        </el-form-item>

        <el-divider content-position="left">自定义变量（可选）</el-divider>
        <el-alert type="info" :closable="false" style="margin-bottom: 16px">
          除内置变量外，您可以为此模板定义额外的自定义变量。
        </el-alert>
        <div v-for="(v, idx) in form.customVars" :key="idx" class="custom-var-row">
          <el-input v-model="v.key" placeholder="变量名" style="width: 120px" />
          <el-input v-model="v.desc" placeholder="说明" style="width: 160px; margin: 0 8px" />
          <el-input v-model="v.example" placeholder="示例值" style="width: 140px; margin-right: 8px" />
          <el-button type="danger" circle size="small" :icon="Delete" @click="removeCustomVar(idx)" />
        </div>
        <el-button type="primary" link size="small" @click="addCustomVar" style="margin-top: 8px">
          + 添加自定义变量
        </el-button>
      </el-form>

      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" @click="handleSave" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { Delete } from "@element-plus/icons-vue";
import { securityApi } from "../../api";

const loading = ref(false);
const saving = ref(false);
const templates = ref<any[]>([]);
const showDialog = ref(false);
const editingId = ref<number | null>(null);

// 场景用途说明映射
const sceneDescMap: Record<string, string> = {
  verify_code: "用户自助找回密码时发送的验证码",
  password_reset: "用户自助重置密码时发送的验证码",
  password_reset_notify: "管理员重置密码后，将新密码通知给用户",
  account_created: "钉钉同步创建新用户后，将账号和初始密码通知给用户",
  security_alert: "安全告警（员工侧）通知内容",
  admin_alert: "安全告警（管理员侧）通知内容",
  test: "测试消息，用于验证通知渠道是否正常",
};

// 内置变量
const builtinVars = [
  { key: "username", desc: "用户名", example: "zhangsan" },
  { key: "nickname", desc: "用户昵称", example: "张三" },
  { key: "name", desc: "姓名（真实姓名）", example: "张三" },
  { key: "password", desc: "密码（仅密码重置/账号开通场景）", example: "Abc@1234" },
  { key: "department", desc: "所属部门", example: "技术部" },
  { key: "code", desc: "验证码", example: "283746" },
  { key: "time", desc: "当前时间", example: "2026-02-08 11:30:00" },
  { key: "ip", desc: "来源IP地址", example: "192.168.1.100" },
  { key: "app_name", desc: "系统名称", example: "统一身份认证平台" },
];

const wrapVar = (key: string) => `{{${key}}}`;

const form = reactive({
  name: "",
  scene: "",
  content: "",
  isActive: true,
  customVars: [] as { key: string; desc: string; example: string }[],
});

// 合并内置变量和自定义变量
const allVars = computed(() => {
  const custom = form.customVars.filter(v => v.key.trim());
  return [...builtinVars, ...custom];
});

const previewContent = computed(() => {
  let msg = form.content || "";
  const now = new Date().toLocaleString("zh-CN");
  // 内置变量替换
  msg = msg.replace(/\{\{username\}\}/g, "testuser");
  msg = msg.replace(/\{\{nickname\}\}/g, "测试用户");
  msg = msg.replace(/\{\{name\}\}/g, "张三");
  msg = msg.replace(/\{\{code\}\}/g, "283746");
  msg = msg.replace(/\{\{time\}\}/g, now);
  msg = msg.replace(/\{\{ip\}\}/g, "192.168.1.100");
  msg = msg.replace(/\{\{app_name\}\}/g, "统一身份认证平台");
  // 自定义变量替换
  for (const v of form.customVars) {
    if (v.key && v.example) {
      const re = new RegExp(`\\{\\{${v.key}\\}\\}`, "g");
      msg = msg.replace(re, v.example);
    }
  }
  return msg;
});

const insertVar = (key: string) => {
  form.content += `{{${key}}}`;
};

const addCustomVar = () => {
  form.customVars.push({ key: "", desc: "", example: "" });
};

const removeCustomVar = (idx: number) => {
  form.customVars.splice(idx, 1);
};

const loadTemplates = async () => {
  loading.value = true;
  try {
    const res = await securityApi.getTemplates();
    if (res.data.success) {
      templates.value = res.data.data || [];
    }
  } finally {
    loading.value = false;
  }
};

const openAdd = () => {
  editingId.value = null;
  form.name = "";
  form.scene = "";
  form.content = "";
  form.isActive = true;
  form.customVars = [];
  showDialog.value = true;
};

const openEdit = (row: any) => {
  editingId.value = row.id;
  form.name = row.name;
  form.scene = row.scene;
  form.content = row.content;
  form.isActive = row.isActive;

  // 解析 variables JSON，提取自定义变量（排除内置的）
  form.customVars = [];
  if (row.variables) {
    try {
      const vars = typeof row.variables === "string" ? JSON.parse(row.variables) : row.variables;
      const builtinKeys = new Set(builtinVars.map(v => v.key));
      form.customVars = vars.filter((v: any) => !builtinKeys.has(v.key));
    } catch { /* ignore */ }
  }

  showDialog.value = true;
};

const handleSave = async () => {
  if (!form.name || !form.scene || !form.content) {
    ElMessage.warning("请填写完整信息");
    return;
  }

  // 组装 variables: 内置 + 自定义
  const customFiltered = form.customVars.filter(v => v.key.trim());
  const allVariables = [...builtinVars, ...customFiltered];
  const variablesJSON = JSON.stringify(allVariables);

  saving.value = true;
  try {
    if (editingId.value) {
      await securityApi.updateTemplate(editingId.value, {
        name: form.name,
        content: form.content,
        variables: variablesJSON,
        isActive: form.isActive,
      });
    } else {
      await securityApi.createTemplate({
        name: form.name,
        scene: form.scene,
        content: form.content,
        variables: variablesJSON,
      });
    }
    ElMessage.success("保存成功");
    showDialog.value = false;
    loadTemplates();
  } catch (e: any) {
    const msg = e?.response?.data?.message || "保存失败";
    ElMessage.error(msg);
  } finally {
    saving.value = false;
  }
};

const handleDelete = async (id: number) => {
  try {
    await securityApi.deleteTemplate(id);
    ElMessage.success("删除成功");
    loadTemplates();
  } catch (e: any) {
    const msg = e?.response?.data?.message || "删除失败";
    ElMessage.error(msg);
  }
};

onMounted(() => {
  loadTemplates();
});
</script>

<style scoped>
.template-page {
  max-width: 1100px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.scene-code {
  background: var(--color-fill-secondary);
  color: var(--color-text-secondary);
  padding: 2px 8px;
  border-radius: 3px;
  font-size: 12px;
  font-family: monospace;
}
.scene-desc {
  color: var(--color-text-placeholder);
  font-size: 12px;
  margin-top: 4px;
  line-height: 1.4;
}

.content-preview {
  color: var(--color-text-secondary);
  font-size: 13px;
  line-height: 1.5;
  word-break: break-all;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.var-code {
  background: var(--color-fill-secondary);
  color: var(--color-text-secondary);
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 13px;
  font-family: monospace;
}

.var-tag {
  display: inline-block;
  background: #ecf5ff;
  color: var(--color-primary);
  padding: 3px 8px;
  border-radius: 3px;
  margin: 3px 4px;
  font-size: 12px;
  font-family: monospace;
}

.var-tag.clickable {
  cursor: pointer;
  transition: background 0.2s;
}

.var-tag.clickable:hover {
  background: #d9ecff;
}

.var-insert-area {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.preview-box {
  background: var(--color-fill-secondary);
  border: 1px solid var(--color-border-secondary);
  border-radius: 4px;
  padding: 10px 14px;
  font-size: 13px;
  color: var(--color-text-primary);
  line-height: 1.6;
  min-height: 40px;
  word-break: break-all;
  width: 100%;
}

.form-tip {
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-top: 4px;
}

.form-tip code {
  background: var(--color-fill-secondary);
  padding: 1px 4px;
  border-radius: 2px;
}

.custom-var-row {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
}
</style>
