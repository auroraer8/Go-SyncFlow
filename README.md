# Go-SyncFlow 统一身份同步与管理平台

基于 Go + Vue3 的企业级统一身份同步与管理平台。集成用户同步、SSO 单点登录、VPN 远程接入、安全管控于一体，开箱即用。

## 核心功能

### 身份同步

- **上游同步**：从外部系统同步用户/部门到本地
  - IM 平台：钉钉、企业微信、飞书、WeLink
  - 目录服务：LDAP/AD、OpenLDAP
  - 数据库：MySQL、PostgreSQL、Oracle、SQL Server
  - 其他：RADIUS、HTTP API
- **下游同步**：将本地用户推送到外部系统
  - 目录服务：LDAP/AD、OpenLDAP、通用 LDAP
  - 数据库：MySQL、PostgreSQL、Oracle、SQL Server
- **同步策略**：定时同步、手动触发、增量同步、字段映射、群组/OU 级联同步
- **事件同步**：用户创建、删除、禁用、信息更新、密码修改、密码生成、角色变更等事件自动触发下游同步
- **密码生成**：上游同步创建用户时自动生成复杂密码，通过配置的通知渠道（钉钉工作通知、短信等）推送给员工

### SSO 单点登录

- **协议支持**：CAS、SAML2、OAuth2 / OIDC
- **IM 免登**：钉钉扫码登录、企业微信免登、飞书免登
- **SSO 应用中心**：可视化管理 SSO 应用、权限授权、审计日志

### VPN 远程接入

- **协议兼容**：Cisco AnyConnect / OpenConnect / SyncFlow 客户端
- **用户组管理**：按组配置路由策略、DNS、带宽限制、访问控制（ACL）
- **认证方式**：本地用户、SyncFlow 系统用户、LDAP/AD 连接器、RADIUS
- **安全增强**：短信双因子认证（2FA）、OTP 动态口令、密码学习
- **网络功能**：全局/分流路由、域名分流、地址组管理
- **客户端下载**：443 端口内置下载页，提供 macOS / Windows 安装包

### 安全中心

- **双因子认证（2FA）**：登录短信验证码，支持豁免角色和白名单 IP
- **密码策略**：复杂度要求、密码历史检查、强制修改
- **登录安全**：CSRF 防护、RSA 加密传输、登录锁定、IP 黑白名单
- **会话管理**：活跃会话查看、强制登出
- **安全仪表板**：安全评分、威胁来源分析、登录趋势
- **告警规则**：按事件类型和严重级别触发通知

### 密码代理认证

- 将登录认证转发至上游连接器（LDAP/AD、数据库、RADIUS、HTTP API）
- 认证成功后自动学习密码到本地（bcrypt + Samba NT Hash）
- 支持本地优先 + 代理回退模式
- VPN / Web / API 登录均支持密码学习与下游同步

### 通知系统

- **短信**：阿里云、腾讯云、华为云、百度云、天翼云、移动云 MAS、移动 5G 消息、云片、创蓝、企业微信短信、钉钉短信、飞书短信、HTTPS 自定义（14 家）
- **其他**：邮件、Webhook、钉钉工作通知
- **消息策略**：按场景（登录验证、密码重置等）配置通道，支持多通道顺序回退

### 内置 LDAP 服务

- 内嵌轻量 LDAP 服务器，无需额外部署
- 默认支持 Samba 属性（sambaNTPassword、sambaSID 等）
- 兼容群晖 NAS、Windows 域等场景

### OpenAPI

- AppID / AppKey 认证，支持 IP 白名单、速率限制、过期管理
- 用户认证接口、用户同步接口（含密码哈希、增量同步）
- 内置 API 文档页面

### 日志与审计

- 登录日志、操作日志、同步日志、API 调用日志、VPN 日志
- 日志保留策略配置
- 可视化日志查询与导出

## 技术栈

| 层 | 技术 |
|------|------|
| 后端 | Go 1.22+ / Gin / GORM |
| 前端 | Vue3 / Vite / TypeScript / Element Plus / ECharts |
| 数据库 | PostgreSQL（内置，开箱即用） |
| 内嵌服务 | LDAP 服务器（gldap）、HTTPS（自动自签证书） |
| VPN | AnyConnect 协议兼容、DTLS |

## 目录结构

