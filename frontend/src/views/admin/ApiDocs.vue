<template>
  <div class="api-docs-page">
    <!-- 工具栏 -->
    <div class="docs-toolbar">
      <el-button @click="$router.back()">
        <el-icon style="margin-right: 4px"><ArrowLeft /></el-icon>
        返回
      </el-button>
      <el-button type="primary" @click="downloadPDF" :loading="pdfLoading">
        <el-icon style="margin-right: 4px"><Download /></el-icon>
        下载PDF文档
      </el-button>
    </div>

    <!-- 文档内容 -->
    <div class="docs-content" ref="docsRef">
      <!-- 封面 -->
      <div class="doc-cover">
        <h1>API 接口文档</h1>
        <p class="doc-subtitle">统一身份认证管理平台</p>
        <div class="doc-meta">
          <p>版本：v1.0</p>
          <p>生成日期：{{ today }}</p>
        </div>
      </div>

      <!-- 目录 -->
      <div class="doc-section">
        <h2>目录</h2>
        <ul class="toc">
          <li><a href="#overview">1. 概述</a></li>
          <li><a href="#auth-api">2. 认证接口</a>
            <ul>
              <li><a href="#api-login">2.1 用户登录</a></li>
            </ul>
          </li>
          <li><a href="#user-api">3. 用户管理接口</a>
            <ul>
              <li><a href="#api-user-list">3.1 获取用户列表</a></li>
              <li><a href="#api-user-create">3.2 新增用户</a></li>
              <li><a href="#api-user-get">3.3 获取用户详情</a></li>
              <li><a href="#api-user-update">3.4 更新用户信息</a></li>
              <li><a href="#api-user-delete">3.5 删除用户</a></li>
              <li><a href="#api-user-status">3.6 启用/禁用用户</a></li>
              <li><a href="#api-user-reset-pwd">3.7 重置用户密码</a></li>
              <li><a href="#api-user-change-pwd">3.8 修改密码（当前用户）</a></li>
              <li><a href="#api-user-export">3.9 导出所有用户（含加密密码）</a></li>
            </ul>
          </li>
          <li><a href="#role-api">4. 角色管理接口</a>
            <ul>
              <li><a href="#api-role-list">4.1 获取角色列表</a></li>
              <li><a href="#api-role-create">4.2 新增角色</a></li>
              <li><a href="#api-role-delete">4.3 删除角色</a></li>
              <li><a href="#api-role-auto-assign-get">4.4 获取自动分配规则</a></li>
              <li><a href="#api-role-auto-assign-update">4.5 更新自动分配规则</a></li>
              <li><a href="#api-role-auto-assign-apply">4.6 立即执行自动分配</a></li>
            </ul>
          </li>
          <li><a href="#log-api">5. 日志管理接口</a>
            <ul>
              <li><a href="#api-log-login">5.1 查询登录日志</a></li>
              <li><a href="#api-log-operation">5.2 查询操作日志</a></li>
            </ul>
          </li>
          <li><a href="#error-codes">6. 错误码说明</a></li>
        </ul>
      </div>

      <!-- 1. 概述 -->
      <div class="doc-section" id="overview">
        <h2>1. 概述</h2>
        <h3>基础信息</h3>
        <table class="doc-table">
          <tr><td class="label-col">Base URL</td><td><code>http://&lt;服务器地址&gt;:&lt;端口&gt;/api</code></td></tr>
          <tr><td class="label-col">协议</td><td>HTTP / HTTPS</td></tr>
          <tr><td class="label-col">数据格式</td><td>JSON</td></tr>
          <tr><td class="label-col">字符编码</td><td>UTF-8</td></tr>
          <tr><td class="label-col">认证方式</td><td>Bearer Token（JWT）</td></tr>
        </table>

        <h3>认证说明</h3>
        <p>除登录接口外，所有接口均需在请求头中携带 Token：</p>
        <pre class="code-block">Authorization: Bearer &lt;token&gt;</pre>
        <p>Token 通过登录接口获取，有效期由系统管理员配置。</p>

        <h3>通用响应格式</h3>
        <pre class="code-block">{
  "success": true,
  "data": { ... },
  "message": "操作成功"
}</pre>
        <p>请求失败时：</p>
        <pre class="code-block">{
  "success": false,
  "message": "错误原因描述"
}</pre>

        <h3>IP 白名单</h3>
        <p>系统支持 API IP 白名单功能。当白名单为空时不做限制；当白名单有条目时，仅白名单内 IP 可访问 API。</p>

        <h3>频率限制</h3>
        <table class="doc-table">
          <tr><th>类型</th><th>限制</th></tr>
          <tr><td>通用 API</td><td>100 次/分钟/IP</td></tr>
          <tr><td>登录接口</td><td>10 次/分钟/IP</td></tr>
          <tr><td>敏感操作</td><td>5 次/分钟/IP</td></tr>
        </table>
      </div>

      <!-- 2. 认证接口 -->
      <div class="doc-section" id="auth-api">
        <h2>2. 认证接口</h2>

        <div class="api-item" id="api-login">
          <h3>2.1 用户登录</h3>
          <div class="api-method"><span class="method post">POST</span> <code>/api/auth/login</code></div>
          <p class="api-desc">用户登录并获取访问令牌。密码需在客户端进行 SHA-256 哈希后传输。</p>
          <p><strong>权限要求：</strong>无（公开接口）</p>
          <h4>请求参数（Body）</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
            <tr><td>username</td><td>string</td><td>是</td><td>用户名</td></tr>
            <tr><td>password</td><td>string</td><td>是</td><td>密码（SHA-256 哈希值）</td></tr>
            <tr><td>_encrypted</td><td>boolean</td><td>否</td><td>标记密码已哈希，传 true</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "_encrypted": true
  }'</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": 1,
      "username": "admin",
      "nickname": "管理员",
      "roles": [{ "id": 1, "name": "超级管理员", "code": "super_admin" }]
    }
  }
}</pre>
          <h4>错误响应</h4>
          <pre class="code-block">{
  "success": false,
  "message": "用户名或密码错误"
}</pre>
        </div>
      </div>

      <!-- 3. 用户管理接口 -->
      <div class="doc-section" id="user-api">
        <h2>3. 用户管理接口</h2>

        <!-- 3.1 获取用户列表 -->
        <div class="api-item" id="api-user-list">
          <h3>3.1 获取用户列表</h3>
          <div class="api-method"><span class="method get">GET</span> <code>/api/users</code></div>
          <p class="api-desc">分页获取用户列表，支持关键词搜索和分组筛选。</p>
          <p><strong>权限要求：</strong><code>user:list</code></p>
          <h4>请求参数（Query）</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
            <tr><td>pageIndex</td><td>int</td><td>否</td><td>页码，从 0 开始，默认 0</td></tr>
            <tr><td>pageSize</td><td>int</td><td>否</td><td>每页数量，默认 20</td></tr>
            <tr><td>keyword</td><td>string</td><td>否</td><td>搜索关键词（匹配用户名、昵称、手机号）</td></tr>
            <tr><td>groupId</td><td>int</td><td>否</td><td>用户组ID筛选</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X GET "http://localhost:8080/api/users?pageIndex=0&pageSize=10&keyword=zhang" \
  -H "Authorization: Bearer &lt;token&gt;"</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": {
    "list": [
      {
        "id": 2,
        "username": "zhangsan",
        "nickname": "张三",
        "phone": "13800138000",
        "email": "zhangsan@example.com",
        "status": 1,
        "source": "local",
        "groupId": 1,
        "roles": [{ "id": 2, "name": "普通用户", "code": "user" }],
        "createdAt": "2024-01-15T10:30:00Z",
        "updatedAt": "2024-01-15T10:30:00Z"
      }
    ],
    "total": 1
  }
}</pre>
        </div>

        <!-- 3.2 新增用户 -->
        <div class="api-item" id="api-user-create">
          <h3>3.2 新增用户</h3>
          <div class="api-method"><span class="method post">POST</span> <code>/api/users</code></div>
          <p class="api-desc">创建一个新的本地用户。</p>
          <p><strong>权限要求：</strong><code>user:create</code></p>
          <h4>请求参数（Body）</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
            <tr><td>username</td><td>string</td><td>是</td><td>用户名（唯一）</td></tr>
            <tr><td>password</td><td>string</td><td>是</td><td>密码（至少6位）</td></tr>
            <tr><td>nickname</td><td>string</td><td>否</td><td>昵称</td></tr>
            <tr><td>phone</td><td>string</td><td>否</td><td>手机号</td></tr>
            <tr><td>email</td><td>string</td><td>否</td><td>邮箱</td></tr>
            <tr><td>status</td><td>int</td><td>否</td><td>状态：1=启用 0=禁用，默认1</td></tr>
            <tr><td>roleIds</td><td>int[]</td><td>否</td><td>角色ID数组</td></tr>
            <tr><td>groupId</td><td>int</td><td>否</td><td>用户组ID</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X POST http://localhost:8080/api/users \
  -H "Authorization: Bearer &lt;token&gt;" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "lisi",
    "password": "Abc123456",
    "nickname": "李四",
    "phone": "13900139000",
    "email": "lisi@example.com",
    "status": 1,
    "roleIds": [2],
    "groupId": 1
  }'</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": {
    "id": 3,
    "username": "lisi",
    "nickname": "李四",
    "status": 1,
    "source": "local"
  }
}</pre>
        </div>

        <!-- 3.3 获取用户详情 -->
        <div class="api-item" id="api-user-get">
          <h3>3.3 获取用户详情</h3>
          <div class="api-method"><span class="method get">GET</span> <code>/api/users/:id</code></div>
          <p class="api-desc">根据用户ID获取单个用户的详细信息。</p>
          <p><strong>权限要求：</strong><code>user:list</code></p>
          <h4>路径参数</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>说明</th></tr>
            <tr><td>id</td><td>int</td><td>用户ID</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X GET http://localhost:8080/api/users/2 \
  -H "Authorization: Bearer &lt;token&gt;"</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": {
    "id": 2,
    "username": "zhangsan",
    "nickname": "张三",
    "phone": "13800138000",
    "email": "zhangsan@example.com",
    "status": 1,
    "source": "local",
    "groupId": 1,
    "roles": [{ "id": 2, "name": "普通用户", "code": "user" }],
    "createdAt": "2024-01-15T10:30:00Z"
  }
}</pre>
        </div>

        <!-- 3.4 更新用户信息 -->
        <div class="api-item" id="api-user-update">
          <h3>3.4 更新用户信息</h3>
          <div class="api-method"><span class="method put">PUT</span> <code>/api/users/:id</code></div>
          <p class="api-desc">更新指定用户的信息和角色。</p>
          <p><strong>权限要求：</strong><code>user:update</code></p>
          <h4>路径参数</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>说明</th></tr>
            <tr><td>id</td><td>int</td><td>用户ID</td></tr>
          </table>
          <h4>请求参数（Body）</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
            <tr><td>nickname</td><td>string</td><td>否</td><td>昵称</td></tr>
            <tr><td>phone</td><td>string</td><td>否</td><td>手机号</td></tr>
            <tr><td>email</td><td>string</td><td>否</td><td>邮箱</td></tr>
            <tr><td>status</td><td>int</td><td>否</td><td>状态：1=启用 0=禁用</td></tr>
            <tr><td>roleIds</td><td>int[]</td><td>否</td><td>角色ID数组（会覆盖现有角色）</td></tr>
            <tr><td>groupId</td><td>int</td><td>否</td><td>用户组ID</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X PUT http://localhost:8080/api/users/2 \
  -H "Authorization: Bearer &lt;token&gt;" \
  -H "Content-Type: application/json" \
  -d '{
    "nickname": "张三（已更新）",
    "phone": "13800138001",
    "roleIds": [2, 3]
  }'</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": null
}</pre>
        </div>

        <!-- 3.5 删除用户 -->
        <div class="api-item" id="api-user-delete">
          <h3>3.5 删除用户</h3>
          <div class="api-method"><span class="method delete">DELETE</span> <code>/api/users/:id</code></div>
          <p class="api-desc">软删除指定用户（标记为已删除，不会物理删除）。不能删除自己。</p>
          <p><strong>权限要求：</strong><code>user:delete</code></p>
          <h4>路径参数</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>说明</th></tr>
            <tr><td>id</td><td>int</td><td>用户ID</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X DELETE http://localhost:8080/api/users/3 \
  -H "Authorization: Bearer &lt;token&gt;"</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": null
}</pre>
        </div>

        <!-- 3.6 启用/禁用用户 -->
        <div class="api-item" id="api-user-status">
          <h3>3.6 启用/禁用用户</h3>
          <div class="api-method"><span class="method put">PUT</span> <code>/api/users/:id/status</code></div>
          <p class="api-desc">更新指定用户的启用/禁用状态。</p>
          <p><strong>权限要求：</strong><code>user:update</code></p>
          <h4>路径参数</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>说明</th></tr>
            <tr><td>id</td><td>int</td><td>用户ID</td></tr>
          </table>
          <h4>请求参数（Body）</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
            <tr><td>status</td><td>int</td><td>是</td><td>1=启用 0=禁用</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X PUT http://localhost:8080/api/users/2/status \
  -H "Authorization: Bearer &lt;token&gt;" \
  -H "Content-Type: application/json" \
  -d '{ "status": 0 }'</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": null
}</pre>
        </div>

        <!-- 3.7 重置用户密码 -->
        <div class="api-item" id="api-user-reset-pwd">
          <h3>3.7 重置用户密码</h3>
          <div class="api-method"><span class="method put">PUT</span> <code>/api/users/:id/reset-password</code></div>
          <p class="api-desc">管理员重置指定用户的密码。</p>
          <p><strong>权限要求：</strong><code>user:update</code></p>
          <h4>路径参数</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>说明</th></tr>
            <tr><td>id</td><td>int</td><td>用户ID</td></tr>
          </table>
          <h4>请求参数（Body）</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
            <tr><td>password</td><td>string</td><td>是</td><td>新密码（至少6位）</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X PUT http://localhost:8080/api/users/2/reset-password \
  -H "Authorization: Bearer &lt;token&gt;" \
  -H "Content-Type: application/json" \
  -d '{ "password": "NewPassword123" }'</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": null
}</pre>
        </div>

        <!-- 3.8 修改密码 -->
        <div class="api-item" id="api-user-change-pwd">
          <h3>3.8 修改密码（当前用户）</h3>
          <div class="api-method"><span class="method put">PUT</span> <code>/api/auth/password</code></div>
          <p class="api-desc">当前登录用户修改自己的密码。密码需要 SHA-256 哈希后传输。修改成功后所有会话将失效，需重新登录。</p>
          <p><strong>权限要求：</strong>已登录即可</p>
          <h4>请求参数（Body）</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
            <tr><td>oldPassword</td><td>string</td><td>是</td><td>原密码（SHA-256 哈希值）</td></tr>
            <tr><td>newPassword</td><td>string</td><td>是</td><td>新密码（SHA-256 哈希值）</td></tr>
            <tr><td>_encrypted</td><td>boolean</td><td>否</td><td>标记密码已哈希，传 true</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X PUT http://localhost:8080/api/auth/password \
  -H "Authorization: Bearer &lt;token&gt;" \
  -H "Content-Type: application/json" \
  -d '{
    "oldPassword": "旧密码的SHA256哈希",
    "newPassword": "新密码的SHA256哈希",
    "_encrypted": true
  }'</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "message": "密码修改成功"
}</pre>
        </div>

        <!-- 3.9 导出用户 -->
        <div class="api-item" id="api-user-export">
          <h3>3.9 导出所有用户（含加密密码）</h3>
          <div class="api-method"><span class="method get">GET</span> <code>/api/users/export</code></div>
          <p class="api-desc">导出所有用户的完整信息，包括 bcrypt 加密后的密码哈希和 Samba NT 密码哈希。此接口为敏感操作，调用会被记录到操作日志。</p>
          <p><strong>权限要求：</strong><code>settings:system</code>（仅管理员）</p>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X GET http://localhost:8080/api/users/export \
  -H "Authorization: Bearer &lt;token&gt;"</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": {
    "list": [
      {
        "id": 1,
        "username": "admin",
        "nickname": "管理员",
        "phone": "",
        "email": "admin@example.com",
        "status": 1,
        "source": "local",
        "groupId": 0,
        "password": "$2a$10$xxxx...（bcrypt哈希）",
        "sambaNTPassword": "A4F49C40...",
        "createdAt": "2024-01-01 00:00:00",
        "updatedAt": "2024-06-15 12:00:00",
        "passwordChangedAt": "2024-06-15 12:00:00",
        "roles": [
          { "id": 1, "name": "超级管理员", "code": "super_admin" }
        ]
      }
    ],
    "total": 15
  }
}</pre>
          <div class="api-warning">
            <strong>安全警告：</strong>此接口返回用户的加密密码信息，属于高度敏感数据。请确保仅在安全的网络环境中调用，并妥善保管返回的数据。每次调用都会记录操作日志。
          </div>
        </div>
      </div>

      <!-- 4. 角色管理接口 -->
      <div class="doc-section" id="role-api">
        <h2>4. 角色管理接口</h2>

        <!-- 4.1 获取角色列表 -->
        <div class="api-item" id="api-role-list">
          <h3>4.1 获取角色列表</h3>
          <div class="api-method"><span class="method get">GET</span> <code>/api/roles</code></div>
          <p class="api-desc">获取系统中所有角色的列表。</p>
          <p><strong>权限要求：</strong><code>role:list</code></p>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X GET http://localhost:8080/api/roles \
  -H "Authorization: Bearer &lt;token&gt;"</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": [
    {
      "id": 1,
      "name": "超级管理员",
      "code": "super_admin",
      "description": "系统超级管理员",
      "status": 1,
      "createdAt": "2024-01-01T00:00:00Z"
    },
    {
      "id": 2,
      "name": "普通用户",
      "code": "user",
      "description": "默认用户角色",
      "status": 1,
      "createdAt": "2024-01-01T00:00:00Z"
    }
  ]
}</pre>
        </div>

        <!-- 4.2 新增角色 -->
        <div class="api-item" id="api-role-create">
          <h3>4.2 新增角色</h3>
          <div class="api-method"><span class="method post">POST</span> <code>/api/roles</code></div>
          <p class="api-desc">创建一个新的角色。</p>
          <p><strong>权限要求：</strong><code>role:create</code></p>
          <h4>请求参数（Body）</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
            <tr><td>name</td><td>string</td><td>是</td><td>角色名称</td></tr>
            <tr><td>code</td><td>string</td><td>是</td><td>角色编码（唯一）</td></tr>
            <tr><td>description</td><td>string</td><td>否</td><td>角色描述</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X POST http://localhost:8080/api/roles \
  -H "Authorization: Bearer &lt;token&gt;" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "部门经理",
    "code": "dept_manager",
    "description": "部门管理人员"
  }'</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": {
    "id": 3,
    "name": "部门经理",
    "code": "dept_manager",
    "description": "部门管理人员",
    "status": 1
  }
}</pre>
        </div>

        <!-- 4.3 删除角色 -->
        <div class="api-item" id="api-role-delete">
          <h3>4.3 删除角色</h3>
          <div class="api-method"><span class="method delete">DELETE</span> <code>/api/roles/:id</code></div>
          <p class="api-desc">删除指定角色。删除角色后，已分配该角色的用户会自动解除关联。</p>
          <p><strong>权限要求：</strong><code>role:delete</code></p>
          <h4>路径参数</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>说明</th></tr>
            <tr><td>id</td><td>int</td><td>角色ID</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X DELETE http://localhost:8080/api/roles/3 \
  -H "Authorization: Bearer &lt;token&gt;"</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": null
}</pre>
        </div>

        <!-- 4.4 获取自动分配规则 -->
        <div class="api-item" id="api-role-auto-assign-get">
          <h3>4.4 获取自动分配规则</h3>
          <div class="api-method"><span class="method get">GET</span> <code>/api/roles/:id/auto-assign</code></div>
          <p class="api-desc">获取指定角色的自动分配规则列表。</p>
          <p><strong>权限要求：</strong><code>role:list</code></p>
          <h4>路径参数</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>说明</th></tr>
            <tr><td>id</td><td>int</td><td>角色ID</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X GET http://localhost:8080/api/roles/2/auto-assign \
  -H "Authorization: Bearer &lt;token&gt;"</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": [
    {
      "id": 1,
      "roleId": 2,
      "ruleType": "group",
      "ruleValue": "3",
      "createdAt": "2024-01-15T10:00:00Z"
    }
  ]
}</pre>
        </div>

        <!-- 4.5 更新自动分配规则 -->
        <div class="api-item" id="api-role-auto-assign-update">
          <h3>4.5 更新自动分配规则</h3>
          <div class="api-method"><span class="method put">PUT</span> <code>/api/roles/:id/auto-assign</code></div>
          <p class="api-desc">更新指定角色的自动分配规则（会覆盖现有规则）。</p>
          <p><strong>权限要求：</strong><code>role:update</code></p>
          <h4>路径参数</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>说明</th></tr>
            <tr><td>id</td><td>int</td><td>角色ID</td></tr>
          </table>
          <h4>请求参数（Body）</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
            <tr><td>rules</td><td>array</td><td>是</td><td>规则列表</td></tr>
            <tr><td>rules[].ruleType</td><td>string</td><td>是</td><td>规则类型：group / job_title</td></tr>
            <tr><td>rules[].ruleValue</td><td>string</td><td>是</td><td>规则值（分组ID 或 职位名称）</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X PUT http://localhost:8080/api/roles/2/auto-assign \
  -H "Authorization: Bearer &lt;token&gt;" \
  -H "Content-Type: application/json" \
  -d '{
    "rules": [
      { "ruleType": "group", "ruleValue": "3" },
      { "ruleType": "job_title", "ruleValue": "工程师" }
    ]
  }'</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": null
}</pre>
        </div>

        <!-- 4.6 立即执行自动分配 -->
        <div class="api-item" id="api-role-auto-assign-apply">
          <h3>4.6 立即执行自动分配</h3>
          <div class="api-method"><span class="method post">POST</span> <code>/api/roles/auto-assign/apply</code></div>
          <p class="api-desc">根据当前配置的所有角色自动分配规则，立即对现有用户执行角色分配。</p>
          <p><strong>权限要求：</strong><code>role:update</code></p>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X POST http://localhost:8080/api/roles/auto-assign/apply \
  -H "Authorization: Bearer &lt;token&gt;"</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": {
    "totalRules": 3,
    "totalAssigned": 12,
    "details": [
      { "roleId": 2, "roleName": "普通用户", "assigned": 8 },
      { "roleId": 3, "roleName": "部门经理", "assigned": 4 }
    ]
  }
}</pre>
        </div>
      </div>

      <!-- 5. 日志管理接口 -->
      <div class="doc-section" id="log-api">
        <h2>5. 日志管理接口</h2>
        <p>日志接口均为只读（GET），不提供删除和修改日志的能力。</p>

        <!-- 5.1 查询登录日志 -->
        <div class="api-item" id="api-log-login">
          <h3>5.1 查询登录日志</h3>
          <div class="api-method"><span class="method get">GET</span> <code>/api/logs/login</code></div>
          <p class="api-desc">查询用户登录日志记录，支持分页和搜索。</p>
          <p><strong>权限要求：</strong><code>log:login</code></p>
          <h4>请求参数（Query）</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
            <tr><td>pageIndex</td><td>int</td><td>否</td><td>页码，从 0 开始</td></tr>
            <tr><td>pageSize</td><td>int</td><td>否</td><td>每页数量，默认 20</td></tr>
            <tr><td>username</td><td>string</td><td>否</td><td>按用户名筛选</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X GET "http://localhost:8080/api/logs/login?pageIndex=0&pageSize=20" \
  -H "Authorization: Bearer &lt;token&gt;"</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": {
    "list": [
      {
        "id": 100,
        "userId": 1,
        "username": "admin",
        "ip": "192.168.1.100",
        "userAgent": "Mozilla/5.0 ...",
        "status": 1,
        "message": "登录成功",
        "createdAt": "2024-06-15T09:30:00Z"
      }
    ],
    "total": 258
  }
}</pre>
          <h4>字段说明</h4>
          <table class="doc-table">
            <tr><th>字段</th><th>说明</th></tr>
            <tr><td>status</td><td>1=成功，0=失败</td></tr>
            <tr><td>message</td><td>登录结果描述（如"登录成功"、"密码错误"）</td></tr>
          </table>
        </div>

        <!-- 5.2 查询操作日志 -->
        <div class="api-item" id="api-log-operation">
          <h3>5.2 查询操作日志</h3>
          <div class="api-method"><span class="method get">GET</span> <code>/api/logs/operation</code></div>
          <p class="api-desc">查询系统操作日志记录（包含用户管理、角色管理、系统配置等操作的审计记录）。</p>
          <p><strong>权限要求：</strong><code>log:operation</code></p>
          <h4>请求参数（Query）</h4>
          <table class="doc-table">
            <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
            <tr><td>pageIndex</td><td>int</td><td>否</td><td>页码，从 0 开始</td></tr>
            <tr><td>pageSize</td><td>int</td><td>否</td><td>每页数量，默认 20</td></tr>
            <tr><td>username</td><td>string</td><td>否</td><td>按操作人筛选</td></tr>
            <tr><td>module</td><td>string</td><td>否</td><td>按模块筛选</td></tr>
          </table>
          <h4>请求示例</h4>
          <pre class="code-block">curl -X GET "http://localhost:8080/api/logs/operation?pageIndex=0&pageSize=20&module=用户管理" \
  -H "Authorization: Bearer &lt;token&gt;"</pre>
          <h4>成功响应</h4>
          <pre class="code-block">{
  "success": true,
  "data": {
    "list": [
      {
        "id": 50,
        "userId": 1,
        "username": "admin",
        "module": "用户管理",
        "action": "新增用户",
        "target": "zhangsan",
        "content": "",
        "ip": "192.168.1.100",
        "createdAt": "2024-06-15T10:00:00Z"
      }
    ],
    "total": 120
  }
}</pre>
          <h4>模块值参考</h4>
          <table class="doc-table">
            <tr><th>模块值</th><th>说明</th></tr>
            <tr><td>用户管理</td><td>用户的增删改、状态更新、密码重置</td></tr>
            <tr><td>角色管理</td><td>角色的增删改、权限分配</td></tr>
            <tr><td>安全中心</td><td>白名单管理、安全配置变更</td></tr>
            <tr><td>系统设置</td><td>UI配置、LDAP、HTTPS等系统配置</td></tr>
            <tr><td>钉钉管理</td><td>钉钉同步、配置变更</td></tr>
          </table>
        </div>
      </div>

      <!-- 6. 错误码说明 -->
      <div class="doc-section" id="error-codes">
        <h2>6. 错误码说明</h2>
        <table class="doc-table">
          <tr><th>HTTP 状态码</th><th>含义</th><th>说明</th></tr>
          <tr><td>200</td><td>成功</td><td>请求处理成功</td></tr>
          <tr><td>400</td><td>请求错误</td><td>请求参数有误</td></tr>
          <tr><td>401</td><td>未认证</td><td>Token 无效或已过期，需重新登录</td></tr>
          <tr><td>403</td><td>无权限</td><td>当前用户无权访问此接口，或 IP 不在白名单中</td></tr>
          <tr><td>404</td><td>不存在</td><td>请求的资源不存在</td></tr>
          <tr><td>429</td><td>请求过多</td><td>超出频率限制，请稍后重试</td></tr>
          <tr><td>500</td><td>服务器错误</td><td>服务器内部错误</td></tr>
        </table>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ArrowLeft, Download } from "@element-plus/icons-vue";

