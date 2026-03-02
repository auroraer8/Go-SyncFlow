<template>
  <div class="sso-config">
    <div class="sso-config__header">
      <div>
        <h2>单点登录 / 系统配置</h2>
        <p class="sso-config__desc">IdP 全局配置、证书管理、协议端点信息</p>
      </div>
      <el-button @click="router.push('/admin/sso')">
        <el-icon><Back /></el-icon>
        返回应用中心
      </el-button>
    </div>

    <el-row :gutter="20">
      <!-- 左侧：IdP 信息 -->
      <el-col :xs="24" :lg="12">
        <el-card class="config-card" v-loading="loading">
          <template #header>
            <div class="card-header">
              <span>IdP 基本信息</span>
              <el-tag type="success" v-if="overview">运行中</el-tag>
            </div>
          </template>

          <div class="info-list" v-if="overview">
            <div class="info-item">
              <span class="info-label">Issuer URL</span>
              <div class="info-value">
                <el-input :model-value="overview.issuer" readonly>
                  <template #append>
                    <el-button @click="copyText(overview.issuer)">复制</el-button>
                  </template>
                </el-input>
              </div>
            </div>
          </div>

          <el-divider />

          <div class="section-title">签名证书</div>
          <div class="cert-info">
            <div class="cert-item">
              <el-icon :size="40" color="#1677ff"><Document /></el-icon>
              <div class="cert-detail">
                <div class="cert-name">RSA 签名证书</div>
                <div class="cert-meta">
                  <span>算法：RS256</span>
                  <span>Kid：{{ overview?.oidc?.kid || '-' }}</span>
                </div>
              </div>
            </div>
            <div class="cert-actions">
              <el-button @click="downloadJwks">下载 JWKS</el-button>
              <el-button type="primary" @click="regenerateCert">重新生成</el-button>
            </div>
          </div>
        </el-card>

        <!-- 统计信息 -->
        <el-card class="config-card" style="margin-top: 20px">
          <template #header>
            <span>登录统计</span>
          </template>
          <div class="stats-grid">
            <div class="stats-item">
              <div class="stats-value">{{ stats.today }}</div>
              <div class="stats-label">今日登录</div>
            </div>
            <div class="stats-item">
              <div class="stats-value">{{ stats.week }}</div>
              <div class="stats-label">本周登录</div>
            </div>
            <div class="stats-item">
              <div class="stats-value">{{ stats.failed }}</div>
              <div class="stats-label">失败次数</div>
            </div>
            <div class="stats-item">
              <div class="stats-value">{{ stats.apps }}</div>
              <div class="stats-label">应用数量</div>
            </div>
          </div>
          <el-button link type="primary" @click="router.push('/admin/sso')">
            查看所有日志 →
          </el-button>
        </el-card>
      </el-col>

      <!-- 右侧：端点列表 -->
      <el-col :xs="24" :lg="12">
        <el-card class="config-card" v-loading="loading">
          <template #header>
            <span>协议端点</span>
          </template>

          <el-collapse v-model="activeCollapse">
            <el-collapse-item title="CAS 协议" name="cas">
              <div class="endpoint-list" v-if="overview">
                <div class="endpoint-item">
                  <span class="endpoint-label">登录地址</span>
                  <el-input :model-value="overview.cas?.login" readonly size="small">
                    <template #append>
                      <el-button size="small" @click="copyText(overview.cas?.login)">复制</el-button>
                    </template>
                  </el-input>
                </div>
                <div class="endpoint-item">
                  <span class="endpoint-label">验证地址 (serviceValidate)</span>
                  <el-input :model-value="overview.cas?.serviceValidate" readonly size="small">
                    <template #append>
                      <el-button size="small" @click="copyText(overview.cas?.serviceValidate)">复制</el-button>
                    </template>
                  </el-input>
                </div>
                <div class="endpoint-item">
                  <span class="endpoint-label">登出地址</span>
                  <el-input :model-value="overview.cas?.logout" readonly size="small">
                    <template #append>
                      <el-button size="small" @click="copyText(overview.cas?.logout)">复制</el-button>
                    </template>
                  </el-input>
                </div>
              </div>
            </el-collapse-item>

            <el-collapse-item title="OAuth2 / OIDC 协议" name="oidc">
              <div class="endpoint-list" v-if="overview">
                <div class="endpoint-item">
                  <span class="endpoint-label">Discovery 文档</span>
                  <el-input :model-value="overview.oidc?.discovery" readonly size="small">
                    <template #append>
                      <el-button size="small" @click="copyText(overview.oidc?.discovery)">复制</el-button>
                    </template>
                  </el-input>
                </div>
                <div class="endpoint-item">
                  <span class="endpoint-label">JWKS</span>
                  <el-input :model-value="overview.oidc?.jwks" readonly size="small">
                    <template #append>
                      <el-button size="small" @click="copyText(overview.oidc?.jwks)">复制</el-button>
                    </template>
                  </el-input>
                </div>
                <div class="endpoint-item">
                  <span class="endpoint-label">Authorization</span>
                  <el-input :model-value="overview.oidc?.authorize" readonly size="small">
                    <template #append>
                      <el-button size="small" @click="copyText(overview.oidc?.authorize)">复制</el-button>
                    </template>
                  </el-input>
                </div>
                <div class="endpoint-item">
                  <span class="endpoint-label">Token</span>
                  <el-input :model-value="overview.oidc?.token" readonly size="small">
                    <template #append>
                      <el-button size="small" @click="copyText(overview.oidc?.token)">复制</el-button>
                    </template>
                  </el-input>
                </div>
                <div class="endpoint-item">
                  <span class="endpoint-label">UserInfo</span>
                  <el-input :model-value="overview.oidc?.userinfo" readonly size="small">
                    <template #append>
                      <el-button size="small" @click="copyText(overview.oidc?.userinfo)">复制</el-button>
                    </template>
                  </el-input>
                </div>
              </div>
            </el-collapse-item>

            <el-collapse-item title="SAML 2.0 协议" name="saml">
              <div class="endpoint-list" v-if="overview">
                <div class="endpoint-item">
                  <span class="endpoint-label">元数据地址 (按应用)</span>
                  <el-input :model-value="overview.saml?.metadataPattern" readonly size="small">
                    <template #append>
                      <el-button size="small" @click="copyText(overview.saml?.metadataPattern)">复制</el-button>
                    </template>
                  </el-input>
                </div>
                <div class="endpoint-item">
                  <span class="endpoint-label">SSO 地址 (按应用)</span>
                  <el-input :model-value="overview.saml?.ssoPattern" readonly size="small">
                    <template #append>
                      <el-button size="small" @click="copyText(overview.saml?.ssoPattern)">复制</el-button>
                    </template>
                  </el-input>
                </div>
              </div>
              <el-alert
                title="SAML 端点按应用区分"
                description="每个应用有独立的元数据和 SSO 地址，请在应用配置中查看具体地址"
                type="info"
                :closable="false"
                style="margin-top: 12px"
              />
            </el-collapse-item>
          </el-collapse>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Back, Document } from "@element-plus/icons-vue";
