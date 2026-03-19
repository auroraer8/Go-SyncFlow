<template>
  <div class="login-container" :class="loginThemeClass">
    <!-- 顶部导航条 -->
    <div class="top-bar">
      <div class="brand">
        <svg class="brand-logo" viewBox="0 0 32 32" fill="none">
          <circle cx="16" cy="16" r="14" fill="#3b82f6"/>
          <path d="M16 6L26 12V20L16 26L6 20V12L16 6Z" fill="white" fill-opacity="0.9"/>
          <path d="M16 10L22 14V18L16 22L10 18V14L16 10Z" fill="#3b82f6"/>
        </svg>
        <span class="brand-title">{{ uiConfig.loginBrandName || '统一用户管理平台' }}</span>
      </div>
      <div class="top-bar-right">
        <!-- 可以放其他链接 -->
      </div>
    </div>
    
    <!-- 背景动画层 -->
    <div class="bg-layer">
      <!-- 物联网图形 - 偏左显示 -->
      <div class="iot-graphic">
        <!-- 中心圆环 -->
        <div class="center-ring">
          <div class="ring ring-1"></div>
          <div class="ring ring-2"></div>
          <div class="ring ring-3"></div>
          <div class="center-icon">
            <svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="32" cy="32" r="8" fill="#3b82f6"/>
              <path d="M32 12V20M32 44V52M12 32H20M44 32H52" stroke="#3b82f6" stroke-width="3" stroke-linecap="round"/>
              <circle cx="32" cy="12" r="4" fill="#60a5fa"/>
              <circle cx="32" cy="52" r="4" fill="#60a5fa"/>
              <circle cx="12" cy="32" r="4" fill="#60a5fa"/>
              <circle cx="52" cy="32" r="4" fill="#60a5fa"/>
            </svg>
          </div>
        </div>
        
        <!-- 浮动图标 -->
        <div class="floating-device device-1">
          <svg viewBox="0 0 40 40" fill="none">
            <circle cx="20" cy="16" r="8" stroke="#3b82f6" stroke-width="2"/>
            <path d="M12 30C12 25.5817 15.5817 22 20 22C24.4183 22 28 25.5817 28 30" stroke="#3b82f6" stroke-width="2" stroke-linecap="round"/>
          </svg>
          <span>用户</span>
        </div>
        
        <div class="floating-device device-2">
          <svg viewBox="0 0 40 40" fill="none">
            <rect x="8" y="6" width="24" height="28" rx="3" stroke="#0ea5e9" stroke-width="2"/>
            <circle cx="20" cy="14" r="5" stroke="#0ea5e9" stroke-width="2"/>
            <path d="M12 28C12 24.6863 15.5817 22 20 22C24.4183 22 28 24.6863 28 28" stroke="#0ea5e9" stroke-width="2"/>
          </svg>
          <span>组织</span>
        </div>
        
        <div class="floating-device device-3">
          <svg viewBox="0 0 40 40" fill="none">
            <path d="M20 6L32 14V26L20 34L8 26V14L20 6Z" stroke="#6366f1" stroke-width="2"/>
            <circle cx="20" cy="20" r="5" fill="#6366f1" fill-opacity="0.2" stroke="#6366f1" stroke-width="2"/>
          </svg>
          <span>角色</span>
        </div>
        
        <div class="floating-device device-4">
          <svg viewBox="0 0 40 40" fill="none">
            <rect x="6" y="10" width="28" height="20" rx="3" stroke="#8b5cf6" stroke-width="2"/>
            <circle cx="20" cy="20" r="6" stroke="#8b5cf6" stroke-width="2"/>
            <path d="M14 20L18 24L26 16" stroke="#8b5cf6" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <span>安全</span>
        </div>
        
        <!-- 连接线动画 -->
        <svg class="connection-lines" viewBox="0 0 400 400">
          <defs>
            <linearGradient id="lineGrad" x1="0%" y1="0%" x2="100%" y2="0%">
              <stop offset="0%" stop-color="#3b82f6" stop-opacity="0"/>
              <stop offset="50%" stop-color="#3b82f6" stop-opacity="0.6"/>
              <stop offset="100%" stop-color="#3b82f6" stop-opacity="0"/>
            </linearGradient>
          </defs>
          <path class="conn-line line-1" d="M80,100 Q150,150 200,200" stroke="url(#lineGrad)" stroke-width="2" fill="none"/>
          <path class="conn-line line-2" d="M320,100 Q250,150 200,200" stroke="url(#lineGrad)" stroke-width="2" fill="none"/>
          <path class="conn-line line-3" d="M80,300 Q150,250 200,200" stroke="url(#lineGrad)" stroke-width="2" fill="none"/>
          <path class="conn-line line-4" d="M320,300 Q250,250 200,200" stroke="url(#lineGrad)" stroke-width="2" fill="none"/>
        </svg>
        
        <!-- 数据粒子 -->
        <div class="data-particles">
          <div class="particle" v-for="i in 12" :key="'p'+i" :style="particleStyle(i)"></div>
        </div>
      </div>
      
      <!-- 装饰圆 -->
      <div class="circle circle-1"></div>
      <div class="circle circle-2"></div>
      <div class="circle circle-3"></div>
    </div>
    
    <!-- 透明登录框 - 右侧四分之三位置 -->
    <div class="login-card">
      <div class="card-content">
        <div class="card-header">
          <div class="logo-icon">
            <svg viewBox="0 0 48 48" fill="none">
              <circle cx="24" cy="24" r="20" fill="#3b82f6" fill-opacity="0.1"/>
              <path d="M24 8L36 16V32L24 40L12 32V16L24 8Z" fill="#3b82f6"/>
              <path d="M24 16L30 20V28L24 32L18 28V20L24 16Z" fill="white"/>
            </svg>
          </div>
          <h2>{{ uiConfig.loginTitle || '账号登录' }}</h2>
          <p class="subtitle">{{ uiConfig.loginSubtitle || '欢迎使用统一用户管理平台' }}</p>
        </div>
        
        <!-- 表单/扫码区域容器 -->
        <div class="login-body" ref="loginBodyRef">

        <!-- 账号密码登录表单 -->
        <el-form v-show="loginMode === 'password'" :model="form" @submit.prevent="handleLoginOrVerify" class="login-form">
          <el-form-item>
            <el-input 
              v-model="form.username" 
              placeholder="请输入用户名" 
              size="large"
              :prefix-icon="User"
              :disabled="show2FAInput"
            />
          </el-form-item>
          <el-form-item>
            <el-input
              v-model="form.password"
              type="password"
              placeholder="请输入密码"
              size="large"
              :prefix-icon="Lock"
              show-password
              :disabled="show2FAInput"
              @keyup.enter="handleLoginOrVerify"
            />
          </el-form-item>
          
          <!-- 2FA 验证码输入（内嵌在表单中） -->
          <el-form-item v-if="show2FAInput">
            <div class="twofa-inline">
              <div class="twofa-inline-hint">
                <svg viewBox="0 0 24 24" fill="none" width="16" height="16">
                  <path d="M12 2C9.24 2 7 4.24 7 7v3H6c-1.1 0-2 .9-2 2v8c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2v-8c0-1.1-.9-2-2-2h-1V7c0-2.76-2.24-5-5-5zm0 2c1.66 0 3 1.34 3 3v3H9V7c0-1.66 1.34-3 3-3z" fill="#3b82f6"/>
                </svg>
                <span>验证码已发送至 {{ twoFactorTarget }}</span>
              </div>
              <el-input
                v-model="twoFactorCode"
                placeholder="请输入6位验证码"
                size="large"
                maxlength="6"
                @keyup.enter="handleLoginOrVerify"
              >
                <template #append>
                  <el-button
                    :disabled="twoFactorCooldown > 0"
                    @click="resend2FACode"
                    class="resend-btn"
                  >
                    {{ twoFactorCooldown > 0 ? `${twoFactorCooldown}s` : '重新发送' }}
                  </el-button>
                </template>
              </el-input>
            </div>
          </el-form-item>
          
          <el-form-item v-if="!show2FAInput">
            <div class="remember-row">
              <el-checkbox v-model="rememberMe">记住账号</el-checkbox>
              <a class="forgot-link" @click="openForgotPassword">忘记密码？</a>
            </div>
          </el-form-item>
          <el-form-item>
            <el-button 
              type="primary" 
              size="large" 
              class="login-btn"
              :loading="loading || twoFactorLoading" 
              @click="handleLoginOrVerify"
            >
              <span v-if="show2FAInput">{{ twoFactorLoading ? '验证中...' : '验证并登录' }}</span>
              <span v-else-if="loading">登录中...</span>
              <span v-else>登 录</span>
            </el-button>
          </el-form-item>
          <el-form-item v-if="show2FAInput">
            <div class="twofa-cancel">
              <span @click="cancel2FA">取消登录</span>
            </div>
          </el-form-item>
        </el-form>
        
        <!-- 钉钉扫码登录面板 -->
        <div v-show="loginMode === 'dingtalk_qr'" class="qrcode-panel">
          <div class="qrcode-wrapper">
            <div id="dingtalk-qr-container" class="qrcode-container"></div>
            <div v-if="dingtalkQRError" class="qrcode-overlay qrcode-error">
              <svg viewBox="0 0 24 24" fill="none" width="40" height="40">
                <circle cx="12" cy="12" r="10" stroke="#f56c6c" stroke-width="2"/>
                <path d="M12 8v4M12 16h.01" stroke="#f56c6c" stroke-width="2" stroke-linecap="round"/>
              </svg>
              <p>{{ dingtalkQRError }}</p>
              <el-button size="small" @click="initDingTalkQRCode">重新加载</el-button>
            </div>
          </div>
          <p v-show="!dingtalkQRError" class="qrcode-tip">请使用钉钉扫描二维码登录</p>
        </div>

        <!-- 飞书扫码登录面板 -->
        <div v-show="loginMode === 'feishu_qr'" class="qrcode-panel">
          <div class="qrcode-wrapper">
            <div id="feishu-qr-container" class="qrcode-container"></div>
            <div v-if="feishuQRError" class="qrcode-overlay qrcode-error">
              <svg viewBox="0 0 24 24" fill="none" width="40" height="40">
                <circle cx="12" cy="12" r="10" stroke="#f56c6c" stroke-width="2"/>
                <path d="M12 8v4M12 16h.01" stroke="#f56c6c" stroke-width="2" stroke-linecap="round"/>
              </svg>
              <p>{{ feishuQRError }}</p>
              <el-button size="small" @click="initFeishuQRCode">重新加载</el-button>
            </div>
          </div>
          <p v-show="!feishuQRError" class="qrcode-tip">请使用飞书扫描二维码登录</p>
        </div>

        <!-- 企业微信扫码登录面板 -->
        <div v-show="loginMode === 'wechatwork_qr'" class="qrcode-panel">
          <div class="qrcode-wrapper">
            <div id="wechatwork-qr-container" class="qrcode-container"></div>
            <div v-if="wechatworkQRError" class="qrcode-overlay qrcode-error">
              <svg viewBox="0 0 24 24" fill="none" width="40" height="40">
                <circle cx="12" cy="12" r="10" stroke="#f56c6c" stroke-width="2"/>
                <path d="M12 8v4M12 16h.01" stroke="#f56c6c" stroke-width="2" stroke-linecap="round"/>
              </svg>
              <p>{{ wechatworkQRError }}</p>
              <el-button size="small" @click="initWeChatWorkQRCode">重新加载</el-button>
            </div>
          </div>
          <p v-show="!wechatworkQRError" class="qrcode-tip">请使用企业微信扫描二维码登录</p>
        </div>

        </div><!-- /login-body -->

        <!-- 底部标签切换 -->
        <div class="login-mode-tabs" v-if="hasAnyQRLogin">
          <div 
            class="mode-tab" 
            :class="{ active: loginMode === 'password' }"
            @click="switchLoginMode('password')"
          >
            账号密码
          </div>
          <div 
            v-if="hasDingTalkSSO"
            class="mode-tab" 
            :class="{ active: loginMode === 'dingtalk_qr' }"
            @click="switchLoginMode('dingtalk_qr')"
          >
            钉钉扫码
          </div>
          <div 
            v-if="hasFeishuSSO"
            class="mode-tab" 
            :class="{ active: loginMode === 'feishu_qr' }"
            @click="switchLoginMode('feishu_qr')"
          >
            飞书扫码
          </div>
          <div 
            v-if="hasWeChatWorkSSO"
            class="mode-tab" 
            :class="{ active: loginMode === 'wechatwork_qr' }"
            @click="switchLoginMode('wechatwork_qr')"
          >
            企微扫码
          </div>
        </div>

        <div class="card-footer" v-if="!hasAnyQRLogin" style="margin-top:auto">
          <p>{{ uiConfig.footerText || '统一身份 · 高效管理' }}</p>
        </div>
      </div>
    </div>
    
    <!-- 双因素认证对话框（已改为内嵌表单，此处保留但不再使用） -->

    <!-- 忘记密码弹窗 -->
    <el-dialog
      v-model="forgotVisible"
      title=""
      width="420px"
      :close-on-click-modal="false"
      class="forgot-dialog"
      destroy-on-close
    >
      <div class="forgot-content">
        <div class="forgot-header">
          <svg viewBox="0 0 48 48" fill="none" width="40" height="40">
            <circle cx="24" cy="24" r="20" fill="#eff6ff"/>
            <path d="M24 14a6 6 0 0 0-6 6v2h-1a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2h-1v-2a6 6 0 0 0-6-6zm-3 8v-2a3 3 0 1 1 6 0v2h-6z" fill="#3b82f6"/>
          </svg>
          <h3>忘记密码</h3>
          <p>通过验证码重置您的登录密码</p>
        </div>

        <!-- 步骤 1：输入用户名 -->
        <div v-if="forgotStep === 1" class="forgot-step">
          <div class="forgot-field">
            <label>用户名</label>
            <el-input v-model="forgotForm.username" placeholder="用户名 / 手机号 / 邮箱" size="large" @keyup.enter="checkForgotUser" />
          </div>
          <el-button type="primary" size="large" class="forgot-btn" @click="checkForgotUser" :loading="forgotLoading">下一步</el-button>
        </div>

        <!-- 步骤 2：选择验证方式并输入验证码 -->
        <div v-if="forgotStep === 2" class="forgot-step">
          <div class="forgot-user-info">
            <span>{{ forgotNickname }}</span>
            <a @click="forgotStep = 1" class="change-user">换一个账号</a>
          </div>

          <div class="forgot-field" v-if="forgotMethods.length > 1">
            <label>验证方式</label>
            <div class="method-options">
              <button
                v-for="m in forgotMethods"
                :key="m.key"
                :class="['method-btn', { active: forgotForm.method === m.key }]"
                @click="forgotForm.method = m.key"
              >{{ m.name }}</button>
            </div>
          </div>

          <div class="forgot-field">
            <label>验证码</label>
            <div class="code-row">
              <el-input v-model="forgotForm.code" placeholder="请输入6位验证码" size="large" />
              <el-button
                size="large"
                @click="sendForgotCode"
                :disabled="forgotCooldown > 0"
                :loading="forgotSendingCode"
              >{{ forgotCooldown > 0 ? `${forgotCooldown}s` : '获取验证码' }}</el-button>
            </div>
            <span class="forgot-hint" v-if="currentMethodHint">{{ currentMethodHint }}</span>
          </div>

          <div class="forgot-field">
            <label>新密码</label>
            <el-input v-model="forgotForm.newPassword" type="password" show-password placeholder="至少8位，需包含大小写字母和数字" size="large" />
          </div>

          <div class="forgot-field">
            <label>确认新密码</label>
            <el-input v-model="forgotForm.confirmPassword" type="password" show-password placeholder="再次输入新密码" size="large" @keyup.enter="submitForgotReset" />
          </div>

          <el-button type="primary" size="large" class="forgot-btn" @click="submitForgotReset" :loading="forgotLoading">重置密码</el-button>
        </div>

        <!-- 步骤 3：重置成功 -->
        <div v-if="forgotStep === 3" class="forgot-step forgot-success">
          <svg viewBox="0 0 64 64" fill="none" width="56" height="56">
            <circle cx="32" cy="32" r="28" fill="#f0fdf4"/>
            <path d="M20 32l8 8 16-16" stroke="#22c55e" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <h4>密码重置成功</h4>
          <p>请使用新密码登录</p>
          <el-button type="primary" size="large" class="forgot-btn" @click="closeForgotAndFillUser">返回登录</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- 页脚 -->
    <div class="page-footer" v-if="uiConfig.footerShortName || uiConfig.footerCompany || uiConfig.footerICP">
      <span v-if="uiConfig.footerShortName">Powered By {{ uiConfig.footerShortName }}</span>
      <span v-if="uiConfig.footerCompany" class="footer-divider">{{ uiConfig.footerCompany }}</span>
      <a v-if="uiConfig.footerICP" class="footer-divider footer-icp-link" href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer">{{ uiConfig.footerICP }}</a>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElLoading } from "element-plus";
