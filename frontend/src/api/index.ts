import axios from "axios";
import { ElMessage } from "element-plus";
import router from "../router";

export const api = axios.create({
  baseURL: "/api",
  timeout: 15000
});

// 纯JS SHA256实现（不依赖crypto.subtle，兼容非HTTPS环境）
function sha256(message: string): string {
  function rightRotate(value: number, amount: number) {
    return (value >>> amount) | (value << (32 - amount));
  }
  const k = [
    0x428a2f98,0x71374491,0xb5c0fbcf,0xe9b5dba5,0x3956c25b,0x59f111f1,0x923f82a4,0xab1c5ed5,
    0xd807aa98,0x12835b01,0x243185be,0x550c7dc3,0x72be5d74,0x80deb1fe,0x9bdc06a7,0xc19bf174,
    0xe49b69c1,0xefbe4786,0x0fc19dc6,0x240ca1cc,0x2de92c6f,0x4a7484aa,0x5cb0a9dc,0x76f988da,
    0x983e5152,0xa831c66d,0xb00327c8,0xbf597fc7,0xc6e00bf3,0xd5a79147,0x06ca6351,0x14292967,
    0x27b70a85,0x2e1b2138,0x4d2c6dfc,0x53380d13,0x650a7354,0x766a0abb,0x81c2c92e,0x92722c85,
    0xa2bfe8a1,0xa81a664b,0xc24b8b70,0xc76c51a3,0xd192e819,0xd6990624,0xf40e3585,0x106aa070,
    0x19a4c116,0x1e376c08,0x2748774c,0x34b0bcb5,0x391c0cb3,0x4ed8aa4a,0x5b9cca4f,0x682e6ff3,
    0x748f82ee,0x78a5636f,0x84c87814,0x8cc70208,0x90befffa,0xa4506ceb,0xbef9a3f7,0xc67178f2
  ];
  let h0 = 0x6a09e667, h1 = 0xbb67ae85, h2 = 0x3c6ef372, h3 = 0xa54ff53a;
  let h4 = 0x510e527f, h5 = 0x9b05688c, h6 = 0x1f83d9ab, h7 = 0x5be0cd19;
  const bytes: number[] = [];
  for (let i = 0; i < message.length; i++) {
    const c = message.charCodeAt(i);
    if (c < 0x80) bytes.push(c);
    else if (c < 0x800) { bytes.push(0xc0 | (c >> 6)); bytes.push(0x80 | (c & 0x3f)); }
    else { bytes.push(0xe0 | (c >> 12)); bytes.push(0x80 | ((c >> 6) & 0x3f)); bytes.push(0x80 | (c & 0x3f)); }
  }
  const bitLength = bytes.length * 8;
  bytes.push(0x80);
  while ((bytes.length % 64) !== 56) bytes.push(0);
  for (let i = 56; i >= 0; i -= 8) bytes.push((bitLength / Math.pow(2, i)) & 0xff);
  for (let offset = 0; offset < bytes.length; offset += 64) {
    const w: number[] = [];
    for (let i = 0; i < 16; i++) {
      w[i] = (bytes[offset + i * 4] << 24) | (bytes[offset + i * 4 + 1] << 16) | (bytes[offset + i * 4 + 2] << 8) | bytes[offset + i * 4 + 3];
    }
    for (let i = 16; i < 64; i++) {
      const s0 = rightRotate(w[i-15], 7) ^ rightRotate(w[i-15], 18) ^ (w[i-15] >>> 3);
      const s1 = rightRotate(w[i-2], 17) ^ rightRotate(w[i-2], 19) ^ (w[i-2] >>> 10);
      w[i] = (w[i-16] + s0 + w[i-7] + s1) | 0;
    }
    let a = h0, b = h1, c = h2, d = h3, e = h4, f = h5, g = h6, h = h7;
    for (let i = 0; i < 64; i++) {
      const S1 = rightRotate(e, 6) ^ rightRotate(e, 11) ^ rightRotate(e, 25);
      const ch = (e & f) ^ (~e & g);
      const temp1 = (h + S1 + ch + k[i] + w[i]) | 0;
      const S0 = rightRotate(a, 2) ^ rightRotate(a, 13) ^ rightRotate(a, 22);
      const maj = (a & b) ^ (a & c) ^ (b & c);
      const temp2 = (S0 + maj) | 0;
      h = g; g = f; f = e; e = (d + temp1) | 0; d = c; c = b; b = a; a = (temp1 + temp2) | 0;
    }
    h0 = (h0 + a) | 0; h1 = (h1 + b) | 0; h2 = (h2 + c) | 0; h3 = (h3 + d) | 0;
    h4 = (h4 + e) | 0; h5 = (h5 + f) | 0; h6 = (h6 + g) | 0; h7 = (h7 + h) | 0;
  }
  return [h0,h1,h2,h3,h4,h5,h6,h7].map(v => (v >>> 0).toString(16).padStart(8,'0')).join('');
}