const docsRef = ref<HTMLElement>();
const pdfLoading = ref(false);
const today = new Date().toLocaleDateString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" });

const downloadPDF = async () => {
  pdfLoading.value = true;
  try {
    const html2pdf = (await import("html2pdf.js")).default;
    const element = docsRef.value;
    if (!element) return;

    const opt = {
      margin: [10, 15, 10, 15],
      filename: `API接口文档_${today}.pdf`,
      image: { type: "jpeg", quality: 0.98 },
      html2canvas: {
        scale: 2,
        useCORS: true,
        letterRendering: true,
      },
      jsPDF: {
        unit: "mm",
        format: "a4",
        orientation: "portrait",
      },
      pagebreak: { mode: ["avoid-all", "css", "legacy"] },
    };

    await html2pdf().set(opt).from(element).save();
  } catch (e) {
    console.error("PDF generation failed:", e);
  } finally {
    pdfLoading.value = false;
  }
};
</script>

<style scoped>
/* ============================================
   Corporate API Docs - 企业风格
   ============================================ */
.api-docs-page {
  padding: 16px;
  background: var(--page-bg);
  min-height: 100vh;
  
}


.docs-toolbar {
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
  background: var(--card-bg);
  padding: 12px 20px;
  border-radius: var(--card-radius);
  box-shadow: var(--card-shadow);
  border: 1px solid var(--card-border);
}

