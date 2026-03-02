<template>
  <div class="downstream-page">
    <el-tabs v-model="activeTab" type="border-card">
      <!-- ==================== 下游连接器 ==================== -->
      <el-tab-pane label="下游连接器" name="connectors">
        <div class="tab-toolbar">
          <span class="tab-desc">配置LDAP/AD或数据库作为下游同步目标</span>
          <el-button type="primary" @click="openConnDialog()">
            <el-icon><Plus /></el-icon> 新增连接器
          </el-button>
        </div>

        <el-table :data="connectors" v-loading="connLoading" stripe size="small">
          <el-table-column prop="name" label="名称" min-width="160">
            <template #default="{ row }"><span class="row-name">{{ row.name }}</span></template>
          </el-table-column>
          <el-table-column label="类型" width="140" align="center">
            <template #default="{ row }">
              <el-tag :type="row.type === 'ldap_ad' ? 'primary' : row.type === 'ldap_generic' ? 'warning' : 'success'" size="small" effect="light">
                {{ typeLabel(row) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="地址" min-width="180">
            <template #default="{ row }">
              <span class="conn-addr">{{ row.host }}:{{ row.port }}</span>
            </template>
          </el-table-column>
          <el-table-column label="健康" width="90" align="center">
            <template #default="{ row }">
              <span class="health-dot" :class="{ online: row.lastTestOk, offline: row.lastTestAt && !row.lastTestOk }"></span>
              {{ row.lastTestOk ? '正常' : (row.lastTestAt ? '异常' : '未测试') }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="200" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" link size="small" @click="testConn(row)" :loading="testingId === row.id">测试</el-button>
              <el-button type="primary" link size="small" @click="openConnDialog(row)">编辑</el-button>
              <el-button type="danger" link size="small" @click="deleteConn(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty>
            <el-empty description="暂无下游连接器" :image-size="100" />
          </template>
        </el-table>
      </el-tab-pane>

      <!-- ==================== 同步规则 ==================== -->
      <el-tab-pane label="同步规则" name="rules">
        <div class="tab-toolbar">
          <span class="tab-desc">定义将本地用户数据同步到下游系统的规则和属性映射</span>
          <el-button type="primary" @click="openRuleDialog()">
            <el-icon><Plus /></el-icon> 新增规则
          </el-button>
        </div>

        <el-table :data="rules" v-loading="ruleLoading" stripe size="small">
          <el-table-column prop="name" label="规则名称" min-width="150">
            <template #default="{ row }"><span class="row-name">{{ row.name }}</span></template>
          </el-table-column>
          <el-table-column label="连接器" width="150">
            <template #default="{ row }">
              <el-tag size="small" effect="light" :type="row.connector?.type === 'ldap_ad' ? 'primary' : 'success'">
                {{ row.connector?.name || '-' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="触发方式" min-width="160">
            <template #default="{ row }">
              <div class="trigger-tags">
                <el-tag v-if="row.enableEvent" type="warning" size="small" effect="light">事件</el-tag>
                <el-tag v-if="row.enableSchedule" type="primary" size="small" effect="light">
                  定时 {{ row.scheduleType === 'interval' ? (row.scheduleInterval || 60) + '分钟' : formatScheduleTimes(row.scheduleTime) }}
                </el-tag>
                <span v-if="!row.enableEvent && !row.enableSchedule" class="text-muted">手动</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small" effect="light">
                {{ row.status === 1 ? '启用' : '禁用' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="250" fixed="right">
            <template #default="{ row }">
              <el-button type="success" link size="small" @click="triggerRule(row)" :loading="triggeringId === row.id">
                <el-icon><Refresh /></el-icon> 同步
              </el-button>
              <el-button type="primary" link size="small" @click="editRuleDetail(row)">映射</el-button>
              <el-button type="primary" link size="small" @click="openRuleDialog(row)">编辑</el-button>
              <el-button type="danger" link size="small" @click="deleteRule(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty>
            <el-empty description="暂无同步规则" :image-size="100" />
          </template>
        </el-table>
      </el-tab-pane>

      <!-- ==================== 属性映射 ==================== -->
      <el-tab-pane label="属性映射" name="mappings" v-if="editingRuleId" :disabled="!editingRuleId">
        <div class="tab-toolbar">
          <div>
            <span class="row-name">{{ editingRuleName }}</span>
            <el-button link type="info" @click="exitMappings" style="margin-left: 12px">← 返回规则列表</el-button>
          </div>
          <el-button type="primary" size="small" @click="addMapping">
            <el-icon><Plus /></el-icon> 新增映射
          </el-button>
        </div>

        <div v-if="editingConnType === 'ldap_ad'" style="margin-bottom: 12px; display: flex; align-items: center; gap: 8px;">
          <span style="font-size: 13px; color: #606266; white-space: nowrap;">安全组容器：</span>
          <el-input v-model="editingRoleGroupContainer" placeholder="留空则放在目标容器根目录下，如：Users 或 安全组"
            style="max-width: 350px;" size="small" clearable />
          <el-button type="primary" size="small" @click="saveRoleGroupContainer" :loading="savingRoleGroupContainer">保存</el-button>
          <span style="font-size: 12px; color: #909399;">角色安全组将统一存放在此 OU/容器下</span>
        </div>

        <el-table :data="mappings" size="small">
          <el-table-column label="对象" width="100" align="center">
            <template #default="{ row }">
              <el-select v-model="row.objectType" size="small">
                <el-option label="用户" value="user" />
                <el-option label="群组" value="group" />
                <el-option label="角色" value="role" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="本地属性" min-width="180">
            <template #default="{ row }">
              <el-select v-model="row.sourceAttribute" size="small" class="full-width" filterable allow-create placeholder="选择或输入">
                <el-option v-for="opt in getDsLocalOptions(row.objectType)" :key="opt.value" :label="opt.label" :value="opt.value" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="" width="50" align="center">
            <template #default><span class="mapping-arrow">&rarr;</span></template>
          </el-table-column>
          <el-table-column label="目标属性" min-width="180">
            <template #default="{ row }">
              <el-select v-model="row.targetAttribute" size="small" class="full-width" filterable allow-create placeholder="选择或输入">
                <el-option v-for="opt in getDsTargetOptions(row.objectType)" :key="opt.value" :label="opt.label" :value="opt.value" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="映射方式" width="130">
            <template #default="{ row }">
              <!-- AD 密码/UPN行：锁定为转换，不可修改 -->
              <template v-if="editingConnType === 'ldap_ad' && (row.sourceAttribute === 'password_raw' || row.targetAttribute === 'unicodePwd' || row.targetAttribute === 'userPrincipalName')">
                <span style="font-size: 12px; color: #c0c4cc;">转换（AD固定）</span>
              </template>
              <el-select v-else v-model="row.mappingType" size="small" class="full-width">
                <el-option label="直接映射" value="mapping" />
                <el-option label="转换" value="transform" />
                <el-option label="常量" value="constant" />
                <el-option label="表达式" value="expression" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="转换规则" min-width="200">
            <template #default="{ row }">
              <!-- AD 密码行：固定显示 password_to_unicode，不可修改 -->
              <template v-if="editingConnType === 'ldap_ad' && (row.sourceAttribute === 'password_raw' || row.targetAttribute === 'unicodePwd')">
                <span style="font-size: 12px; color: #c0c4cc;">密码转Unicode（AD固定规则）</span>
              </template>
              <!-- AD UPN行：固定显示，由连接器 UPN 后缀自动拼接 -->
              <template v-else-if="editingConnType === 'ldap_ad' && row.targetAttribute === 'userPrincipalName'">
                <span style="font-size: 12px; color: #c0c4cc;">自动追加UPN后缀（来自连接器配置）</span>
              </template>
              <template v-else-if="row.mappingType === 'transform'">
                <!-- 用户名相关 -->
                <el-select v-if="row.sourceAttribute === 'nickname' || row.targetAttribute === 'sAMAccountName' || row.targetAttribute === 'uid'" v-model="row.transformRule" size="small" class="full-width" filterable allow-create placeholder="选择或输入规则">
                  <el-option label="中文转拼音" value="pinyin" />
                  <el-option label="邮箱前缀" value="email_prefix" />
                  <el-option label="提取中文姓氏" value="chinese_surname" />
                  <el-option label="提取中文名字" value="chinese_given_name" />
                  <el-option label="追加域名后缀" value="append:@domain.com" />
                  <el-option label="转大写" value="upper" />
                  <el-option label="转小写" value="lower" />
                </el-select>
                <!-- 状态相关 -->
                <el-select v-else-if="row.sourceAttribute === 'status' || row.targetAttribute === 'userAccountControl'" v-model="row.transformRule" size="small" class="full-width" filterable allow-create placeholder="选择或输入">
                  <el-option label="状态转账户控制 - 禁用时AD禁用" value="status_to_uac" />
                  <el-option label="状态转账户控制 - 禁用时AD删除" value="status_to_delete" />
                </el-select>
                <!-- 非AD密码相关 -->
                <el-select v-else-if="row.sourceAttribute === 'password_raw' || row.targetAttribute === 'userPassword'" v-model="row.transformRule" size="small" class="full-width" filterable allow-create placeholder="选择或输入">
                  <el-option label="追加域名后缀" value="append:@domain.com" />
                  <el-option label="转大写" value="upper" />
                  <el-option label="转小写" value="lower" />
                </el-select>
                <!-- 其他字段：通用选项 + 自定义值映射 -->
                <el-input v-else v-model="row.transformRule" size="small" placeholder="如: 源值1=目标值1,源值2=目标值2" clearable />
              </template>
              <template v-else-if="row.mappingType === 'constant'">
                <el-input v-model="row.transformRule" size="small" placeholder="输入常量值" clearable />
              </template>
              <template v-else-if="row.mappingType === 'expression'">
                <el-input v-model="row.transformRule" size="small" placeholder="如: {{.username}}@domain.com" clearable />
              </template>
              <span v-else class="text-muted" style="font-size: 12px; color: #c0c4cc;">{{ row.sourceAttribute }} → {{ row.targetAttribute }}</span>
            </template>
          </el-table-column>
          <el-table-column label="密码格式" width="150">
            <template #default="{ row }">
              <!-- AD 密码行：不显示密码格式选项，AD 固定用 Unicode -->
              <template v-if="editingConnType === 'ldap_ad' && (row.sourceAttribute === 'password_raw' || row.targetAttribute === 'unicodePwd')">
                <span class="text-muted">-</span>
              </template>
              <template v-else-if="row.sourceAttribute === 'password_raw' || row.targetAttribute === 'password' || row.targetAttribute === 'userPassword'">
                <el-select v-model="row.passwordFormat" size="small" class="full-width" placeholder="选择格式" clearable>
                  <el-option-group label="推荐">
                    <el-option label="bcrypt + SHA256" value="bcrypt_sha256" />
                    <el-option label="Argon2id" value="argon2id" />
                    <el-option label="bcrypt" value="bcrypt" />
                    <el-option label="PBKDF2" value="pbkdf2" />
                  </el-option-group>
                  <el-option-group label="通用">
                    <el-option label="SHA256" value="sha256" />
                    <el-option label="SHA512" value="sha512" />
                    <el-option label="SHA384" value="sha384" />
                    <el-option label="SHA3-256" value="sha3_256" />
                    <el-option label="SHA3-512" value="sha3_512" />
                  </el-option-group>
                  <el-option-group label="兼容">
                    <el-option label="LDAP SSHA" value="ssha" />
                    <el-option label="SSHA256" value="ssha256" />
                    <el-option label="NT Hash (AD/Samba)" value="nt_hash" />
                    <el-option label="SCRAM-SHA-256" value="scram_sha256" />
                    <el-option label="Django PBKDF2" value="django" />
                  </el-option-group>
                  <el-option-group label="其他">
                    <el-option label="SHA1 [不推荐]" value="sha1" />
                    <el-option label="MD5 [不推荐]" value="md5" />
                    <el-option label="明文 [不安全]" value="plaintext" />
                    <el-option label="不转换（直接映射）" value="" />
                  </el-option-group>
                </el-select>
              </template>
              <span v-else class="text-muted">-</span>
            </template>
          </el-table-column>
          <el-table-column label="启用" width="60" align="center">
            <template #default="{ row }"><el-switch v-model="row.isEnabled" size="small" /></template>
          </el-table-column>
          <el-table-column label="操作" width="60" align="center">
            <template #default="{ $index }">
              <el-button type="danger" link size="small" @click="mappings.splice($index, 1)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="mapping-footer">
          <el-button type="primary" @click="saveMappings" :loading="savingMappings">保存映射</el-button>
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- ==================== 连接器编辑弹窗 ==================== -->
    <el-dialog v-model="connDialogVisible" :title="connIsEdit ? '编辑下游连接器' : '新增下游连接器'" width="640px" destroy-on-close>
      <el-form :model="connForm" label-width="110px">
        <el-form-item label="连接器名称" required>
          <el-input v-model="connForm.name" placeholder="如：AD域服务器" />
        </el-form-item>
        <el-form-item label="类型" required>
          <el-radio-group v-model="connForm.type" :disabled="connIsEdit" @change="onTypeChange">
            <el-radio-button value="ldap_ad">LDAP AD</el-radio-button>
            <el-radio-button value="ldap_generic">LDAP 通用</el-radio-button>
            <el-radio-button value="database">数据库</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="数据库类型" required v-if="connForm.type === 'database'">
          <el-select v-model="connForm.dbType" class="full-width" @change="onDBTypeChange" :disabled="connIsEdit">
            <el-option label="MySQL" value="mysql" />
            <el-option label="PostgreSQL" value="postgresql" />
            <el-option label="Oracle" value="oracle" />
            <el-option label="SQL Server" value="sqlserver" />
          </el-select>
        </el-form-item>

        <el-divider content-position="left">连接参数</el-divider>

        <el-form-item label="地址" required>
          <div class="addr-row">
            <el-input v-model="connForm.host" placeholder="服务器地址" class="addr-host" />
            <el-input-number v-model="connForm.port" :min="1" :max="65535" class="addr-port" />
          </div>
        </el-form-item>

        <template v-if="connForm.type === 'ldap_ad' || connForm.type === 'ldap_generic'">
          <el-form-item label="LDAPS">
            <el-switch v-model="connForm.useTls" />
            <span class="field-hint" v-if="connForm.type === 'ldap_ad'">AD密码同步必须启用LDAPS</span>
            <span class="field-hint" v-else>启用TLS加密连接</span>
          </el-form-item>
          <el-form-item label="Base DN" required>
            <el-input v-model="connForm.baseDn" placeholder="dc=example,dc=com" />
          </el-form-item>
          <el-form-item label="Bind DN" required>
            <el-input v-model="connForm.bindDn" :placeholder="connForm.type === 'ldap_ad' ? 'cn=administrator,cn=users,dc=example,dc=com' : 'cn=admin,dc=example,dc=com'" />
          </el-form-item>
          <el-form-item label="密码" required>
            <el-input v-model="connForm.bindPassword" type="password" show-password :placeholder="connIsEdit ? '留空不修改' : '请输入密码'" />
          </el-form-item>
          <el-form-item label="UPN后缀" v-if="connForm.type === 'ldap_ad'">
            <el-input v-model="connForm.upnSuffix" placeholder="@example.com" />
          </el-form-item>
        </template>

        <template v-if="connForm.type === 'database'">
          <el-form-item :label="connForm.dbType === 'oracle' ? '服务名' : '数据库名'" required>
            <el-input v-model="connForm.database" placeholder="数据库名" />
          </el-form-item>
          <el-form-item label="用户名" required>
            <el-input v-model="connForm.dbUser" placeholder="数据库用户名" />
          </el-form-item>
          <el-form-item label="密码" required>
            <el-input v-model="connForm.dbPassword" type="password" show-password :placeholder="connIsEdit ? '留空不修改' : '请输入密码'" />
          </el-form-item>
          <el-form-item label="用户表名">
            <el-input v-model="connForm.userTable" placeholder="如: users" />
          </el-form-item>
          <el-form-item label="组织表名">
            <el-input v-model="connForm.groupTable" placeholder="如: departments / groups" />
          </el-form-item>
          <el-form-item label="角色表名">
            <el-input v-model="connForm.roleTable" placeholder="如: roles" />
          </el-form-item>
        </template>

        <el-form-item label="超时(秒)">
          <el-input-number v-model="connForm.timeout" :min="1" :max="60" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="connDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveConn" :loading="connSaving">保存</el-button>
      </template>
    </el-dialog>

    <!-- ==================== 规则编辑弹窗 ==================== -->
    <el-dialog v-model="ruleDialogVisible" :title="ruleIsEdit ? '编辑同步规则' : '新增同步规则'" width="560px" destroy-on-close>
      <el-form :model="ruleForm" label-width="110px">
        <el-form-item label="规则名称" required>
          <el-input v-model="ruleForm.name" placeholder="如：用户同步到AD" />
        </el-form-item>
        <el-form-item label="下游连接器" required>
          <el-select v-model="ruleForm.connectorId" class="full-width">
            <el-option v-for="c in connectors" :key="c.id" :label="c.name + ' (' + typeLabel(c) + ')'" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="目标容器" v-if="false">
          <el-input v-model="ruleForm.targetContainer" />
        </el-form-item>
        <el-form-item label="安全组容器" v-if="selectedConnType === 'ldap_ad'">
          <el-input v-model="ruleForm.roleGroupContainer" placeholder="如：安全组（留空则放在目标容器根目录下）" />
          <span class="field-hint">角色安全组将统一存放在此 OU 下，留空则与部门 OU 混放</span>
        </el-form-item>
        <el-form-item label="事件触发">
          <el-switch v-model="ruleForm.enableEvent" />
          <span class="field-hint">用户变更时自动触发同步</span>
        </el-form-item>
        <el-form-item label="定时同步" v-if="!isSelectedConnectorDatabase">
          <el-switch v-model="ruleForm.enableSchedule" />
        </el-form-item>
        <el-form-item v-if="isSelectedConnectorDatabase" label="定时同步">
          <el-tag type="info" size="small" effect="light">不支持</el-tag>
          <span class="field-hint">数据库连接器仅支持事件同步和手动同步</span>
        </el-form-item>
        <template v-if="ruleForm.enableSchedule && !isSelectedConnectorDatabase">
          <el-form-item label="调度模式">
            <el-radio-group v-model="ruleForm.scheduleType">
              <el-radio value="times">定点时间</el-radio>
              <el-radio value="interval">固定间隔</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="自动同步时间" v-if="ruleForm.scheduleType === 'times'">
            <div class="schedule-times-list">
              <div v-for="(t, idx) in ruleForm.scheduleTimes" :key="idx" class="schedule-time-row">
                <el-time-picker v-model="ruleForm.scheduleTimes[idx]" format="HH:mm" value-format="HH:mm" placeholder="选择时间" style="width: 140px;" />
                <el-button type="danger" size="small" @click="ruleForm.scheduleTimes.splice(idx, 1)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
              <el-button type="primary" size="small" @click="ruleForm.scheduleTimes.push('08:00')">
                <el-icon><Plus /></el-icon>
              </el-button>
            </div>
          </el-form-item>
          <el-form-item label="同步间隔(分钟)" v-if="ruleForm.scheduleType === 'interval'">
            <el-input-number v-model="ruleForm.scheduleInterval" :min="5" :max="1440" :step="5" />
          </el-form-item>
        </template>
        <el-form-item label="状态">
          <el-switch v-model="ruleForm.statusBool" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ruleDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveRule" :loading="ruleSaving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Plus, Delete, Refresh } from "@element-plus/icons-vue";
import { syncApi } from "../../api";

const dbTypeLabels: Record<string, string> = { mysql: 'MySQL', postgresql: 'PostgreSQL', oracle: 'Oracle', sqlserver: 'SQL Server' };
const dbTypePorts: Record<string, number> = { mysql: 3306, postgresql: 5432, oracle: 1521, sqlserver: 1433 };

const formatScheduleTimes = (raw: string) => {
  if (!raw) return '-';
  try {
    const arr = JSON.parse(raw);
    if (Array.isArray(arr)) return arr.join(', ');
  } catch {}
  return raw;
};

const typeLabel = (row: any) => {
  if (row.type === 'ldap_ad') return 'LDAP AD';
  if (row.type === 'ldap_generic') return 'LDAP 通用';
  return dbTypeLabels[row.dbType || row.type] || row.type;
};

const activeTab = ref('connectors');

// ===== 连接器 =====
const connectors = ref<any[]>([]);
const connLoading = ref(false);
const testingId = ref(0);
const connDialogVisible = ref(false);
const connIsEdit = ref(false);
const connEditingId = ref(0);
const connSaving = ref(false);

const defaultConnForm = {
  name: '', type: 'ldap_ad', direction: 'downstream',
  host: '', port: 636, useTls: true,
  baseDn: '', bindDn: '', bindPassword: '', upnSuffix: '',
  database: '', dbUser: '', dbPassword: '', dbType: 'mysql',
  userTable: '', groupTable: '', roleTable: '', timeout: 5
};
const connForm = ref({ ...defaultConnForm });

const baseDnToUpn = (baseDn: string): string => {
  if (!baseDn) return '';
  const parts = baseDn.toLowerCase().split(',')
    .map(p => p.trim())
    .filter(p => p.startsWith('dc='))
    .map(p => p.replace('dc=', ''));
  return parts.length > 0 ? '@' + parts.join('.') : '';
};

watch(() => connForm.value.baseDn, (newVal) => {
  if (connForm.value.type === 'ldap_ad' && !connIsEdit.value) {
    connForm.value.upnSuffix = baseDnToUpn(newVal);
  }
});

const loadConnectors = async () => {
  connLoading.value = true;
  try {
    const res = await syncApi.downstreamConnectors();
    connectors.value = (res as any).data?.data || [];
  } finally { connLoading.value = false; }
};

const openConnDialog = (row?: any) => {
  connForm.value = { ...defaultConnForm };
  if (row) {
    connIsEdit.value = true;
    connEditingId.value = row.id;
    const t = row.type === 'mysql' ? 'database' : row.type;
    Object.assign(connForm.value, {
      name: row.name, type: t, direction: 'downstream',
      host: row.host, port: row.port, useTls: row.useTls,
      baseDn: row.baseDn, bindDn: row.bindDn, bindPassword: '',
      upnSuffix: row.upnSuffix || baseDnToUpn(row.baseDn || ''),
      database: row.database, dbUser: row.dbUser, dbPassword: '',
      dbType: row.dbType || 'mysql', userTable: row.userTable, groupTable: row.groupTable || '', roleTable: row.roleTable || '', timeout: row.timeout
    });
  } else {
    connIsEdit.value = false;
    connEditingId.value = 0;
  }
  connDialogVisible.value = true;
};

const onTypeChange = (val: string) => {
  connForm.value.port = (val === 'ldap_ad' || val === 'ldap_generic') ? (connForm.value.useTls ? 636 : 389) : (dbTypePorts[connForm.value.dbType] || 3306);
};
const onDBTypeChange = (val: string) => {
  connForm.value.port = dbTypePorts[val] || 3306;
};

const saveConn = async () => {
  if (!connForm.value.name || !connForm.value.host) { ElMessage.warning('请填写必填项'); return; }
  connSaving.value = true;
  try {
    if (connIsEdit.value) {
      await syncApi.updateDownstreamConnector(connEditingId.value, connForm.value);
    } else {
      await syncApi.createDownstreamConnector(connForm.value);
    }
    ElMessage.success('保存成功');
    connDialogVisible.value = false;
    loadConnectors();
  } finally { connSaving.value = false; }
};

const testConn = async (row: any) => {
  testingId.value = row.id;
  try {
    const res = await syncApi.testDownstreamConnector(row.id);
    const d = (res as any).data?.data;
    if (d?.ok) {
      ElMessage.success(d?.message || '连接成功');
    } else {
      ElMessage.error(d?.message || '连接失败');
    }
    loadConnectors();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '连接失败');
    loadConnectors();
  } finally { testingId.value = 0; }
};

const deleteConn = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确定删除连接器「${row.name}」？`, '确认删除', { type: 'warning' });
    await syncApi.deleteDownstreamConnector(row.id);
    ElMessage.success('删除成功');
    loadConnectors();
  } catch {}
};

// ===== 同步规则 =====
const rules = ref<any[]>([]);
const ruleLoading = ref(false);
const triggeringId = ref(0);
const ruleDialogVisible = ref(false);
const ruleIsEdit = ref(false);
const ruleEditingId = ref(0);
const ruleSaving = ref(false);

const allSyncEvents = ["user_create","user_update","user_delete","user_enable","user_disable","password_change","group_change","role_change"];
const defaultRuleForm = {
  name: '', connectorId: 0, direction: 'downstream',
  targetContainer: '', roleGroupContainer: '',
  enableEvent: true, enableSchedule: false,
  events: [...allSyncEvents] as string[],
  syncUsers: true, syncGroups: true, syncRoles: true,
  scheduleType: 'times' as string,
  scheduleTimes: [] as string[],
  scheduleInterval: 60,
  statusBool: true
};
const ruleForm = ref({ ...defaultRuleForm });

const parseScheduleTimes = (raw: string): string[] => {
  if (!raw) return [];
  try {
    const arr = JSON.parse(raw);
    if (Array.isArray(arr)) return arr;
  } catch {}
  if (raw.includes(',')) return raw.split(',').map((s: string) => s.trim()).filter(Boolean);
  return raw ? [raw] : [];
};

const selectedConnType = computed(() => {
  const c = connectors.value.find((x: any) => x.id === ruleForm.value.connectorId);
  return c?.type || '';
});

const isSelectedConnectorDatabase = computed(() => {
  const ct = selectedConnType.value;
  return ct === 'database' || ct === 'mysql' || ct?.startsWith('db_');
});

const loadRules = async () => {
  ruleLoading.value = true;
  try {
    const res = await syncApi.downstreamRules();
    rules.value = (res as any).data?.data || [];
  } finally { ruleLoading.value = false; }
};

const openRuleDialog = (row?: any) => {
  ruleForm.value = { ...defaultRuleForm };
  if (row) {
    ruleIsEdit.value = true;
    ruleEditingId.value = row.id;
    // 解析 events：后端存的是 JSON 字符串，需要还原为数组
    let rowEvents = [...allSyncEvents]; // 默认全部事件
    if (row.events) {
      try {
        const parsed = typeof row.events === 'string' ? JSON.parse(row.events) : row.events;
        if (Array.isArray(parsed) && parsed.length > 0) rowEvents = parsed;
      } catch {}
    }
    ruleForm.value = {
      name: row.name, connectorId: row.connectorId, direction: 'downstream',
      targetContainer: row.targetContainer || '',
      roleGroupContainer: row.roleGroupContainer || '',
      enableEvent: row.enableEvent ?? true,
      enableSchedule: row.enableSchedule ?? false,
      events: rowEvents,
      syncUsers: row.syncUsers ?? true,
      syncGroups: row.syncGroups ?? true,
      syncRoles: row.syncRoles ?? true,
      scheduleType: row.scheduleType || 'times',
      scheduleTimes: parseScheduleTimes(row.scheduleTime),
      scheduleInterval: row.scheduleInterval || 60,
      statusBool: row.status === 1
    };
  } else {
    ruleIsEdit.value = false;
    ruleEditingId.value = 0;
    if (connectors.value.length > 0) ruleForm.value.connectorId = connectors.value[0].id;
  }
  ruleDialogVisible.value = true;
};

const saveRule = async () => {
  if (!ruleForm.value.name || !ruleForm.value.connectorId) { ElMessage.warning('请填写必填项'); return; }
  ruleSaving.value = true;
  try {
    const payload = { ...ruleForm.value, status: ruleForm.value.statusBool ? 1 : 0 };
    if (ruleIsEdit.value) {
      await syncApi.updateDownstreamRule(ruleEditingId.value, payload);
    } else {
      await syncApi.createDownstreamRule(payload);
    }
    ElMessage.success('保存成功');
    ruleDialogVisible.value = false;
    loadRules();
  } finally { ruleSaving.value = false; }
};

const triggerRule = async (row: any) => {
  const conn = connectors.value.find((c: any) => c.id === row.connectorId);
  const isDB = conn?.type === 'database' || conn?.type === 'mysql' || conn?.type?.startsWith('db_');
  
  if (isDB) {
    try {
      await ElMessageBox.confirm(
        `<div style="line-height: 1.8;">
          <p style="margin-bottom: 12px;"><strong>⚠️ 密码同步说明</strong></p>
          <p>手动同步时，密码将采取 <strong>系统默认加密方式</strong> 同步至下游数据库。</p>
          <p style="margin-top: 8px;">当前系统密码策略：<el-tag size="small" effect="light">bcrypt + SHA256</el-tag></p>
          <p style="margin-top: 12px; color: #909399; font-size: 12px;">如需转换为其他格式，请使用【事件同步】并配置属性映射的密码格式转换。</p>
        </div>`,
        '同步确认',
        {
          dangerouslyUseHTMLString: true,
          confirmButtonText: '确认同步',
          cancelButtonText: '取消',
          type: 'warning'
        }
      );
    } catch {
      return;
    }
  }
  
  triggeringId.value = row.id;
  try {
    await syncApi.triggerDownstreamRule(row.id);
    ElMessage.success('下游同步已触发');
    setTimeout(loadRules, 3000);
  } finally { triggeringId.value = 0; }
};

const deleteRule = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确定删除规则「${row.name}」？`, '确认删除', { type: 'warning' });
    await syncApi.deleteDownstreamRule(row.id);
    ElMessage.success('删除成功');
    loadRules();
  } catch {}
};

// ===== 属性映射选项（中文标签） =====
// 下游本地属性（源）
const dsLocalOptions = [
  { value: 'name', label: '姓名 (name)' },
  { value: 'username', label: '用户名 (username)' },
  { value: 'nickname', label: '昵称 (nickname)' },
  { value: 'email', label: '邮箱 (email)' },
  { value: 'phone', label: '手机号 (phone)' },
  { value: 'avatar', label: '头像 (avatar)' },
  { value: 'position', label: '职位 (position)' },
  { value: 'department', label: '部门 (department)' },
  { value: 'group_name', label: '群组名称 (group_name)' },
  { value: 'description', label: '描述 (description)' },
  { value: 'status', label: '状态 (status)' },
  { value: 'password_raw', label: '明文密码 (password_raw)' },
  { value: 'code', label: '工号 (code)' },
  { value: 'job_title', label: '岗位 (job_title)' },
];
// 下游目标属性 - AD
const dsTargetOptionsAD = [
  { value: 'sAMAccountName', label: '登录名 (sAMAccountName)' },
  { value: 'cn', label: '通用名 (cn)' },
  { value: 'displayName', label: '显示名 (displayName)' },
  { value: 'mail', label: '邮箱 (mail)' },
  { value: 'mobile', label: '手机号 (mobile)' },
  { value: 'telephoneNumber', label: '电话 (telephoneNumber)' },
  { value: 'title', label: '职位 (title)' },
  { value: 'department', label: '部门 (department)' },
  { value: 'description', label: '描述 (description)' },
  { value: 'givenName', label: '名 (givenName)' },
  { value: 'sn', label: '姓 (sn)' },
  { value: 'ou', label: '组织单元 (ou)' },
  { value: 'userPrincipalName', label: 'UPN (userPrincipalName)' },
  { value: 'unicodePwd', label: 'Unicode密码 (unicodePwd)' },
  { value: 'userAccountControl', label: '账户控制 (userAccountControl)' },
];
// 下游目标属性 - 通用 LDAP
const dsTargetOptionsGenericLDAP = [
  { value: 'uid', label: '用户ID (uid)' },
  { value: 'cn', label: '通用名 (cn)' },
  { value: 'displayName', label: '显示名 (displayName)' },
  { value: 'sn', label: '姓 (sn)' },
  { value: 'givenName', label: '名 (givenName)' },
  { value: 'mail', label: '邮箱 (mail)' },
  { value: 'telephoneNumber', label: '电话 (telephoneNumber)' },
  { value: 'title', label: '职位 (title)' },
  { value: 'ou', label: '组织单元 (ou)' },
  { value: 'userPassword', label: '密码 (userPassword)' },
  { value: 'description', label: '描述 (description)' },
  { value: 'employeeNumber', label: '工号 (employeeNumber)' },
  { value: 'postalAddress', label: '地址 (postalAddress)' },
];
// 下游目标属性 - 数据库（可自定义输入）
const dsTargetOptionsDB = [
  { value: 'username', label: '用户名 (username)' },
  { value: 'password', label: '密码 (password)' },
  { value: 'display_name', label: '显示名 (display_name)' },
  { value: 'email', label: '邮箱 (email)' },
  { value: 'phone', label: '手机号 (phone)' },
  { value: 'department', label: '部门 (department)' },
  { value: 'position', label: '职位 (position)' },
  { value: 'status', label: '状态 (status)' },
  { value: 'created_at', label: '创建时间 (created_at)' },
];
// 根据当前编辑的规则连接器类型获取目标属性选项
const currentDsTargetOptions = computed(() => {
  const ct = editingConnType.value;
  if (ct === 'ldap_ad') return dsTargetOptionsAD;
  if (ct === 'ldap_generic') return dsTargetOptionsGenericLDAP;
  return dsTargetOptionsDB;
});
// 群组本地属性
const dsLocalGroupOptions = [
  { value: 'name', label: '群组名称 (name)' },
  { value: 'description', label: '描述 (description)' },
  { value: 'parent_name', label: '上级群组名称 (parent_name)' },
];
// 角色本地属性
const dsLocalRoleOptions = [
  { value: 'name', label: '角色名称 (name)' },
  { value: 'code', label: '角色编码 (code)' },
  { value: 'description', label: '描述 (description)' },
];
// 群组目标属性 - AD
const dsTargetGroupOptionsAD = [
  { value: 'ou', label: '组织单元 (ou)' },
  { value: 'description', label: '描述 (description)' },
  { value: 'name', label: '名称 (name)' },
];
// 群组目标属性 - 通用 LDAP
const dsTargetGroupOptionsGenericLDAP = [
  { value: 'ou', label: '组织单元 (ou)' },
  { value: 'description', label: '描述 (description)' },
];
// 群组目标属性 - 数据库
const dsTargetGroupOptionsDB = [
  { value: 'name', label: '名称 (name)' },
  { value: 'description', label: '描述 (description)' },
  { value: 'parent_id', label: '上级ID (parent_id)' },
];
// 角色目标属性
const dsTargetRoleOptionsDB = [
  { value: 'name', label: '名称 (name)' },
  { value: 'code', label: '编码 (code)' },
  { value: 'description', label: '描述 (description)' },
];

// 根据 objectType 获取本地属性选项
const getDsLocalOptions = (objectType: string) => {
  if (objectType === 'group') return dsLocalGroupOptions;
  if (objectType === 'role') return dsLocalRoleOptions;
  return dsLocalOptions;
};
// 根据 objectType + 连接器类型获取目标属性选项
const getDsTargetOptions = (objectType: string) => {
  const ct = editingConnType.value;
  if (objectType === 'group') {
    if (ct === 'ldap_ad') return dsTargetGroupOptionsAD;
    if (ct === 'ldap_generic') return dsTargetGroupOptionsGenericLDAP;
    return dsTargetGroupOptionsDB;
  }
  if (objectType === 'role') return dsTargetRoleOptionsDB;
  // user
  if (ct === 'ldap_ad') return dsTargetOptionsAD;
  if (ct === 'ldap_generic') return dsTargetOptionsGenericLDAP;
  return dsTargetOptionsDB;
};
// 兼容旧引用
const dsTargetOptions = dsTargetOptionsAD;
// 下游转换规则选项（中文标签）
const dsTransformOptions = [
  { value: 'chinese_surname', label: '提取中文姓氏' },
  { value: 'chinese_given_name', label: '提取中文名字' },
  { value: 'password_to_unicode', label: '密码转Unicode（AD）' },
  { value: 'status_to_uac', label: '状态转账户控制 - 禁用时AD禁用' },
  { value: 'status_to_delete', label: '状态转账户控制 - 禁用时AD删除' },
  { value: 'append:@domain.com', label: '追加域名后缀' },
  { value: 'pinyin', label: '中文转拼音' },
  { value: 'email_prefix', label: '邮箱前缀' },
  { value: 'to_upper', label: '转大写' },
  { value: 'to_lower', label: '转小写' },
];

// ===== 属性映射 =====
const editingRuleId = ref(0);
const editingRuleName = ref('');
const editingConnType = ref('ldap_ad');
const editingRoleGroupContainer = ref('');
const mappings = ref<any[]>([]);
const savingMappings = ref(false);

const editRuleDetail = async (row: any) => {
  editingRuleId.value = row.id;
  editingRuleName.value = row.name;
  editingRoleGroupContainer.value = row.roleGroupContainer || '';
  const conn = connectors.value.find((c: any) => c.id === row.connectorId);
  editingConnType.value = conn?.type || 'ldap_ad';
  activeTab.value = 'mappings';
  try {
    const res = await syncApi.downstreamRuleMappings(row.id);
    mappings.value = (res as any).data?.data || [];
  } catch { mappings.value = []; }
};

const exitMappings = () => {
  editingRuleId.value = 0;
  activeTab.value = 'rules';
};

const savingRoleGroupContainer = ref(false);
const saveRoleGroupContainer = async () => {
  savingRoleGroupContainer.value = true;
  try {
    await syncApi.updateDownstreamRule(editingRuleId.value, { roleGroupContainer: editingRoleGroupContainer.value });
    ElMessage.success('安全组容器已保存');
    loadRules();
  } catch { ElMessage.error('保存失败'); }
  finally { savingRoleGroupContainer.value = false; }
};

const addMapping = () => {
  mappings.value.push({
    sourceAttribute: '', targetAttribute: '',
    mappingType: 'mapping', transformRule: '',
    objectType: 'user', isEnabled: true, priority: (mappings.value.length || 0) + 1,
    passwordFormat: ''
  });
};

const saveMappings = async () => {
  savingMappings.value = true;
  try {
    let p = 0;
    for (const m of mappings.value) m.priority = p++;
    await syncApi.updateDownstreamRuleMappings(editingRuleId.value, mappings.value);
    ElMessage.success('映射已保存');
  } finally { savingMappings.value = false; }
};

onMounted(() => { loadConnectors(); loadRules(); });
</script>

<style scoped>
/* ============================================
   Corporate Downstream Sync - 企业风格
   ============================================ */
.downstream-page { display: flex; flex-direction: column; gap: var(--spacing-lg); width: 100%; }

/* Tab 工具栏：描述文字和按钮两端对齐 */
.tab-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  padding: 0;
}
.tab-desc {
  color: var(--color-text-secondary);
  font-size: 14px;
}

.health-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  background: var(--color-text-quaternary);
}
.health-dot.online { background: var(--color-success); box-shadow: 0 0 0 3px rgba(82,196,26,0.15); }
.health-dot.offline { background: var(--color-error); box-shadow: 0 0 0 3px rgba(255,77,79,0.15); }
.mapping-arrow { font-size: var(--font-size-lg); color: var(--color-primary); font-weight: 700; }
.mapping-footer { margin-top: var(--spacing-lg); display: flex; justify-content: flex-end; }
.schedule-times-list { display: flex; flex-direction: column; gap: 8px; }
.schedule-time-row { display: flex; align-items: center; gap: 8px; }
</style>