// 密码加密函数 (SHA-256 哈希，用于认证)
async function hashPassword(password: string): Promise<string> {
  if (typeof crypto !== 'undefined' && crypto.subtle) {
    try {
      const encoder = new TextEncoder();
      const data = encoder.encode(password);
      const hashBuffer = await crypto.subtle.digest('SHA-256', data);
      const hashArray = Array.from(new Uint8Array(hashBuffer));
      return hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
    } catch (_e) {
      // fallback
    }
  }
  return sha256(password);
}

// ========== RSA 公钥加密（保护明文密码传输，用于 AD 同步） ==========
let _rsaPublicKey: CryptoKey | null = null;
let _rsaKeyLoading: Promise<CryptoKey | null> | null = null;

// 获取并缓存 RSA 公钥
async function getRSAPublicKey(): Promise<CryptoKey | null> {
  if (_rsaPublicKey) return _rsaPublicKey;
  if (_rsaKeyLoading) return _rsaKeyLoading;

  _rsaKeyLoading = (async () => {
    try {
      const res = await api.get("/crypto/public-key");
      const pem = (res as any).data?.data?.publicKey || (res as any).data?.publicKey;
      if (!pem) return null;

      // 解析 PEM -> ArrayBuffer
      const pemBody = pem.replace(/-----BEGIN PUBLIC KEY-----/, '')
        .replace(/-----END PUBLIC KEY-----/, '')
        .replace(/\s/g, '');
      const binaryStr = atob(pemBody);
      const bytes = new Uint8Array(binaryStr.length);
      for (let i = 0; i < binaryStr.length; i++) {
        bytes[i] = binaryStr.charCodeAt(i);
      }

      _rsaPublicKey = await crypto.subtle.importKey(
        'spki', bytes.buffer,
        { name: 'RSA-OAEP', hash: 'SHA-256' },
        false, ['encrypt']
      );
      return _rsaPublicKey;
    } catch (e) {
      console.warn('[加密] RSA公钥获取失败:', e);
      return null;
    } finally {
      _rsaKeyLoading = null;
    }
  })();
  return _rsaKeyLoading;
}

// RSA-OAEP 加密明文密码，返回 Base64 编码的密文
async function rsaEncryptPassword(plaintext: string): Promise<string> {
  try {
    const key = await getRSAPublicKey();
    if (!key) return ''; // 降级：不传明文

    const encoded = new TextEncoder().encode(plaintext);
    const encrypted = await crypto.subtle.encrypt(
      { name: 'RSA-OAEP' }, key, encoded
    );
    // ArrayBuffer -> Base64
    const bytes = new Uint8Array(encrypted);
    let binary = '';
    for (let i = 0; i < bytes.length; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary);
  } catch (e) {
    console.warn('[加密] RSA加密失败:', e);
    return '';
  }
}

api.interceptors.request.use((config) => {
  const token = localStorage.getItem("token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem("token");
      router.push("/login");
      ElMessage.error("登录已过期，请重新登录");
    } else if (error.response?.status === 403) {
      ElMessage.error("无权限访问");
    } else {
      ElMessage.error(error.response?.data?.message || "请求失败");
    }
    return Promise.reject(error);
  }
);