.docs-content {
  background: var(--card-bg);
  border-radius: var(--card-radius);
  padding: 40px 60px;
  max-width: 960px;
  margin: 0 auto;
  box-shadow: var(--card-shadow);
  border: 1px solid var(--card-border);
  font-size: var(--font-size-base);
  line-height: 1.8;
  color: var(--color-text-primary);
}

/* 封面 */
.doc-cover {
  text-align: center;
  padding: 60px 0 40px;
  border-bottom: 2px solid var(--color-border-secondary);
  margin-bottom: 40px;
}

.doc-cover h1 {
  font-size: var(--font-size-3xl);
  font-weight: 700;
  color: var(--color-text-primary);
  margin-bottom: 8px;
}

.doc-subtitle {
  font-size: 16px;
  color: var(--color-text-secondary);
  margin-bottom: 30px;
}

.doc-meta {
  font-size: 13px;
  color: var(--color-text-tertiary);
}

.doc-meta p {
  margin: 4px 0;
}

/* 目录 */
.toc {
  list-style: none;
  padding-left: 0;
}

.toc li {
  padding: 4px 0;
}

.toc li a {
  color: var(--color-primary);
  text-decoration: none;
}

.toc li a:hover {
  text-decoration: underline;
}

.toc ul {
  list-style: none;
  padding-left: 24px;
}