import { ssoAdminApi } from "../../api";

const router = useRouter();

const loading = ref(false);
const overview = ref<any>(null);
const activeCollapse = ref(["cas", "oidc", "saml"]);

const stats = ref({
  today: 0,
  week: 0,
  failed: 0,
  apps: 0,
});

async function loadOverview() {
  loading.value = true;
  try {
    const [overviewRes, appsRes] = await Promise.all([
      ssoAdminApi.overview(),
      ssoAdminApi.listApps({}),
    ]);
    overview.value = overviewRes.data?.data || null;
    const apps = appsRes.data?.data?.list || [];
    stats.value.apps = apps.length;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "加载失败");
  } finally {
    loading.value = false;
  }
}

function copyText(text: string) {
  if (!text) return;
  navigator.clipboard.writeText(text);
  ElMessage.success("已复制");
}

function downloadJwks() {
  if (overview.value?.oidc?.jwks) {
    window.open(overview.value.oidc.jwks, "_blank");
  }
}

async function regenerateCert() {
  try {
    await ElMessageBox.confirm(
      "重新生成证书将导致所有已接入的 OIDC/SAML 应用需要更新公钥，确定继续？",
      "警告",
      { type: "warning" }
    );
    ElMessage.info("证书重新生成功能开发中");
  } catch {}
}

onMounted(() => {
  loadOverview();
});
</script>

<style scoped>
/* ============================================
   Corporate SSO Config - 企业风格
   ============================================ */
.sso-config {
  padding: 0;
  
}


.sso-config__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.sso-config__header h2 {
  margin: 0 0 6px 0;
  font-size: var(--font-size-xl);
  font-weight: 600;
  color: var(--color-text-primary);
}

.sso-config__desc {
  margin: 0;
  font-size: var(--font-size-base);
  color: var(--color-text-tertiary);
}

.config-card {
  border-radius: var(--card-radius);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.info-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.info-label {
  font-size: 14px;
  color: #6b7280;
}

.section-title {
  font-size: 15px;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 16px;
}

.cert-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
}

.cert-item {
  display: flex;
  align-items: center;
  gap: 16px;
}

.cert-name {
  font-size: var(--font-size-base);
  font-weight: 500;
  color: var(--color-text-primary);
}

.cert-meta {
  display: flex;
  gap: 16px;
  font-size: var(--font-size-sm);
  color: var(--color-text-tertiary);
  margin-top: 4px;
}

.cert-actions {
  display: flex;
  gap: 8px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 16px;
}

.stats-item {
  text-align: center;
  padding: 16px;
  background: var(--color-fill-secondary);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
}

.stats-value {
  font-size: var(--font-size-2xl);
  font-weight: 700;
  color: var(--color-primary);
}

.stats-label {
  font-size: var(--font-size-sm);
  color: var(--color-text-tertiary);
  margin-top: 4px;
}

.endpoint-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.endpoint-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.endpoint-label {
  font-size: var(--font-size-xs);
  color: var(--color-text-tertiary);
}
</style>
