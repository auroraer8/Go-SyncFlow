<template>
  <div class="profile-page">
    <!-- 顶部用户信息条 -->
    <div class="user-bar">
      <div class="user-bar-left">
        <div class="avatar" :style="{ background: avatarColor }">{{ avatarText }}</div>
        <div class="user-brief">
          <span class="user-name">{{ profile.nickname || profile.username }}</span>
          <span class="user-detail">
            {{ profile.username }}
            <template v-if="profile.phone"> · {{ profile.phone }}</template>
            <template v-if="profile.email"> · {{ profile.email }}</template>
          </span>
        </div>
      </div>
      <div class="user-bar-right">
        <span class="pwd-time" v-if="profile.passwordChangedAt">
          上次修改密码：{{ formatTime(profile.passwordChangedAt) }}
        </span>
        <span class="pwd-time never" v-else>从未修改过密码</span>
        <button class="logout-btn" @click="handleLogout">退出登录</button>
      </div>
    </div>

    <!-- 密码修改主卡片 -->
    <div class="password-card">
      <div class="card-header">
        <div class="card-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="24" height="24">
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
            <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
          </svg>
        </div>
        <div>
          <div class="card-title">修改登录密码</div>
          <div class="card-desc">定期更换密码有助于保护账户安全</div>
        </div>
      </div>

      <div class="card-body">
        <!-- 验证方式选择 -->
        <div class="section">
          <div class="section-label">验证身份</div>
          <div class="method-options">
            <button
              v-for="m in availableMethods"
              :key="m.key"
              :class="['method-btn', { active: passwordForm.method === m.key }]"
              @click="switchMethod(m.key)"
            >
              <span class="method-icon" v-if="m.key === 'password'">
                <svg viewBox="0 0 20 20" fill="currentColor" width="16" height="16"><path fill-rule="evenodd" d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z" clip-rule="evenodd"/></svg>
              </span>
              <span class="method-icon" v-else-if="m.key === 'dingtalk'">
                <svg viewBox="0 0 20 20" fill="currentColor" width="16" height="16"><path d="M2 5a2 2 0 012-2h7a2 2 0 012 2v4a2 2 0 01-2 2H9l-3 3v-3H4a2 2 0 01-2-2V5z"/><path d="M15 7v2a4 4 0 01-4 4H9.828l-1.766 1.767c.28.149.599.233.938.233h2l3 3v-3h2a2 2 0 002-2V9a2 2 0 00-2-2h-1z"/></svg>
              </span>
              <span class="method-icon" v-else>
                <svg viewBox="0 0 20 20" fill="currentColor" width="16" height="16"><path d="M2 3a1 1 0 011-1h2.153a1 1 0 01.986.836l.74 4.435a1 1 0 01-.54 1.06l-1.548.773a11.037 11.037 0 006.105 6.105l.774-1.548a1 1 0 011.059-.54l4.435.74a1 1 0 01.836.986V17a1 1 0 01-1 1h-2C7.82 18 2 12.18 2 5V3z"/></svg>
              </span>
              {{ m.name }}
            </button>
          </div>
        </div>

        <div class="form-divider"></div>

        <!-- 表单主体 -->
        <div class="form-body">
          <div class="form-grid">
            <!-- 原密码 -->
            <div class="form-field" v-if="passwordForm.method === 'password'">
              <label class="field-label">原密码</label>
              <el-input
                v-model="passwordForm.oldPassword"
                type="password"
                show-password
                placeholder="请输入当前密码"
                size="large"
              />
            </div>

            <!-- 钉钉验证码 -->
            <div class="form-field" v-if="passwordForm.method === 'dingtalk'">
              <label class="field-label">钉钉验证码</label>
              <div class="code-row">
                <el-input v-model="passwordForm.code" placeholder="请输入6位验证码" size="large" />
                <el-button
                  size="large"
                  class="code-btn"
                  @click="sendCode('dingtalk')"
                  :disabled="dingtalkCooldown > 0"
                  :loading="sendingCode"
                >{{ dingtalkCooldown > 0 ? `${dingtalkCooldown}s` : '获取验证码' }}</el-button>
              </div>
              <span class="field-hint">验证码将通过钉钉工作通知发送</span>
            </div>

            <!-- 短信验证码 -->
            <div class="form-field" v-if="passwordForm.method === 'sms'">
              <label class="field-label">短信验证码</label>
              <div class="code-row">
                <el-input v-model="passwordForm.code" placeholder="请输入6位验证码" size="large" />
                <el-button
                  size="large"
                  class="code-btn"
                  @click="sendCode('sms')"
                  :disabled="smsCooldown > 0"
                  :loading="sendingCode"
                >{{ smsCooldown > 0 ? `${smsCooldown}s` : '获取验证码' }}</el-button>
              </div>
              <span class="field-hint">验证码将发送至 {{ maskedPhone }}</span>
            </div>

            <!-- 新密码 -->
            <div class="form-field">
              <label class="field-label">新密码</label>
              <el-input
                v-model="passwordForm.newPassword"
                type="password"
                show-password
                placeholder="至少8位，需包含大小写字母和数字"
                size="large"
              />
            </div>

            <!-- 确认新密码 -->
            <div class="form-field">
              <label class="field-label">确认新密码</label>
              <el-input
                v-model="passwordForm.confirmPassword"
                type="password"
                show-password
                placeholder="再次输入新密码"
                size="large"
              />
            </div>
          </div>

          <el-button
            type="primary"
            size="large"
            class="submit-btn"
            @click="submitPasswordChange"
            :loading="changingPassword"
          >确认修改</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { profileApi, authApi } from "../../api";
