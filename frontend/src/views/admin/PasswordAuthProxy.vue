<template>
  <div class="proxy-page">
    <div class="page-header">
      <div class="header-left">
        <h2>密码代理认证</h2>
        <p class="header-desc">当本地用户密码为空或验证失败时，通过外部认证服务验证密码并学习</p>
      </div>
    </div>

    <div class="section-card">
      <div class="switch-row">
        <div class="switch-info">
          <span class="switch-label">启用密码代理认证</span>
          <span class="switch-desc">启用后，本地密码验证失败时将尝试通过外部连接器验证</span>
        </div>
        <el-switch v-model="form.enabled" />
      </div>

      <transition name="fade">
        <div v-if="form.enabled" class="sub-fields">
          <div class="form-field">
            <label>认证连接器</label>
            <el-select v-model="form.connectorId" placeholder="选择认证连接器" class="full-width">
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
            <span class="field-hint">选择用于代理认证的上游连接器（支持 LDAP、数据库、RADIUS、HTTP API）</span>
          </div>

          <div class="form-field">
            <label>用户匹配字段</label>
            <el-select v-model="form.matchField" class="full-width">
              <el-option label="用户名" value="username" />
              <el-option label="邮箱" value="email" />
              <el-option label="手机号" value="phone" />
            </el-select>
            <span class="field-hint">用于在远程系统中查找用户的本地字段</span>
          </div>

          <div class="switch-row" style="margin-top: 16px;">
            <div class="switch-info">
              <span class="switch-label">学习密码</span>
              <span class="switch-desc">认证成功后将密码存储到本地用户</span>
            </div>
            <el-switch v-model="form.learnPassword" />
          </div>

          <div class="switch-row" v-if="form.learnPassword">
            <div class="switch-info">
              <span class="switch-label">同步 Samba NT Hash</span>
              <span class="switch-desc">同时更新用户的 Samba NT 密码哈希（用于 LDAP 服务）</span>
            </div>
            <el-switch v-model="form.learnSambaNT" />
          </div>

          <div class="switch-row">
            <div class="switch-info">
              <span class="switch-label">本地优先</span>
              <span class="switch-desc">先验证本地密码，失败后再尝试代理认证</span>
            </div>
            <el-switch v-model="form.fallbackToLocal" />
          </div>
        </div>
      </transition>
    </div>

    <div class="section-card" v-if="form.enabled && form.connectorId">
      <div class="section-title">测试认证</div>
      <p class="section-desc">输入用户名和密码测试代理认证是否正常工作</p>
      <div class="test-form">
        <el-input v-model="testUsername" placeholder="用户名" class="test-input" />
        <el-input v-model="testPassword" type="password" show-password placeholder="密码" class="test-input" />
        <el-button type="primary" @click="testAuth" :loading="testing" :disabled="!testUsername || !testPassword">
          测试
        </el-button>
      </div>
      <div v-if="testResult" :class="['test-result', testResult.success ? 'success' : 'error']">
        <el-icon v-if="testResult.success"><CircleCheckFilled /></el-icon>
        <el-icon v-else><CircleCloseFilled /></el-icon>
        <span>{{ testResult.message }}</span>
      </div>
    </div>

    <div class="actions-bar">
      <el-button type="primary" @click="saveConfig" :loading="saving" size="large">
        保存配置
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from "vue";
import { ElMessage } from "element-plus";
import { CircleCheckFilled, CircleCloseFilled } from "@element-plus/icons-vue";
import { passwordProxyApi } from "../../api";

interface ConnectorItem {
  id: number;
  name: string;
  type: string;
  typeName: string;
  category: string;
}

const saving = ref(false);
const testing = ref(false);
const connectors = ref<ConnectorItem[]>([]);

const form = reactive({
  enabled: false,
  connectorId: 0,
  matchField: "username",
  learnPassword: true,
  learnSambaNT: true,
  fallbackToLocal: true
});

const testUsername = ref("");
const testPassword = ref("");
const testResult = ref<{ success: boolean; message: string } | null>(null);