import { User, Lock, Loading } from "@element-plus/icons-vue";
import { useUserStore } from "../store/user";
import { settingsApi, authApi, syncApi } from "../api";

const router = useRouter();
const route = useRoute();
const userStore = useUserStore();

const form = reactive({ username: "", password: "" });
const loading = ref(false);
const rememberMe = ref(false);

// 登录模式：password | dingtalk_qr | feishu_qr | wechatwork_qr
const loginMode = ref<'password' | 'dingtalk_qr' | 'feishu_qr' | 'wechatwork_qr'>('password');
const loginBodyRef = ref<HTMLElement | null>(null);

// 双因素认证（内嵌表单模式）
const show2FAInput = ref(false);
const twoFactorCode = ref("");
const twoFactorPendingToken = ref("");
const twoFactorTarget = ref("");
const twoFactorChannelType = ref("");
const twoFactorLoading = ref(false);
const twoFactorCooldown = ref(0);
let twoFactorTimer: ReturnType<typeof setInterval> | null = null;

// 钉钉相关
const dingtalkLoading = ref(false);
const dingtalkQRLoading = ref(false);
const dingtalkQRError = ref("");
const hasDingTalkSSO = ref(false);
let dingtalkProvider: any = null;

// 飞书相关
const feishuQRLoading = ref(false);
const feishuQRError = ref("");
const hasFeishuSSO = ref(false);
let feishuProvider: any = null;

