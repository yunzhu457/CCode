# C Code 对话 MVP 任务清单

## 任务

- [ ] 1. 初始化 Go 项目结构
  - 影响文件：`go.mod`、`cmd/ccode/main.go`、`internal/`
  - 依赖任务：无
  - 参考位置：`spec.md`

- [ ] 2. 定义配置模型和示例 YAML
  - 影响文件：`internal/config/config.go`、`internal/config/config_test.go`、`configs/example.yaml`、`.gitignore`
  - 依赖任务：任务 1
  - 参考位置：`spec.md` 中的 YAML 配置要求

- [ ] 3. 实现配置加载和校验
  - 影响文件：`internal/config/config.go`、`internal/config/config_test.go`、`cmd/ccode/main.go`
  - 依赖任务：任务 2
  - 参考位置：必填字段 `protocol`、`model`、`base_url`、`api_key`

- [ ] 4. 定义 Provider 无关的对话类型和流式接口
  - 影响文件：`internal/provider/provider.go`、`internal/provider/types.go`、`internal/chat/session.go`
  - 依赖任务：任务 1
  - 参考位置：`spec.md` 中的 Provider 抽象和多轮记忆要求

- [ ] 5. 构建可复用 SSE 流读取器
  - 影响文件：`internal/sse/reader.go`、`internal/sse/reader_test.go`
  - 依赖任务：任务 4
  - 参考位置：OpenAI streaming guide、Anthropic streaming messages guide

- [ ] 6. 实现 OpenAI Provider
  - 影响文件：`internal/provider/openai/openai.go`、`internal/provider/openai/openai_test.go`
  - 依赖任务：任务 4、任务 5
  - 参考位置：https://developers.openai.com/api/docs/guides/streaming-responses

- [ ] 7. 实现 Anthropic Provider
  - 影响文件：`internal/provider/anthropic/anthropic.go`、`internal/provider/anthropic/anthropic_test.go`
  - 依赖任务：任务 4、任务 5
  - 参考位置：https://platform.claude.com/docs/en/build-with-claude/streaming

- [ ] 8. 增加 Claude extended thinking 处理
  - 影响文件：`internal/provider/anthropic/thinking.go`、`internal/provider/anthropic/anthropic_test.go`、`configs/example-anthropic-thinking.yaml`
  - 依赖任务：任务 7
  - 参考位置：https://platform.claude.com/docs/en/build-with-claude/extended-thinking

- [ ] 9. 实现内存中的多轮会话历史
  - 影响文件：`internal/chat/session.go`、`internal/chat/session_test.go`
  - 依赖任务：任务 4
  - 参考位置：`spec.md` 中的多轮记忆要求

- [ ] 10. 构建初始终端 TUI 循环
  - 影响文件：`internal/tui/app.go`、`internal/tui/app_test.go`、`cmd/ccode/main.go`
  - 依赖任务：任务 3、任务 4、任务 9
  - 参考位置：`spec.md` 中的交互式 TUI 和流式输出要求

- [ ] 11. 从配置接入 Provider 选择
  - 影响文件：`internal/provider/factory.go`、`internal/provider/factory_test.go`、`cmd/ccode/main.go`
  - 依赖任务：任务 3、任务 6、任务 7
  - 参考位置：`protocol` 字段决定后端协议

- [ ] 12. 增加终端安全错误处理和 API Key 脱敏
  - 影响文件：`internal/config/`、`internal/provider/`、`internal/tui/`、`internal/logging/`
  - 依赖任务：任务 3、任务 6、任务 7、任务 10
  - 参考位置：`spec.md` 中的非功能要求

- [ ] 13. 接入主流程
  - 影响文件：`cmd/ccode/main.go`、`README.md`、`configs/example.yaml`
  - 依赖任务：任务 1 至任务 12
  - 参考位置：`spec.md` 中的启动和使用流程

- [ ] 14. 端到端验证
  - 影响文件：`README.md`、`internal/**/testdata/` 下的测试夹具、可选 smoke test 脚本
  - 依赖任务：任务 13
  - 参考位置：`checklist.md`
