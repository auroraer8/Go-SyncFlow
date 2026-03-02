<template>
  <div class="connectors-page">
    <!-- 统计卡片 -->
    <div class="stat-row">
      <div class="stat-card">
        <span class="stat-value">{{ list.length }}</span>
        <span class="stat-label">连接器总数</span>
      </div>
      <div class="stat-card stat-success">
        <span class="stat-value">{{ list.filter(r => r.lastTestOk).length }}</span>
        <span class="stat-label">在线</span>
      </div>
      <div class="stat-card stat-error">
        <span class="stat-value">{{ list.filter(r => r.lastTestAt && !r.lastTestOk).length }}</span>
        <span class="stat-label">异常</span>
      </div>
    </div>

    <el-card>
      <template #header>
        <div class="card-header-row">
          <span class="card-title">连接器管理</span>
          <el-button type="primary" @click="openDialog()">
            <el-icon><Plus /></el-icon> 新增连接器
          </el-button>
        </div>
      </template>

      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="name" label="名称" min-width="160">
          <template #default="{ row }">
            <span class="conn-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="type" label="类型" width="140" align="center">
          <template #default="{ row }">
            <el-tag :type="row.type === 'ldap_ad' ? 'primary' : 'success'" size="small" effect="light">
              {{ connectorTypeLabel(row) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="地址" min-width="200">
          <template #default="{ row }">
            <span class="conn-addr">{{ row.host }}:{{ row.port }}</span>
          </template>
        </el-table-column>
        <el-table-column label="健康状态" width="160" align="center">
          <template #default="{ row }">
            <div class="health-status">
              <span class="health-dot" :class="{ online: row.lastTestOk, offline: row.lastTestAt && !row.lastTestOk }"></span>
              <span>{{ row.lastTestOk ? '在线' : (row.lastTestAt ? '异常' : '未测试') }}</span>
              <span class="health-sep">·</span>
              <span :class="row.status === 1 ? 'text-success' : 'text-error'">{{ row.status === 1 ? '启用' : '禁用' }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-tooltip content="测试连接" placement="top">
              <el-button type="primary" link size="small" @click="testConn(row)" :loading="testingId === row.id">
                <el-icon><Connection /></el-icon>
              </el-button>
            </el-tooltip>
            <el-button type="primary" link size="small" @click="openDialog(row)">编辑</el-button>
            <el-dropdown trigger="click" @command="(cmd: string) => handleMore(cmd, row)">
              <el-button type="info" link size="small">更多</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="delete" class="text-error">删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无连接器，点击上方按钮新增" :image-size="120" />
        </template>
      </el-table>
    </el-card>

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑连接器' : '新增连接器'" width="640px" destroy-on-close>
      <el-form :model="form" label-width="110px">
        <el-form-item label="连接器名称" required>
          <el-input v-model="form.name" placeholder="如：AD域服务器" />
        </el-form-item>
        <el-form-item label="类型" required>
          <el-radio-group v-model="form.type" :disabled="isEdit" @change="onTypeChange">
            <el-radio-button value="ldap_ad">LDAP AD</el-radio-button>
            <el-radio-button value="database">数据库</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="数据库类型" required v-if="form.type === 'database'">
          <el-select v-model="form.dbType" class="full-width" @change="onDBTypeChange" :disabled="isEdit">
            <el-option label="MySQL" value="mysql" />
            <el-option label="PostgreSQL" value="postgresql" />
            <el-option label="Oracle" value="oracle" />
            <el-option label="SQL Server" value="sqlserver" />
          </el-select>
        </el-form-item>

        <el-divider content-position="left">连接参数</el-divider>

        <el-form-item label="地址" required>
          <div class="addr-row">
            <el-input v-model="form.host" placeholder="服务器地址" class="addr-host" />
            <el-input-number v-model="form.port" :min="1" :max="65535" class="addr-port" />
          </div>
        </el-form-item>

        <template v-if="form.type === 'ldap_ad'">
          <el-form-item label="备用地址">
            <div class="addr-row">
              <el-input v-model="form.backupHost" placeholder="备用服务器" class="addr-host" />
              <el-input-number v-model="form.backupPort" :min="1" :max="65535" class="addr-port" />
            </div>
          </el-form-item>
          <el-form-item label="LDAPS">
            <el-switch v-model="form.useTls" />
            <span class="field-hint">AD密码同步必须启用LDAPS</span>
          </el-form-item>
          <el-form-item label="Base DN" required>
            <el-input v-model="form.baseDn" placeholder="dc=example,dc=com" />
          </el-form-item>
          <el-form-item label="Bind DN" required>
            <el-input v-model="form.bindDn" placeholder="cn=administrator,cn=users,dc=example,dc=com" />
          </el-form-item>
          <el-form-item label="密码" required>
            <el-input v-model="form.bindPassword" type="password" show-password :placeholder="isEdit ? '留空不修改' : '请输入密码'" />
          </el-form-item>
          <el-form-item label="UPN后缀">
            <el-input v-model="form.upnSuffix" placeholder="@example.com" />
          </el-form-item>
          <el-form-item label="用户过滤">
            <el-input v-model="form.userFilter" placeholder="(&(objectClass=user)(objectCategory=person))" />
          </el-form-item>
        </template>

        <template v-if="form.type === 'database'">
          <el-form-item :label="form.dbType === 'oracle' ? '服务名' : '数据库名'" required>
            <el-input v-model="form.database" :placeholder="form.dbType === 'oracle' ? 'ORCL' : '数据库名'" />
          </el-form-item>
          <el-form-item label="Service Name" v-if="form.dbType === 'oracle'">
            <el-input v-model="form.serviceName" placeholder="Oracle Service Name（可选，默认取数据库名）" />
          </el-form-item>
          <el-form-item label="用户名" required>
            <el-input v-model="form.dbUser" placeholder="数据库用户名" />
          </el-form-item>
          <el-form-item label="密码" required>
            <el-input v-model="form.dbPassword" type="password" show-password :placeholder="isEdit ? '留空不修改' : '请输入密码'" />
          </el-form-item>
          <el-form-item label="字符集" v-if="form.dbType === 'mysql'">
            <el-input v-model="form.charset" placeholder="utf8mb4" />
          </el-form-item>
          <el-form-item label="用户表名">
            <el-input v-model="form.userTable" placeholder="如: users" />
          </el-form-item>
          <el-form-item label="分组表名">
            <el-input v-model="form.groupTable" placeholder="如: departments" />
          </el-form-item>
          <el-form-item label="角色表名">
            <el-input v-model="form.roleTable" placeholder="如: roles" />
          </el-form-item>
          <el-form-item label="密码格式">
            <el-select v-model="form.pwdFormat" class="full-width">
              <el-option label="bcrypt" value="bcrypt" />
              <el-option label="MD5" value="md5" />
              <el-option label="SHA256" value="sha256" />
              <el-option label="明文" value="plain" />
            </el-select>
          </el-form-item>
        </template>

        <el-form-item label="超时(秒)">
          <el-input-number v-model="form.timeout" :min="1" :max="60" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveConnector" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Plus, Connection } from "@element-plus/icons-vue";
import { connectorApi } from "../../api";

const list = ref<any[]>([]);
const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const isEdit = ref(false);
const editingId = ref(0);
const testingId = ref(0);

const dbTypeLabels: Record<string, string> = {
  mysql: "MySQL", postgresql: "PostgreSQL", oracle: "Oracle", sqlserver: "SQL Server"
};

const dbTypeDefaultPorts: Record<string, number> = {
  mysql: 3306, postgresql: 5432, oracle: 1521, sqlserver: 1433
};

const connectorTypeLabel = (row: any) => {
  if (row.type === "ldap_ad") return "LDAP AD";
  if (row.type === "database" || row.type === "mysql") {
    const dt = row.dbType || (row.type === "mysql" ? "mysql" : "mysql");
    return dbTypeLabels[dt] || "数据库";
  }
  return row.type;
};

const defaultForm = {
  name: "", type: "ldap_ad", host: "", port: 636,
  backupHost: "", backupPort: 389, useTls: true,
  baseDn: "", bindDn: "", bindPassword: "",
  database: "", dbUser: "", dbPassword: "",
  dbType: "mysql", serviceName: "",
  charset: "utf8mb4", userTable: "", groupTable: "", roleTable: "",
  pwdFormat: "bcrypt", timeout: 5, upnSuffix: "", userFilter: ""
};

const onTypeChange = (val: string) => {
  if (val === "ldap_ad") {
    form.port = form.useTls ? 636 : 389;
  } else {
    form.port = dbTypeDefaultPorts[form.dbType] || 3306;
  }
};

const onDBTypeChange = (val: string) => {
  form.port = dbTypeDefaultPorts[val] || 3306;
  if (val !== "mysql") {
    form.charset = "";
  } else {
    form.charset = "utf8mb4";
  }
};

const form = reactive({ ...defaultForm });

const loadList = async () => {
  loading.value = true;
  try {
    const res = await connectorApi.list();
    list.value = (res as any).data?.data || [];
  } finally {
    loading.value = false;
  }
};

const openDialog = (row?: any) => {
  Object.assign(form, defaultForm);
  if (row) {
    isEdit.value = true;
    editingId.value = row.id;
    // 兼容旧类型 "mysql" -> "database"
    const connType = row.type === "mysql" ? "database" : row.type;
    const dbType = row.dbType || (row.type === "mysql" ? "mysql" : "mysql");
    Object.assign(form, {
      name: row.name, type: connType, host: row.host, port: row.port,
      backupHost: row.backupHost, backupPort: row.backupPort, useTls: row.useTls,
      baseDn: row.baseDn, bindDn: row.bindDn, bindPassword: "",
      database: row.database, dbUser: row.dbUser, dbPassword: "",
      dbType: dbType, serviceName: row.serviceName || "",
      charset: row.charset, userTable: row.userTable, groupTable: row.groupTable, roleTable: row.roleTable,
      pwdFormat: row.pwdFormat, timeout: row.timeout, upnSuffix: row.upnSuffix, userFilter: row.userFilter
    });
  } else {
    isEdit.value = false;
    editingId.value = 0;
  }
  dialogVisible.value = true;
};

const saveConnector = async () => {
  if (!form.name || !form.host) {
    ElMessage.warning("请填写必填项");
    return;
  }
  saving.value = true;
  try {
    if (isEdit.value) {
      await connectorApi.update(editingId.value, { ...form });
    } else {
      await connectorApi.create({ ...form });
    }
    ElMessage.success("保存成功");
    dialogVisible.value = false;
    loadList();
  } finally {
    saving.value = false;
  }
};

const testConn = async (row: any) => {
  testingId.value = row.id;
  try {
    const res = await connectorApi.test(row.id);
    ElMessage.success((res as any).data?.data?.message || "连接成功");
    loadList();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || "连接失败");
    loadList();
  } finally {
    testingId.value = 0;
  }
};

const handleMore = (cmd: string, row: any) => {
  if (cmd === 'delete') deleteConn(row);
};

const deleteConn = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确定删除连接器「${row.name}」？删除后，依赖此连接器的同步器将无法工作。`, "确认删除", { type: "warning" });
    await connectorApi.delete(row.id);
    ElMessage.success("删除成功");
    loadList();
  } catch {}
};

onMounted(loadList);
</script>

<style scoped>
/* ============================================
   Corporate Connectors - 企业风格
   ============================================ */
.connectors-page { display: flex; flex-direction: column; gap: var(--spacing-lg);  }
.stat-card:hover { box-shadow: var(--card-shadow-hover); }
.stat-value { font-size: var(--font-size-2xl); font-weight: 700; color: var(--color-text-primary); }
.stat-label { font-size: var(--font-size-sm); color: var(--color-text-tertiary); }
.stat-success .stat-value { color: var(--color-success); }
.stat-error .stat-value { color: var(--color-error); }

/* 卡片头 */
.card-header-row { display: flex; justify-content: space-between; align-items: center; }
.card-title { font-size: var(--font-size-lg); font-weight: 600; color: var(--color-text-primary); }

/* 连接器名称 */
.conn-name { font-weight: 500; color: var(--color-text-primary); }
.conn-addr { color: var(--color-text-secondary); font-family: monospace; font-size: var(--font-size-sm); }

/* 健康状态 */
.health-status { display: flex; align-items: center; gap: 6px; font-size: var(--font-size-sm); color: var(--color-text-secondary); }
.health-dot {
  width: 8px; height: 8px; border-radius: 50%; background: var(--color-text-quaternary); flex-shrink: 0;
}
.health-dot.online { background: var(--color-success); box-shadow: 0 0 0 3px rgba(82, 196, 26, 0.15); }
.health-dot.offline { background: var(--color-error); box-shadow: 0 0 0 3px rgba(255, 77, 79, 0.15); }
.health-sep { color: var(--color-text-quaternary); }

/* 表单 */
.addr-row { display: flex; gap: var(--spacing-sm); }
.addr-host { flex: 1; }
.addr-port { width: 120px; }
.field-hint { margin-left: var(--spacing-sm); color: var(--color-text-tertiary); font-size: var(--font-size-xs); }
.full-width { width: 100%; }
</style>
