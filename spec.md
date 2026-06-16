# C Code 对话 MVP 规格说明

## 背景

C Code 是一个受 Claude Code 启发的命令行 AI 助手。长期方向是先提供终端里的 Coding Agent 能力，后续再支持 iOS 客户端远程控制，就像 Codex 与 Codex 移动端之间的关系。

当前仓库还没有业务代码，所以这份规格只定义第一个可实现切片：一个使用 Go 开发的本地终端对话体验。

## 目标用户

- 希望先在终端里使用 AI 对话助手的开发者。
- 项目早期开发者，他们需要一个清晰的 Provider 抽象，方便后续扩展工具调用、文件操作、代码编辑和远程控制。
- 未来的 Swift/iOS 客户端开发者；本阶段不实现移动端，但架构需要避免把模型调用逻辑和终端渲染强绑定。

## 目标

- 用户可以从终端启动 C Code，并进入交互式 TUI 对话界面。
- 用户可以连续输入多轮问题，AI 能在当前运行会话中记住上下文。
- AI 回复需要以流式方式逐步打印，而不是等完整回复生成后再一次性显示。
- 支持 Anthropic Claude 和 OpenAI 两种 API 后端。
- 使用统一 Provider 接口屏蔽不同模型供应商的协议差异。
- 使用 YAML 配置文件管理模型供应商、模型、地址和认证信息。
- 本阶段只做纯对话，不做工具调用、文件操作、代码编辑或其他 agent 功能。

## 能力范围

- 终端入口：用户可以运行 Go 构建出的 C Code CLI。
- 交互式 TUI：用户可以输入消息、提交消息，并看到 AI 回复持续流式输出。
- 多轮记忆：当前运行会话会保留用户和助手的历史消息，用于后续请求。
- YAML 配置：配置文件包含四个核心字段 `protocol`、`model`、`base_url`、`api_key`。
- 后端切换：通过修改 `protocol` 和相关配置即可在 Anthropic 与 OpenAI 之间切换。
- SSE 流式处理：Provider 负责消费服务端 SSE 事件流，并向 TUI 输出统一的文本增量。
- Claude extended thinking：Anthropic Provider 支持请求 Claude extended thinking，并能处理 thinking 相关流式事件。
- Provider 抽象：新增后端时应优先新增 Provider 实现，而不是改写 TUI 或会话主流程。

## 非功能要求

- 后端和 CLI 使用 Go 开发。
- 终端 UI、会话状态、配置加载、Provider 协议适配需要保持模块边界清晰。
- `api_key` 不得被打印、记录到日志、提交到仓库或出现在普通错误信息中。
- 流式输出应让用户在完整回复结束前看到可见进展。
- 配置错误、Provider 错误、网络中断和流式解析失败都需要给出可理解的终端反馈。
- 架构需要为未来 Swift/iOS 客户端留出空间，避免模型调用逻辑只能被 TUI 使用。

## 设计骨架

C Code 的首个版本按四层组织：

- CLI/启动层：解析启动参数和配置路径，加载配置，启动 TUI，并完成依赖组装。
- TUI 层：负责终端输入、提交行为、流式输出渲染和退出流程。
- Chat/Session 层：维护当前运行会话中的多轮消息，并生成 Provider 无关的请求。
- Provider 层：把统一请求转换成 Anthropic 或 OpenAI 协议请求，解析 SSE 响应，并向上游输出统一事件。

Provider 接口应以“流式对话”为核心，而不是只提供阻塞式完整回复。Anthropic 内容块、OpenAI 事件格式、Claude thinking 事件等细节应限制在 Provider 实现或协议适配器内部。

## 不做范围

- 工具调用、函数调用、MCP、Shell 命令、文件读取、文件写入、代码编辑、仓库索引和自主任务执行。
- 跨应用重启的持久化会话历史。
- 用户账号、云同步、远程会话或 iOS 客户端。
- 多 Provider 自动降级、模型路由、成本统计、Embedding、图片/音频输入或联网搜索。
- 插件系统、沙箱、权限提示和完整工作区安全策略。
- 完整复刻 Claude Code。

## 完成定义

- 仓库中存在可构建的 Go CLI 项目。
- 示例 YAML 配置说明 `protocol`、`model`、`base_url`、`api_key` 四个核心字段。
- 用户可以启动 C Code 并进入终端交互式对话循环。
- Anthropic 和 OpenAI Provider 都实现同一个流式 Provider 接口。
- 支持的 Provider 能在终端中逐步输出模型回复。
- 当前运行会话中保留多轮上下文。
- Anthropic Provider 可以启用 Claude extended thinking，且不会破坏最终文本回复的流式输出。
- 配置加载、Provider 选择、SSE 解析和会话上下文都有针对性测试。
