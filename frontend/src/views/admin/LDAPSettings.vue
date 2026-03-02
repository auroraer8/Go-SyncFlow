<template>
  <div class="ldap-page">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2>LDAP 服务</h2>
        <p class="header-desc">为第三方系统提供用户和角色信息的 LDAP 目录服务</p>
      </div>
      <div class="header-right">
        <span :class="['status-dot', status.running ? 'active' : '']"></span>
        <span class="status-text">{{ status.running ? '运行中' : '已停止' }}</span>
      </div>
    </div>

    <!-- 服务开关 & 端口 -->
    <div class="section-card">
      <div class="switch-row">
        <div class="switch-info">
          <span class="switch-label">启用 LDAP 服务</span>
          <span class="switch-desc">开启后，第三方系统可通过 LDAP 协议接入</span>
        </div>
        <el-switch v-model="form.enabled" />
      </div>

      <transition name="fade">
        <div v-if="form.enabled" class="sub-fields">
          <div class="field-row">
            <label>端口</label>
            <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" size="default" />
          </div>
          <div class="field-row">
            <div class="switch-info" style="flex:1">
              <span class="switch-label" style="font-size:13px">启用 LDAPS（TLS 加密）</span>
            </div>
            <el-switch v-model="form.useTLS" size="small" />
          </div>
          <template v-if="form.useTLS">
            <div class="field-row">
              <label>LDAPS 端口</label>
              <el-input-number v-model="form.tlsPort" :min="1" :max="65535" controls-position="right" size="default" />
            </div>
            <div class="field-row">
              <label>TLS 证书</label>
              <el-input v-model="form.tlsCertFile" placeholder="留空复用 HTTPS 证书" size="default" />
            </div>
            <div class="field-row">
              <label>TLS 私钥</label>
              <el-input v-model="form.tlsKeyFile" placeholder="留空复用 HTTPS 私钥" size="default" />
            </div>
          </template>
        </div>
      </transition>
    </div>

    <!-- 域设置 -->
    <div class="section-card" v-if="form.enabled">
      <div class="section-title">域设置</div>
      <div class="form-grid">
        <div class="form-field">
          <label>域名</label>
          <el-input v-model="form.domain" placeholder="example.com" @change="onDomainChange" />
          <span class="field-hint">输入域名后自动生成 Base DN</span>
        </div>
        <div class="form-field">
          <label>Base DN</label>
          <el-input v-model="form.baseDN" placeholder="dc=example,dc=com" />
        </div>
      </div>
    </div>

    <!-- 服务账号 -->
    <div class="section-card" v-if="form.enabled">
      <div class="section-title">服务账号</div>
      <p class="section-desc">Manager 拥有完整权限，Readonly 仅可查询。建议第三方系统使用 Readonly 账号。</p>

      <div class="accounts-grid">
        <!-- Manager -->
        <div class="account-card">
          <div class="account-header">
            <div class="account-badge manager">Manager</div>
            <span class="account-role">管理员 · 完整权限</span>
          </div>
          <div class="account-body">
            <div class="form-field">
              <label>Bind DN</label>
              <el-input v-model="form.managerDN" placeholder="cn=Manager,dc=example,dc=com" />
            </div>
            <div class="form-field">
              <label>密码</label>
              <el-input v-model="form.managerPassword" type="password" show-password placeholder="设置密码" />
            </div>
          </div>
        </div>

        <!-- Readonly -->
        <div class="account-card">
          <div class="account-header">
            <div class="account-badge readonly">Readonly</div>
            <span class="account-role">只读 · 仅查询权限</span>
          </div>
          <div class="account-body">
            <div class="form-field">
              <label>Bind DN</label>
              <el-input v-model="form.readonlyDN" placeholder="cn=readonly,dc=example,dc=com" />
            </div>
            <div class="form-field">
              <label>密码</label>
              <el-input v-model="form.readonlyPassword" type="password" show-password placeholder="设置密码" />
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 底部操作 -->
    <div class="actions-bar" v-if="form.enabled">
      <el-button type="primary" @click="saveConfig" :loading="saving" size="large">
        保存配置
      </el-button>
      <el-button @click="testConnection" :loading="testing" :disabled="!status.running" size="large" plain>
        测试连接
      </el-button>
    </div>
    <div class="actions-bar" v-else>
      <el-button type="primary" @click="saveConfig" :loading="saving" size="large">
        保存配置
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { ldapApi } from "../../api";

const saving = ref(false);
const testing = ref(false);

const form = reactive({
  enabled: false,
  port: 389,
  useTLS: false,
  tlsPort: 636,
  tlsCertFile: "",
  tlsKeyFile: "",
  domain: "",
  baseDN: "",
  managerDN: "",
  managerPassword: "",
  readonlyDN: "",
  readonlyPassword: "",
  sambaEnabled: true,
  sambaSID: ""
});

const status = reactive({
  running: false,
  enabled: false
});

const onDomainChange = () => {
  if (form.domain) {
    const parts = form.domain.split(".");
    form.baseDN = parts.map((p: string) => "dc=" + p).join(",");
    form.managerDN = "cn=Manager," + form.baseDN;
    form.readonlyDN = "cn=readonly," + form.baseDN;
  }
};