import { useUserStore } from "../../store/user";

const router = useRouter();
const userStore = useUserStore();

const profile = reactive<any>({
  id: 0, username: "", nickname: "", phone: "", email: "",
  avatar: "", source: "local", dingtalkUid: "",
  departmentName: "", jobTitle: "", roles: [],
  createdAt: "", lastLoginAt: "", lastLoginIp: "", passwordChangedAt: ""
});

const avatarColors = ['#6366f1','#8b5cf6','#ec4899','#14b8a6','#f97316','#409eff','#67c23a','#e6a23c','#f56c6c','#909399'];
const avatarColor = computed(() => {
  const n = profile.nickname || profile.username || '';
  return n ? avatarColors[n.charCodeAt(0) % avatarColors.length] : avatarColors[0];
});
const avatarText = computed(() => (profile.nickname || profile.username || '?').charAt(0).toUpperCase());

const maskedPhone = computed(() => {
  const p = profile.phone || '';
  return p.length >= 7 ? p.substring(0,3) + '****' + p.substring(p.length-4) : p;
});

const formatTime = (t: string) => {
  if (!t) return '';
  return new Date(t).toLocaleString('zh-CN', { year:'numeric', month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit' });
};

const loadProfile = async () => {
  try {
    const res = await profileApi.get();
    if (res.data.success && res.data.data) Object.assign(profile, res.data.data);
  } catch (e) {}
};

// ===== 修改密码 =====
const availableMethods = computed(() => {
  const m: {key:string; name:string}[] = [{ key: 'password', name: '原密码验证' }];
  const allowed = profile.allowedVerifyMethods as string[] | undefined;
  // 如果配置了消息策略，仅显示策略允许的方式；未配置则显示所有可用方式
  if (profile.dingtalkUid && (!allowed || allowed.length === 0 || allowed.includes('dingtalk'))) {
    m.push({ key: 'dingtalk', name: '钉钉验证码' });
  }
  if (profile.phone && (!allowed || allowed.length === 0 || allowed.includes('sms'))) {
    m.push({ key: 'sms', name: '短信验证码' });
  }
  return m;
});

const passwordForm = reactive({ method: 'password', oldPassword: '', code: '', newPassword: '', confirmPassword: '' });
const changingPassword = ref(false);
const sendingCode = ref(false);
const dingtalkCooldown = ref(0);
const smsCooldown = ref(0);
let dingtalkTimer: ReturnType<typeof setInterval> | null = null;
let smsTimer: ReturnType<typeof setInterval> | null = null;

const switchMethod = (key: string) => {
  passwordForm.method = key;
  passwordForm.oldPassword = '';
  passwordForm.code = '';
};

const sendCode = async (method: string) => {
  sendingCode.value = true;
  try {
    const res = await profileApi.sendVerifyCode(method);
    if (res.data.success) {
      ElMessage.success(res.data.data?.message || '验证码已发送');
      if (method === 'dingtalk') {
        dingtalkCooldown.value = 60;
        if (dingtalkTimer) clearInterval(dingtalkTimer);
        dingtalkTimer = setInterval(() => { if (--dingtalkCooldown.value <= 0 && dingtalkTimer) { clearInterval(dingtalkTimer); dingtalkTimer = null; } }, 1000);
      } else {
        smsCooldown.value = 60;
        if (smsTimer) clearInterval(smsTimer);
        smsTimer = setInterval(() => { if (--smsCooldown.value <= 0 && smsTimer) { clearInterval(smsTimer); smsTimer = null; } }, 1000);
      }
    }
  } catch (e) {} finally { sendingCode.value = false; }
};

// 前端密码策略校验（因密码哈希后后端无法校验原文）
const validatePasswordPolicy = (pwd: string): string | null => {
  if (pwd.length < 8) return '密码长度至少8位';
  if (pwd.length > 128) return '密码长度不能超过128位';
  if (!/[A-Z]/.test(pwd)) return '密码必须包含大写字母';
  if (!/[a-z]/.test(pwd)) return '密码必须包含小写字母';
  if (!/[0-9]/.test(pwd)) return '密码必须包含数字';
  return null;
};

const submitPasswordChange = async () => {
  if (!passwordForm.newPassword) return ElMessage.warning('请输入新密码');
  const policyError = validatePasswordPolicy(passwordForm.newPassword);
  if (policyError) return ElMessage.warning(policyError);
  if (passwordForm.newPassword !== passwordForm.confirmPassword) return ElMessage.warning('两次输入的密码不一致');
  if (passwordForm.method === 'password' && !passwordForm.oldPassword) return ElMessage.warning('请输入原密码');
  if (passwordForm.method !== 'password' && !passwordForm.code) return ElMessage.warning('请输入验证码');

  changingPassword.value = true;
  try {
    const res = await profileApi.changePassword({
      method: passwordForm.method,
      oldPassword: passwordForm.method === 'password' ? passwordForm.oldPassword : undefined,
      code: passwordForm.method !== 'password' ? passwordForm.code : undefined,
      newPassword: passwordForm.newPassword
    });
    if (res.data.success) {
      ElMessage.success('密码修改成功，即将返回登录页...');
      setTimeout(async () => {
        await userStore.logout();
        router.push('/login');
      }, 1500);
    }
  } catch (e) {} finally { changingPassword.value = false; }
};

const handleLogout = async () => {
  await userStore.logout();
  router.push('/login');
};

onMounted(() => loadProfile());
</script>

<style scoped>
/* ============================================
   Corporate Profile Page - 企业风格
   ============================================ */
.profile-page {
  min-height: 100%;
  padding: 40px;
  background: var(--page-bg);
  display: flex;
  flex-direction: column;
  align-items: center;
  
}


/* 顶部用户条 */
.user-bar {
  width: 100%;
  max-width: 720px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  padding: 20px 28px;
  background: var(--card-bg);
  border-radius: var(--card-radius);
  box-shadow: var(--card-shadow);
  border: 1px solid var(--card-border);
}

.user-bar-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.avatar {
  width: 46px;
  height: 46px;
  border-radius: var(--radius-xl);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  font-weight: 700;
  color: var(--color-bg-container);
  flex-shrink: 0;
}

.user-brief {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.user-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
  line-height: 1.3;
}

.user-detail {
  font-size: 13px;
  color: #a0a4ad;
}

.pwd-time {
  font-size: 13px;
  color: var(--color-text-tertiary);
  white-space: nowrap;
}

.pwd-time.never {
  color: #f5a623;
}

.user-bar-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.logout-btn {
  padding: 6px 16px;
  border: 1px solid var(--color-border-secondary);
  border-radius: var(--radius-lg);
  background: var(--color-bg-container);
  font-size: 13px;
  color: var(--color-text-tertiary);
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.logout-btn:hover {
  color: var(--color-error);
  border-color: var(--color-error);
  background: #fef0f0;
}

/* 密码修改主卡片 */
.password-card {
  width: 100%;
  max-width: 720px;
  background: var(--card-bg);
  border-radius: var(--card-radius);
  box-shadow: var(--card-shadow);
  border: 1px solid var(--card-border);
  overflow: hidden;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 28px 32px 0;
}

.card-icon {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-md);
  background: var(--color-primary-bg);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-primary);
  flex-shrink: 0;
}

.card-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  line-height: 1.3;
}