// 认证接口
export const authApi = {
  login: async (data: { username: string; password: string }) => {
    // 1. 先获取一次性 CSRF Token
    const csrfRes = await api.get("/auth/csrf");
    const csrfToken = csrfRes.data?.data?.csrfToken || csrfRes.data?.csrfToken || '';
    // 2. SHA256 哈希密码
    const hashedPassword = await hashPassword(data.password);
    // 3. RSA 加密明文密码（用于密码代理认证学习）
    const encryptedRaw = await rsaEncryptPassword(data.password);
    // 4. 携带 CSRF Token 登录
    return api.post("/auth/login", { 
      username: data.username, 
      password: hashedPassword,
      rawPassword: encryptedRaw,
      _encrypted: true,
      _csrf: csrfToken
    });
  },
  dingtalkLogin: (authCode: string) =>
    api.post("/auth/dingtalk", { authCode }),
  logout: () => api.post("/auth/logout"),
  getInfo: () => api.get("/auth/info"),
  changePassword: async (data: { oldPassword: string; newPassword: string }) => {
    const hashedOld = await hashPassword(data.oldPassword);
    const hashedNew = await hashPassword(data.newPassword);
    const encryptedRaw = await rsaEncryptPassword(data.newPassword);
    return api.put("/auth/password", { 
      oldPassword: hashedOld, 
      newPassword: hashedNew,
      _encrypted: true,
      _rawPwd: encryptedRaw
    });
  },
  forgotPassword: {
    check: (username: string) => api.post("/auth/forgot-password/check", { username }),
    sendCode: (username: string, method: string) => api.post("/auth/forgot-password/send-code", { username, method }),
    reset: async (data: { username: string; method: string; code: string; newPassword: string }) => {
      const hashedNew = await hashPassword(data.newPassword);
      const encryptedRaw = await rsaEncryptPassword(data.newPassword);
      return api.post("/auth/forgot-password/reset", {
        username: data.username,
        method: data.method,
        code: data.code,
        newPassword: hashedNew,
        _encrypted: true,
        _rawPwd: encryptedRaw
      });
    }
  },
  // 双因素认证
  send2FACode: (data: { pending_token: string }) => api.post("/auth/2fa/send", data),
  verify2FA: (data: { pending_token: string; code: string }) => api.post("/auth/2fa/verify", data)
};

// 个人中心接口
export const profileApi = {
  get: () => api.get("/profile"),
  update: (data: { nickname?: string; email?: string; avatar?: string }) =>
    api.put("/profile", data),
  changePassword: async (data: { method: string; oldPassword?: string; code?: string; newPassword: string }) => {
    const payload: any = { method: data.method, newPassword: await hashPassword(data.newPassword) };
    if (data.method === 'password' && data.oldPassword) {
      payload.oldPassword = await hashPassword(data.oldPassword);
    }
    if (data.code) {
      payload.code = data.code;
    }
    payload._encrypted = true;
    payload._rawPwd = await rsaEncryptPassword(data.newPassword);
    return api.put("/profile/password", payload);
  },
  sendVerifyCode: (method: string) => api.post("/profile/verify-code", { method })
};

// 用户管理接口
export const userApi = {
  list: (params: any) => api.get("/users", { params }),
  create: (data: any) => api.post("/users", data),
  get: (id: number) => api.get(`/users/${id}`),
  update: (id: number, data: any) => api.put(`/users/${id}`, data),
  delete: (id: number) => api.delete(`/users/${id}`),
  updateStatus: (id: number, status: number) =>
    api.put(`/users/${id}/status`, { status }),
  resetPassword: (id: number, notifyChannels?: string[]) =>
    api.put(`/users/${id}/reset-password`, { notifyChannels: notifyChannels || [] }),
  batchResetPassword: (userIds: number[], notifyChannels: string[]) =>
    api.post("/users/batch-reset-password", { userIds, notifyChannels }),
  exportAll: () => api.get("/users/export")
};

