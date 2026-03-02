<template>
  <div class="settings-page">
    <el-tabs v-model="activeTab" type="border-card">
      <el-tab-pane label="界面配置" name="ui">
        <el-card class="config-card">
          <template #header>
            <span>基本信息</span>
          </template>
          <el-form :model="uiForm" label-width="140px" class="config-form">
            <el-row :gutter="24">
              <el-col :span="12">
                <el-form-item label="浏览器标题">
                  <el-input v-model="uiForm.browserTitle" placeholder="统一用户管理平台" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="登录页标题">
                  <el-input v-model="uiForm.loginTitle" placeholder="账号登录" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="24">
              <el-col :span="12">
                <el-form-item label="登录页品牌名称">
                  <el-input v-model="uiForm.loginBrandName" placeholder="统一用户管理平台（左上角）" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="登录页副标题">
                  <el-input v-model="uiForm.loginSubtitle" placeholder="欢迎使用统一用户管理平台" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-form-item label="Logo URL">
              <el-input v-model="uiForm.logo" placeholder="可选，留空使用默认Logo" />
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="config-card">
          <template #header>
            <span>页脚配置</span>
          </template>
          <el-form :model="uiForm" label-width="120px" class="config-form">
            <el-row :gutter="24">
              <el-col :span="12">
                <el-form-item label="公司简称">
                  <el-input v-model="uiForm.footerShortName" placeholder="如：xxGroup" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="公司名称">
                  <el-input v-model="uiForm.footerCompany" placeholder="如：xx科技有限公司" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-form-item label="ICP备案号">
              <el-input v-model="uiForm.footerICP" placeholder="如：浙ICP备xxxxxxxx号" />
            </el-form-item>
            <el-form-item label="登录页底部文字">
              <el-input v-model="uiForm.footerText" placeholder="如：统一身份 · 高效管理" />
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="config-card">
          <template #header>
            <span>主题设置</span>
          </template>
          <el-form :model="uiForm" label-width="120px" class="config-form">
            <el-form-item label="管理后台主题">
              <div class="theme-options">
                <div 
                  v-for="theme in adminThemes" 
                  :key="theme.value"
                  :class="['theme-option', { active: uiForm.adminTheme === theme.value }]"
                  @click="selectAdminTheme(theme.value)"
                >
                  <div class="theme-preview" :style="{ background: theme.preview.content }">
                    <div class="theme-sidebar" :style="{ background: theme.preview.sidebar }"></div>
                    <div class="theme-content">
                      <div class="theme-header" :style="{ background: theme.preview.header }"></div>
                      <div class="theme-body" :style="{ background: theme.value === 'dark' ? '#374151' : '#ffffff' }"></div>
                    </div>
                  </div>
                  <span class="theme-name">{{ theme.label }}</span>
                </div>
              </div>
            </el-form-item>
            <el-form-item label="登录页主题">
              <div class="theme-options">
                <div 
                  v-for="theme in loginThemes" 
                  :key="theme.value"
                  :class="['theme-option', { active: uiForm.loginTheme === theme.value }]"
                  @click="uiForm.loginTheme = theme.value"
                >
                  <div class="theme-preview login-preview" :style="{ background: theme.preview }">
                    <div class="login-card-mini"></div>
                  </div>
                  <span class="theme-name">{{ theme.label }}</span>
                </div>
              </div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveUI" :loading="savingUI">保存配置</el-button>
              <span class="save-tip">保存后刷新页面生效</span>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="HTTPS证书" name="https">
        <el-card class="config-card">
          <template #header>
            <div class="card-header">
              <span>SSL/TLS证书配置</span>
              <el-tag :type="httpsForm.enabled ? 'success' : 'info'" size="small">
                {{ httpsForm.enabled ? 'HTTPS已启用' : 'HTTPS未启用' }}
              </el-tag>
            </div>
          </template>
          
          <!-- 证书状态 -->
          <div class="cert-status" v-if="httpsForm.certExists && httpsForm.keyExists">
            <el-alert type="success" :closable="false" show-icon>
              <template #title>证书已上传</template>
              <template #default>
                <div class="cert-info">
                  <p><strong>域名：</strong>{{ httpsForm.domain || '未知' }}</p>
                  <p><strong>过期时间：</strong>{{ formatExpiry(httpsForm.certExpiry) }}</p>
                  <p><strong>证书主题：</strong>{{ httpsForm.certSubject || '未知' }}</p>
                </div>
              </template>
            </el-alert>
          </div>
          <div class="cert-status" v-else>
            <el-alert type="warning" :closable="false" show-icon>
              <template #title>未上传证书</template>
              <template #default>请上传SSL证书和私钥文件以启用HTTPS</template>
            </el-alert>
          </div>

          <el-divider />

          <!-- HTTPS配置 -->
          <el-form :model="httpsForm" label-width="120px" class="config-form">
            <el-row :gutter="24">
              <el-col :span="12">
                <el-form-item label="启用HTTPS">
                  <el-switch v-model="httpsForm.enabled" :disabled="!httpsForm.certExists || !httpsForm.keyExists" />
                  <div class="form-tip" v-if="!httpsForm.certExists">请先上传证书</div>
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="HTTPS端口">
                  <el-input v-model="httpsForm.port" placeholder="8443" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-form-item>
              <el-button type="primary" @click="saveHttps" :loading="savingHttps">保存配置</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="config-card">
          <template #header>
            <span>上传证书</span>
          </template>
          <el-form label-width="120px" class="config-form">
            <el-row :gutter="24">
              <el-col :span="12">
                <el-form-item label="证书文件">
                  <div class="upload-wrapper">
                    <el-button type="primary" plain @click="triggerCertUpload">选择证书</el-button>
                    <input
                      ref="certInputRef"
                      type="file"
                      accept=".crt,.pem,.cer"
                      style="display: none"
                      @change="handleCertFileChange"
                    />
                    <div class="upload-tip">.crt, .pem, .cer</div>
                    <div v-if="certFileName" class="file-name">
                      <el-icon><Document /></el-icon>
                      {{ certFileName }}
                      <el-icon class="remove-icon" @click="removeCertFile"><Close /></el-icon>
                    </div>
                  </div>
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="私钥文件">
                  <div class="upload-wrapper">
                    <el-button type="primary" plain @click="triggerKeyUpload">选择私钥</el-button>
                    <input
                      ref="keyInputRef"
                      type="file"
                      accept=".key,.pem"
                      style="display: none"
                      @change="handleKeyFileChange"
                    />
                    <div class="upload-tip">.key, .pem</div>
                    <div v-if="keyFileName" class="file-name">
                      <el-icon><Document /></el-icon>
                      {{ keyFileName }}
                      <el-icon class="remove-icon" @click="removeKeyFile"><Close /></el-icon>
                    </div>
                  </div>
                </el-form-item>
              </el-col>
            </el-row>
            <el-form-item>
              <el-button type="success" @click="uploadCert" :loading="uploading" :disabled="!certFile || !keyFile">
                上传证书
              </el-button>
              <el-button type="danger" @click="deleteCert" v-if="httpsForm.certExists" :loading="deleting">
                删除证书
              </el-button>
            </el-form-item>
          </el-form>

          <el-divider />

          <div class="help-content">
            <h4>配置说明</h4>
            <ol>
              <li>准备SSL证书文件（.crt/.pem）和私钥文件（.key/.pem）</li>
              <li>上传证书和私钥文件</li>
              <li>配置HTTPS端口（默认8443）</li>
              <li>启用HTTPS并保存配置</li>
              <li>重启服务后HTTPS生效</li>
            </ol>
          </div>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="文档" name="docs">
        <div class="docs-header">
          <p class="docs-desc">可以下载本系统的所有相关文档，包括API文档、系统文档、帮助文档</p>
        </div>
        <div class="docs-grid">
          <el-card v-for="doc in docList" :key="doc.id" class="doc-card" shadow="hover">
            <div class="doc-card-body">
              <div class="doc-icon">
                <el-icon :size="36" :style="{ color: 'var(--color-primary)' }"><component :is="doc.icon" /></el-icon>
              </div>
              <div class="doc-info">
                <h3 class="doc-name">{{ doc.name }}</h3>
                <p class="doc-description">{{ doc.description }}</p>
                <p class="doc-filename">{{ doc.filename }}</p>
              </div>
              <div class="doc-action">
                <el-button type="primary" @click="previewDoc(doc.id)">
                  <el-icon><View /></el-icon>预览
                </el-button>
                <el-button @click="downloadDoc(doc.id, doc.filename)">
                  <el-icon><Download /></el-icon>下载
                </el-button>
              </div>
            </div>
          </el-card>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Download, View, Setting, Document, Connection, Close } from "@element-plus/icons-vue";
