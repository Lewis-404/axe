# Axe - Vibe Coding Agent

## 项目概述
Go 写的 CLI vibe coding agent。用户在终端用自然语言描述需求，axe 自动读取项目上下文、调用 LLM 生成代码、创建/修改文件、执行命令。

## 技术栈
- Go 1.25
- Module: github.com/Lewis-404/axe
- LLM: Anthropic Claude API（兼容中转，支持自定义 base URL）
- TUI: github.com/charmbracelet/bubbletea + lipgloss + glamour
- 配置: ~/.axe/config.yaml

## MVP 功能（第一版）

### 核心流程
1. 用户启动 `axe` 进入交互模式，或 `axe "prompt"` 单次执行
2. axe 读取当前目录的项目上下文（文件树、关键文件）
3. 将上下文 + 用户 prompt 发给 Claude API
4. Claude 返回 tool calls（读文件、写文件、执行命令、搜索等）
5. axe 执行 tool calls，将结果返回 Claude
6. 循环直到任务完成

### Tool 定义（Claude function calling）
- `read_file(path)` — 读取文件内容
- `write_file(path, content)` — 创建或覆盖文件
- `edit_file(path, old_text, new_text)` — 精确替换文件内容
- `list_directory(path)` — 列出目录结构
- `execute_command(command)` — 执行 shell 命令（需用户确认）
- `search_files(pattern, path)` — grep 搜索文件内容

### 配置文件 ~/.axe/config.yaml
```yaml
api_key: "sk-xxx"
base_url: "https://api.anthropic.com"  # 支持中转
model: "claude-sonnet-4-20250514"
max_tokens: 8192
```

### CLI 命令
- `axe` — 进入交互式对话
- `axe "prompt"` — 单次执行模式
- `axe init` — 初始化配置文件
- `axe version` — 版本信息

## 项目结构
```
axe/
├── main.go              # 入口
├── cmd/                  # CLI 命令定义
│   └── root.go
├── internal/
│   ├── agent/            # Agent 核心循环（prompt → tool call → execute → loop）
│   │   └── agent.go
│   ├── llm/              # Claude API 客户端
│   │   ├── client.go
│   │   ├── types.go
│   │   └── tools.go
│   ├── tools/            # Tool 实现
│   │   ├── registry.go
│   │   ├── read_file.go
│   │   ├── write_file.go
│   │   ├── edit_file.go
│   │   ├── list_dir.go
│   │   ├── exec_cmd.go
│   │   └── search.go
│   ├── context/          # 项目上下文收集
│   │   └── collector.go
│   ├── config/           # 配置管理
│   │   └── config.go
│   └── ui/               # TUI 界面
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
- 不自动执行 rm、sudo 等危险命令
- API key 不硬编码，从配置文件或环境变量读取
