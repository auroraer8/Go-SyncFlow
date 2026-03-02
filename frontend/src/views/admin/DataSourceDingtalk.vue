<template>
  <div class="dingtalk-page">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2>钉钉对接</h2>
        <p class="header-desc">对接钉钉组织架构，同步用户信息并支持钉钉免登</p>
      </div>
      <div class="header-right">
        <span :class="['status-dot', dingtalkForm.enabled ? 'active' : '']"></span>
        <span class="status-text">{{ dingtalkForm.enabled ? '已启用' : '未启用' }}</span>
      </div>
    </div>

    <!-- 主开关 -->
    <div class="section-card">
      <div class="switch-row">
        <div class="switch-info">
          <span class="switch-label">启用钉钉对接</span>
          <span class="switch-desc">开启后，系统将同步钉钉组织架构和用户信息，并支持钉钉工作台免登</span>
        </div>
        <el-switch v-model="dingtalkForm.enabled" />
      </div>
    </div>

    <!-- 应用凭证 -->
    <template v-if="dingtalkForm.enabled">
      <div class="section-card">
        <div class="section-title">应用凭证</div>
        <p class="section-desc">在钉钉开放平台创建企业内部应用后获取以下信息</p>
        <div class="form-grid">
          <div class="form-field">
            <label>AppKey</label>
            <el-input v-model="dingtalkForm.appKey" placeholder="企业内部应用的 AppKey" />
          </div>
          <div class="form-field">
            <label>AgentId</label>
            <el-input v-model="dingtalkForm.agentId" placeholder="应用的 AgentId" />
          </div>
          <div class="form-field">
            <label>AppSecret</label>
            <el-input v-model="dingtalkForm.appSecret" type="password" show-password placeholder="留空保持不变" />
          </div>
          <div class="form-field">
            <label>CorpId</label>
            <el-input v-model="dingtalkForm.corpId" placeholder="企业的 CorpId" />
          </div>
        </div>
      </div>

      <!-- 用户匹配 -->
      <div class="section-card">
        <div class="section-title">用户匹配</div>
        <div class="form-grid">
          <div class="form-field">
            <label>匹配字段</label>
            <el-select v-model="dingtalkForm.matchField" style="width: 100%">
              <el-option label="手机号" value="mobile" />
              <el-option label="邮箱" value="email" />
              <el-option label="钉钉 UserId" value="userid" />
            </el-select>
            <span class="field-hint">选择用钉钉用户的哪个字段与系统用户进行匹配</span>
          </div>
          <div class="form-field">
            <label>自动注册用户</label>
            <div class="inline-switch">
              <el-switch v-model="dingtalkForm.autoRegister" />
              <span class="switch-hint">首次登录的钉钉用户自动创建系统账号</span>
            </div>
          </div>
        </div>
        <div class="form-grid" v-if="dingtalkForm.autoRegister" style="margin-top: 16px;">
          <div class="form-field">
            <label>默认角色</label>
            <el-select v-model="dingtalkForm.defaultRoleId" placeholder="选择默认角色" style="width: 100%">
              <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
            </el-select>
            <span class="field-hint">自动注册的用户将被分配此角色</span>
          </div>
        </div>
      </div>
    </template>

    <!-- 操作按钮 -->
    <div class="actions-bar">
      <el-button type="primary" @click="saveDingtalk" :loading="saving" size="large">保存配置</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { settingsApi, roleApi } from "../../api";

const saving = ref(false);
const roles = ref<any[]>([]);

const dingtalkForm = reactive({
  enabled: false,
  appKey: "",
  appSecret: "",
  agentId: "",
  corpId: "",
  matchField: "mobile",
  autoRegister: false,
  defaultRoleId: 0
});

const loadDingtalk = async () => {
  try {
    const res = await settingsApi.getDingtalk();
    if (res.data.success && res.data.data) {
      Object.assign(dingtalkForm, res.data.data);
      dingtalkForm.appSecret = "";
    }
  } catch (e) {}
};

const loadRoles = async () => {
  try {
    const res = await roleApi.list();
    if (res.data.success) {
      roles.value = res.data.data || [];
    }
  } catch (e) {}
};

const saveDingtalk = async () => {
  saving.value = true;
  try {
    await settingsApi.updateDingtalk(dingtalkForm);
    ElMessage.success("保存成功");
  } finally {
    saving.value = false;
  }
};

onMounted(() => {
  loadDingtalk();
  loadRoles();
});
</script>

<style scoped>
.dingtalk-page {
  max-width: 780px;
  margin: 0 auto;
  padding: 0 0 40px;
}

/* 顶部 */
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
  color: #1d2129;
  letter-spacing: -0.3px;
}
.header-desc {
  margin: 0;
  font-size: 13px;
  color: #86909c;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-top: 4px;
}
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #c9cdd4;
  flex-shrink: 0;
}
.status-dot.active {
  background: #00b42a;
  box-shadow: 0 0 0 3px rgba(0,180,42,0.15);
}
.status-text {
  font-size: 13px;
  color: #86909c;
  font-weight: 500;
}

/* 区块卡片 */
.section-card {
  background: #fff;
  border: 1px solid #f0f0f0;
  border-radius: 12px;
  padding: 24px;
  margin-bottom: 16px;
}
.section-title {
  font-size: 15px;
  font-weight: 600;
  color: #1d2129;
  margin-bottom: 4px;
}
.section-desc {
  font-size: 13px;
  color: #86909c;
  margin: 2px 0 20px;
  line-height: 1.5;
}

/* 开关行 */
.switch-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.switch-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.switch-label {
  font-size: 14px;
  font-weight: 500;
  color: #1d2129;
}
.switch-desc {
  font-size: 12px;
  color: #86909c;
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
  color: #4e5969;
  font-weight: 500;
}
.field-hint {
  font-size: 11px;
  color: #c0c4cc;
}

/* 行内开关 */
.inline-switch {
  display: flex;
  align-items: center;
  gap: 10px;
  padding-top: 2px;
}
.switch-hint {
  font-size: 12px;
  color: #86909c;
}

/* 底部操作栏 */
.actions-bar {
  display: flex;
  gap: 12px;
  padding-top: 8px;
}

/* 覆盖 Element Plus */
:deep(.el-input__wrapper) {
  border-radius: 8px;
}
:deep(.el-button--large) {
  border-radius: 8px;
  padding: 12px 28px;
  font-weight: 500;
}
</style>