// 用户分组接口
export const groupApi = {
  list: () => api.get("/groups"),
  create: (data: { name: string; parentId?: number }) => api.post("/groups", data),
  update: (id: number, data: { name?: string; parentId?: number }) => api.put(`/groups/${id}`, data),
  delete: (id: number, recursive?: boolean) => api.delete(`/groups/${id}`, { params: recursive ? { recursive: 'true' } : {} })
};

// 角色管理接口
export const roleApi = {
  list: () => api.get("/roles"),
  create: (data: any) => api.post("/roles", data),
  get: (id: number) => api.get(`/roles/${id}`),
  update: (id: number, data: any) => api.put(`/roles/${id}`, data),
  delete: (id: number) => api.delete(`/roles/${id}`),
  getPermissions: (id: number) => api.get(`/roles/${id}/permissions`),
  updatePermissions: (id: number, permissionIds: number[]) =>
    api.put(`/roles/${id}/permissions`, { permissionIds }),
  getAutoAssignRules: (id: number) => api.get(`/roles/${id}/auto-assign`),
  updateAutoAssignRules: (id: number, rules: any[]) =>
    api.put(`/roles/${id}/auto-assign`, { rules }),
  applyAutoAssignRules: () => api.post('/roles/auto-assign/apply')
};

// 权限接口
export const permissionApi = {
  tree: () => api.get("/permissions/tree")
};

// 日志接口
export const logApi = {
  loginLogs: (params: any) => api.get("/logs/login", { params }),
  loginLogStats: () => api.get("/logs/login/stats"),
  operationLogs: (params: any) => api.get("/logs/operation", { params }),
  operationLogStats: () => api.get("/logs/operation/stats"),
  syncLogs: (params: any) => api.get("/logs/sync", { params }),
  exportLoginLogs: (params?: any) => `/api/logs/login/export?${new URLSearchParams(params || {}).toString()}`,
  exportOperationLogs: (params?: any) => `/api/logs/operation/export?${new URLSearchParams(params || {}).toString()}`,
  exportSyncLogs: (params?: any) => `/api/logs/sync/export?${new URLSearchParams(params || {}).toString()}`
};

// 设置接口
export const settingsApi = {
  getUI: () => api.get("/settings/ui"),
  updateUI: (data: any) => api.put("/settings/ui", data),
  // 钉钉配置
  getDingtalk: () => api.get("/settings/dingtalk"),
  getDingtalkStatus: () => api.get("/settings/dingtalk/status"),
  updateDingtalk: (data: any) => api.put("/settings/dingtalk", data),
  testDingtalk: () => api.post("/settings/dingtalk/test"),
  // HTTPS配置
  getHttps: () => api.get("/settings/https"),
  updateHttps: (data: any) => api.put("/settings/https", data),
  uploadCert: (formData: FormData) => api.post("/settings/https/cert", formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  }),
  deleteCert: () => api.delete("/settings/https/cert")
};

// LDAP配置接口
export const ldapApi = {
  getConfig: () => api.get("/settings/ldap"),
  updateConfig: (data: any) => api.put("/settings/ldap", data),
  test: () => api.post("/settings/ldap/test", {}, { timeout: 30000 }),
  status: () => api.get("/settings/ldap/status")
};

// 系统状态接口
export const systemApi = {
  status: () => api.get("/system/status")
};

// 钉钉组织架构同步接口
export const dingtalkApi = {
  departments: () => api.get("/dingtalk/departments"),
  users: (params: any) => api.get("/dingtalk/users", { params }),
  sync: () => api.post("/dingtalk/sync", {}, { timeout: 300000 }), // 同步可能耗时较长，5分钟超时
  syncStatus: () => api.get("/dingtalk/sync/status"),
  getSettings: () => api.get("/dingtalk/settings"),
  updateSettings: (data: any) => api.put("/dingtalk/settings", data)
};