/* 章节 */
.doc-section {
  margin-bottom: 40px;
  page-break-inside: avoid;
}

.doc-section h2 {
  font-size: 22px;
  font-weight: 600;
  color: #1d1d1f;
  border-bottom: 2px solid var(--color-primary);
  padding-bottom: 8px;
  margin-bottom: 20px;
}

.doc-section h3 {
  font-size: 17px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin-top: 24px;
  margin-bottom: 12px;
}

.doc-section h4 {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-secondary);
  margin-top: 16px;
  margin-bottom: 8px;
}

.doc-section p {
  margin: 8px 0;
}

/* API 方法标签 */
.api-item {
  margin-bottom: 36px;
  padding-bottom: 28px;
  border-bottom: 1px solid var(--color-border-secondary);
}

.api-method {
  margin-bottom: 8px;
  font-size: 15px;
}

.api-method code {
  font-size: 14px;
  color: var(--color-text-primary);
  margin-left: 8px;
}

.method {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 700;
  color: white;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.method.get { background: var(--color-success); }
.method.post { background: var(--color-primary); }
.method.put { background: var(--color-warning); }
.method.delete { background: var(--color-error); }

.api-desc {
  color: var(--color-text-secondary);
}

/* 表格 */
.doc-table {
  width: 100%;
  border-collapse: collapse;
  margin: 8px 0 16px;
  font-size: 13px;
}

.doc-table th,
.doc-table td {
  border: 1px solid var(--color-border-secondary);
  padding: 8px 12px;
  text-align: left;
}

.doc-table th {
  background: var(--color-fill-secondary);
  font-weight: 600;
  color: var(--color-text-primary);
}

.doc-table td {
  color: var(--color-text-secondary);
}

.doc-table .label-col {
  width: 120px;
  font-weight: 600;
  color: var(--color-text-primary);
  background: var(--color-fill-secondary);
}

.doc-table code {
  background: var(--color-bg-layout);
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 12px;
}

/* 代码块 */
.code-block {
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 16px 20px;
  border-radius: 6px;
  font-family: "Fira Code", "Consolas", monospace;
  font-size: 12.5px;
  line-height: 1.6;
  overflow-x: auto;
  white-space: pre;
  margin: 8px 0 16px;
}

/* 警告 */
.api-warning {
  background: #fdf6ec;
  border-left: 4px solid var(--color-warning);
  padding: 12px 16px;
  margin-top: 12px;
  border-radius: 0 4px 4px 0;
  font-size: 13px;
  color: var(--color-warning);
}

.api-warning strong {
  color: #c45e00;
}

/* 通用 code */
code {
  background: var(--color-bg-layout);
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 12px;
  font-family: "Fira Code", "Consolas", monospace;
}

/* 打印优化 */
@media print {
  .docs-toolbar { display: none; }
  .api-docs-page { padding: 0; background: var(--color-bg-container); }
  .docs-content { box-shadow: none; padding: 20px; max-width: none; }
  .code-block { background: var(--color-bg-layout) !important; color: #333 !important; border: 1px solid #ddd; }
}
</style>