// 企微相关
const wechatworkQRLoading = ref(false);
const wechatworkQRError = ref("");
const hasWeChatWorkSSO = ref(false);
let wechatworkProvider: any = null;

// 是否有任意扫码登录可用
const hasAnyQRLogin = computed(() => hasDingTalkSSO.value || hasFeishuSSO.value || hasWeChatWorkSSO.value);

// 保留旧的弹窗变量用于兼容（但不再使用）
const show2FADialog = ref(false);

const uiConfig = ref({ 
  browserTitle: '', 
  loginTitle: '',
  footerShortName: '',
  footerCompany: '',
  footerICP: '',
  footerText: '',
  loginBrandName: '',
  loginSubtitle: '',
  loginTheme: 'light'
});

// 计算登录主题类
const loginThemeClass = computed(() => {
  return `theme-${uiConfig.value.loginTheme || 'light'}`;
});

const particleStyle = (index: number) => {
  const angle = (index * 30) * Math.PI / 180;
  const radius = 120 + (index % 3) * 40;
  const x = Math.cos(angle) * radius;
  const y = Math.sin(angle) * radius;
  const delay = index * 0.3;
  const duration = 3 + (index % 3);
  return {
    '--x': `${x}px`,
    '--y': `${y}px`,
    '--delay': `${delay}s`,
    '--duration': `${duration}s`
  };
};

// 切换登录模式
const switchLoginMode = (mode: 'password' | 'dingtalk_qr' | 'feishu_qr' | 'wechatwork_qr') => {
  if (mode === loginMode.value) return;
  
  if (show2FAInput.value) {
    cancel2FA();
  }
  
  loginMode.value = mode;
  
  // 切换到对应模式时初始化二维码
  nextTick(() => {
    if (mode === 'feishu_qr' && hasFeishuSSO.value) {
      initFeishuQRCode();
    } else if (mode === 'wechatwork_qr' && hasWeChatWorkSSO.value) {
      initWeChatWorkQRCode();
    }
  });
};

// 取消 2FA 验证
const cancel2FA = () => {
  show2FAInput.value = false;
  twoFactorPendingToken.value = "";
  twoFactorCode.value = "";
  if (twoFactorTimer) {
    clearInterval(twoFactorTimer);
    twoFactorTimer = null;
  }
  twoFactorCooldown.value = 0;
};

// 统一的登录/验证处理
const handleLoginOrVerify = async () => {
  if (show2FAInput.value) {
    await verify2FACode();
  } else {
    await handleLogin();
  }
};

const handleLogin = async () => {
  if (!form.username || !form.password) {
    ElMessage.warning("请输入用户名和密码");
    return;
  }
  loading.value = true;
  try {
    const result = await userStore.login(form.username, form.password);
    if (result.success) {
      // 检查是否需要双因素认证
      if (result.require_2fa) {
        twoFactorPendingToken.value = result.pending_token;
        twoFactorTarget.value = result.target || result.masked_phone || "";
        twoFactorChannelType.value = result.channel_type || "sms";
        twoFactorCode.value = "";
        // 使用内嵌表单模式
        show2FAInput.value = true;
        // 开始冷却计时（因为验证码已发送）
        start2FACooldown();
        loading.value = false;
        return;
      }

      if (rememberMe.value) {
        localStorage.setItem('rememberedUser', form.username);
      } else {
        localStorage.removeItem('rememberedUser');
      }
      ElMessage.success("登录成功");
      redirectAfterLogin();
    } else {
      ElMessage.error(result.message || "登录失败");
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || "登录失败");
  } finally {
    loading.value = false;
  }
};

// 登录后重定向
const redirectAfterLogin = () => {
  const rawRedirect = typeof route.query.redirect === "string" ? route.query.redirect : "";
  if (rawRedirect) {
    let target = rawRedirect;
    try {
      target = decodeURIComponent(rawRedirect);
    } catch {
    }
    window.location.href = target;
    return;
  }
  const landing = userStore.layoutConfig?.landingPage;
  router.push(landing || "/admin");
};

// 开始 2FA 冷却计时
const start2FACooldown = () => {
  twoFactorCooldown.value = 60;
  if (twoFactorTimer) clearInterval(twoFactorTimer);
  twoFactorTimer = setInterval(() => {
    twoFactorCooldown.value--;
    if (twoFactorCooldown.value <= 0) {
      clearInterval(twoFactorTimer!);
      twoFactorTimer = null;
    }
  }, 1000);
};

// 重新发送 2FA 验证码
const resend2FACode = async () => {
  if (twoFactorCooldown.value > 0) return;
  try {
    const result = await userStore.resend2FACode(twoFactorPendingToken.value);
    if (result.success) {
      ElMessage.success(result.data?.message || "验证码已发送");
      start2FACooldown();
    } else {
      ElMessage.error(result.message || "发送失败");
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || "发送失败");
  }
};

// 验证 2FA 验证码
const verify2FACode = async () => {
  if (!twoFactorCode.value) {
    ElMessage.warning("请输入验证码");
    return;
  }
  twoFactorLoading.value = true;
  try {
    const result = await userStore.verify2FA(twoFactorPendingToken.value, twoFactorCode.value);
    if (result.success) {
      // 重置状态
      show2FAInput.value = false;
      show2FADialog.value = false;
      if (rememberMe.value) {
        localStorage.setItem('rememberedUser', form.username);
      } else {
        localStorage.removeItem('rememberedUser');
      }
      ElMessage.success("登录成功");
      redirectAfterLogin();
    } else {
      ElMessage.error(result.message || "验证失败");
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || "验证失败");
  } finally {
    twoFactorLoading.value = false;
  }
};

// 关闭 2FA 对话框（保留兼容）
const close2FADialog = () => {
  show2FADialog.value = false;
  cancel2FA();
};

// ========== 忘记密码 ==========
const forgotVisible = ref(false);
const forgotStep = ref(1);
const forgotLoading = ref(false);
const forgotSendingCode = ref(false);
const forgotCooldown = ref(0);
let forgotTimer: ReturnType<typeof setInterval> | null = null;
const forgotNickname = ref('');
const forgotMethods = ref<{key:string; name:string; hint:string}[]>([]);
const forgotForm = reactive({
  username: '',
  method: '',
  code: '',
  newPassword: '',
  confirmPassword: ''
});

const currentMethodHint = computed(() => {
  const m = forgotMethods.value.find(x => x.key === forgotForm.method);
  return m?.hint || '';
});

const openForgotPassword = () => {
  forgotStep.value = 1;
  forgotForm.username = form.username || '';
  forgotForm.method = '';
  forgotForm.code = '';
  forgotForm.newPassword = '';
  forgotForm.confirmPassword = '';
  forgotCooldown.value = 0;
  forgotVisible.value = true;
};

const checkForgotUser = async () => {
  if (!forgotForm.username) return ElMessage.warning('请输入用户名');
  forgotLoading.value = true;
  try {
    const res = await authApi.forgotPassword.check(forgotForm.username);
    if (res.data.success) {
      const methods = res.data.data.methods || [];
      if (methods.length === 0) {
        ElMessage.warning('该用户无可用的验证方式，请联系管理员重置密码');
        return;
      }
      forgotNickname.value = res.data.data.nickname || forgotForm.username;
      // 使用后端返回的真实用户名（支持手机号/邮箱查找后映射到真实用户名）
      if (res.data.data.username) {
        forgotForm.username = res.data.data.username;
      }
      forgotMethods.value = methods;
      forgotForm.method = methods[0].key;
      forgotStep.value = 2;
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '查询失败');
  } finally {
    forgotLoading.value = false;
  }
};

const sendForgotCode = async () => {
  if (!forgotForm.method) return ElMessage.warning('请选择验证方式');
  forgotSendingCode.value = true;
  try {
    const res = await authApi.forgotPassword.sendCode(forgotForm.username, forgotForm.method);
    if (res.data.success) {
      ElMessage.success(res.data.data?.message || '验证码已发送');
      forgotCooldown.value = 60;
      if (forgotTimer) clearInterval(forgotTimer);
      forgotTimer = setInterval(() => {
        if (--forgotCooldown.value <= 0 && forgotTimer) {
          clearInterval(forgotTimer);
          forgotTimer = null;
        }
      }, 1000);
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '发送失败');
  } finally {
    forgotSendingCode.value = false;
  }
};