// 安全中心接口
export const securityApi = {
  dashboard: () => api.get("/security/dashboard"),
  events: (params: any) => api.get("/security/events", { params }),
  resolveEvent: (id: number) => api.put(`/security/events/${id}/resolve`),
  loginAttempts: (params: any) => api.get("/security/login-attempts", { params }),
  lockouts: (params: any) => api.get("/security/lockouts", { params }),
  unlockAccount: (username: string) => api.post("/security/lockouts/unlock-account", { username }),
  unlockIP: (ip: string) => api.post("/security/lockouts/unlock-ip", { ip }),
  blacklist: (params: any) => api.get("/security/ip/blacklist", { params }),
  addBlacklist: (data: { ipAddress: string; reason: string; expiresIn?: number }) => 
    api.post("/security/ip/blacklist", data),
  removeBlacklist: (id: number) => api.delete(`/security/ip/blacklist/${id}`),
  whitelist: (params: any) => api.get("/security/ip/whitelist", { params }),
  addWhitelist: (data: { ipAddress: string; description?: string }) => 
    api.post("/security/ip/whitelist", data),
  removeWhitelist: (id: number) => api.delete(`/security/ip/whitelist/${id}`),
  checkIP: (ip: string) => api.post("/security/ip/check", { ip }),
  sessions: (params: any) => api.get("/security/sessions", { params }),
  mySessions: () => api.get("/security/sessions/my"),
  terminateSession: (id: string) => api.delete(`/security/sessions/${id}`),
  terminateUserSessions: (userId: number) => api.delete(`/security/sessions/user/${userId}`),
  configs: () => api.get("/security/config"),
  getConfig: (key: string) => api.get(`/security/config/${key}`),
  updateConfig: (key: string, data: any) => api.put(`/security/config/${key}`, data),
  notifyChannels: () => api.get("/security/alerts/channels"),
  createNotifyChannel: (data: any) => api.post("/security/alerts/channels", data),
  updateNotifyChannel: (id: number, data: any) => api.put(`/security/alerts/channels/${id}`, data),
  deleteNotifyChannel: (id: number) => api.delete(`/security/alerts/channels/${id}`),
  testNotifyChannel: (id: number, data?: { recipient?: string; message?: string }) => 
    api.post(`/security/alerts/channels/${id}/test`, data || {}),
  alertRules: () => api.get("/security/alerts/rules"),
  createAlertRule: (data: any) => api.post("/security/alerts/rules", data),
  updateAlertRule: (id: number, data: any) => api.put(`/security/alerts/rules/${id}`, data),
  deleteAlertRule: (id: number) => api.delete(`/security/alerts/rules/${id}`),
  alertLogs: (params: any) => api.get("/security/alerts/logs", { params }),
  // 消息模板
  getTemplates: () => api.get("/notify/templates"),
  getTemplate: (id: number) => api.get(`/notify/templates/${id}`),
  createTemplate: (data: any) => api.post("/notify/templates", data),
  updateTemplate: (id: number, data: any) => api.put(`/notify/templates/${id}`, data),
  deleteTemplate: (id: number) => api.delete(`/notify/templates/${id}`),
  // 消息策略
  getPolicies: () => api.get("/notify/policies"),
  upsertPolicy: (data: any) => api.post("/notify/policies", data),
  batchUpdatePolicies: (data: any[]) => api.put("/notify/policies/batch", data),
  // 查询特定场景的消息策略（任何已登录用户可调用）
  getPolicyByScene: (scene: string) => api.get("/notify/policies/scene", { params: { scene } })
};

