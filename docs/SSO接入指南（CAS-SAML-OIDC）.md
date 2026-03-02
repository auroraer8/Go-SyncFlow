# Go-SyncFlow 第三方接入指南（CAS / SAML2.0 / OAuth2 / OpenID Connect）

本文档说明如何把第三方系统接入 Go-SyncFlow 的“单点登录”能力，并获取角色/权限信息用于权限控制。

## 1. 概念

- Go-SyncFlow：作为统一身份源与 SSO 服务端（IdP / Authorization Server）
- 第三方系统：作为 CAS Service / SAML SP / OIDC Client
- 角色/权限输出：
  - roles：角色数组（默认输出本地角色 code；若配置了映射则输出映射后的值）
  - permissions：权限数组（默认输出本地 permission code；若配置了映射则输出映射后的值）

## 2. 管理端配置（必做）

1) 进入：单点登录 → 应用管理
2) 创建应用：填写 name/code、访问控制、启用协议（CAS/SAML/OIDC）
3) 进入：单点登录 → 授权策略
   - 映射：把 Go-SyncFlow 的角色/权限映射为第三方应用可识别的值（可选）
   - 应用权限项：维护第三方应用侧的角色/权限/Scope 列表（推荐）
   - 授权分配：把应用权限项分配给 Go-SyncFlow 的角色或用户（推荐）
   - 输出模板：为 SAML 输出额外 Attribute（云厂商角色联合等场景，按需）
4) 进入：单点登录 → 接入配置
   - 查看各协议端点

说明：应用的协议参数（redirect_uris、ACS URL、service 白名单等）已支持在 UI 里直接填写，不需要手写 JSON；底层会以 config JSON 形式保存。
说明：协议开关以“启用的协议”为准保存，避免出现只勾选 CAS 但保存后全选的问题。

## 3. OIDC / OAuth2（推荐）

### 3.1 端点

- Discovery：`/.well-known/openid-configuration`
- JWKS：`/oidc/jwks`
- Authorize：`/oauth/authorize`
- Token：`/oauth/token`
- UserInfo：`/oidc/userinfo`

### 3.2 授权码 + PKCE 流程

1) 生成 code_verifier 与 code_challenge（S256）
2) 浏览器跳转到 authorize：

```text
GET /oauth/authorize?
  response_type=code&
  client_id=YOUR_CLIENT_ID&
  redirect_uri=YOUR_REDIRECT_URI&
  scope=openid%20profile%20roles%20permissions&
  state=STATE&
  code_challenge=CODE_CHALLENGE&
  code_challenge_method=S256
```

3) 获取回调 `redirect_uri?code=...&state=...`
4) 用 code 换 token：

```bash
curl -X POST "https://YOUR_HOST/oauth/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=authorization_code" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  -d "redirect_uri=YOUR_REDIRECT_URI" \
  -d "code=AUTH_CODE" \
  -d "code_verifier=CODE_VERIFIER"
```

5) 使用 access_token 调用 userinfo：

```bash
curl "https://YOUR_HOST/oidc/userinfo" \
  -H "Authorization: Bearer ACCESS_TOKEN"
```

### 3.3 返回数据（示例）

ID Token / Access Token 可能包含：
- `sub`
- `preferred_username`
- `email` / `phone`
- `roles` / `permissions`

UserInfo 返回同样的 claims。

## 4. CAS（兼容传统系统）

### 4.1 端点

- 登录：`/cas/login?service=...`
- 校验：`/cas/serviceValidate?service=...&ticket=...`
- 注销：`/cas/logout`

### 4.2 流程（简化）

1) 浏览器跳转：

```text
GET /cas/login?service=https%3A%2F%2Fapp.example.com%2Fcas
```

2) 成功后回跳到 service，并带上 `ticket=ST-...`
3) 应用后端调用校验端点：

```text
GET /cas/serviceValidate?service=...&ticket=...
```

4) 响应为 CAS XML，包含 user 与 attributes（roles/permissions/email/phone）。

## 5. SAML2.0（企业应用常用）

### 5.1 端点

- 元数据：`/saml/{appCode}/metadata`
- SSO（POST Binding）：`/saml/{appCode}/sso`

### 5.2 SP 配置要点

- 从 metadata 导入 IdP 信息（证书、SSO URL）
- 配置 SP 的 ACS URL 与 EntityID（需要在应用的 SAML 配置中保持一致）
- Attribute 读取：roles/permissions/email/phone

说明：当前版本输出的是可用的 SAML Response/Assertion（含 attributes），签名/加密增强可以在后续版本开启与完善。

## 6. 登录态承载说明

当前协议端点支持两种登录态承载：
- API 调用：`Authorization: Bearer <本系统JWT>`
- 浏览器跳转：登录成功后会写入 `gsf_token`（HttpOnly Cookie），用于 CAS/OIDC 的浏览器跳转式 SSO

未登录访问 `/cas/login`、`/oauth/authorize` 会自动跳转到 `/login?redirect=...`，登录成功后回跳继续流程。

## 6.1 “统一权限管理”如何生效

Go-SyncFlow 会在单点登录下发阶段合并两部分信息：
- 来自用户本身的 Go-SyncFlow 角色/权限（可选映射）
- 来自“授权分配”的应用侧权限项（按 Go-SyncFlow 角色/用户继承）

下发位置：
- OIDC：`roles` / `permissions` / `scope`
- SAML：`roles` / `permissions`（以及模板配置的额外 Attribute）
- CAS：`roles` / `permissions`

## 7. 群晖（DSM）对接建议

群晖 DSM 常见的对接方式是 **CAS**（兼容性最好、对签名要求最低）。SAML 在不同 DSM 版本上往往要求签名/绑定方式一致，当前版本建议优先使用 CAS。

### 7.1 在 Go-SyncFlow 创建“群晖”应用

1) 单点登录 → 应用管理：创建应用，例如：
- name：Synology
- code：synology
- 协议：勾选 CAS

2) 单点登录 → 授权策略（可选）：按需配置 role/permission 映射

3) 单点登录 → 应用管理 →（协议配置 JSON）

CAS 需要配置 service 白名单（必须与 DSM 发起的 service 完全一致）。示例（以 5000 为例）：

```json
{
  "services": [
    "http://172.22.1.2:5000"
  ]
}
```

如果你的 DSM 实际跳转携带的 service 不是根域名（例如包含路径），请按实际的 service 值填写到白名单。

### 7.2 在 DSM 启用 CAS SSO

DSM：控制面板 → 登录门户/域/SSO（不同版本入口名称略有差异）→ 启用 CAS SSO 服务，填写：

- CAS 服务器 URL（Base URL）：`http(s)://<Go-SyncFlow域名或IP>:8080/cas`
  - DSM 会自动拼接为：
    - `.../cas/login`
    - `.../cas/serviceValidate`
    - `.../cas/logout`

保存后，在 DSM 登录页选择 SSO 登录，浏览器会跳转到 Go-SyncFlow 登录页；登录成功后回跳 DSM 并完成登录。