.card-desc {
  font-size: 13px;
  color: #a0a4ad;
  margin-top: 2px;
}

.card-body {
  padding: 28px 32px 36px;
}

/* 验证方式 */
.section {
  margin-bottom: 0;
}

.section-label {
  font-size: 14px;
  font-weight: 500;
  color: #4e5969;
  margin-bottom: 12px;
}

.method-options {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.method-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 10px 20px;
  border: 1.5px solid var(--color-border-secondary);
  border-radius: 10px;
  background: var(--color-bg-container);
  font-size: 14px;
  color: #4e5969;
  cursor: pointer;
  transition: all 0.2s ease;
}

.method-btn:hover {
  border-color: #c9cdd4;
  background: var(--color-fill-secondary);
}

.method-btn.active {
  border-color: #3b82f6;
  color: #3b82f6;
  background: #eff6ff;
  font-weight: 500;
}

.method-icon {
  display: flex;
  align-items: center;
}

.form-divider {
  height: 1px;
  background: var(--color-border-secondary);
  margin: 24px 0 28px;
}

/* 表单区域 */
.form-body {
  max-width: 480px;
}

.form-grid {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.field-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.field-hint {
  font-size: 12px;
  color: #c0c4cc;
  margin-top: -2px;
}

.code-row {
  display: flex;
  gap: 10px;
}

.code-row .el-input {
  flex: 1;
}

.code-btn {
  min-width: 110px;
  flex-shrink: 0;
}

.submit-btn {
  margin-top: 32px;
  width: 100%;
  height: 48px;
  font-size: 16px;
  font-weight: 500;
  border-radius: var(--radius-md);
  letter-spacing: 1px;
}

/* Element Plus 覆盖 */
:deep(.el-input--large .el-input__wrapper) {
  border-radius: var(--radius-md);
  padding: 4px 14px;
  box-shadow: 0 0 0 1px var(--color-border) inset;
  transition: box-shadow var(--transition-fast);
}

:deep(.el-input--large .el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px var(--color-primary-border) inset;
}

:deep(.el-input--large .el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1.5px var(--color-primary) inset;
}

@media (max-width: 768px) {
  .profile-page {
    padding: 16px;
  }

  .user-bar {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
    padding: 16px 20px;
  }

  .card-header {
    padding: 20px 20px 0;
  }

  .card-body {
    padding: 20px 20px 28px;
  }

  .method-options {
    flex-direction: column;
  }

  .method-btn {
    justify-content: center;
  }
}
</style>