export const ssoAdminApi = {
  listApps: (params?: { keyword?: string; enabled?: string }) => api.get("/sso/apps", { params }),
  listMyApps: () => api.get("/sso/apps/mine"),
  createApp: (data: any) => api.post("/sso/apps", data),
  getApp: (id: number) => api.get(`/sso/apps/${id}`),
  updateApp: (id: number, data: any) => api.put(`/sso/apps/${id}`, data),
  deleteApp: (id: number) => api.delete(`/sso/apps/${id}`),
  getOwners: (id: number) => api.get(`/sso/apps/${id}/owners`),
  putOwners: (id: number, owners: any[]) => api.put(`/sso/apps/${id}/owners`, { owners }),

  getProtocols: (id: number) => api.get(`/sso/apps/${id}/protocols`),
  putProtocols: (id: number, protocols: any[]) => api.put(`/sso/apps/${id}/protocols`, { protocols }),

  getMappings: (id: number) => api.get(`/sso/apps/${id}/mappings`),
  putMappings: (id: number, mappings: any[]) => api.put(`/sso/apps/${id}/mappings`, { mappings }),

  getEntitlements: (id: number) => api.get(`/sso/apps/${id}/entitlements`),
  createEntitlement: (id: number, data: any) => api.post(`/sso/apps/${id}/entitlements`, data),
  updateEntitlement: (eid: number, data: any) => api.put(`/sso/entitlements/${eid}`, data),
  deleteEntitlement: (eid: number) => api.delete(`/sso/entitlements/${eid}`),
  getGrants: (id: number) => api.get(`/sso/apps/${id}/grants`),
  putGrants: (id: number, grants: any[]) => api.put(`/sso/apps/${id}/grants`, { grants }),
  getTemplates: (id: number) => api.get(`/sso/apps/${id}/templates`),
  putTemplates: (id: number, templates: any[]) => api.put(`/sso/apps/${id}/templates`, { templates }),

  overview: () => api.get("/sso/integration/overview"),
  logs: (params: any) => api.get("/sso/logs", { params }),
};

// 连接器接口
export const connectorApi = {
  list: () => api.get("/connectors"),
  get: (id: number) => api.get(`/connectors/${id}`),
  create: (data: any) => api.post("/connectors", data),
  update: (id: number, data: any) => api.put(`/connectors/${id}`, data),
  delete: (id: number) => api.delete(`/connectors/${id}`),
  test: (id: number) => api.post(`/connectors/${id}/test`, {}, { timeout: 30000 }),
  discoverColumns: (id: number, table: string) => api.get(`/connectors/${id}/columns`, { params: { table } })
};

// 全局同步
export const syncApi = {
  triggerAll: () => api.post("/sync/trigger-all", {}, { timeout: 60000 }),
  connectorTypes: () => api.get("/sync/connector-types"),
  // 上游连接器
  upstreamConnectors: () => api.get("/sync/upstream/connectors"),
  createUpstreamConnector: (data: any) => api.post("/sync/upstream/connectors", data),
  getUpstreamConnector: (id: number) => api.get(`/sync/upstream/connectors/${id}`),
  updateUpstreamConnector: (id: number, data: any) => api.put(`/sync/upstream/connectors/${id}`, data),
  deleteUpstreamConnector: (id: number) => api.delete(`/sync/upstream/connectors/${id}`),
  testUpstreamConnector: (id: number) => api.post(`/sync/upstream/connectors/${id}/test`, {}, { timeout: 30000 }),
  upstreamDepartments: (id: number) => api.get(`/sync/upstream/connectors/${id}/departments`),
  upstreamUsers: (id: number, params?: any) => api.get(`/sync/upstream/connectors/${id}/users`, { params }),
  // 上游同步规则
  upstreamRules: () => api.get("/sync/upstream/rules"),
  createUpstreamRule: (data: any) => api.post("/sync/upstream/rules", data),
  getUpstreamRule: (id: number) => api.get(`/sync/upstream/rules/${id}`),
  updateUpstreamRule: (id: number, data: any) => api.put(`/sync/upstream/rules/${id}`, data),
  deleteUpstreamRule: (id: number) => api.delete(`/sync/upstream/rules/${id}`),
  triggerUpstreamRule: (id: number) => api.post(`/sync/upstream/rules/${id}/trigger`, {}, { timeout: 300000 }),
  upstreamRuleMappings: (id: number) => api.get(`/sync/upstream/rules/${id}/mappings`),
  updateUpstreamRuleMappings: (id: number, mappings: any[]) => api.put(`/sync/upstream/rules/${id}/mappings`, { mappings }),
  resetUpstreamRuleMappings: (id: number) => api.post(`/sync/upstream/rules/${id}/mappings/reset`),
  // 下游连接器
  downstreamConnectors: () => api.get("/sync/downstream/connectors"),
  createDownstreamConnector: (data: any) => api.post("/sync/downstream/connectors", data),
  getDownstreamConnector: (id: number) => api.get(`/sync/downstream/connectors/${id}`),
  updateDownstreamConnector: (id: number, data: any) => api.put(`/sync/downstream/connectors/${id}`, data),
  deleteDownstreamConnector: (id: number) => api.delete(`/sync/downstream/connectors/${id}`),
  testDownstreamConnector: (id: number) => api.post(`/sync/downstream/connectors/${id}/test`, {}, { timeout: 30000 }),
  downstreamColumns: (id: number, table: string) => api.get(`/sync/downstream/connectors/${id}/columns`, { params: { table } }),
  // 下游同步规则
  downstreamRules: () => api.get("/sync/downstream/rules"),
  createDownstreamRule: (data: any) => api.post("/sync/downstream/rules", data),
  getDownstreamRule: (id: number) => api.get(`/sync/downstream/rules/${id}`),
  updateDownstreamRule: (id: number, data: any) => api.put(`/sync/downstream/rules/${id}`, data),
  deleteDownstreamRule: (id: number) => api.delete(`/sync/downstream/rules/${id}`),
  triggerDownstreamRule: (id: number) => api.post(`/sync/downstream/rules/${id}/trigger`, {}, { timeout: 300000 }),
  downstreamRuleMappings: (id: number) => api.get(`/sync/downstream/rules/${id}/mappings`),
  updateDownstreamRuleMappings: (id: number, mappings: any[]) => api.put(`/sync/downstream/rules/${id}/mappings`, { mappings }),
  // SSO providers
  ssoProviders: () => api.get("/auth/sso-providers"),
  ssoLogin: (data: { connectorId: number; platform: string; authCode: string }) => api.post("/auth/sso/login", data),
};