import { settingsApi, api } from "../../api";
import axios from "axios";
import { adminThemes as themeList, loginThemes as loginThemeList, applyAdminTheme } from "../../utils/theme";

const activeTab = ref("ui");
const savingUI = ref(false);
const savingHttps = ref(false);
const uploading = ref(false);
const deleting = ref(false);

const uiForm = reactive({
  browserTitle: "",
  loginTitle: "",
  logo: "",
  footerShortName: "",
  footerCompany: "",
  footerICP: "",
  footerText: "",
  loginBrandName: "",
  loginSubtitle: "",
  adminTheme: "default",
  loginTheme: "light"
});

// 使用导入的主题配置
const adminThemes = themeList;
const loginThemes = loginThemeList;

const httpsForm = reactive({
  enabled: false,
  port: "8443",
  domain: "",
  certExpiry: "",
  certSubject: "",
  certExists: false,
  keyExists: false
});

const certFile = ref<File | null>(null);
const keyFile = ref<File | null>(null);
const certFileName = ref("");
const keyFileName = ref("");
const certInputRef = ref<HTMLInputElement | null>(null);
const keyInputRef = ref<HTMLInputElement | null>(null);

const triggerCertUpload = () => {
  certInputRef.value?.click();
};

const triggerKeyUpload = () => {
  keyInputRef.value?.click();
};