const validatePasswordPolicy = (pwd: string): string | null => {
  if (pwd.length < 8) return '密码长度至少8位';
  if (pwd.length > 128) return '密码长度不能超过128位';
  if (!/[A-Z]/.test(pwd)) return '密码必须包含大写字母';
  if (!/[a-z]/.test(pwd)) return '密码必须包含小写字母';
  if (!/[0-9]/.test(pwd)) return '密码必须包含数字';
  return null;
};

const submitForgotReset = async () => {
  if (!forgotForm.code) return ElMessage.warning('请输入验证码');
  if (!forgotForm.newPassword) return ElMessage.warning('请输入新密码');
  const policyError = validatePasswordPolicy(forgotForm.newPassword);
  if (policyError) return ElMessage.warning(policyError);
  if (forgotForm.newPassword !== forgotForm.confirmPassword) return ElMessage.warning('两次输入的密码不一致');

  forgotLoading.value = true;
  try {
    const res = await authApi.forgotPassword.reset({
      username: forgotForm.username,
      method: forgotForm.method,
      code: forgotForm.code,
      newPassword: forgotForm.newPassword
    });
    if (res.data.success) {
      forgotStep.value = 3;
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '重置失败');
  } finally {
    forgotLoading.value = false;
  }
};

const closeForgotAndFillUser = () => {
  form.username = forgotForm.username;
  form.password = '';
  forgotVisible.value = false;
};

// ========== SSO 登录 ==========
const ssoProviders = ref<any[]>([]);
const ssoLoading = ref(false);

const ssoIcon = (platform: string) => {
  const map: Record<string, string> = {
    im_dingtalk: '钉', im_wechatwork: '企', im_feishu: '飞', im_welink: 'W'
  };
  return map[platform] || '?';
};

const loadSSOProviders = async () => {
  try {
    const res = await syncApi.ssoProviders();
    ssoProviders.value = (res as any).data?.data || [];
  } catch {
    ssoProviders.value = [];
  }
};

const handleSSOLogin = async (provider: any) => {
  ssoLoading.value = true;
  try {
    if (provider.platform === 'im_dingtalk') {
      await ssoViaDingTalk(provider);
    } else if (provider.platform === 'im_feishu') {
      await ssoViaFeiShu(provider);
    } else if (provider.platform === 'im_wechatwork') {
      await ssoViaWeChatWork(provider);
    } else {
      ElMessage.warning('该平台暂不支持SSO登录');
    }
  } catch (e: any) {
    ElMessage.error(e?.message || 'SSO登录失败');
  } finally {
    ssoLoading.value = false;
  }
};

const completeSSOLogin = async (provider: any, authCode: string) => {
  const res = await syncApi.ssoLogin({
    connectorId: provider.connectorId,
    platform: provider.platform,
    authCode
  });
  if ((res as any).data?.success) {
    const { token, user } = (res as any).data.data;
    localStorage.setItem('token', token);
    userStore.setToken(token);
    userStore.setUser(user);
    await userStore.fetchUserInfo();
    ElMessage.success('登录成功');
    const landing = userStore.layoutConfig?.landingPage;
    router.push(landing || '/');
  } else {
    throw new Error((res as any).data?.message || 'SSO登录失败');
  }
};

const ssoViaDingTalk = async (provider: any) => {
  const corpId = provider.corpId;
  if (!corpId) {
    throw new Error('钉钉连接器未配置 CorpID，请在上游同步管理中配置后重试');
  }
  
  if (typeof (window as any).dd === 'undefined') {
    await loadDingTalkSDK();
  }
  const dd = (window as any).dd;
  if (!dd) throw new Error('钉钉SDK不可用');

  await new Promise<void>((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error('获取授权码超时')), 8000);
    dd.ready(() => {
      dd.runtime.permission.requestAuthCode({
        corpId,
        onSuccess: async (result: { code: string }) => {
          clearTimeout(timeout);
          try {
            await completeSSOLogin(provider, result.code);
            resolve();
          } catch (e) { reject(e); }
        },
        onFail: (err: any) => { clearTimeout(timeout); reject(err); }
      });
    });
    dd.error?.((err: any) => { clearTimeout(timeout); reject(err); });
  });
};

const ssoViaFeiShu = async (provider: any) => {
  // 飞书SSO: 使用OAuth重定向方式
  const appId = provider.appId;
  const redirectUri = encodeURIComponent(window.location.origin + '/login?sso=feishu&cid=' + provider.connectorId);
  window.location.href = `https://open.feishu.cn/open-apis/authen/v1/authorize?app_id=${appId}&redirect_uri=${redirectUri}&state=feishu`;
};

const ssoViaWeChatWork = async (provider: any) => {
  // 企微SSO: 使用OAuth重定向方式
  const corpId = provider.corpId;
  const agentId = provider.agentId;
  const redirectUri = encodeURIComponent(window.location.origin + '/login?sso=wechatwork&cid=' + provider.connectorId);
  window.location.href = `https://open.work.weixin.qq.com/wwopen/sso/qrConnect?appid=${corpId}&agentid=${agentId}&redirect_uri=${redirectUri}&state=wechatwork`;
};

// 处理SSO回调（飞书/企微OAuth回调后重定向回来带code参数）
const handleSSOCallback = async () => {
  const urlParams = new URLSearchParams(window.location.search);
  const ssoType = urlParams.get('sso');
  const code = urlParams.get('code');
  const cid = urlParams.get('cid');
  if (!ssoType || !code || !cid) return false;

  ssoLoading.value = true;
  try {
    const platform = ssoType === 'feishu' ? 'im_feishu' : 'im_wechatwork';
    await completeSSOLogin({ connectorId: parseInt(cid), platform }, code);
    return true;
  } catch (e: any) {
    ElMessage.error(e?.message || 'SSO登录失败');
    // 清除URL中的SSO参数
    window.history.replaceState({}, '', '/login');
    return false;
  } finally {
    ssoLoading.value = false;
  }
};

// 检测是否在钉钉环境中
const isDingTalkEnv = () => {
  const ua = navigator.userAgent.toLowerCase();
  return ua.indexOf('dingtalk') > -1;
};

// 钉钉免登
const dingTalkLogin = async () => {
  let loadingInstance: any = null;
  
  try {
    // 1. 确定 corpId：优先从新 SSO 连接器获取，降级到旧配置
    let corpId = '';
    let ssoProvider: any = null;

    // 尝试新 SSO 系统
    try {
      const ssoRes = await syncApi.ssoProviders();
      const providers = (ssoRes as any).data?.data || [];
      ssoProvider = providers.find((p: any) => p.platform === 'im_dingtalk');
      if (ssoProvider?.corpId) {
        corpId = ssoProvider.corpId;
      }
    } catch (_e) {
      // SSO 接口调用失败，降级到旧配置
    }

    // 新 SSO 系统未找到，检查旧钉钉配置
    if (!corpId) {
      try {
        const statusRes = await settingsApi.getDingtalkStatus();
        if (statusRes.data.success && statusRes.data.data.corpId) {
          corpId = statusRes.data.data.corpId;
        }
      } catch (_e) {
        // 旧配置也不可用
      }
    }

    // 无可用 SSO provider，不自动免登
    if (!ssoProvider) {
      console.log('[钉钉免登] 未找到钉钉 SSO 配置');
      return;
    }
    
    // 有 SSO provider 但没有 corpId，无法免登
    if (!corpId) {
      console.warn('[钉钉免登] 连接器未配置 CorpID，无法进行免登。请在上游同步管理 > IM 连接器中配置 CorpID');
      return;
    }

    dingtalkLoading.value = true;
    loadingInstance = ElLoading.service({
      text: '钉钉免登中...',
      background: 'rgba(255, 255, 255, 0.8)'
    });

    // 2. 检查钉钉JSAPI是否可用
    if (typeof (window as any).dd === 'undefined') {
      try {
        await loadDingTalkSDK();
      } catch (_e) {
        loadingInstance?.close();
        dingtalkLoading.value = false;
        return;
      }
    }

    const dd = (window as any).dd;
    if (!dd) {
      loadingInstance?.close();
      dingtalkLoading.value = false;
      return;
    }

    // 3. 获取免登授权码 - 使用Promise包装以支持超时
    await new Promise<void>((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error('获取授权码超时'));
      }, 5000);
      
      dd.ready(() => {
        // 构建请求参数，corpId 为空时不传（钉钉内部应用可自动获取）
        const authParams: any = {
          onSuccess: async (result: { code: string }) => {
            clearTimeout(timeout);
            try {
              // 4. 调用后端免登接口
              // 优先使用新 SSO 系统（如果有 provider），否则用旧接口
              let loginRes: any;
              if (ssoProvider) {
                loginRes = await syncApi.ssoLogin({
                  connectorId: ssoProvider.connectorId,
                  platform: 'im_dingtalk',
                  authCode: result.code
                });
              } else {
                loginRes = await authApi.dingtalkLogin(result.code);
              }

              if (loginRes.data.success) {
                const { token, user } = loginRes.data.data;
                localStorage.setItem('token', token);
                userStore.setToken(token);
                userStore.setUser(user);
                
                await userStore.fetchUserInfo();
                
                ElMessage.success('钉钉免登成功');
                const landing = userStore.layoutConfig?.landingPage;
                router.push(landing || '/');
                resolve();
              } else {
                ElMessage.error(loginRes.data.message || '钉钉免登失败');
                reject(new Error(loginRes.data.message));
              }
            } catch (e: any) {
              ElMessage.error(e.response?.data?.message || '钉钉免登失败');
              reject(e);
            }
          },
          onFail: (err: any) => {
            clearTimeout(timeout);
            reject(err);
          }
        };
        // 只有配置了 corpId 时才传递，否则让钉钉自动获取
        if (corpId) {
          authParams.corpId = corpId;
        }
        dd.runtime.permission.requestAuthCode(authParams);
      });
      
      dd.error?.((err: any) => {
        clearTimeout(timeout);
        reject(err);
      });
    });
  } catch (_e) {
    // 异常静默处理，显示登录页面
  } finally {
    loadingInstance?.close();
    dingtalkLoading.value = false;
  }
};