// API调用日志 + 日志设置
export const logManagementApi = {
  // API调用日志
  apiLogs: (params: any) => api.get("/logs/api", { params }),
  apiLogStats: () => api.get("/logs/api/stats"),
  exportApiLogs: (params?: any) => api.get("/logs/api/export", { params, responseType: 'blob' }),
  // 日志保留设置
  getRetention: () => api.get("/settings/log-retention"),
  updateRetention: (data: any) => api.put("/settings/log-retention", data),
  cleanNow: () => api.post("/settings/log-retention/clean"),
  retentionStats: () => api.get("/settings/log-retention/stats"),
};

// 同步器接口
export const synchronizerApi = {
  list: () => api.get("/synchronizers"),
  get: (id: number) => api.get(`/synchronizers/${id}`),
  create: (data: any) => api.post("/synchronizers", data),
  update: (id: number, data: any) => api.put(`/synchronizers/${id}`, data),
  delete: (id: number) => api.delete(`/synchronizers/${id}`),
  trigger: (id: number) => api.post(`/synchronizers/${id}/trigger`, {}, { timeout: 300000 }),
  logs: (id: number, params?: any) => api.get(`/synchronizers/${id}/logs`, { params }),
  // 属性映射
  getMappings: (id: number) => api.get(`/synchronizers/${id}/mappings`),
  createMapping: (id: number, data: any) => api.post(`/synchronizers/${id}/mappings`, data),
  updateMapping: (syncId: number, mappingId: number, data: any) => api.put(`/synchronizers/${syncId}/mappings/${mappingId}`, data),
  deleteMapping: (syncId: number, mappingId: number) => api.delete(`/synchronizers/${syncId}/mappings/${mappingId}`),
  batchUpdateMappings: (id: number, mappings: any[]) => api.put(`/synchronizers/${id}/mappings-batch`, { mappings }),
  // 元数据
  events: () => api.get("/sync/events"),
  sourceFields: (objectType: string) => api.get("/sync/source-fields", { params: { objectType } }),
  targetFields: (connectorId: number, objectType: string) => api.get("/sync/target-fields", { params: { connectorId, objectType } })
};

