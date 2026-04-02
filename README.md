# Arcee Bridge

> 自动注册 Arcee 账号，并将会话能力暴露为 OpenAI 兼容接口。

---

## 概览

这个项目做两件事：

- 用 YYDS Mail 自动创建邮箱并完成 Arcee 注册
- 将拿到的 `access_token` 封装成 OpenAI 风格服务，方便接入现有客户端

它不是一个空壳代理，而是一条完整链路：

`创建邮箱 -> 注册账号 -> 收取验证邮件 -> 访问验证链接 -> 登录 -> 保存 access_token -> 启动 OpenAI 兼容网关`

---

## 快速开始

### 1. 注册账号

```powershell
go run .
```

默认会输出：

```text
signup email=xxx@xiaodai.eu.cc
password=xxxxx
verify_link=https://api.arcee.ai/app/v1/verify-email/xxxx
verified status=200
access_token=xxxx
```

注册成功后，会自动写入 [access_token.json](C:\Users\HUAWEI\Desktop\arcee\access_token.json)。

### 2. 启动服务

```powershell
go run . -mode serve
```

默认监听：

```text
http://127.0.0.1:8787
```

---

## 功能清单

### 已完成

- 自动注册 Arcee
- 自动轮询确认邮件
- 自动提取 `verify-email` 链接
- 自动访问验证链接
- 自动登录获取 `access_token`
- 自动写入 `access_token.json`
- 提供 `/models`
- 提供 `/v1/models`
- 提供 `/v1/chat/completions`
- 支持三种模型名映射
- 支持最小 `web_search` 工具开关传递

### 当前定位

- 适合接入 OpenAI 风格客户端
- 适合做 Codex / 代理层 / 网关层实验
- 不是 100% 完整 OpenAI 协议复刻

---

## 项目结构

```text
arcee/
├─ main.go
├─ signup.go
├─ server.go
├─ config/
│  └─ config.go
├─ arcee/
│  ├─ client.go
│  ├─ flow.go
│  └─ chat.go
├─ yydsmail/
│  ├─ client.go
│  ├─ mailbox.go
│  ├─ messages.go
│  └─ inspect.go
├─ config.json
├─ access_token.json
└─ README.md
```

### 核心职责

- [main.go](C:\Users\HUAWEI\Desktop\arcee\main.go)
  入口层。只做参数解析和模式分发。
- [signup.go](C:\Users\HUAWEI\Desktop\arcee\signup.go)
  注册工作流。负责注册、轮询邮件、验证邮箱、登录、保存 token。
- [server.go](C:\Users\HUAWEI\Desktop\arcee\server.go)
  OpenAI 兼容网关。负责模型列表和聊天接口。
- [config.go](C:\Users\HUAWEI\Desktop\arcee\config\config.go)
  配置模块。负责 `config.json` 和 `access_token.json` 的读取与写入。
- [client.go](C:\Users\HUAWEI\Desktop\arcee\arcee\client.go)
  Arcee HTTP 客户端。
- [flow.go](C:\Users\HUAWEI\Desktop\arcee\arcee\flow.go)
  Arcee 注册到登录的流程编排。
- [chat.go](C:\Users\HUAWEI\Desktop\arcee\arcee\chat.go)
  Arcee 对话接口封装。
- [messages.go](C:\Users\HUAWEI\Desktop\arcee\yydsmail\messages.go)
  邮件解析和验证链接提取。

---

## 运行模式

### Signup 模式

默认模式就是注册模式：

```powershell
go run .
```

执行后会自动：

1. 创建 YYDS Mail 邮箱
2. 调用 Arcee 注册接口
3. 轮询邮箱消息
4. 抽取验证链接
5. 请求验证链接
6. 调用登录接口
7. 保存 token 到本地

### Serve 模式

服务模式命令：

```powershell
go run . -mode serve
```

`access_token` 读取顺序：

1. `config.json` 中的 `server.access_token`
2. [access_token.json](C:\Users\HUAWEI\Desktop\arcee\access_token.json)

这意味着通常只需先跑一次注册，后续服务可以直接复用上一次的 token。

---

## 配置说明

配置文件是 [config.json](C:\Users\HUAWEI\Desktop\arcee\config.json)。

```json
{
  "mode": "signup",
  "signup": {
    "api_key": "你的YYDS邮箱密钥",
    "domain": "xiaodai.eu.cc"
  },
  "server": {
    "access_token": "",
    "listen": "127.0.0.1:8787",
    "openai_api_key": "daiju",
    "base_model_name": "trinity-large-thinking",
    "enabled_tools": [
      "web_search"
    ]
  }
}
```

### 字段含义

| 字段 | 说明 |
| --- | --- |
| `mode` | 默认运行模式，支持 `signup` / `serve` |
| `signup.api_key` | YYDS Mail 密钥 |
| `signup.domain` | 固定邮箱域名 |
| `server.access_token` | 手动指定 token，留空时从 `access_token.json` 读取 |
| `server.listen` | 服务监听地址 |
| `server.openai_api_key` | 本地 OpenAI 兼容服务的 Bearer Key |
| `server.base_model_name` | 默认模型名 |
| `server.enabled_tools` | 传给 Arcee 的工具开关列表 |