const handleCertFileChange = (e: Event) => {
  const input = e.target as HTMLInputElement;
  if (input.files && input.files[0]) {
    certFile.value = input.files[0];
    certFileName.value = input.files[0].name;
  }
};

const handleKeyFileChange = (e: Event) => {
  const input = e.target as HTMLInputElement;
  if (input.files && input.files[0]) {
    keyFile.value = input.files[0];
    keyFileName.value = input.files[0].name;
  }
};

const removeCertFile = () => {
  certFile.value = null;
  certFileName.value = "";
  if (certInputRef.value) certInputRef.value.value = "";
};

const removeKeyFile = () => {
  keyFile.value = null;
  keyFileName.value = "";
  if (keyInputRef.value) keyInputRef.value.value = "";
};

const loadUI = async () => {
  try {
    const res = await settingsApi.getUI();
    if (res.data.success) {
      Object.assign(uiForm, res.data.data);
    }
  } catch (e) {}
};

// 选择管理后台主题（立即预览）
const selectAdminTheme = (theme: string) => {
  uiForm.adminTheme = theme;
  applyAdminTheme(theme);
};

const saveUI = async () => {
  savingUI.value = true;
  try {
    await settingsApi.updateUI(uiForm);
    ElMessage.success("保存成功");
    if (uiForm.browserTitle) {
      document.title = uiForm.browserTitle;
    }
    // 应用主题
    applyAdminTheme(uiForm.adminTheme);
  } finally {
    savingUI.value = false;
  }
};

const loadHttps = async () => {
  try {
    const res = await settingsApi.getHttps();
    if (res.data.success && res.data.data) {
      Object.assign(httpsForm, res.data.data);
    }
  } catch (e) {}
};