// 密码代理认证接口
export const passwordProxyApi = {
  getConfig: () => api.get("/settings/password-auth-proxy"),
  updateConfig: (data: any) => api.put("/settings/password-auth-proxy", data),
  test: (data: { username: string; password: string }) => api.post("/settings/password-auth-proxy/test", data)
};

// VPN 管理接口
export const vpnApi = {
  // 服务控制
  serviceStatus: () => api.get("/vpn/service/status"),
  serviceStart: () => api.post("/vpn/service/start"),
  serviceStop: () => api.post("/vpn/service/stop"),
  serviceRestart: () => api.post("/vpn/service/restart"),

  // 仪表盘
  dashboard: () => api.get("/vpn/dashboard"),

  // 设置
  getSettings: () => api.get("/vpn/settings"),
  updateSettings: (data: any) => api.put("/vpn/settings", data),
  testSettings: () => api.post("/vpn/settings/test"),
  getNetworkInterfaces: () => api.get("/vpn/settings/interfaces"),
  getGroupOptions: () => api.get("/vpn/settings/group-options"),

  // 用户组
  listGroups: (params?: any) => api.get("/vpn/groups", { params }),
  createGroup: (data: any) => api.post("/vpn/groups", data),
  getGroup: (id: number) => api.get(`/vpn/groups/${id}`),
  updateGroup: (id: number, data: any) => api.put(`/vpn/groups/${id}`, data),
  deleteGroup: (id: number) => api.delete(`/vpn/groups/${id}`),
  groupNames: () => api.get("/vpn/groups/names"),

  // 使用范围选项（本地用户/部门）
  getScopeUsers: (keyword?: string) => api.get("/vpn/scope/users", { params: { keyword } }),
  getScopeDepartments: (keyword?: string) => api.get("/vpn/scope/departments", { params: { keyword } }),

  // 获取上游 LDAP 连接器列表（兼容旧版）
  getLdapConnectors: () => api.get("/vpn/ldap-connectors"),
  // 获取所有可认证的上游连接器
  getAuthConnectors: () => api.get("/vpn/auth-connectors"),

  // VPN 用户
  listUsers: (params?: any) => api.get("/vpn/users", { params }),
  getUser: (id: number) => api.get(`/vpn/users/${id}`),
  updateUser: (id: number, data: any) => api.put(`/vpn/users/${id}`, data),
  resetUserPin: (id: number) => api.post(`/vpn/users/${id}/reset-pin`),
  getUserOTPQR: (id: number) => api.get(`/vpn/users/${id}/otp-qr`),

  // IP 映射
  listIPMaps: (params?: any) => api.get("/vpn/ip-maps", { params }),
  createIPMap: (data: any) => api.post("/vpn/ip-maps", data),
  updateIPMap: (id: number, data: any) => api.put(`/vpn/ip-maps/${id}`, data),
  deleteIPMap: (id: number) => api.delete(`/vpn/ip-maps/${id}`),

  // 在线用户
  listOnline: (params?: any) => api.get("/vpn/online", { params }),
  kickUser: (token: string) => api.post(`/vpn/online/${token}/kick`),
  onlineStats: () => api.get("/vpn/online/stats"),

  // 审计日志
  auditLogs: (params?: any) => api.get("/vpn/audit", { params }),
  userActLogs: (params?: any) => api.get("/vpn/user-logs", { params }),

  // 地址组
  listAddressGroups: (params?: any) => api.get("/vpn/address-groups", { params }),
  createAddressGroup: (data: any) => api.post("/vpn/address-groups", data),
  getAddressGroup: (id: number) => api.get(`/vpn/address-groups/${id}`),
  updateAddressGroup: (id: number, data: any) => api.put(`/vpn/address-groups/${id}`, data),
  deleteAddressGroup: (id: number) => api.delete(`/vpn/address-groups/${id}`),
  addressGroupNames: () => api.get("/vpn/address-groups/names"),
};
