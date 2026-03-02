import { defineStore } from "pinia";
import { ref } from "vue";
import { authApi } from "../api";

export const useUserStore = defineStore("user", () => {
  const token = ref(localStorage.getItem("token") || "");
  const user = ref<any>(null);
  const permissions = ref<string[]>([]);
  const layoutConfig = ref<{ sidebarMode: string; landingPage: string }>({ sidebarMode: 'auto', landingPage: '' });

  const login = async (username: string, password: string) => {
    const res = await authApi.login({ username, password });
    if (res.data.success) {
      // 检查是否需要 2FA
      if (res.data.data.require_2fa) {
        return {
          success: true,
          require_2fa: true,
          pending_token: res.data.data.pending_token,
          channel_type: res.data.data.channel_type,
          target: res.data.data.target,
          masked_phone: res.data.data.masked_phone,
          message: res.data.data.message
        };
      }
      token.value = res.data.data.token;
      user.value = res.data.data.user;
      localStorage.setItem("token", token.value);
      await fetchUserInfo();
    }
    return res.data;
  };

  const verify2FA = async (pendingToken: string, code: string) => {
    const res = await authApi.verify2FA({ pending_token: pendingToken, code });
    if (res.data.success) {
      token.value = res.data.data.token;
      user.value = res.data.data.user;
      localStorage.setItem("token", token.value);
      await fetchUserInfo();
    }
    return res.data;
  };

  const resend2FACode = async (pendingToken: string) => {
    const res = await authApi.send2FACode({ pending_token: pendingToken });
    return res.data;
  };

  const logout = async () => {
    try {
      await authApi.logout();
    } catch (e) {
      // 忽略登出错误
    }
    clearAuth();
  };

  const clearAuth = () => {
    token.value = "";
    user.value = null;
    permissions.value = [];
    layoutConfig.value = { sidebarMode: 'auto', landingPage: '' };
    localStorage.removeItem("token");
  };

  const fetchUserInfo = async () => {
    if (!token.value) return;
    try {
      const res = await authApi.getInfo();
      if (res.data.success) {
        user.value = res.data.data;
        permissions.value = res.data.data.permissions || [];
        if (res.data.data.layoutConfig) {
          layoutConfig.value = res.data.data.layoutConfig;
        }
      }
    } catch (e) {
      // ignore
    }
  };

  const hasPermission = (code: string | string[]) => {
    const list = Array.isArray(code) ? code : [code];
    return list.some((c) => permissions.value.includes(c));
  };

  const setToken = (newToken: string) => {
    token.value = newToken;
    localStorage.setItem("token", newToken);
  };

  const setUser = (newUser: any) => {
    user.value = newUser;
  };

  return {
    token,
    user,
    permissions,
    layoutConfig,
    login,
    verify2FA,
    resend2FACode,
    logout,
    clearAuth,
    fetchUserInfo,
    hasPermission,
    setToken,
    setUser
  };
});