const saveHttps = async () => {
  savingHttps.value = true;
  try {
    await settingsApi.updateHttps({
      enabled: httpsForm.enabled,
      port: httpsForm.port
    });
    ElMessage.success("保存成功，重启服务后生效");
  } finally {
    savingHttps.value = false;
  }
};

const uploadCert = async () => {
  if (!certFile.value || !keyFile.value) {
    ElMessage.warning("请选择证书文件和私钥文件");
    return;
  }
  
  uploading.value = true;
  try {
    const formData = new FormData();
    formData.append("cert", certFile.value);
    formData.append("key", keyFile.value);
    
    const res = await settingsApi.uploadCert(formData);
    if (res.data.success) {
      ElMessage.success("证书上传成功");
      // 清空文件选择状态
      certFile.value = null;
      keyFile.value = null;
      certFileName.value = "";
      keyFileName.value = "";
      if (certInputRef.value) certInputRef.value.value = "";
      if (keyInputRef.value) keyInputRef.value.value = "";
      // 重新加载证书配置以更新 UI 状态
      await loadHttps();
    }
  } finally {
    uploading.value = false;
  }
};

const deleteCert = async () => {
  try {
    await ElMessageBox.confirm("确定要删除SSL证书吗？删除后HTTPS将无法使用。", "确认删除", {
      type: "warning"
    });
    
    deleting.value = true;
    const res = await settingsApi.deleteCert();
    if (res.data.success) {
      ElMessage.success("证书已删除");
      await loadHttps();
    }
  } catch (e) {
    // 取消删除
  } finally {
    deleting.value = false;
  }
};

const formatExpiry = (expiry: string) => {
  if (!expiry) return "未知";
  try {
    const date = new Date(expiry);
    const now = new Date();
    const diff = date.getTime() - now.getTime();
    const days = Math.floor(diff / (1000 * 60 * 60 * 24));
    
    const formatted = date.toLocaleDateString("zh-CN");
    
    if (days < 0) {
      return `${formatted} (已过期)`;
    } else if (days < 30) {
      return `${formatted} (剩余 ${days} 天)`;
    }
    return formatted;
  } catch {
    return expiry;
  }
};

// ========== 文档管理 ==========
const docList = ref<any[]>([]);

const loadDocs = async () => {
  try {
    const res = await api.get('/docs');
    if (res.data?.success) {
      docList.value = res.data.data || [];
    }
  } catch {
    docList.value = [
      { id: 'manual', name: '系统使用手册', description: '各功能模块操作指南、配置说明与常见问题解答', filename: '系统使用手册.pdf', icon: 'Document' },
      { id: 'api', name: 'API接口文档', description: 'REST API 接口规范、认证方式与调用示例', filename: 'API接口文档.pdf', icon: 'Connection' },
    ];
  }
};