const loadConfig = async () => {
  try {
    const res = await ldapApi.getConfig();
    if (res.data.success && res.data.data) {
      const cfg = res.data.data;
      Object.assign(form, {
        enabled: cfg.enabled || false,
        port: cfg.port || 389,
        useTLS: cfg.useTLS || false,
        tlsPort: cfg.tlsPort || 636,
        tlsCertFile: cfg.tlsCertFile || "",
        tlsKeyFile: cfg.tlsKeyFile || "",
        domain: cfg.domain || "",
        baseDN: cfg.baseDN || "",
        managerDN: cfg.managerDN || "",
        managerPassword: "",
        readonlyDN: cfg.readonlyDN || "",
        readonlyPassword: "",
        sambaEnabled: cfg.sambaEnabled !== false,
        sambaSID: cfg.sambaSID || ""
      });
    }
  } catch (e) {
    // ignore
  }
};

const loadStatus = async () => {
  try {
    const res = await ldapApi.status();
    if (res.data.success && res.data.data) {
      status.running = res.data.data.running;
      status.enabled = res.data.data.enabled;
    }
  } catch (e) {
    // ignore
  }
};

const saveConfig = async () => {
  if (form.enabled && !form.baseDN) {
    ElMessage.warning("请设置域名和 Base DN");
    return;
  }
  if (form.enabled && !form.managerDN) {
    ElMessage.warning("请设置 Manager Bind DN");
    return;
  }

  saving.value = true;
  try {
    const res = await ldapApi.updateConfig(form);
    if (res.data.success) {
      ElMessage.success("配置已保存");
      await loadStatus();
    }
  } catch (e) {
    // handled by interceptor
  } finally {
    saving.value = false;
  }
};

const testConnection = async () => {
  testing.value = true;
  try {
    const res = await ldapApi.test();
    if (res.data.success) {
      const data = res.data.data;
      const mgr = data.results?.manager;
      const ro = data.results?.readonly;
      if (mgr === '连接成功' && (ro === '连接成功' || ro === '未配置密码')) {
        ElMessage.success(`测试通过 · Manager: ${mgr} · Readonly: ${ro} · 条目: ${data.entries}`);
      } else {
        ElMessage.warning(`Manager: ${mgr} · Readonly: ${ro}`);
      }
    }
  } catch (e) {
    // handled by interceptor
  } finally {
    testing.value = false;
  }
};

onMounted(() => {
  loadConfig();
  loadStatus();
});
</script>

<style scoped>
/* ============================================
   Glassmorphism LDAP Settings Page
   ============================================ */
.ldap-page {
  max-width: 820px;
  margin: 0 auto;
  padding: 0 0 40px;
  
}


/* 顶部 */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--spacing-2xl);
  padding: var(--spacing-xl);
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  box-shadow: var(--card-shadow);
}
.page-header h2 {
  margin: 0 0 6px;
  font-size: 20px;
  font-weight: 700;
  color: var(--color-primary);
}
.header-desc {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-tertiary);
}
.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  background: var(--color-fill-secondary);
  border-radius: var(--radius-full);
  border: 1px solid var(--color-border);
}
.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--color-text-quaternary);
  flex-shrink: 0;
}
.status-dot.active {
  background: var(--color-success);
  box-shadow: 0 0 0 4px rgba(16, 185, 129, 0.2);
  animation: breathing 2s ease-in-out infinite;
}
@keyframes breathing {
  0%, 100% { box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.4); }
  50% { box-shadow: 0 0 0 8px rgba(16, 185, 129, 0); }
}
.status-text {
  font-size: 13px;
  color: var(--color-text-secondary);
  font-weight: 500;
}

/* 区块卡片 */
.section-card {
  background: var(--card-bg);
  border: 1px solid var(--card-border);
  border-radius: var(--card-radius);
  padding: var(--spacing-2xl);
  margin-bottom: var(--spacing-xl);
  box-shadow: var(--card-shadow);
  transition: box-shadow var(--transition-fast);
}
.section-card:hover {
  box-shadow: var(--card-shadow-hover);
}
.section-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: 6px;
}
.section-desc {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 4px 0 var(--spacing-xl);
  line-height: 1.6;
}

/* 开关行 */
.switch-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) 0;
}
.switch-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.switch-label {
  font-size: 15px;
  font-weight: 500;
  color: var(--color-text-primary);
}
.switch-desc {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

/* 子字段区 */
.sub-fields {
  margin-top: var(--spacing-xl);
  padding-top: var(--spacing-xl);
  border-top: 1px solid var(--color-border);
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.field-row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 10px 14px;
  background: var(--color-fill-secondary);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
}
.field-row > label {
  min-width: 85px;
  font-size: 13px;
  color: var(--color-text-secondary);
  text-align: right;
  flex-shrink: 0;
  font-weight: 500;
}

/* 表单网格 */
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
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

/* 账号卡片 */
.accounts-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--spacing-lg);
}
.account-card {
  background: var(--color-fill-secondary);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
  transition: box-shadow var(--transition-fast);
}
.account-card:hover {
  border-color: var(--color-primary-border);
  box-shadow: var(--card-shadow-hover);
}
.account-header {
  padding: 14px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  border-bottom: 1px solid var(--color-border);
  background: var(--card-bg);
}
.account-badge {
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.3px;
}
.account-badge.manager {
  background: var(--color-error);
  color: white;
}
.account-badge.readonly {
  background: var(--color-success);
  color: white;
}
.account-role {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.account-body {
  padding: var(--spacing-lg);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* 底部操作栏 */
.actions-bar {
  display: flex;
  gap: 14px;
  padding-top: var(--spacing-md);
}

/* 过渡动画 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s var(--ease-out);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* 覆盖 Element Plus 默认样式 */
:deep(.el-input__wrapper) {
  border-radius: var(--radius-md);
}
:deep(.el-input-number) {
  width: 150px;
}
:deep(.el-button--large) {
  border-radius: var(--radius-md);
  padding: 12px 28px;
  font-weight: 600;
}
</style>