// 动态加载钉钉JSAPI SDK（带超时）
const loadDingTalkSDK = () => {
  return new Promise<void>((resolve, reject) => {
    if ((window as any).dd) {
      resolve();
      return;
    }
    
    // 设置5秒超时
    const timeout = setTimeout(() => {
      reject(new Error('加载钉钉SDK超时'));
    }, 5000);
    
    const script = document.createElement('script');
    script.src = 'https://g.alicdn.com/dingding/dingtalk-jsapi/3.0.12/dingtalk.open.js';
    script.onload = () => {
      clearTimeout(timeout);
      resolve();
    };
    script.onerror = () => {
      clearTimeout(timeout);
      reject(new Error('加载钉钉SDK失败'));
    };
    document.head.appendChild(script);
  });
};

// 等待 DTFrameLogin SDK 就绪（已在 index.html 中 async 加载）
const waitForDTFrameLogin = () => {
  return new Promise<void>((resolve, reject) => {
    if ((window as any).DTFrameLogin) {
      resolve();
      return;
    }
    const timeout = setTimeout(() => reject(new Error('钉钉登录SDK加载超时')), 5000);
    const check = setInterval(() => {
      if ((window as any).DTFrameLogin) {
        clearInterval(check);
        clearTimeout(timeout);
        resolve();
      }
    }, 50);
  });
};

// 初始化钉钉扫码二维码（DTFrameLogin 内嵌方式）
const initDingTalkQRCode = async () => {
  if (!dingtalkProvider) {
    dingtalkQRError.value = "钉钉登录未配置";
    return;
  }
  
  dingtalkQRLoading.value = true;
  dingtalkQRError.value = "";
  
  try {
    await waitForDTFrameLogin();
    
    const DTFrameLogin = (window as any).DTFrameLogin;
    if (!DTFrameLogin) {
      throw new Error('钉钉登录SDK加载失败');
    }
    
    const clientId = dingtalkProvider.appId;
    if (!clientId) {
      throw new Error('钉钉扫码登录需要配置 AppKey（client_id），请在连接器配置中填写');
    }
    
    const corpId = dingtalkProvider.corpId || '';
    const redirectUri = dingtalkProvider.callbackUrl || (window.location.origin + '/login');
    const state = `dingtalk_qr_${dingtalkProvider.connectorId}`;
    
    await nextTick();
    const container = document.getElementById('dingtalk-qr-container');
    if (!container) {
      throw new Error('二维码容器不存在');
    }
    container.innerHTML = '';
    
    DTFrameLogin(
      {
        id: 'dingtalk-qr-container',
        width: 245,
        height: 245,
      },
      {
        redirect_uri: encodeURIComponent(redirectUri),
        client_id: clientId,
        scope: 'openid corpid',
        response_type: 'code',
        state,
        prompt: 'consent',
        ...(corpId ? { corpId } : {}),
      },
      async (loginResult: { redirectUrl: string; authCode: string; state: string }) => {
        // 扫码成功，直接用 authCode 调后端，无需页面跳转
        await completeDingTalkQRLogin(loginResult.authCode);
      },
      (errorMsg: string) => {
        dingtalkQRError.value = errorMsg || '扫码登录失败';
      },
    );
    
  } catch (e: any) {
    dingtalkQRError.value = e.message || '加载二维码失败';
  } finally {
    dingtalkQRLoading.value = false;
  }
};

