# C Code

C Code 是一个早期命令行 AI 助手。当前阶段只实现终端里的多轮流式对话，不包含工具调用、文件操作、代码编辑或 agent 自动执行能力。

## 本地配置

远端仓库只保留模板文件，真实密钥只放在本地被 Git 忽略的配置文件中。

```bash
cp configs/config.example.yaml configs/config.local.yaml
```

编辑 `configs/config.local.yaml`：

```yaml
provider: deepseek
protocol: anthropic
compatibility: auto
api: auto
model: deepseek-v4-flash
base_url: https://api.deepseek.com/anthropic
api_key: YOUR_API_KEY_HERE
stream:
  idle_timeout: 30s
```

也可以参考 `configs/chatgpt-relay.example.yaml` 配置 OpenAI-compatible 中转服务。

配置路由字段说明：

- `provider`: 供应商标识，可选 `auto`、`openai`、`anthropic`、`deepseek`、`custom`。
- `protocol`: 请求协议，当前支持 `openai` 和 `anthropic`。
- `compatibility`: `auto` 会自动判断官方或兼容模式；非官方供应商和中转会降级到 compatible。
- `api`: 默认 `auto`。OpenAI 协议可显式写 `chat_completions` 或 `responses`；Anthropic 协议可写 `messages`。
- `stream.idle_timeout`: 流式输出的空闲超时。连续一段时间收不到任何 SSE 事件时，会按网络错误退出，避免终端一直卡住。默认 `30s`。

当前版本已经实现 compatible 路径。官方 SDK / OpenAI Responses 的专用路径是预留模式；`auto` 会先使用现有可运行实现，显式写 `compatibility: official` 或 `api: responses` 时会提示该模式尚未实现。

Anthropic Claude extended thinking 可以参考 `configs/anthropic-thinking.example.yaml`：

```yaml
provider: anthropic
protocol: anthropic
compatibility: auto
api: auto
model: claude-sonnet-4-6
base_url: https://api.anthropic.com
api_key: YOUR_API_KEY_HERE
max_tokens: 4096
stream:
  idle_timeout: 30s
thinking:
  enabled: true
  budget_tokens: 1024
  display: omitted
```

## 运行

```bash
go run ./cmd/ccode
```

指定配置文件：

```bash
go run ./cmd/ccode -config configs/config.local.yaml
```

在交互界面输入问题后回车，使用 `/exit` 或 `/quit` 退出。

## 验证

```bash
go test ./...
go build ./cmd/ccode
```
