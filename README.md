# C Code

C Code 是一个早期命令行 AI 助手。当前阶段只实现终端里的多轮流式对话，不包含工具调用、文件操作、代码编辑或 agent 自动执行能力。

## 本地配置

远端仓库只保留模板文件，真实密钥只放在本地被 Git 忽略的配置文件中。

```bash
cp configs/config.example.yaml configs/config.local.yaml
```

编辑 `configs/config.local.yaml`：

```yaml
protocol: openai
model: deepseek-v4-flash
base_url: https://api.deepseek.com
api_key: YOUR_API_KEY_HERE
```

也可以参考 `configs/chatgpt-relay.example.yaml` 配置 OpenAI-compatible 中转服务。

Anthropic Claude extended thinking 可以参考 `configs/anthropic-thinking.example.yaml`：

```yaml
protocol: anthropic
model: claude-sonnet-4-6
base_url: https://api.anthropic.com
api_key: YOUR_API_KEY_HERE
max_tokens: 4096
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
