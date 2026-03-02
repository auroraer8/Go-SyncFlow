<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2>单点登录 / 接入配置</h2>
        <p class="page-desc">提供 CAS/SAML/OIDC 端点、元数据/发现文档、证书与回调白名单配置</p>
      </div>
    </div>

    <el-card class="modern-card" v-loading="loading">
      <template v-if="overview">
        <el-descriptions title="端点总览" :column="1" border>
          <el-descriptions-item label="Issuer">{{ overview.issuer }}</el-descriptions-item>
          <el-descriptions-item label="OIDC Discovery">{{ overview.oidc.discovery }}</el-descriptions-item>
          <el-descriptions-item label="OIDC JWKS">{{ overview.oidc.jwks }}</el-descriptions-item>
          <el-descriptions-item label="OAuth Authorize">{{ overview.oidc.authorize }}</el-descriptions-item>
          <el-descriptions-item label="OAuth Token">{{ overview.oidc.token }}</el-descriptions-item>
          <el-descriptions-item label="OIDC UserInfo">{{ overview.oidc.userinfo }}</el-descriptions-item>
          <el-descriptions-item label="OIDC kid">{{ overview.oidc.kid }}</el-descriptions-item>
          <el-descriptions-item label="CAS Login">{{ overview.cas.login }}</el-descriptions-item>
          <el-descriptions-item label="CAS Validate">{{ overview.cas.serviceValidate }}</el-descriptions-item>
          <el-descriptions-item label="CAS Logout">{{ overview.cas.logout }}</el-descriptions-item>
          <el-descriptions-item label="SAML Metadata">{{ overview.saml.metadataPattern }}</el-descriptions-item>
          <el-descriptions-item label="SAML SSO">{{ overview.saml.ssoPattern }}</el-descriptions-item>
        </el-descriptions>
        <div class="text-muted" style="margin-top: 12px">
          说明：目前协议端点用于对接测试时可通过 Authorization: Bearer &lt;本系统JWT&gt; 进行登录态承载；后续可扩展为独立浏览器会话。
        </div>
      </template>
      <el-empty v-else description="暂无数据" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { ssoAdminApi } from "../../api";

const loading = ref(false);
const overview = ref<any>(null);

async function load() {
  loading.value = true;
  try {
    const res = await ssoAdminApi.overview();
    overview.value = res.data?.data || null;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "加载失败");
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>