```
Go-SyncFlow/
├── backend/              # Go 后端
│   ├── internal/
│   │   ├── handlers/     # HTTP 控制器（认证、SSO、安全、API Key 等）
│   │   ├── models/       # 数据模型
│   │   ├── services/     # 业务服务（密码代理、通知等）
│   │   ├── sync/         # 同步引擎（上游/下游）
│   │   ├── vpn/          # VPN 接入模块
│   │   ├── ldapserver/   # 内嵌 LDAP 服务
│   │   ├── sms/          # 短信服务商实现（14 家）
│   │   ├── imclient/     # IM 客户端（钉钉/企微/飞书/WeLink）
│   │   ├── middleware/   # 中间件（JWT、限流、CORS 等）
│   │   └── crypto/       # 加解密
│   └── static/           # 前端编译产物
├── frontend/             # Vue3 前端源码
├── scripts/              # 部署脚本
├── docs/                 # 系统文档（MD + PDF）
├── postgres/             # 内置 PostgreSQL
├── pgdata/               # PostgreSQL 数据目录
├── data/                 # 运行时数据（证书、VPN 配置等）
└── README.md
```

## 运行环境

| 项目 | 要求 |
|------|------|
| 操作系统 | Ubuntu 24.04 LTS 及以上 |
| 架构 | x86_64 (amd64) |
| 内存 | 建议 2GB+ |
| 磁盘 | 建议 10GB+ |
| 依赖 | 无需预装，内置 PostgreSQL |

> **注意**：预编译部署包基于 Ubuntu 24.04 构建，内置的 PostgreSQL 和系统库依赖 glibc 2.39+，因此**不兼容**较低版本的系统（如 Ubuntu 22.04、CentOS 7/8 等）。如需在其他系统运行，请自行从源码编译。

## 快速部署

### 一键安装

```bash
tar -xzf go-syncflow-v3.5.tar.gz -C /opt/
cd /opt/Go-SyncFlow && chmod +x scripts/*.sh
./scripts/start.sh
```

内置 PostgreSQL 数据库，无需预装任何依赖，首次启动自动初始化。

### 访问地址

| 服务 | 地址 | 说明 |
|------|------|------|
| 管理后台 | `https://服务器IP:8443` | 首次启动自动生成自签证书 |
| HTTP | `http://服务器IP:8080` | 自动重定向到 HTTPS |
| VPN 入口 | `https://服务器IP` (443) | 含客户端下载页和安装包 |
| LDAP | `服务器IP:389` / `636`(SSL) | 内嵌 LDAP 服务 |

### 默认账号

- 用户名：`admin`
- 密码：`Admin@2024`

## 常用命令

```bash
./scripts/start.sh            # 一键启动（PostgreSQL + 应用）
./scripts/stop.sh             # 停止服务
./scripts/restart.sh          # 重启服务
./scripts/reset-admin.sh      # 重置管理员密码
./scripts/pack.sh             # 打包发布（v3.5）

# 数据库外部连接管理（默认关闭，仅本地访问）
./scripts/enable-external-db.sh           # 交互式菜单
./scripts/enable-external-db.sh enable    # 开启外部连接（允许远程访问）
./scripts/enable-external-db.sh disable   # 关闭外部连接（仅本地）
./scripts/enable-external-db.sh status    # 查看当前状态

systemctl status go-syncflow  # 查看服务状态
journalctl -u go-syncflow -f  # 查看实时日志
```

## 系统文档

| 文档 | 说明 |
|------|------|
| 系统使用手册 | 完整操作指南 |
| VPN 使用手册 | VPN 接入与客户端配置 |
| SSO 使用手册 | 单点登录配置 |
| SSO 接入指南（CAS/SAML/OIDC） | 各协议接入详解 |
| API 接口文档 | OpenAPI 接口说明 |
| 技术架构文档 | 系统架构与设计 |

## 说明

- 部署包为预编译版本，不含源码，解压即用
- 首次启动自动生成 HTTPS 自签证书，也可在管理后台上传自有证书
- LDAP 服务默认启用 Samba 属性支持，可直接对接群晖 NAS
- 通知渠道、同步连接器、SSO 应用等需在管理界面中配置
- VPN 443 端口自带客户端下载页面及安装包

## 致谢

本项目的 VPN 模块基于 [AnyLink](https://github.com/bjdgyc/anylink) 开源项目进行开发，感谢 AnyLink 项目及其贡献者的出色工作。

## 开源协议

本项目采用 [AGPL-3.0](LICENSE) 协议开源。

- 您可以自由使用、修改和分发本软件
- 如果您修改了代码并通过网络提供服务，必须公开修改后的源码
- 所有衍生作品必须同样采用 AGPL-3.0 协议
- 详见 [LICENSE](LICENSE) 文件

### 第三方开源组件

| 组件 | 协议 | 用途 |
|------|------|------|
| [AnyLink](https://github.com/bjdgyc/anylink) | AGPL-3.0 | VPN 远程接入模块（基于 OpenConnect 协议，兼容 AnyConnect 客户端） |
