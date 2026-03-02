# Go-SyncFlow - SSO 使用手册（CAS / SAML / OIDC）

> 版本：v3.4 | 更新日期：2026-02-12

本文档面向管理员与应用接入方，描述如何在 Go-SyncFlow 中完成第三方应用的单点登录接入、统一权限下发与常见排错。

## 1. 术语与模型

- 应用：在“单点登录 → 应用管理”里创建的第三方系统条目
- 协议：CAS / SAML2.0 / OAuth2(OIDC)
- 应用权限项（Entitlement）：第三方应用侧的角色/权限/Scope 值
- 授权分配（Grant）：把应用权限项分配给 Go-SyncFlow 的角色或用户
- 输出模板（Template）：对 SAML 追加 Attribute（云厂商角色联合等场景）

## 2. 推荐接入路线

- 内部网站 / 可改代码系统：优先 OIDC（JWT + scope/roles/permissions）
- 需要标准企业联合认证：SAML
- 群晖等兼容性优先：CAS

## 3. 通用配置流程（必做）

1) 单点登录 → 应用管理：创建应用
2) 勾选协议并填写配置（无需手写 JSON）
3) 单点登录 → 权限管理
   - 应用权限项：先定义应用侧 role/permission/scope
   - 授权分配：将这些权限项分配给角色或用户
   - 输出模板：按需（SAML 追加 Attribute）
   - 映射：按需（把 Go-SyncFlow 本地角色/权限映射为应用侧值）
4) 单点登录 → 接入配置：核对端点
5) 单点登录 → 登录日志：排错与审计

## 4. CAS（以群晖 DSM 为例）

### 4.1 Go-SyncFlow 侧

- 应用管理：启用 CAS
- CAS 配置：添加 service 白名单（与群晖跳转的 service 必须完全一致，含 http/https、端口、是否带路径）

示例：

```
https://172.16.220.188:5001
```

### 4.2 群晖侧（CAS SSO 设置）

- 服务 ID：填写 DSM 访问地址，例如 `https://172.16.220.188:5001`
- 服务器 URL：`http(s)://<Go-SyncFlow>:8080/cas`
- 服务器验证 URL：`http(s)://<Go-SyncFlow>:8080/cas/serviceValidate`

### 4.3 常见问题

- 一直转圈：通常是校验失败或账号映射失败；看 Go-SyncFlow 登录日志的 validate 记录
- “帐号或密码错误”：CAS 返回的用户名在 DSM 里找不到匹配账号，或帐户类型选择不正确

## 5. OIDC（内部网站/第三方 SaaS）

### 5.1 关键端点

- 授权：`/oauth/authorize`
- 换取 token：`/oauth/token`
- UserInfo：`/oidc/userinfo`
- JWKS：`/oidc/jwks`

### 5.2 下发字段（默认）

- `preferred_username`
- `roles`（数组）
- `permissions`（数组）
- `scope`（字符串：请求 scope + 授权分配 scope 合并）

### 5.3 应用侧验签建议

- 通过 `/.well-known/openid-configuration` 获取 `jwks_uri`
- 校验 `iss/aud/exp` + RS256 签名

## 6. SAML（通用 + 云厂商模板）

### 6.1 下发字段（默认 Attribute）

- roles
- permissions
- email / phone（存在则下发）

### 6.2 输出模板（追加 Attribute）

在“权限管理 → 输出模板”中填 JSON：

```json
{
  "extraAttributes": {
    "https://example.com/Attr": ["v1", "v2"]
  }
}
```

### 6.3 云厂商参考

- 阿里云：常见为角色联合，需固定 Attribute 名 `https://www.aliyun.com/SAML-Role/Attributes/Role` 与 `RoleSessionName`
- 腾讯云：CAM 身份提供商角色，通常需要固定字段并按文档要求换取临时凭证

## 7. 排错清单（先看这里）

- 协议是否启用（应用管理里是否勾选并保存）
- service/redirect_uri/ACS 是否严格匹配
- 账号是否存在与是否需要本地映射（尤其是群晖 CAS）
- 登录日志中是否有 login/validate/token/sso 失败记录
- 时间同步（NTP）是否一致（ticket/断言有效期相关）
