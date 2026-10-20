# gmail-coursework-archiver

Gmail 附件归档与课程作业 API 服务。Go 服务从邮件中保存附件、记录 SQLite 元数据，并依据花名册与作业规则生成提交状态、历史版本和归档。

QQ 事件、身份权限和对话指令由独立 AstrBot 插件维护。两者通过受信 HTTP API 对接；接口与身份核验边界见 [v3 后端契约](docs/QQ_HOMEWORK_V3_API.md)。

## 开发与验证

需要 Go 1.26.9 或更高的安全修订版本。常用检查：

```sh
go test ./...
go test -race ./internal/api ./internal/db ./internal/rule ./internal/parser
go build ./cmd/server
nix flake check .
nix build .#packages.x86_64-linux.default --no-link
```

测试使用合成输入和临时目录，不连接邮箱或读取运行时 `data/`。Nix 构建默认检查当前主机架构；目标架构还需在对应构建器上验证。

## 运行配置

| 环境变量 | 用途 |
| --- | --- |
| `IMAP_SERVER` | IMAP 主机与端口 |
| `IMAP_USER` | 邮箱账号，由运行环境注入 |
| `IMAP_PASSWORD` | 邮箱应用密码，由凭据管理器注入 |
| `DATA_DIR` | 数据库、花名册、规则和附件目录 |
| `HTTP_PORT` | HTTP 服务端口 |
| `API_KEY` | 管理级 API 凭据；未设置时 `/api/*` 不可用 |

API 凭据只通过服务环境注入，并使用请求头传递。服务应置于受限网络及 HTTPS 入口之后；不要把凭据放进 URL、聊天消息、源码或 Nix store。运行数据和花名册不属于本仓库。

## 信任边界

后端按 API key 提供管理级能力，不把请求携带的 QQ 身份视为已认证的平台身份。调用方负责验证平台实例、机器人、发送者和角色。姓名与学号不是账号归属证明；自述绑定不会获得提交历史或上传权限，需经过独立核验。旧数字 QQ 绑定不自动转换为 QQ 官方 OpenID。
