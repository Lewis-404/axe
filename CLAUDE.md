# Axe - Vibe Coding Agent

## 项目概述
Go 写的 CLI vibe coding agent。用户在终端用自然语言描述需求，axe 自动读取项目上下文、调用 LLM 生成代码、创建/修改文件、执行命令。

## 技术栈
- Go 1.25
- Module: github.com/Lewis-404/axe
- LLM: Anthropic Claude API + OpenAI 兼容接口（支持中转、自定义 base URL）
- 终端输入: github.com/chzyer/readline（支持 CJK 多字节字符）
- 配置: ~/.axe/config.yaml

## 功能

### 核心流程
1. 用户启动 `axe` 进入交互模式，或 `axe "prompt"` 单次执行
2. axe 读取当前目录的项目上下文（CLAUDE.md、文件树、关键文件）
3. 将上下文 + 用户 prompt 通过 SSE streaming 发给 LLM
4. LLM 返回 tool calls（读文件、写文件、执行命令、搜索等）
5. axe 执行 tool calls（文件修改需用户确认），将结果返回 LLM
6. 循环直到任务完成
7. 自动保存对话历史，自动 git commit

### Tool 定义
- `read_file(path)` — 读取文件内容
- `write_file(path, content)` — 创建或覆盖文件（已有文件显示 diff 预览，需确认）
- `edit_file(path, old_text, new_text)` — 精确替换文件内容（显示变更对比，需确认）
- `list_directory(path)` — 列出目录结构
- `execute_command(command)` — 执行 shell 命令（需用户确认）
- `search_files(pattern, path)` — grep 搜索文件内容

### 配置文件 ~/.axe/config.yaml
```yaml
# 至少配置一个模型，支持多个模型自动 fallback
models:
  - provider: anthropic          # anthropic 或 openai
    api_key: "sk-xxx"
    base_url: "https://api.anthropic.com"
    model: "claude-sonnet-4-20250514"
    max_tokens: 8192
  - provider: openai             # 备用模型
    api_key: "sk-xxx"
    base_url: "https://api.openai.com"
    model: "gpt-4o"
    max_tokens: 8192
```

### CLI 命令
- `axe` — 进入交互式对话
- `axe "prompt"` — 单次执行模式
- `axe --resume` — 恢复最近一次对话
- `axe --list` — 列出最近 10 次对话
- `axe init` — 初始化配置文件
- `axe version` — 版本信息

### 交互命令
- `/clear` — 清空对话上下文和 token 累计
- `/model` — 查看当前和可用模型
- `/model <name>` — 运行时切换模型
- `/cost` — 查看累计 token 用量
- `/help` — 显示命令列表

## 项目结构
```
axe/
├── main.go              # 入口
├── cmd/                  # CLI 命令定义
│   └── root.go
├── internal/
│   ├── agent/            # Agent 核心循环（prompt → tool call → execute → loop）
│   │   └── agent.go
│   ├── llm/              # LLM API 客户端
│   │   ├── client.go     # Provider 接口 + Anthropic 实现
│   │   ├── openai.go     # OpenAI 兼容实现
│   │   └── types.go      # 消息类型 + SSE 事件类型
│   ├── tools/            # Tool 实现
│   │   ├── registry.go
│   │   ├── read_file.go
│   │   ├── write_file.go # 含 diff 预览确认
│   │   ├── edit_file.go  # 含变更对比确认
│   │   ├── list_dir.go
│   │   ├── exec_cmd.go
│   │   └── search.go
│   ├── context/          # 项目上下文收集（CLAUDE.md、.axeignore、智能检测）
│   │   └── collector.go
│   ├── config/           # 配置管理
│   │   └── config.go
│   ├── history/          # 对话历史持久化
│   │   └── history.go
│   ├── git/              # 自动 git commit
│   │   └── git.go
│   └── ui/               # 终端 UI（readline + streaming 输出）
│       └── chat.go
├── go.mod
├── go.sum
└── README.md
```

## 代码规范
- 简洁直接，不过度抽象
- 错误处理用 fmt.Errorf + %w
- 不写不必要的注释
- 不写测试（除非明确要求）
- 先跑通再优化

## 安全
- execute_command 默认需要用户确认（y/n）
- write_file 覆盖已有文件需确认（显示行数变化）
- edit_file 替换内容需确认（显示变更对比）
- 不自动执行 rm、sudo 等危险命令
- API key 不硬编码，从配置文件或环境变量读取