const connectorGroups = computed(() => {
  const groups: Record<string, { label: string; items: ConnectorItem[] }> = {};
  const categoryLabels: Record<string, string> = {
    ldap: "LDAP 目录服务",
    database: "数据库",
    radius: "RADIUS",
    http: "HTTP API"
  };

  for (const c of connectors.value) {
    const cat = c.category || "other";
    if (!groups[cat]) {
      groups[cat] = { label: categoryLabels[cat] || "其他", items: [] };
    }
    groups[cat].items.push(c);
  }
  return groups;
});

const loadConfig = async () => {
  try {
    const res = await passwordProxyApi.getConfig();
    if (res.data.success && res.data.data) {
      const cfg = res.data.data.config || {};
      connectors.value = res.data.data.connectors || [];

      form.enabled = cfg.enabled || false;
      form.connectorId = cfg.connectorId || 0;
      form.matchField = cfg.matchField || "username";
      form.learnPassword = cfg.learnPassword !== false;
      form.learnSambaNT = cfg.learnSambaNT !== false;
      form.fallbackToLocal = cfg.fallbackToLocal !== false;
    }
  } catch (e) {
    // ignore
  }
};

const saveConfig = async () => {
  if (form.enabled && !form.connectorId) {
    ElMessage.warning("请选择认证连接器");
    return;
  }

  saving.value = true;
  try {
    const res = await passwordProxyApi.updateConfig(form);
    if (res.data.success) {
      ElMessage.success("配置已保存");
    }
  } catch (e) {
    // handled by interceptor
  } finally {
    saving.value = false;
  }
};

const testAuth = async () => {
  testResult.value = null;
  testing.value = true;
  try {
    const res = await passwordProxyApi.test({
      username: testUsername.value,
      password: testPassword.value
    });
    if (res.data.success && res.data.data) {
      testResult.value = {
        success: res.data.data.success,
        message: res.data.data.message
      };
    }
  } catch (e) {
    testResult.value = { success: false, message: "请求失败" };
  } finally {
    testing.value = false;
  }
};

onMounted(() => {
  loadConfig();
});
</script>

<style scoped>
/* ============================================
   Corporate Password Auth Proxy - 企业风格
   ============================================ */
.proxy-page {
  max-width: 680px;
  margin: 0 auto;
  padding: 0 0 40px;
  
}


.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 28px;
}
.page-header h2 {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  letter-spacing: -0.3px;
}
.header-desc {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-tertiary);
}

.section-card {
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  padding: 24px;
  margin-bottom: 16px;
  box-shadow: var(--card-shadow);
}
.section-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: 4px;
}
.section-desc {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 2px 0 20px;
  line-height: 1.5;
}

.switch-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.switch-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.switch-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
}
.switch-desc {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.sub-fields {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid var(--color-border-secondary);
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.form-field > label {
  font-size: 13px;
  color: var(--color-text-secondary);
  font-weight: 500;
}
.field-hint {
  font-size: 11px;
  color: var(--color-text-quaternary);
}

.full-width {
  width: 100%;
}

.connector-type-tag {
  color: var(--color-text-tertiary);
  font-size: 11px;
  margin-left: 8px;
  padding: 2px 6px;
  background: var(--color-fill-secondary);
  border-radius: 4px;
}

.test-form {
  display: flex;
  gap: 12px;
  align-items: center;
}
.test-input {
  flex: 1;
}

.test-result {
  margin-top: 16px;
  padding: 12px 16px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}
.test-result.success {
  background: var(--color-success-bg);
  color: var(--color-success);
}
.test-result.error {
  background: var(--color-error-bg);
  color: var(--color-error);
}

.actions-bar {
  display: flex;
  gap: 12px;
  padding-top: 8px;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s, transform 0.2s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

:deep(.el-input__wrapper) {
  border-radius: var(--radius-md);
}
:deep(.el-button--large) {
  border-radius: var(--radius-md);
  padding: 12px 28px;
  font-weight: 500;
}
</style>