const downloadDoc = async (docId: string, filename: string) => {
  try {
    const res = await fetch(`/api/docs/${docId}`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    if (!res.ok) throw new Error('下载失败');
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
    ElMessage.success('下载成功 ' + filename);
  } catch {
    ElMessage.error('文档下载失败，请确认已登录');
  }
};

const previewDoc = async (docId: string) => {
  try {
    const res = await fetch(`/api/docs/${docId}?mode=preview`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    if (!res.ok) throw new Error('预览失败');
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    window.open(url, '_blank');
  } catch {
    ElMessage.error('文档预览失败，请确认已登录');
  }
};

onMounted(() => {
  loadUI();
  loadHttps();
  loadDocs();
});
</script>

<style scoped>
/* ============================================
   Corporate Settings Page - 企业风格
   ============================================ */
.settings-page {
  width: 100%;
  max-width: none;
}


.config-card {
  margin-bottom: var(--spacing-xl);
}

.config-card :deep(.el-card__header) {
  padding: 14px 20px;
  background: var(--color-fill-secondary);
  font-weight: 600;
  border-bottom: 1px solid var(--color-border);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.config-form {
  padding: var(--spacing-md) 0;
}

.form-tip {
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-top: 6px;
  padding: 6px 10px;
  background: var(--color-fill-secondary);
  border-radius: var(--radius-sm);
  line-height: 1.5;
}

.cert-status {
  margin-bottom: var(--spacing-lg);
}

.cert-info p {
  margin: 6px 0;
  font-size: 13px;
}

.upload-tip {
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-top: 6px;
}

.help-content {
  background: var(--color-fill-secondary);
  border-radius: var(--radius-md);
  padding: var(--spacing-xl);
  border: 1px solid var(--color-border);
}

.help-content h4 {
  margin: 0 0 14px 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.help-content ol {
  margin: 0;
  padding-left: 22px;
  color: var(--color-text-secondary);
  font-size: 13px;
  line-height: 2;
}

/* 文档Tab样式 */
.docs-header {
  margin-bottom: var(--spacing-2xl);
}

.docs-desc {
  font-size: 14px;
  color: var(--color-text-tertiary);
  margin: 0;
}

.docs-grid {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-lg);
}

.doc-card {
  border-radius: var(--radius-md);
  transition: box-shadow var(--transition-fast);
}

.doc-card:hover {
  border-color: var(--color-primary-border);
  box-shadow: var(--card-shadow-hover);
}

.doc-card :deep(.el-card__body) {
  padding: var(--spacing-xl) var(--spacing-2xl);
}

.doc-card-body {
  display: flex;
  align-items: center;
  gap: var(--spacing-xl);
}

.doc-icon {
  flex-shrink: 0;
  width: 56px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-primary);
  border-radius: var(--radius-md);
  color: white;
}

.doc-info {
  flex: 1;
  min-width: 0;
}

.doc-name {
  margin: 0 0 8px 0;
  font-size: 17px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.doc-description {
  margin: 0 0 6px 0;
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.6;
}

.doc-filename {
  margin: 0;
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.doc-action {
  flex-shrink: 0;
}

/* 上传组件样式 */
.upload-wrapper {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.upload-wrapper .upload-tip {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.upload-wrapper .file-name {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  background: var(--color-fill-light);
  border-radius: var(--radius-sm);
  font-size: 13px;
  color: var(--color-text-primary);
}

.upload-wrapper .file-name .remove-icon {
  margin-left: auto;
  cursor: pointer;
  color: var(--color-text-tertiary);
  transition: color 0.2s;
}

.upload-wrapper .file-name .remove-icon:hover {
  color: var(--el-color-danger);
}

/* 主题选择器 */
.theme-options {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.theme-option {
  cursor: pointer;
  text-align: center;
  transition: all 0.2s;
}

.theme-option:hover .theme-preview {
  border-color: var(--color-primary);
  transform: translateY(-2px);
}

.theme-option.active .theme-preview {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

.theme-preview {
  width: 100px;
  height: 70px;
  border-radius: 6px;
  border: 2px solid var(--color-border);
  overflow: hidden;
  display: flex;
  transition: all 0.2s;
}

.theme-preview .theme-sidebar {
  width: 24px;
  height: 100%;
}

.theme-preview .theme-content {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.theme-preview .theme-header {
  height: 14px;
}

.theme-preview .theme-body {
  flex: 1;
  background: rgba(255, 255, 255, 0.9);
}

.theme-preview.login-preview {
  justify-content: center;
  align-items: center;
  position: relative;
}

.theme-preview.login-preview .login-card-mini {
  width: 32px;
  height: 40px;
  background: white;
  border-radius: 4px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  position: absolute;
  right: 16px;
}

.theme-name {
  display: block;
  margin-top: 8px;
  font-size: 13px;
  color: var(--color-text-secondary);
}

.theme-option.active .theme-name {
  color: var(--color-primary);
  font-weight: 500;
}

.save-tip {
  margin-left: 12px;
  font-size: 13px;
  color: var(--color-text-tertiary);
}
</style>