---

## Token 文件

[access_token.json](C:\Users\HUAWEI\Desktop\arcee\access_token.json) 会在注册成功后自动生成。

示例：

```json
{
  "access_token": "xxxx",
  "email": "xxx@xiaodai.eu.cc",
  "password": "xxxx",
  "verify_link": "https://api.arcee.ai/app/v1/verify-email/xxxx",
  "created_at": "2026-04-02T10:00:00+08:00"
}
```

它用于：

- 保存本次注册得到的账号信息
- 为服务模式提供默认 token 来源

这个文件已经被 [.gitignore](C:\Users\HUAWEI\Desktop\arcee\.gitignore) 忽略。

---

## OpenAI 兼容接口

服务启动后默认提供以下接口：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/healthz` | 健康检查 |
| `GET` | `/models` | 模型列表 |
| `GET` | `/v1/models` | OpenAI 风格模型列表 |
| `POST` | `/v1/chat/completions` | 聊天补全 |

### 获取模型列表

```powershell
curl http://127.0.0.1:8787/v1/models `
  -H "Authorization: Bearer daiju"
```

也支持：

```powershell
curl http://127.0.0.1:8787/models `
  -H "Authorization: Bearer daiju"
```

### 发起聊天

```powershell
curl http://127.0.0.1:8787/v1/chat/completions `
  -H "Authorization: Bearer daiju" `
  -H "Content-Type: application/json" `
  --data-raw "{\"model\":\"trinity-mini\",\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}"
```

### 流式输出

```powershell
curl http://127.0.0.1:8787/v1/chat/completions `
  -H "Authorization: Bearer daiju" `
  -H "Content-Type: application/json" `
  --data-raw "{\"model\":\"trinity-large-thinking\",\"stream\":true,\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}"
```

---

## 模型支持

当前会暴露三个模型名：

- `trinity-mini`
- `trinity-large-preview`
- `trinity-large-thinking`

模型解析规则：

- 请求里传了上述三者之一，就按请求走
- 没传时，回退到 `config.server.base_model_name`
- 如果配置也没给，则默认 `trinity-large-thinking`

---

## 工具支持

当前工具兼容走最小实现，核心是：

- `web_search`

也就是说，项目当前更偏向“可用代理层”，不是完整的 OpenAI Tools 协议镜像。

---

## 鉴权说明

### Arcee 上游

Arcee 对话调用实际依赖：

- `access_token`

当前项目转发到 Arcee 时使用：

```text
Cookie: access_token=...
```

### 本地兼容服务

如果 [config.json](C:\Users\HUAWEI\Desktop\arcee\config.json) 里设置了：

```json
"openai_api_key": "daiju"
```

则本地调用需要：

```text
Authorization: Bearer daiju
```

如果该值为空，则本地服务不校验这层 Bearer Key。

---

## 推荐使用方式

### 方式一：完整链路

1. 配置 [config.json](C:\Users\HUAWEI\Desktop\arcee\config.json) 中的 `signup.api_key`
2. 执行 `go run .`
3. 确认 [access_token.json](C:\Users\HUAWEI\Desktop\arcee\access_token.json) 已生成
4. 执行 `go run . -mode serve`
5. 将客户端 Base URL 指向 `http://127.0.0.1:8787/v1`

### 方式二：已有 token 直接起服务

1. 将 token 写入 `config.server.access_token`
2. 执行 `go run . -mode serve`

---

## 常见问题

### 为什么注册后要等邮件

因为程序会轮询 YYDS Mail，直到确认邮件到达并成功提取验证链接。

当前默认轮询策略：

- 间隔 `2s`
- 超时 `20s`

### 为什么验证完还要登录

因为邮箱验证只是激活账号；真正拿到会话能力，还需要登录获取 `access_token`。

### 为什么要保留 `access_token.json`

因为服务模式会自动读取它。这样注册和服务之间不需要手动复制 token。

### 为什么同时有 `/models` 和 `/v1/models`

为了兼容不同客户端。有些客户端只探测 `/models`，有些严格使用 `/v1/models`。

---

## 安全提示

- 不要提交 [config.json](C:\Users\HUAWEI\Desktop\arcee\config.json)
- 不要提交 [access_token.json](C:\Users\HUAWEI\Desktop\arcee\access_token.json)
- `access_token` 有时效，不是永久凭证
- 这套兼容层是实用封装，不是完整协议替身

---

## 当前状态

这个项目现在已经不是“半成品脚本”，而是一个可直接使用的桥接器：

- 上游会自动拿号
- 中间会自动验证和登录
- 下游能以 OpenAI 风格接口直接消费

如果继续扩展，下一步最值得做的是：

- 更完整的 Tools 兼容
- 更标准的 SSE 分块
- 显式 `chat_id` 会话续聊管理

---

## Star History

[![Star History Chart](https://starchart.cc/xn030523/arcee.svg?variant=adaptive)](https://starchart.cc/xn030523/arcee)