// 钉钉扫码成功后直接调后端完成登录
const completeDingTalkQRLogin = async (authCode: string) => {
  if (!dingtalkProvider) return;
  
  const loadingInstance = ElLoading.service({
    text: '钉钉登录中...',
    background: 'rgba(255, 255, 255, 0.8)'
  });
  
  try {
    const res = await syncApi.dingtalkQRLogin({
      connectorId: dingtalkProvider.connectorId,
      code: authCode
    });
    
    if ((res as any).data?.success) {
      const { token, user } = (res as any).data.data;
      localStorage.setItem('token', token);
      userStore.setToken(token);
      userStore.setUser(user);
      await userStore.fetchUserInfo();
      ElMessage.success('钉钉登录成功');
      const landing = userStore.layoutConfig?.landingPage;
      router.push(landing || '/');
    } else {
      throw new Error((res as any).data?.message || '钉钉登录失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '钉钉登录失败');
    // 重新初始化二维码
    nextTick(() => initDingTalkQRCode());
  } finally {
    loadingInstance.close();
  }
};

// 处理钉钉扫码回调
const handleDingTalkQRCallback = async () => {
  const urlParams = new URLSearchParams(window.location.search);
  const code = urlParams.get('code');
  const state = urlParams.get('state');
  
  // 检查是否是钉钉扫码回调（state 格式: dingtalk_qr_<connectorId>）
  if (!code || !state || !state.startsWith('dingtalk_qr_')) {
    return false;
  }
  
  const cid = state.replace('dingtalk_qr_', '');
  
  ssoLoading.value = true;
  const loadingInstance = ElLoading.service({
    text: '钉钉登录中...',
    background: 'rgba(255, 255, 255, 0.8)'
  });
  
  try {
    // 调用扫码登录接口
    const res = await syncApi.dingtalkQRLogin({
      connectorId: parseInt(cid),
      code
    });
    
    if ((res as any).data?.success) {
      const { token, user } = (res as any).data.data;
      localStorage.setItem('token', token);
      userStore.setToken(token);
      userStore.setUser(user);
      await userStore.fetchUserInfo();
      ElMessage.success('钉钉登录成功');
      const landing = userStore.layoutConfig?.landingPage;
      router.push(landing || '/');
      return true;
    } else {
      throw new Error((res as any).data?.message || '钉钉登录失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '钉钉登录失败');
    // 清除 URL 参数
    window.history.replaceState({}, '', '/login');
    return false;
  } finally {
    loadingInstance.close();
    ssoLoading.value = false;
  }
};

// ========== 飞书扫码登录 ==========

// 等待飞书 QRLogin SDK 就绪
const waitForFeishuQRLogin = () => {
  return new Promise<void>((resolve, reject) => {
    if ((window as any).QRLogin) {
      resolve();
      return;
    }
    const timeout = setTimeout(() => reject(new Error('飞书登录SDK加载超时')), 5000);
    const check = setInterval(() => {
      if ((window as any).QRLogin) {
        clearInterval(check);
        clearTimeout(timeout);
        resolve();
      }
    }, 50);
  });
};

// 初始化飞书扫码二维码
const initFeishuQRCode = async () => {
  if (!feishuProvider) {
    feishuQRError.value = "飞书登录未配置";
    return;
  }
  
  feishuQRLoading.value = true;
  feishuQRError.value = "";
  
  try {
    await waitForFeishuQRLogin();
    
    const QRLogin = (window as any).QRLogin;
    if (!QRLogin) {
      throw new Error('飞书登录SDK加载失败');
    }
    
    const appId = feishuProvider.appId;
    if (!appId) {
      throw new Error('飞书扫码登录需要配置 App ID');
    }
    
    const redirectUri = encodeURIComponent(window.location.origin + '/login');
    const state = `feishu_qr_${feishuProvider.connectorId}`;
    
    // 构建 goto 参数（飞书授权链接）
    const goto = encodeURIComponent(
      `https://passport.feishu.cn/suite/passport/oauth/authorize?client_id=${appId}&redirect_uri=${redirectUri}&response_type=code&state=${state}`
    );
    
    await nextTick();
    const container = document.getElementById('feishu-qr-container');
    if (!container) {
      throw new Error('二维码容器不存在');
    }
    container.innerHTML = '';
    
    // 渲染二维码
    QRLogin({
      id: 'feishu-qr-container',
      goto,
      width: '245',
      height: '245',
      style: 'width:245px;height:245px;border:none;'
    });
    
    // 监听扫码消息
    const handleFeishuMessage = (event: MessageEvent) => {
      const origin = event.origin;
      // 校验来源
      if (!['https://passport.feishu.cn', 'https://open.feishu.cn'].includes(origin)) {
        return;
      }
      
      const data = event.data;
      // 飞书 SDK 返回 tmp_code
      if (typeof data === 'string') {
        // URL 格式: ...?tmp_code=xxx 或直接 tmp_code=xxx
        const match = data.match(/tmp_code=([^&]+)/);
        if (match) {
          const tmpCode = match[1];
          window.removeEventListener('message', handleFeishuMessage);
          // 跳转到飞书授权页面换取真正的 code
          window.location.href = `https://passport.feishu.cn/suite/passport/oauth/authorize?client_id=${appId}&redirect_uri=${redirectUri}&response_type=code&state=${state}&tmp_code=${tmpCode}`;
        }
      }
    };
    
    window.addEventListener('message', handleFeishuMessage);
    
  } catch (e: any) {
    feishuQRError.value = e.message || '加载二维码失败';
  } finally {
    feishuQRLoading.value = false;
  }
};

// 处理飞书扫码回调
const handleFeishuQRCallback = async () => {
  const urlParams = new URLSearchParams(window.location.search);
  const code = urlParams.get('code');
  const state = urlParams.get('state');
  
  // 检查是否是飞书扫码回调（state 格式: feishu_qr_<connectorId>）
  if (!code || !state || !state.startsWith('feishu_qr_')) {
    return false;
  }
  
  const cid = state.replace('feishu_qr_', '');
  
  ssoLoading.value = true;
  const loadingInstance = ElLoading.service({
    text: '飞书登录中...',
    background: 'rgba(255, 255, 255, 0.8)'
  });
  
  try {
    const res = await syncApi.feishuQRLogin({
      connectorId: parseInt(cid),
      code
    });
    
    if ((res as any).data?.success) {
      const { token, user } = (res as any).data.data;
      localStorage.setItem('token', token);
      userStore.setToken(token);
      userStore.setUser(user);
      await userStore.fetchUserInfo();
      ElMessage.success('飞书登录成功');
      const landing = userStore.layoutConfig?.landingPage;
      router.push(landing || '/');
      return true;
    } else {
      throw new Error((res as any).data?.message || '飞书登录失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '飞书登录失败');
    window.history.replaceState({}, '', '/login');
    return false;
  } finally {
    loadingInstance.close();
    ssoLoading.value = false;
  }
};

// ========== 企业微信扫码登录 ==========

// 等待企微 WwLogin SDK 就绪
const waitForWwLogin = () => {
  return new Promise<void>((resolve, reject) => {
    if ((window as any).WwLogin) {
      resolve();
      return;
    }
    const timeout = setTimeout(() => reject(new Error('企业微信登录SDK加载超时')), 5000);
    const check = setInterval(() => {
      if ((window as any).WwLogin) {
        clearInterval(check);
        clearTimeout(timeout);
        resolve();
      }
    }, 50);
  });
};

// 初始化企微扫码二维码
const initWeChatWorkQRCode = async () => {
  if (!wechatworkProvider) {
    wechatworkQRError.value = "企业微信登录未配置";
    return;
  }
  
  wechatworkQRLoading.value = true;
  wechatworkQRError.value = "";
  
  try {
    await waitForWwLogin();
    
    const WwLogin = (window as any).WwLogin;
    if (!WwLogin) {
      throw new Error('企业微信登录SDK加载失败');
    }
    
    const corpId = wechatworkProvider.corpId;
    const agentId = wechatworkProvider.agentId;
    if (!corpId || !agentId) {
      throw new Error('企业微信扫码登录需要配置 CorpID 和 AgentID');
    }
    
    const redirectUri = encodeURIComponent(window.location.origin + '/login');
    const state = `wechatwork_qr_${wechatworkProvider.connectorId}`;
    
    await nextTick();
    const container = document.getElementById('wechatwork-qr-container');
    if (!container) {
      throw new Error('二维码容器不存在');
    }
    container.innerHTML = '';
    
    // 创建企微扫码实例
    new WwLogin({
      id: 'wechatwork-qr-container',
      appid: corpId,
      agentid: agentId,
      redirect_uri: redirectUri,
      state: state,
      lang: 'zh'
    });
    
  } catch (e: any) {
    wechatworkQRError.value = e.message || '加载二维码失败';
  } finally {
    wechatworkQRLoading.value = false;
  }
};

// 处理企微扫码回调
const handleWeChatWorkQRCallback = async () => {
  const urlParams = new URLSearchParams(window.location.search);
  const code = urlParams.get('code');
  const state = urlParams.get('state');
  
  // 检查是否是企微扫码回调（state 格式: wechatwork_qr_<connectorId>）
  if (!code || !state || !state.startsWith('wechatwork_qr_')) {
    return false;
  }
  
  const cid = state.replace('wechatwork_qr_', '');
  
  ssoLoading.value = true;
  const loadingInstance = ElLoading.service({
    text: '企业微信登录中...',
    background: 'rgba(255, 255, 255, 0.8)'
  });
  
  try {
    const res = await syncApi.wechatworkQRLogin({
      connectorId: parseInt(cid),
      code
    });
    
    if ((res as any).data?.success) {
      const { token, user } = (res as any).data.data;
      localStorage.setItem('token', token);
      userStore.setToken(token);
      userStore.setUser(user);
      await userStore.fetchUserInfo();
      ElMessage.success('企业微信登录成功');
      const landing = userStore.layoutConfig?.landingPage;
      router.push(landing || '/');
      return true;
    } else {
      throw new Error((res as any).data?.message || '企业微信登录失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '企业微信登录失败');
    window.history.replaceState({}, '', '/login');
    return false;
  } finally {
    loadingInstance.close();
    ssoLoading.value = false;
  }
};

onMounted(async () => {
  const rememberedUser = localStorage.getItem('rememberedUser');
  if (rememberedUser) {
    form.username = rememberedUser;
    rememberMe.value = true;
  }
  
  // 并行加载 UI 配置和 SSO 提供商（减少等待时间）
  const [uiRes] = await Promise.all([
    settingsApi.getUI().catch(() => null),
    loadSSOProviders()
  ]);
  
  if (uiRes?.data?.success) {
    uiConfig.value = uiRes.data.data;
    if (uiConfig.value.browserTitle) {
      document.title = uiConfig.value.browserTitle;
    }
  }
  
  // 检查是否有各平台 SSO 配置（需要启用扫码登录开关）
  dingtalkProvider = ssoProviders.value.find((p: any) => p.platform === 'im_dingtalk' && p.enableQrLogin);
  hasDingTalkSSO.value = !!dingtalkProvider;
  
  feishuProvider = ssoProviders.value.find((p: any) => p.platform === 'im_feishu' && p.enableQrLogin);
  hasFeishuSSO.value = !!feishuProvider;
  
  wechatworkProvider = ssoProviders.value.find((p: any) => p.platform === 'im_wechatwork' && p.enableQrLogin);
  hasWeChatWorkSSO.value = !!wechatworkProvider;

  // 预初始化钉钉二维码（SDK 已在 HTML 中 async 加载，此时应该已就绪）
  if (hasDingTalkSSO.value) {
    initDingTalkQRCode();
  }

  // 检查是否有钉钉扫码回调（DTFrameLogin 重定向兜底）
  const dingtalkQRHandled = await handleDingTalkQRCallback();
  if (dingtalkQRHandled) return;

  // 检查是否有飞书扫码回调
  const feishuQRHandled = await handleFeishuQRCallback();
  if (feishuQRHandled) return;

  // 检查是否有企微扫码回调
  const wechatworkQRHandled = await handleWeChatWorkQRCallback();
  if (wechatworkQRHandled) return;

  // 检查是否有SSO回调
  const ssoHandled = await handleSSOCallback();
  if (ssoHandled) return;

  // 如果在钉钉环境中，自动尝试免登（带超时保护）
  if (isDingTalkEnv()) {
    // 先清除可能失效的旧token，确保重新登录
    const existingToken = localStorage.getItem('token');
    if (existingToken) {
      // 验证token是否有效
      try {
        const res = await authApi.getInfo();
        if (res.data.success && res.data.data) {
          // token有效，直接跳转首页
          userStore.setUser(res.data.data);
          router.push('/');
          return;
        }
      } catch (_e) {
        // token无效，清除并重新登录
        localStorage.removeItem('token');
        userStore.clearAuth();
      }
    }
    
    try {
      // 设置整体超时，防止卡住
      const loginPromise = dingTalkLogin();
      const timeoutPromise = new Promise((_, reject) => 
        setTimeout(() => reject(new Error('钉钉登录超时')), 10000)
      );
      await Promise.race([loginPromise, timeoutPromise]);
    } catch (_e) {
      // 静默处理，显示正常登录界面
      dingtalkLoading.value = false;
    }
  }
});

onUnmounted(() => {
  if (twoFactorTimer) {
    clearInterval(twoFactorTimer);
  }
});
</script>

<style scoped>
.login-container {
  height: 100vh;
  width: 100vw;
  max-height: 100vh;
  max-width: 100vw;
  position: fixed;
  top: 0;
  left: 0;
  overflow: hidden;
  box-sizing: border-box;
}

/* 浅色主题（默认） */
.login-container.theme-light {
  background: linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%);
}

.login-container.theme-light .top-bar {
  background: rgba(255, 255, 255, 0.9);
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}

.login-container.theme-light .brand-title {
  color: #1e293b !important;
}

.login-container.theme-light .floating-device span {
  color: #64748b;
}

.login-container.theme-light .page-footer {
  color: #64748b;
  background: rgba(255, 255, 255, 0.8);
  border-top: 1px solid rgba(0, 0, 0, 0.06);
}

.login-container.theme-light .page-footer .footer-divider::before {
  color: #cbd5e1;
}

/* 深色主题 */
.login-container.theme-dark {
  background: linear-gradient(135deg, #0f172a 0%, #1e3a8a 100%);
}

.login-container.theme-dark .top-bar {
  background: rgba(15, 23, 42, 0.8);
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.login-container.theme-dark .brand-title {
  color: #ffffff !important;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
}

.login-container.theme-dark .floating-device span {
  color: rgba(255, 255, 255, 0.9);
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
}

.login-container.theme-dark .page-footer {
  color: rgba(255, 255, 255, 0.6);
  background: rgba(15, 23, 42, 0.5);
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.login-container.theme-dark .page-footer .footer-divider::before {
  color: rgba(255, 255, 255, 0.3);
}

.login-container.theme-dark .page-footer .footer-icp-link:hover {
  color: #60a5fa;
}

/* 蓝色主题 */
.login-container.theme-blue {
  background: linear-gradient(135deg, #1e40af 0%, #3b82f6 100%);
}

.login-container.theme-blue .top-bar {
  background: rgba(30, 64, 175, 0.8);
  border-bottom: 1px solid rgba(255, 255, 255, 0.15);
}

.login-container.theme-blue .brand-title {
  color: #ffffff !important;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
}

.login-container.theme-blue .floating-device span {
  color: rgba(255, 255, 255, 0.9);
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
}

.login-container.theme-blue .page-footer {
  color: rgba(255, 255, 255, 0.7);
  background: rgba(30, 64, 175, 0.6);
  border-top: 1px solid rgba(255, 255, 255, 0.15);
}

.login-container.theme-blue .page-footer .footer-divider::before {
  color: rgba(255, 255, 255, 0.4);
}

.login-container.theme-blue .page-footer .footer-icp-link:hover {
  color: #bfdbfe;
}

/* 顶部导航条基础样式 */
.top-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 40px;
  z-index: 200;
  backdrop-filter: blur(10px);
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.brand-logo {
  width: 32px;
  height: 32px;
}

.brand-title {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: 1px;
}

.top-bar-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

/* 背景层 */
.bg-layer {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

/* 物联网图形 - 偏左 */
.iot-graphic {
  position: absolute;
  width: 400px;
  height: 400px;
  left: 15%;
  top: 50%;
  transform: translateY(-50%);
}

.center-ring {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
}

.ring {
  position: absolute;
  border: 2px solid #3b82f6;
  border-radius: 50%;
  opacity: 0.3;
  animation: pulse-ring 3s ease-out infinite;
}

.ring-1 {
  width: 100px;
  height: 100px;
  top: -50px;
  left: -50px;
  animation-delay: 0s;
}

.ring-2 {
  width: 160px;
  height: 160px;
  top: -80px;
  left: -80px;
  animation-delay: 1s;
}

.ring-3 {
  width: 220px;
  height: 220px;
  top: -110px;
  left: -110px;
  animation-delay: 2s;
}

@keyframes pulse-ring {
  0% {
    transform: scale(0.8);
    opacity: 0.5;
  }
  100% {
    transform: scale(1.2);
    opacity: 0;
  }
}

.center-icon {
  width: 64px;
  height: 64px;
  background: var(--color-bg-container);
  border-radius: 50%;
  box-shadow: 0 8px 32px rgba(59, 130, 246, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  z-index: 10;
  animation: float 3s ease-in-out infinite;
}

.center-icon svg {
  width: 48px;
  height: 48px;
}

@keyframes float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-10px); }
}

/* 浮动设备 */
.floating-device {
  position: absolute;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  animation: device-float 4s ease-in-out infinite;
}

.floating-device svg {
  width: 48px;
  height: 48px;
  background: rgba(255, 255, 255, 0.9);
  border-radius: var(--radius-xl);
  padding: 8px;
  box-shadow: 0 4px 20px rgba(59, 130, 246, 0.15);
  backdrop-filter: blur(10px);
}

.floating-device span {
  font-size: 12px;
  font-weight: 500;
}

.device-1 { top: 60px; left: 60px; animation-delay: 0s; }
.device-2 { top: 60px; right: 60px; animation-delay: 1s; }
.device-3 { bottom: 60px; left: 60px; animation-delay: 2s; }
.device-4 { bottom: 60px; right: 60px; animation-delay: 3s; }

@keyframes device-float {
  0%, 100% { transform: translateY(0) scale(1); }
  50% { transform: translateY(-15px) scale(1.05); }
}

/* 连接线 */
.connection-lines {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
}

.conn-line {
  stroke-dasharray: 200;
  stroke-dashoffset: 200;
  animation: draw-line 3s ease-in-out infinite;
}

.line-1 { animation-delay: 0s; }
.line-2 { animation-delay: 0.5s; }
.line-3 { animation-delay: 1s; }
.line-4 { animation-delay: 1.5s; }

@keyframes draw-line {
  0% { stroke-dashoffset: 200; }
  50% { stroke-dashoffset: 0; }
  100% { stroke-dashoffset: -200; }
}

/* 数据粒子 */
.data-particles {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
}

.particle {
  position: absolute;
  width: 6px;
  height: 6px;
  background: #3b82f6;
  border-radius: 50%;
  transform: translate(var(--x), var(--y));
  animation: particle-pulse var(--duration) ease-in-out infinite;
  animation-delay: var(--delay);
}

@keyframes particle-pulse {
  0%, 100% {
    opacity: 0.3;
    transform: translate(var(--x), var(--y)) scale(1);
  }
  50% {
    opacity: 1;
    transform: translate(var(--x), var(--y)) scale(1.5);
  }
}

/* 装饰圆 */
.circle {
  position: absolute;
  border-radius: 50%;
}

.circle-1 {
  width: 400px;
  height: 400px;
  background: radial-gradient(circle, rgba(59, 130, 246, 0.1) 0%, transparent 70%);
  top: -100px;
  left: -100px;
  animation: circle-float 8s ease-in-out infinite;
}

.circle-2 {
  width: 300px;
  height: 300px;
  background: radial-gradient(circle, rgba(14, 165, 233, 0.1) 0%, transparent 70%);
  bottom: -50px;
  left: 30%;
  animation: circle-float 10s ease-in-out infinite reverse;
}

.circle-3 {
  width: 200px;
  height: 200px;
  background: radial-gradient(circle, rgba(99, 102, 241, 0.1) 0%, transparent 70%);
  top: 20%;
  left: 40%;
  animation: circle-float 6s ease-in-out infinite;
}

@keyframes circle-float {
  0%, 100% { transform: translate(0, 0); }
  25% { transform: translate(20px, -20px); }
  50% { transform: translate(0, -40px); }
  75% { transform: translate(-20px, -20px); }
}

/* 登录卡片 - 企业风格 */
.login-card {
  position: absolute;
  right: 10%;
  top: 50%;
  transform: translateY(-50%);
  width: 400px;
  z-index: 100;
  animation: fadeIn 0.15s var(--ease-out);
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.card-content {
  background: #ffffff;
  border-radius: var(--radius-xl);
  padding: 20px 40px 28px;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.25);
  width: 380px;
  height: 500px;
  display: flex;
  flex-direction: column;
}

.card-header {
  text-align: center;
  margin-bottom: 12px;
}

.logo-icon {
  width: 48px;
  height: 48px;
  margin: 0 auto 8px;
}

.logo-icon svg {
  width: 100%;
  height: 100%;
}

.card-header h2 {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 6px 0;
}

.subtitle {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 0;
}

/* 登录表单 */
.login-form {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  margin-bottom: 16px;
}

.login-form :deep(.el-input__wrapper) {
  border-radius: var(--radius-md);
  box-shadow: none;
  border: 1px solid var(--color-border);
  padding: 6px 14px;
  transition: all var(--transition-fast);
  background: #ffffff;
}

.login-form :deep(.el-input__wrapper:hover) {
  border-color: var(--color-primary-border);
}

.login-form :deep(.el-input__wrapper.is-focus) {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 2px rgba(30, 64, 175, 0.1);
}

.login-form :deep(.el-input__inner) {
  height: 42px;
}

.remember-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.forgot-link {
  font-size: 13px;
  color: var(--color-primary);
  cursor: pointer;
  text-decoration: none;
  transition: color 0.2s;
}

.forgot-link:hover {
  color: var(--color-primary-hover);
  text-decoration: underline;
}

.login-btn {
  width: 100%;
  height: 48px;
  font-size: 15px;
  font-weight: 600;
  border-radius: var(--radius-md);
  background: var(--color-primary);
  border: none;
  transition: background-color var(--transition-fast);
}

.login-btn:hover {
  background: var(--color-primary-hover);
}

.login-btn:active {
  background: var(--color-primary-active);
}

.card-footer {
  text-align: center;
  padding-top: 16px;
  border-top: 1px solid var(--color-border);
}

.card-footer p {
  font-size: 12px;
  color: var(--color-text-quaternary);
  margin: 0;
  letter-spacing: 2px;
}

/* 页脚 */
.page-footer {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-size: 13px;
  z-index: 100;
}

.page-footer .footer-divider::before {
  content: "|";
  margin-right: 8px;
}

.page-footer .footer-icp-link {
  color: inherit;
  text-decoration: none;
  cursor: pointer;
  transition: color 0.2s;
}

.page-footer .footer-icp-link:hover {
  color: #3b82f6;
  text-decoration: underline;
}

/* SSO 登录按钮 */
.sso-section {
  margin-top: 8px;
}

.sso-divider {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
  gap: 12px;
}

.sso-divider::before,
.sso-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: rgba(148, 163, 184, 0.25);
}

.sso-divider span {
  font-size: 12px;
  color: #94a3b8;
  white-space: nowrap;
}

.sso-buttons {
  display: flex;
  gap: 10px;
  justify-content: center;
}

.sso-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: 1.5px solid rgba(148, 163, 184, 0.3);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.5);
  cursor: pointer;
  font-size: 13px;
  color: #475569;
  transition: all 0.2s;
  flex: 1;
  justify-content: center;
}

.sso-btn:hover {
  border-color: #3b82f6;
  background: rgba(59, 130, 246, 0.05);
  color: #3b82f6;
}

.sso-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.sso-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  color: white;
  background: #3b82f6;
}

.sso-im_dingtalk .sso-icon { background: #0089FF; }
.sso-im_wechatwork .sso-icon { background: #07C160; }
.sso-im_feishu .sso-icon { background: #3370FF; }
.sso-im_welink .sso-icon { background: #E53935; }

/* 响应式 */
@media (max-width: 1200px) {
  .iot-graphic {
    left: 10%;
  }
  .intro-text {
    left: 10%;
  }
  .login-card {
    right: 5%;
  }
}

@media (max-width: 900px) {
  .iot-graphic,
  .intro-text,
  .circle {
    display: none;
  }
  .login-card {
    right: 50%;
    transform: translate(50%, -50%);
  }
}

@media (max-width: 480px) {
  .login-container {
    height: 100%;
    min-height: 100vh;
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
  }
  .login-card {
    width: calc(100% - 32px);
    right: 16px;
    left: auto;
    transform: translateY(-50%);
  }
  .card-content {
    padding: 24px 20px;
  }
  .bg-layer {
    display: none; /* 移动端隐藏复杂动画 */
  }
}

/* 双因素认证弹窗 */
.twofa-content {
  padding: 0 8px 8px;
}

.twofa-header {
  text-align: center;
  margin-bottom: 24px;
}

.twofa-header h3 {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 12px 0 4px;
}

.twofa-header p {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 0;
}

.twofa-field {
  margin-bottom: 20px;
}

.twofa-btn {
  width: 100%;
  height: 44px;
  font-size: 15px;
  border-radius: var(--radius-lg);
}

.twofa-hint {
  text-align: center;
  margin-top: 16px;
}

.twofa-hint span {
  font-size: 13px;
  color: #86909c;
  cursor: pointer;
}

.twofa-hint span:hover {
  color: #3b82f6;
}

/* 忘记密码弹窗 */
.forgot-content {
  padding: 0 8px 8px;
}

.forgot-header {
  text-align: center;
  margin-bottom: 24px;
}

.forgot-header h3 {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 12px 0 4px;
}

.forgot-header p {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 0;
}

.forgot-step {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.forgot-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.forgot-field > label {
  font-size: 13px;
  font-weight: 500;
  color: #4e5969;
}

.forgot-hint {
  font-size: 12px;
  color: #c0c4cc;
}

.forgot-user-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: #f7f8fa;
  border-radius: var(--radius-lg);
  font-size: 14px;
  color: var(--color-text-primary);
  font-weight: 500;
}

.change-user {
  font-size: 12px;
  color: #3b82f6;
  cursor: pointer;
  font-weight: 400;
}

.change-user:hover {
  text-decoration: underline;
}

.method-options {
  display: flex;
  gap: 8px;
}

.method-btn {
  flex: 1;
  padding: 8px 12px;
  border: 1.5px solid #e5e6eb;
  border-radius: var(--radius-lg);
  background: var(--color-bg-container);
  font-size: 13px;
  color: #4e5969;
  cursor: pointer;
  transition: all 0.2s;
  text-align: center;
}

.method-btn:hover {
  border-color: #c9cdd4;
}

.method-btn.active {
  border-color: #3b82f6;
  color: #3b82f6;
  background: #eff6ff;
  font-weight: 500;
}

.code-row {
  display: flex;
  gap: 8px;
}

.code-row .el-input {
  flex: 1;
}

.forgot-btn {
  width: 100%;
  height: 44px;
  font-size: 15px;
  font-weight: 500;
  border-radius: var(--radius-lg);
  margin-top: 4px;
}

.forgot-success {
  align-items: center;
  padding: 16px 0;
}

.forgot-success h4 {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 12px 0 0;
}

.forgot-success p {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 0;
}

/* 2FA 内嵌样式 */
.twofa-inline {
  width: 100%;
}

.twofa-inline-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
  padding: 10px 12px;
  background: #eff6ff;
  border-radius: var(--radius-md);
  font-size: 13px;
  color: #3b82f6;
}

.twofa-inline-hint svg {
  flex-shrink: 0;
}

.twofa-cancel {
  text-align: center;
  margin-top: -8px;
}

.twofa-cancel span {
  font-size: 13px;
  color: #86909c;
  cursor: pointer;
}

.twofa-cancel span:hover {
  color: #3b82f6;
}

.resend-btn {
  min-width: 90px;
}

/* 表单/扫码统一容器 */
.login-body {
  flex: 1;
  display: flex;
  flex-direction: column;
}

/* 扫码面板 */
.qrcode-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  margin-bottom: 16px;
}

.qrcode-wrapper {
  position: relative;
  display: flex;
  justify-content: center;
  align-items: center;
  width: 100%;
  overflow: hidden;
}

.qrcode-container {
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: #fff;
  margin-top: -30px;
}

.qrcode-container :deep(iframe) {
  border: none !important;
  display: block;
}

.qrcode-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  background: #fff;
  color: #86909c;
  font-size: 14px;
  margin-top: 30px;
}

.qrcode-error p {
  color: #f56c6c;
  font-size: 13px;
  margin: 0;
  text-align: center;
  padding: 0 12px;
}

.qrcode-tip {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 12px 0 0 0;
  text-align: center;
}

/* 底部标签切换 */
.login-mode-tabs {
  display: flex;
  justify-content: center;
  gap: 32px;
  padding-top: 12px;
  border-top: 1px solid var(--color-border);
  margin-top: auto;
}

.mode-tab {
  font-size: 14px;
  color: var(--color-text-tertiary);
  cursor: pointer;
  padding: 8px 0;
  position: relative;
  transition: color 0.2s;
}

.mode-tab:hover {
  color: var(--color-primary);
}

.mode-tab.active {
  color: var(--color-primary);
  font-weight: 500;
}

.mode-tab.active::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 24px;
  height: 2px;
  background: var(--color-primary);
  border-radius: 1px;
}
</style>

<style>
/* 登录页面全局样式 - 禁用滚动条 */
html:has(.login-container),
body:has(.login-container) {
  overflow: hidden !important;
  margin: 0;
  padding: 0;
  height: 100%;
  width: 100%;
}

/* 忘记密码弹窗全局样式 */
.forgot-dialog .el-dialog__header {
  display: none;
}

.forgot-dialog .el-dialog {
  border-radius: 16px;
  overflow: hidden;
}

.forgot-dialog .el-dialog__body {
  padding: 28px 24px 20px;
}
</style>
