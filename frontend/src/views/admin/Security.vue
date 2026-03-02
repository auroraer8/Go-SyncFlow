<template>
  <div class="security-page">
    <el-tabs v-model="activeTab" type="border-card">
      <!-- 安全仪表板 -->
      <el-tab-pane label="安全仪表板" name="dashboard">
        <div class="dashboard-content" v-loading="dashboardLoading">
          <!-- 概览卡片 -->
          <div class="overview-cards">
            <div class="overview-card">
              <div class="card-icon users"><el-icon><User /></el-icon></div>
              <div class="card-info">
                <div class="card-value">{{ dashboard.overview?.totalUsers || 0 }}</div>
                <div class="card-label">用户总数</div>
              </div>
            </div>
            <div class="overview-card">
              <div class="card-icon sessions"><el-icon><Connection /></el-icon></div>
              <div class="card-info">
                <div class="card-value">{{ dashboard.overview?.activeSessions || 0 }}</div>
                <div class="card-label">活跃会话</div>
              </div>
            </div>
            <div class="overview-card warning">
              <div class="card-icon"><el-icon><Warning /></el-icon></div>
              <div class="card-info">
                <div class="card-value">{{ dashboard.overview?.blockedIPs || 0 }}</div>
                <div class="card-label">封禁IP</div>
              </div>
            </div>
            <div class="overview-card">
              <div class="card-icon events"><el-icon><Bell /></el-icon></div>
              <div class="card-info">
                <div class="card-value">{{ dashboard.overview?.eventsToday || 0 }}</div>
                <div class="card-label">今日事件</div>
              </div>
            </div>
            <div class="overview-card danger">
              <div class="card-icon"><el-icon><Lock /></el-icon></div>
              <div class="card-info">
                <div class="card-value">{{ dashboard.overview?.failedLogins24h || 0 }}</div>
                <div class="card-label">24h失败登录</div>
              </div>
            </div>
            <div class="overview-card" :class="{ success: (dashboard.overview?.securityScore || 0) >= 80 }">
              <div class="card-icon score"><el-icon><TrendCharts /></el-icon></div>
              <div class="card-info">
                <div class="card-value">{{ dashboard.overview?.securityScore || 0 }}</div>
                <div class="card-label">安全评分</div>
              </div>
            </div>
          </div>

          <!-- 图表区域 -->
          <el-row :gutter="20" class="chart-row">
            <el-col :span="14">
              <el-card>
                <template #header>登录趋势 (24小时)</template>
                <div ref="loginTrendChart" class="chart-container"></div>
              </el-card>
            </el-col>
            <el-col :span="10">
              <el-card>
                <template #header>威胁来源 TOP10</template>
                <div class="threat-list">
                  <div v-for="(item, index) in dashboard.threatSources" :key="index" class="threat-item">
                    <span class="threat-ip">{{ item.ip_address }}</span>
                    <el-progress :percentage="getPercentage(item.count)" :stroke-width="8" :show-text="false" />
                    <span class="threat-count">{{ item.count }}次</span>
                  </div>
                  <el-empty v-if="!dashboard.threatSources?.length" description="暂无威胁数据" />
                </div>
              </el-card>
            </el-col>
          </el-row>

          <!-- 最近事件 -->
          <el-card class="recent-events-card">
            <template #header>最近安全事件</template>
            <el-table :data="dashboard.recentEvents" size="small" max-height="300">
              <el-table-column prop="createdAt" label="时间" width="160">
                <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
              </el-table-column>
              <el-table-column prop="eventType" label="类型" width="120">
                <template #default="{ row }">
                  <el-tag :type="getEventTagType(row.eventType)" size="small">{{ getEventTypeName(row.eventType) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="severity" label="级别" width="80">
                <template #default="{ row }">
                  <el-tag :type="getSeverityType(row.severity)" size="small">{{ getSeverityName(row.severity) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="sourceIp" label="来源IP" width="120" />
              <el-table-column prop="username" label="用户" width="100" />
              <el-table-column prop="description" label="描述" show-overflow-tooltip />
            </el-table>
          </el-card>
        </div>
      </el-tab-pane>

      <!-- 安全事件 -->
      <el-tab-pane label="安全事件" name="events">
        <div class="filter-bar">
          <el-select v-model="eventFilter.eventType" placeholder="事件类型" clearable class="filter-select-md">
            <el-option label="登录成功" value="login_success" />
            <el-option label="登录失败" value="login_failed" />
            <el-option label="账户锁定" value="account_locked" />
            <el-option label="IP封禁" value="ip_blocked" />
            <el-option label="密码修改" value="password_changed" />
            <el-option label="配置变更" value="config_changed" />
          </el-select>
          <el-select v-model="eventFilter.severity" placeholder="严重级别" clearable class="filter-select-sm">
            <el-option label="低" value="low" />
            <el-option label="中" value="medium" />
            <el-option label="高" value="high" />
            <el-option label="严重" value="critical" />
          </el-select>
          <el-input v-model="eventFilter.sourceIp" placeholder="来源IP" clearable class="filter-select-md" />
          <el-input v-model="eventFilter.username" placeholder="用户名" clearable class="filter-select-sm" />
          <el-button type="primary" @click="loadEvents">搜索</el-button>
        </div>
        <el-table :data="events.list" v-loading="eventsLoading">
          <el-table-column prop="createdAt" label="时间" width="160">
            <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
          </el-table-column>
          <el-table-column prop="eventType" label="类型" width="120">
            <template #default="{ row }">
              <el-tag :type="getEventTagType(row.eventType)" size="small">{{ getEventTypeName(row.eventType) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="severity" label="级别" width="80">
            <template #default="{ row }">
              <el-tag :type="getSeverityType(row.severity)" size="small">{{ getSeverityName(row.severity) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="sourceIp" label="来源IP" width="120" />
          <el-table-column prop="username" label="用户" width="100" />
          <el-table-column prop="description" label="描述" show-overflow-tooltip />
          <el-table-column prop="isResolved" label="状态" width="80">
            <template #default="{ row }">
              <el-tag :type="row.isResolved ? 'success' : 'warning'" size="small">
                {{ row.isResolved ? '已处理' : '待处理' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80">
            <template #default="{ row }">
              <el-button v-if="!row.isResolved" type="primary" link size="small" @click="resolveEvent(row.id)">处理</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-pagination
          v-model:current-page="eventFilter.page"
          v-model:page-size="eventFilter.pageSize"
          :total="events.total"
          layout="total, prev, pager, next"
          @current-change="loadEvents"
        />
      </el-tab-pane>

      <!-- IP管理 -->
      <el-tab-pane label="IP管理" name="ip">
        <el-tabs v-model="ipTab" type="card">
          <el-tab-pane label="黑名单" name="blacklist">
            <div class="filter-bar">
              <el-button type="danger" @click="showAddBlacklist = true">添加黑名单</el-button>
              <el-input v-model="ipCheckInput" placeholder="输入IP检查状态" class="ip-check-input" />
              <el-button @click="checkIPStatus">检查</el-button>
            </div>
            <el-table :data="blacklist.list" v-loading="blacklistLoading">
              <el-table-column prop="ipAddress" label="IP地址" width="150" />
              <el-table-column prop="ipType" label="类型" width="80" />
              <el-table-column prop="reason" label="原因" show-overflow-tooltip />
              <el-table-column prop="source" label="来源" width="80">
                <template #default="{ row }">
                  <el-tag size="small">{{ row.source === 'auto' ? '自动' : '手动' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="blockedCount" label="阻止次数" width="100" />
              <el-table-column prop="expiresAt" label="过期时间" width="160">
                <template #default="{ row }">{{ row.expiresAt ? formatTime(row.expiresAt) : '永久' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="80">
                <template #default="{ row }">
                  <el-popconfirm title="确定移除?" @confirm="removeFromBlacklist(row.id)">
                    <template #reference>
                      <el-button type="danger" link size="small">移除</el-button>
                    </template>
                  </el-popconfirm>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
          <el-tab-pane label="白名单" name="whitelist">
            <div class="filter-bar">
              <el-button type="primary" @click="showAddWhitelist = true">添加白名单</el-button>
            </div>
            <el-table :data="whitelist.list" v-loading="whitelistLoading">
              <el-table-column prop="ipAddress" label="IP地址" width="150" />
              <el-table-column prop="ipType" label="类型" width="80" />
              <el-table-column prop="description" label="描述" show-overflow-tooltip />
              <el-table-column prop="createdAt" label="添加时间" width="160">
                <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
              </el-table-column>
              <el-table-column label="操作" width="80">
                <template #default="{ row }">
                  <el-popconfirm title="确定移除?" @confirm="removeFromWhitelist(row.id)">
                    <template #reference>
                      <el-button type="danger" link size="small">移除</el-button>
                    </template>
                  </el-popconfirm>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
          <el-tab-pane label="锁定记录" name="lockouts">
            <el-table :data="lockouts.list" v-loading="lockoutsLoading">
              <el-table-column prop="lockType" label="类型" width="80">
                <template #default="{ row }">{{ row.lockType === 'account' ? '账户' : 'IP' }}</template>
              </el-table-column>
              <el-table-column prop="target" label="目标" width="150" />
              <el-table-column prop="reason" label="原因" show-overflow-tooltip />
              <el-table-column prop="attemptCount" label="尝试次数" width="100" />
              <el-table-column prop="expiresAt" label="过期时间" width="160">
                <template #default="{ row }">{{ formatTime(row.expiresAt) }}</template>
              </el-table-column>
              <el-table-column prop="isActive" label="状态" width="80">
                <template #default="{ row }">
                  <el-tag :type="row.isActive ? 'danger' : 'info'" size="small">
                    {{ row.isActive ? '锁定中' : '已解除' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="80">
                <template #default="{ row }">
                  <el-button v-if="row.isActive" type="primary" link size="small" @click="unlockTarget(row)">解锁</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </el-tab-pane>

      <!-- 会话管理 -->
      <el-tab-pane label="会话管理" name="sessions">
        <!-- 统计卡片 -->
        <div class="session-stats">
          <div class="stat-item">
            <span class="stat-value">{{ sessions.list?.length || 0 }}</span>
            <span class="stat-label">在线会话</span>
          </div>
          <div class="stat-item">
            <span class="stat-value">{{ webSessionCount }}</span>
            <span class="stat-label">Web 登录</span>
          </div>
          <div class="stat-item">
            <span class="stat-value">{{ ldapSessionCount }}</span>
            <span class="stat-label">LDAP 连接</span>
          </div>
          <div class="stat-item-action">
            <el-button size="small" @click="loadSessions">刷新</el-button>
            <el-popconfirm title="确定终止所有会话？当前登录也将失效" @confirm="terminateAllSessions">
              <template #reference>
                <el-button type="danger" size="small" :disabled="!sessions.list?.length">全部终止</el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>

        <el-table :data="sessions.list" v-loading="sessionsLoading" size="small" stripe>
          <el-table-column label="类型" width="90" align="center">
            <template #default="{ row }">
              <el-tag :type="row.userAgent === 'LDAP' ? 'warning' : 'primary'" size="small" effect="light">
                {{ row.userAgent === 'LDAP' ? 'LDAP' : 'Web' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="username" label="用户" width="100" />
          <el-table-column prop="ipAddress" label="IP 地址" width="130">
            <template #default="{ row }">{{ row.ipAddress || '-' }}</template>
          </el-table-column>
          <el-table-column label="客户端" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">
              <span v-if="row.userAgent === 'LDAP'" class="text-tertiary">LDAP Bind</span>
              <span v-else>{{ parseUA(row.userAgent) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="lastActivity" label="最后活动" width="160">
            <template #default="{ row }">
              <span :class="isRecentActivity(row.lastActivity) ? 'text-success' : ''">{{ formatTime(row.lastActivity) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="createdAt" label="创建时间" width="160">
            <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="80" align="center">
            <template #default="{ row }">
              <el-popconfirm title="确定终止该会话？" @confirm="terminateSession(row.id)">
                <template #reference>
                  <el-button type="danger" link size="small">终止</el-button>
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
          <template #empty>
            <el-empty description="暂无活跃会话" :image-size="80" />
          </template>
        </el-table>
      </el-tab-pane>

      <!-- 安全策略 -->
      <el-tab-pane label="安全策略" name="policy">
        <el-tabs v-model="policyTab" type="card">
          <el-tab-pane label="密码策略" name="password">
            <el-form :model="passwordPolicy" label-width="160px" class="policy-form">
              <el-form-item label="最小长度">
                <el-input-number v-model="passwordPolicy.min_length" :min="6" :max="32" />
              </el-form-item>
              <el-form-item label="要求大写字母">
                <el-switch v-model="passwordPolicy.require_uppercase" />
              </el-form-item>
              <el-form-item label="要求小写字母">
                <el-switch v-model="passwordPolicy.require_lowercase" />
              </el-form-item>
              <el-form-item label="要求数字">
                <el-switch v-model="passwordPolicy.require_number" />
              </el-form-item>
              <el-form-item label="要求特殊字符">
                <el-switch v-model="passwordPolicy.require_special" />
              </el-form-item>
              <el-form-item label="密码历史记忆">
                <el-input-number v-model="passwordPolicy.history_count" :min="0" :max="20" />
                <span class="form-hint">次 (不能重复最近N次密码)</span>
              </el-form-item>
              <el-form-item label="密码有效期">
                <el-input-number v-model="passwordPolicy.max_age_days" :min="0" :max="365" />
                <span class="form-hint">天 (0=不限制)</span>
              </el-form-item>
              <el-form-item label="弱密码检查">
                <el-switch v-model="passwordPolicy.weak_password_check" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="savePasswordPolicy">保存</el-button>
              </el-form-item>
            </el-form>
          </el-tab-pane>
          <el-tab-pane label="登录安全" name="login">
            <el-form :model="loginSecurity" label-width="180px" class="policy-form-lg">
              <el-divider content-position="left">账户锁定</el-divider>
              <el-form-item label="启用账户锁定">
                <el-switch v-model="loginSecurity.account_lockout.enabled" />
              </el-form-item>
              <el-form-item label="最大失败次数">
                <el-input-number v-model="loginSecurity.account_lockout.max_attempts" :min="3" :max="20" />
              </el-form-item>
              <el-form-item label="首次锁定时长">
                <el-input-number v-model="loginSecurity.account_lockout.lockout_duration_minutes" :min="5" :max="1440" />
                <span class="form-hint">分钟</span>
              </el-form-item>
              <el-form-item label="渐进式锁定">
                <el-switch v-model="loginSecurity.account_lockout.progressive_lockout" />
              </el-form-item>
              <el-divider content-position="left">IP锁定</el-divider>
              <el-form-item label="启用IP锁定">
                <el-switch v-model="loginSecurity.ip_lockout.enabled" />
              </el-form-item>
              <el-form-item label="IP最大失败次数">
                <el-input-number v-model="loginSecurity.ip_lockout.max_attempts" :min="5" :max="100" />
              </el-form-item>
              <el-form-item label="IP锁定时长">
                <el-input-number v-model="loginSecurity.ip_lockout.lockout_duration_hours" :min="1" :max="168" />
                <span class="form-hint">小时</span>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="saveLoginSecurity">保存</el-button>
              </el-form-item>
            </el-form>
          </el-tab-pane>
          <el-tab-pane label="会话配置" name="session">
            <el-form :model="sessionConfig" label-width="180px" class="policy-form">
              <el-form-item label="Token有效期">
                <el-input-number v-model="sessionConfig.access_token_ttl_minutes" :min="10" :max="1440" />
                <span class="form-hint">分钟</span>
              </el-form-item>
              <el-form-item label="最大并发会话数">
                <el-input-number v-model="sessionConfig.max_concurrent_sessions" :min="1" :max="20" />
              </el-form-item>
              <el-form-item label="单会话模式">
                <el-switch v-model="sessionConfig.single_session_mode" />
                <span class="form-hint">新登录踢掉旧会话</span>
              </el-form-item>
              <el-form-item label="IP绑定">
                <el-switch v-model="sessionConfig.ip_binding" />
                <span class="form-hint">IP变更会话失效</span>
              </el-form-item>
              <el-form-item label="空闲超时">
                <el-input-number v-model="sessionConfig.idle_timeout_minutes" :min="5" :max="1440" />
                <span class="form-hint">分钟</span>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="saveSessionConfig">保存</el-button>
              </el-form-item>
            </el-form>
          </el-tab-pane>

          <!-- 双因素认证 -->
          <el-tab-pane label="双因素认证" name="2fa">
            <el-form :model="twoFactorConfig" label-width="150px" class="policy-form">
              <el-alert
                type="info"
                :closable="false"
                show-icon
                style="margin-bottom: 20px"
              >
                <template #title>启用双因素认证后，用户登录时需要输入密码后，再输入短信验证码才能完成登录。</template>
              </el-alert>

              <el-form-item label="启用双因素认证">
                <el-switch v-model="twoFactorConfig.enabled" @change="onTwoFactorEnableChange" />
              </el-form-item>

              <template v-if="twoFactorConfig.enabled">
                <el-form-item label="管理员手机号" required>
                  <el-input v-model="twoFactorConfig.admin_phone" placeholder="必填，用于接收系统通知和测试" style="max-width: 300px" />
                  <span class="form-hint">启用 2FA 时必须设置管理员手机号</span>
                </el-form-item>

                <el-form-item label="验证码有效期">
                  <el-input-number v-model="twoFactorConfig.code_expire_mins" :min="1" :max="30" />
                  <span class="form-hint">分钟</span>
                </el-form-item>

                <el-form-item label="最大重试次数">
                  <el-input-number v-model="twoFactorConfig.max_retry" :min="3" :max="10" />
                  <span class="form-hint">验证码错误超过此次数需重新登录</span>
                </el-form-item>

                <el-form-item label="白名单IP豁免">
                  <el-switch v-model="twoFactorConfig.exempt_white_ip" />
                  <span class="form-hint">IP白名单中的地址跳过双因素认证</span>
                </el-form-item>

                <el-form-item label="豁免角色">
                  <el-select v-model="twoFactorConfig.exempt_roles" multiple placeholder="选择豁免的角色" style="max-width: 400px">
                    <el-option v-for="role in roleOptions" :key="role.id" :label="role.name" :value="role.id" />
                  </el-select>
                </el-form-item>
              </template>

              <el-form-item>
                <el-button type="primary" @click="saveTwoFactorConfig" :loading="twoFactorSaving">保存配置</el-button>
                <el-button v-if="twoFactorConfig.enabled && twoFactorConfig.admin_phone" @click="testTwoFactor" :loading="twoFactorTesting">发送测试短信</el-button>
              </el-form-item>
            </el-form>
          </el-tab-pane>
        </el-tabs>
      </el-tab-pane>

      <!-- 传输加密 -->
      <el-tab-pane label="传输加密" name="crypto">
        <div class="crypto-section">
          <div class="crypto-header">
            <h3 class="crypto-title">RSA 密钥配置</h3>
            <el-tag :type="cryptoConfig.source === 'auto' ? 'success' : cryptoConfig.source === 'https' ? 'primary' : 'warning'" size="small" effect="light">
              当前：{{ cryptoSourceLabel }}
            </el-tag>
          </div>

          <div class="crypto-cards">
            <div
              class="crypto-card"
              :class="{ active: cryptoForm.source === 'auto' }"
              @click="cryptoForm.source = 'auto'"
            >
              <div class="crypto-card-icon">
                <el-icon size="24"><Setting /></el-icon>
              </div>
              <div class="crypto-card-content">
                <div class="crypto-card-title">自动生成</div>
                <div class="crypto-card-desc">系统自动生成 RSA-2048 密钥对，持久化存储到服务器文件</div>
              </div>
              <el-tag v-if="cryptoForm.source === 'auto'" type="primary" size="small" effect="light" class="crypto-badge">推荐</el-tag>
            </div>

            <div
              class="crypto-card"
              :class="{ active: cryptoForm.source === 'https' }"
              @click="cryptoForm.source = 'https'"
            >
              <div class="crypto-card-icon">
                <el-icon size="24"><Lock /></el-icon>
              </div>
              <div class="crypto-card-content">
                <div class="crypto-card-title">复用 HTTPS 证书</div>
                <div class="crypto-card-desc">使用已配置的 HTTPS SSL 证书中的 RSA 密钥</div>
              </div>
            </div>

            <div
              class="crypto-card"
              :class="{ active: cryptoForm.source === 'custom' }"
              @click="cryptoForm.source = 'custom'"
            >
              <div class="crypto-card-icon">
                <el-icon size="24"><Key /></el-icon>
              </div>
              <div class="crypto-card-content">
                <div class="crypto-card-title">自定义密钥对</div>
                <div class="crypto-card-desc">上传自己的 RSA 私钥（PEM 格式，PKCS#1 或 PKCS#8）</div>
              </div>
              <el-tag v-if="cryptoForm.source === 'custom'" type="warning" size="small" effect="light" class="crypto-badge">高级</el-tag>
            </div>
          </div>

          <div v-if="cryptoForm.source === 'custom'" class="crypto-custom-form">
            <el-form-item label="RSA 私钥" label-width="100px">
              <el-input
                v-model="cryptoForm.privateKey"
                type="textarea"
                :rows="6"
                placeholder="-----BEGIN RSA PRIVATE KEY-----&#10;...&#10;-----END RSA PRIVATE KEY-----"
                class="mono-textarea"
              />
              <div class="field-hint">密钥长度建议 2048 位以上</div>
            </el-form-item>
          </div>

          <div class="crypto-action">
            <el-button type="primary" @click="saveCryptoConfig" :loading="cryptoSaving">保存配置</el-button>
          </div>
        </div>
      </el-tab-pane>

    </el-tabs>

    <!-- 添加黑名单对话框 -->
    <el-dialog v-model="showAddBlacklist" title="添加IP黑名单" width="450px">
      <el-form :model="blacklistForm" label-width="100px">
        <el-form-item label="IP地址" required>
          <el-input v-model="blacklistForm.ipAddress" placeholder="支持单IP或CIDR格式" />
        </el-form-item>
        <el-form-item label="原因" required>
          <el-input v-model="blacklistForm.reason" type="textarea" rows="2" />
        </el-form-item>
        <el-form-item label="过期时间">
          <el-select v-model="blacklistForm.expiresIn" class="full-width">
            <el-option label="永久" :value="0" />
            <el-option label="1小时" :value="1" />
            <el-option label="24小时" :value="24" />
            <el-option label="7天" :value="168" />
            <el-option label="30天" :value="720" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddBlacklist = false">取消</el-button>
        <el-button type="primary" @click="addToBlacklist">确定</el-button>
      </template>
    </el-dialog>

    <!-- 添加白名单对话框 -->
    <el-dialog v-model="showAddWhitelist" title="添加IP白名单" width="450px">
      <el-form :model="whitelistForm" label-width="100px">
        <el-form-item label="IP地址" required>
          <el-input v-model="whitelistForm.ipAddress" placeholder="支持单IP或CIDR格式" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="whitelistForm.description" type="textarea" rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddWhitelist = false">取消</el-button>
        <el-button type="primary" @click="addToWhitelist">确定</el-button>
      </template>
    </el-dialog>

    <!-- API白名单已移至 API 密钥管理页面 -->

  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, nextTick, computed } from "vue";
import { ElMessage } from "element-plus";
import { User, Connection, Warning, Bell, Lock, TrendCharts, Delete, Document, Setting, Key } from "@element-plus/icons-vue";
import * as echarts from "echarts";
import { securityApi, api } from "../../api";

const activeTab = ref("dashboard");
const ipTab = ref("blacklist");
const policyTab = ref("password");
const alertTab = ref("channels");

// 仪表板
const dashboardLoading = ref(false);
const dashboard = reactive<any>({ overview: {}, recentEvents: [], loginTrend: [], threatSources: [] });
const loginTrendChart = ref<HTMLElement>();
let chartInstance: echarts.ECharts | null = null;

// 安全事件
const eventsLoading = ref(false);
const events = reactive<any>({ list: [], total: 0 });
const eventFilter = reactive({ page: 1, pageSize: 20, eventType: "", severity: "", sourceIp: "", username: "" });

// IP管理
const blacklistLoading = ref(false);
const whitelistLoading = ref(false);
const lockoutsLoading = ref(false);
const blacklist = reactive<any>({ list: [], total: 0 });
const whitelist = reactive<any>({ list: [], total: 0 });
const lockouts = reactive<any>({ list: [], total: 0 });
const ipCheckInput = ref("");
const showAddBlacklist = ref(false);
const showAddWhitelist = ref(false);
const blacklistForm = reactive({ ipAddress: "", reason: "", expiresIn: 0 });
const whitelistForm = reactive({ ipAddress: "", description: "" });

// 会话管理
const sessionsLoading = ref(false);
const sessions = reactive<any>({ list: [], total: 0 });
const webSessionCount = computed(() => sessions.list?.filter((s: any) => s.userAgent !== 'LDAP').length || 0);
const ldapSessionCount = computed(() => sessions.list?.filter((s: any) => s.userAgent === 'LDAP').length || 0);
const parseUA = (ua: string) => {
  if (!ua) return '-';
  // 简化 User-Agent 显示
  const m = ua.match(/(Chrome|Firefox|Safari|Edge|Cursor|Opera)[\/\s]([\d.]+)/);
  if (m) return `${m[1]} ${m[2].split('.')[0]}`;
  if (ua.includes('curl')) return 'curl';
  return ua.length > 40 ? ua.substring(0, 40) + '...' : ua;
};
const isRecentActivity = (t: string) => {
  if (!t) return false;
  return (Date.now() - new Date(t).getTime()) < 5 * 60 * 1000; // 5分钟内
};

// 安全策略
const passwordPolicy = reactive<any>({});
const loginSecurity = reactive<any>({ account_lockout: {}, ip_lockout: {} });
const sessionConfig = reactive<any>({});

// 双因素认证
const twoFactorConfig = reactive<any>({
  enabled: false,
  admin_phone: "",
  verify_method: "sms",
  code_expire_mins: 5,
  max_retry: 5,
  lockout_mins: 30,
  exempt_white_ip: true,
  exempt_roles: []
});
const twoFactorSaving = ref(false);
const twoFactorTesting = ref(false);
const roleOptions = ref<any[]>([]);

// 告警配置
const notifyChannels = ref<any[]>([]);
const alertRules = ref<any[]>([]);
const showAddChannel = ref(false);
const showAddRule = ref(false);
const channelForm = reactive<any>({ name: "", channelType: "email", config: {} });
const ruleForm = reactive<any>({
  name: "",
  eventTypes: [],
  severityThreshold: "high",
  channelIds: [],
  cooldownMinutes: 30,
  isActive: true
});

// 加载仪表板
const loadDashboard = async () => {
  dashboardLoading.value = true;
  try {
    const res = await securityApi.dashboard();
    if (res.data.success) {
      Object.assign(dashboard, res.data.data);
      await nextTick();
      renderLoginTrendChart();
    }
  } finally {
    dashboardLoading.value = false;
  }
};

// 渲染登录趋势图
const renderLoginTrendChart = () => {
  if (!loginTrendChart.value) return;
  if (!chartInstance) {
    chartInstance = echarts.init(loginTrendChart.value);
  }
  
  const option = {
    tooltip: { trigger: "axis" },
    legend: { data: ["成功", "失败"] },
    grid: { left: "3%", right: "4%", bottom: "3%", containLabel: true },
    xAxis: {
      type: "category",
      data: dashboard.loginTrend?.map((i: any) => i.hour) || []
    },
    yAxis: { type: "value" },
    series: [
      {
        name: "成功",
        type: "line",
        smooth: true,
        data: dashboard.loginTrend?.map((i: any) => i.success) || [],
        itemStyle: { color: "#67c23a" }
      },
      {
        name: "失败",
        type: "line",
        smooth: true,
        data: dashboard.loginTrend?.map((i: any) => i.failed) || [],
        itemStyle: { color: "#f56c6c" }
      }
    ]
  };
  chartInstance.setOption(option);
};

// 加载安全事件
const loadEvents = async () => {
  eventsLoading.value = true;
  try {
    const res = await securityApi.events(eventFilter);
    if (res.data.success) {
      events.list = res.data.data.list;
      events.total = res.data.data.total;
    }
  } finally {
    eventsLoading.value = false;
  }
};

// 处理事件
const resolveEvent = async (id: number) => {
  await securityApi.resolveEvent(id);
  ElMessage.success("已处理");
  loadEvents();
};

// 加载黑名单
const loadBlacklist = async () => {
  blacklistLoading.value = true;
  try {
    const res = await securityApi.blacklist({ page: 1, pageSize: 100 });
    if (res.data.success) {
      blacklist.list = res.data.data.list;
    }
  } finally {
    blacklistLoading.value = false;
  }
};

// 加载白名单
const loadWhitelist = async () => {
  whitelistLoading.value = true;
  try {
    const res = await securityApi.whitelist({ page: 1, pageSize: 100 });
    if (res.data.success) {
      whitelist.list = res.data.data.list;
    }
  } finally {
    whitelistLoading.value = false;
  }
};

// 加载锁定记录
const loadLockouts = async () => {
  lockoutsLoading.value = true;
  try {
    const res = await securityApi.lockouts({ page: 1, pageSize: 100 });
    if (res.data.success) {
      lockouts.list = res.data.data.list;
    }
  } finally {
    lockoutsLoading.value = false;
  }
};

// 添加黑名单
const addToBlacklist = async () => {
  if (!blacklistForm.ipAddress || !blacklistForm.reason) {
    ElMessage.warning("请填写完整信息");
    return;
  }
  await securityApi.addBlacklist({
    ipAddress: blacklistForm.ipAddress,
    reason: blacklistForm.reason,
    expiresIn: blacklistForm.expiresIn || undefined
  });
  ElMessage.success("添加成功");
  showAddBlacklist.value = false;
  blacklistForm.ipAddress = "";
  blacklistForm.reason = "";
  loadBlacklist();
};

// 移除黑名单
const removeFromBlacklist = async (id: number) => {
  await securityApi.removeBlacklist(id);
  ElMessage.success("移除成功");
  loadBlacklist();
};

// 添加白名单
const addToWhitelist = async () => {
  if (!whitelistForm.ipAddress) {
    ElMessage.warning("请填写IP地址");
    return;
  }
  await securityApi.addWhitelist(whitelistForm);
  ElMessage.success("添加成功");
  showAddWhitelist.value = false;
  whitelistForm.ipAddress = "";
  whitelistForm.description = "";
  loadWhitelist();
};

// 移除白名单
const removeFromWhitelist = async (id: number) => {
  await securityApi.removeWhitelist(id);
  ElMessage.success("移除成功");
  loadWhitelist();
};

// ========== API白名单管理 ==========
// 检查IP状态
const checkIPStatus = async () => {
  if (!ipCheckInput.value) return;
  const res = await securityApi.checkIP(ipCheckInput.value);
  if (res.data.success) {
    const d = res.data.data;
    let msg = `IP: ${d.ip}\n`;
    msg += `黑名单: ${d.inBlacklist ? "是 (" + d.blacklistReason + ")" : "否"}\n`;
    msg += `白名单: ${d.inWhitelist ? "是" : "否"}\n`;
    msg += `锁定: ${d.isLocked ? "是" : "否"}`;
    ElMessage.info({ message: msg, duration: 5000 });
  }
};

// 解锁
const unlockTarget = async (row: any) => {
  if (row.lockType === "account") {
    await securityApi.unlockAccount(row.target);
  } else {
    await securityApi.unlockIP(row.target);
  }
  ElMessage.success("解锁成功");
  loadLockouts();
};

// 加载会话
const loadSessions = async () => {
  sessionsLoading.value = true;
  try {
    const res = await securityApi.sessions({ page: 1, pageSize: 100 });
    if (res.data.success) {
      sessions.list = res.data.data.list;
    }
  } finally {
    sessionsLoading.value = false;
  }
};

// 终止会话
const terminateSession = async (id: string) => {
  await securityApi.terminateSession(id);
  ElMessage.success("已终止");
  loadSessions();
};

// 终止所有会话
const terminateAllSessions = async () => {
  if (!sessions.list?.length) return;
  for (const s of sessions.list) {
    try { await securityApi.terminateSession(s.id); } catch {}
  }
  ElMessage.success("已终止所有会话");
  loadSessions();
};

// 加载安全配置
const loadSecurityConfigs = async () => {
  const res = await securityApi.configs();
  if (res.data.success) {
    const data = res.data.data;
    Object.assign(passwordPolicy, data.password_policy || {});
    Object.assign(loginSecurity, data.login_security || { account_lockout: {}, ip_lockout: {} });
    Object.assign(sessionConfig, data.session || {});
  }
};

// 保存密码策略
const savePasswordPolicy = async () => {
  await securityApi.updateConfig("password_policy", passwordPolicy);
  ElMessage.success("保存成功");
};

// 保存登录安全
const saveLoginSecurity = async () => {
  await securityApi.updateConfig("login_security", loginSecurity);
  ElMessage.success("保存成功");
};

// 保存会话配置
const saveSessionConfig = async () => {
  await securityApi.updateConfig("session", sessionConfig);
  ElMessage.success("保存成功");
};

// ========== 双因素认证 ==========
// 加载双因素认证配置
const loadTwoFactorConfig = async () => {
  try {
    const res = await api.get("/settings/2fa");
    if (res.data.success) {
      Object.assign(twoFactorConfig, res.data.data);
    }
  } catch (e) {
    console.error("加载2FA配置失败", e);
  }
};

// 保存双因素认证配置
const saveTwoFactorConfig = async () => {
  if (twoFactorConfig.enabled && !twoFactorConfig.admin_phone) {
    ElMessage.warning("启用双因素认证时必须设置管理员手机号");
    return;
  }
  twoFactorSaving.value = true;
  try {
    const res = await api.put("/settings/2fa", twoFactorConfig);
    if (res.data.success) {
      ElMessage.success("配置已保存");
    } else {
      ElMessage.error(res.data.message || "保存失败");
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || "保存失败");
  } finally {
    twoFactorSaving.value = false;
  }
};

// 测试双因素认证
const testTwoFactor = async () => {
  twoFactorTesting.value = true;
  try {
    const res = await api.post("/settings/2fa/test");
    if (res.data.success) {
      ElMessage.success(res.data.data?.message || "测试短信已发送");
    } else {
      ElMessage.error(res.data.message || "发送失败");
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || "发送失败");
  } finally {
    twoFactorTesting.value = false;
  }
};

// 启用状态变化
const onTwoFactorEnableChange = (enabled: boolean) => {
  if (enabled && !twoFactorConfig.admin_phone) {
    ElMessage.info("请设置管理员手机号");
  }
};

// 加载角色选项
const loadRoleOptions = async () => {
  try {
    const res = await api.get("/roles");
    if (res.data.success) {
      roleOptions.value = res.data.data?.list || res.data.data || [];
    }
  } catch (e) {
    console.error("加载角色失败", e);
  }
};

// 加载通知渠道
const loadNotifyChannels = async () => {
  const res = await securityApi.notifyChannels();
  if (res.data.success) {
    notifyChannels.value = res.data.data;
  }
};

// 加载告警规则
const loadAlertRules = async () => {
  const res = await securityApi.alertRules();
  if (res.data.success) {
    alertRules.value = res.data.data;
  }
};

// 保存渠道
const saveChannel = async () => {
  if (!channelForm.name || !channelForm.channelType) {
    ElMessage.warning("请填写完整信息");
    return;
  }
  if (channelForm.id) {
    await securityApi.updateNotifyChannel(channelForm.id, channelForm);
  } else {
    await securityApi.createNotifyChannel(channelForm);
  }
  ElMessage.success("保存成功");
  showAddChannel.value = false;
  loadNotifyChannels();
};

// 编辑渠道
const editChannel = (row: any) => {
  Object.assign(channelForm, { ...row, config: JSON.parse(row.config || "{}") });
  showAddChannel.value = true;
};

// 删除渠道
const deleteChannel = async (id: number) => {
  await securityApi.deleteNotifyChannel(id);
  ElMessage.success("删除成功");
  loadNotifyChannels();
};

// 删除规则
const deleteRule = async (id: number) => {
  await securityApi.deleteAlertRule(id);
  ElMessage.success("删除成功");
  loadAlertRules();
};

const editRule = (row: any) => {
  Object.assign(ruleForm, {
    id: row.id,
    name: row.name,
    eventTypes: row.eventTypes ? JSON.parse(row.eventTypes) : [],
    severityThreshold: row.severityThreshold,
    channelIds: row.channelIds ? JSON.parse(row.channelIds) : [],
    cooldownMinutes: row.cooldownMinutes,
    isActive: row.isActive
  });
  showAddRule.value = true;
};

// 保存规则
const saveRule = async () => {
  if (!ruleForm.name || !ruleForm.severityThreshold || ruleForm.channelIds.length === 0) {
    ElMessage.warning("请填写完整信息");
    return;
  }
  const data = {
    ...ruleForm,
    eventTypes: JSON.stringify(ruleForm.eventTypes),
    channelIds: JSON.stringify(ruleForm.channelIds)
  };
  if (ruleForm.id) {
    await securityApi.updateAlertRule(ruleForm.id, data);
  } else {
    await securityApi.createAlertRule(data);
  }
  ElMessage.success("保存成功");
  showAddRule.value = false;
  resetRuleForm();
  loadAlertRules();
};

// 重置规则表单
const resetRuleForm = () => {
  Object.assign(ruleForm, {
    id: undefined,
    name: "",
    eventTypes: [],
    severityThreshold: "high",
    channelIds: [],
    cooldownMinutes: 30,
    isActive: true
  });
};

// 渠道类型变更
const onChannelTypeChange = () => {
  // 初始化不同类型的默认配置
  if (channelForm.channelType === 'sms_https') {
    channelForm.config = {
      url: "",
      method: "POST",
      contentType: "application/json",
      timeout: 15,
      headers: [],
      bodyTemplate: '{"phone": "{{phone}}", "message": "{{message}}"}',
      phones: ""
    };
  } else if (!channelForm.config || typeof channelForm.config !== 'object') {
    channelForm.config = {};
  }
};

// 添加请求头
const addHeader = () => {
  if (!channelForm.config.headers) {
    channelForm.config.headers = [];
  }
  channelForm.config.headers.push({ key: "", value: "" });
};

// 移除请求头
const removeHeader = (index: number) => {
  channelForm.config.headers.splice(index, 1);
};

// 辅助函数
const formatTime = (t: string) => t ? new Date(t).toLocaleString() : "-";
const getPercentage = (count: number) => {
  const max = Math.max(...(dashboard.threatSources?.map((i: any) => i.count) || [1]));
  return (count / max) * 100;
};

const getEventTypeName = (t: string) => {
  const map: any = {
    login_success: "登录成功", login_failed: "登录失败", login_blocked: "登录阻止",
    account_locked: "账户锁定", account_unlocked: "账户解锁", password_changed: "密码修改",
    ip_blocked: "IP封禁", ip_unblocked: "IP解封", config_changed: "配置变更",
    session_terminated: "会话终止"
  };
  return map[t] || t;
};

const getEventTagType = (t: string) => {
  if (t.includes("success") || t.includes("unlocked") || t.includes("unblocked")) return "success";
  if (t.includes("failed") || t.includes("locked") || t.includes("blocked")) return "danger";
  return "info";
};

const getSeverityName = (s: string) => {
  const map: any = { low: "低", medium: "中", high: "高", critical: "严重" };
  return map[s] || s;
};

const getSeverityType = (s: string) => {
  const map: any = { low: "info", medium: "warning", high: "danger", critical: "danger" };
  return map[s] || "info";
};

const getChannelTypeName = (t: string) => {
  const map: any = { 
    email: "邮件", 
    webhook: "Webhook", 
    sms_aliyun: "阿里云短信", 
    sms_tencent: "腾讯云短信",
    sms_https: "自定义HTTPS短信"
  };
  return map[t] || t;
};

// ========== RSA 传输加密配置 ==========
const cryptoConfig = reactive({ source: "auto", privateKey: "" });
const cryptoForm = reactive({ source: "auto", privateKey: "" });
const cryptoSaving = ref(false);
const cryptoSourceLabel = computed(() => {
  const m: Record<string, string> = { auto: "自动生成", https: "HTTPS 证书", custom: "自定义密钥" };
  return m[cryptoConfig.source] || cryptoConfig.source;
});

const loadCryptoConfig = async () => {
  try {
    const res = await api.get("/settings/crypto");
    const data = (res as any).data?.data || (res as any).data;
    cryptoConfig.source = data.source || "auto";
    cryptoForm.source = cryptoConfig.source;
    cryptoForm.privateKey = "";
  } catch {}
};

const saveCryptoConfig = async () => {
  cryptoSaving.value = true;
  try {
    await api.put("/settings/crypto", {
      source: cryptoForm.source,
      privateKey: cryptoForm.source === "custom" ? cryptoForm.privateKey : "",
    });
    ElMessage.success("RSA 密钥配置已更新");
    cryptoConfig.source = cryptoForm.source;
    cryptoForm.privateKey = "";
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || "保存失败");
  } finally {
    cryptoSaving.value = false;
  }
};

onMounted(() => {
  loadDashboard();
  loadEvents();
  loadBlacklist();
  loadWhitelist();
  loadLockouts();
  loadSessions();
  loadSecurityConfigs();
  loadNotifyChannels();
  loadAlertRules();
  loadCryptoConfig();
  loadTwoFactorConfig();
  loadRoleOptions();
});
</script>

<style scoped>
/* ============================================
   Corporate Security Page - 企业风格
   ============================================ */
.security-page { padding: 0; width: 100%; }

/* 概览卡片网格布局 */
.overview-cards {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: var(--spacing-lg);
  margin-bottom: var(--spacing-xl);
}

.overview-card {
  background: var(--card-bg);
  border-radius: var(--card-radius);
  border: 1px solid var(--card-border);
  padding: var(--spacing-xl);
  display: flex;
  align-items: center;
  gap: var(--spacing-lg);
  box-shadow: var(--card-shadow);
  transition: box-shadow var(--transition-fast);
}
.overview-card:hover { box-shadow: var(--card-shadow-hover); }
.overview-card.warning .card-icon { background: var(--color-warning); }
.overview-card.danger .card-icon { background: var(--color-error); }
.overview-card.success .card-icon { background: var(--color-success); }

.card-icon {
  width: 48px; height: 48px; border-radius: var(--radius-md);
  background: var(--color-primary);
  display: flex; align-items: center; justify-content: center;
  color: #fff; font-size: 22px;
}

.card-value { font-size: 28px; font-weight: 600; color: var(--color-text-primary); }
.card-label { font-size: var(--font-size-sm); color: var(--color-text-tertiary); margin-top: var(--spacing-xs); }

.chart-row { margin-bottom: var(--spacing-xl); }
.chart-container { height: 280px; }

.threat-list { max-height: 280px; overflow-y: auto; }
.threat-item { display: flex; align-items: center; gap: var(--spacing-md); padding: var(--spacing-sm) 0; border-bottom: 1px solid var(--color-border-secondary); }
.threat-ip { width: 120px; font-family: monospace; font-size: var(--font-size-sm); }
.threat-item .el-progress { flex: 1; }
.threat-count { width: 60px; text-align: right; font-size: var(--font-size-sm); color: var(--color-text-tertiary); }

.filter-bar { display: flex; gap: var(--spacing-md); margin-bottom: var(--spacing-lg); flex-wrap: wrap; }
.form-hint { margin-left: var(--spacing-sm); color: var(--color-text-tertiary); font-size: var(--font-size-xs); }
.field-hint { font-size: var(--font-size-xs); color: var(--color-text-tertiary); margin-top: var(--spacing-xs); }
.recent-events-card { margin-top: var(--spacing-xl); }

@media (max-width: 1400px) { .overview-cards { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 900px) { .overview-cards { grid-template-columns: repeat(2, 1fr); } }

.api-management { padding: var(--spacing-xs) 0; }

.tips-list { margin: 0; padding-left: 20px; line-height: 2; color: var(--color-text-secondary); font-size: var(--font-size-sm); }
.tips-list code { background: var(--color-fill-secondary); padding: 2px 6px; border-radius: var(--radius-sm); font-size: var(--font-size-xs); color: var(--color-warning); }

/* 传输加密 - 卡片式单选 */
.crypto-section { padding: var(--spacing-xs) 0; }
.crypto-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--spacing-xl); }
.crypto-title { font-size: var(--font-size-lg); font-weight: 600; color: var(--color-text-primary); margin: 0; }

.crypto-cards { display: flex; gap: var(--spacing-lg); margin-bottom: var(--spacing-xl); }
.crypto-card {
  flex: 1;
  border: 2px solid var(--color-border-secondary);
  border-radius: var(--radius-xl);
  padding: var(--spacing-xl);
  cursor: pointer;
  transition: all 0.2s;
  position: relative;
  background: var(--color-bg-container);
}
.crypto-card:hover { border-color: var(--color-primary-border); box-shadow: var(--shadow-sm); }
.crypto-card.active { border-color: var(--color-primary); background: var(--color-primary-bg); }
.crypto-card-icon { margin-bottom: var(--spacing-md); color: var(--color-primary); }
.crypto-card-title { font-size: var(--font-size-base); font-weight: 600; color: var(--color-text-primary); margin-bottom: var(--spacing-xs); }
.crypto-card-desc { font-size: var(--font-size-xs); color: var(--color-text-tertiary); line-height: 1.6; }
.crypto-badge { position: absolute; top: var(--spacing-md); right: var(--spacing-md); }

.crypto-custom-form { max-width: 650px; margin-bottom: var(--spacing-lg); }
.mono-textarea :deep(.el-textarea__inner) { font-family: monospace; font-size: var(--font-size-xs); }
.crypto-action { margin-top: var(--spacing-lg); }

/* 筛选控件 */
.filter-select-md { width: 140px; }
.filter-select-sm { width: 120px; }
.ip-check-input { width: 200px; margin-left: auto; }
.policy-form { max-width: 600px; }
.policy-form-lg { max-width: 650px; }
.full-width { width: 100%; }
.api-alert { margin-bottom: var(--spacing-lg); }
.api-tips { margin-top: var(--spacing-xl); }
.btn-icon { margin-right: var(--spacing-xs); }
.mono-text { font-family: monospace; }

/* 会话管理 */
.session-stats {
  display: flex;
  align-items: center;
  gap: var(--spacing-xl);
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-md) var(--spacing-lg);
  background: var(--color-fill-secondary);
  border-radius: var(--radius-lg);
}
.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}
.stat-value {
  font-size: var(--font-size-xl);
  font-weight: 700;
  color: var(--color-primary);
}
.stat-label {
  font-size: var(--font-size-xs);
  color: var(--color-text-tertiary);
}
.stat-item-action {
  margin-left: auto;
  display: flex;
  gap: var(--spacing-sm);
}
.text-success { color: var(--color-success); }
.text-tertiary { color: var(--color-text-tertiary); }
</style>
