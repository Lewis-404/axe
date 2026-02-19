# Axe - Vibe Coding Agent

## 项目概述
Go 写的 CLI vibe coding agent。用户在终端用自然语言描述需求，axe 自动读取项目上下文、调用 LLM 生成代码、创建/修改文件、执行命令。

## 技术栈
- Go 1.25
- Module: github.com/Lewis-404/axe
- LLM: Anthropic Claude API + OpenAI 兼容接口（支持中转、自定义 base URL）
- 终端输入: github.com/nyaosorg/go-readline-ny（支持 CJK 多字节字符）
- 配置: ~/.axe/config.yaml

## 功能

### 核心流程
1. 用户启动 `axe` 进入交互模式，或 `axe "prompt"` 单次执行
2. axe 读取当前目录的项目上下文（CLAUDE.md、文件树、关键文件）
3. 将上下文 + 用户 prompt 通过 SSE streaming 发给 LLM
4. LLM 返回 tool calls（读文件、写文件、执行命令、搜索等）
5. axe 执行 tool calls（文件修改需用户确认），将结果返回 LLM
6. 循环直到任务完成
7. 自动保存对话历史（按项目维度），自动 git commit

### Tool 定义
- `read_file(path)` — 读取文件内容
- `write_file(path, content)` — 创建或覆盖文件（已有文件显示 diff 预览，需确认）
- `edit_file(path, old_text, new_text)` — 精确替换文件内容（显示变更对比，需确认）
- `list_directory(path)` — 列出目录结构
- `execute_command(command)` — 执行 shell 命令（需用户确认）
- `search_files(pattern, path)` — grep 搜索文件内容
- `glob(pattern, path)` — 按文件名模式搜索（如 **/*.go）
- `bg_command(action, command, id)` — 后台进程管理（start/status/stop/logs）
- `think(thought)` — 内部思考工具，用于任务规划

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
- `axe --print "prompt"` — 非交互模式（只输出文本，自动允许所有操作）
- `echo "prompt" | axe` — 管道模式（等同 --print）
- `axe --resume` — 恢复最近一次对话
- `axe --list` — 列出最近 10 次对话
- `axe init` — 初始化配置文件
- `axe version` — 版本信息

### 交互命令
- `/clear` — 清空对话上下文并清屏
- `/compact [hint]` — 压缩对话上下文（可带提示指导压缩方向）
- `/init` — 为当前项目自动生成 CLAUDE.md
- `/list` — 查看最近对话记录（带编号）
- `/resume` — 列出最近对话，选择并恢复（展示完整历史）
- `/resume <编号>` — 恢复指定对话（编号从 `/list` 获取）
- `/model` — 查看当前和可用模型
- `/model <name>` — 运行时切换模型
- `/cost` — 查看累计 token 用量和费用
- `/project:<name>` — 执行自定义项目命令（从 .axe/commands/*.md 加载）
- `/help` — 显示命令列表

### 配置层级
1. 全局配置：`~/.axe/config.yaml`
2. 项目配置：`.axe/settings.yaml`（覆盖全局配置中的模型等设置）
3. 自定义命令：`.axe/commands/*.md`（文件名即命令名，内容作为 prompt）

## 项目结构
```
axe/
├── main.go              # 入口
├── cmd/                  # CLI 命令定义
│   └── root.go
├── internal/
│   ├── agent/            # Agent 核心循环 + 上下文自动压缩
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
│   │   ├── bg_command.go # 后台进程管理
│   │   ├── search.go
│   │   ├── glob.go       # 文件名模式搜索
│   │   └── think.go
│   ├── commands/         # 自定义项目命令
│   │   └── commands.go
│   ├── context/          # 项目上下文收集 + CLAUDE.md 生成
│   │   └── collector.go
│   ├── config/           # 配置管理
│   │   └── config.go
│   ├── history/          # 对话历史持久化（按项目维度存储）
│   │   └── history.go
│   ├── permissions/      # 权限记忆系统
│   │   └── permissions.go
│   ├── pricing/          # 费用估算
│   │   └── pricing.go
│   ├── git/              # 自动 git commit
│   │   └── git.go
│   └── ui/               # 终端 UI（readline + streaming + Markdown 渲染）
│       ├── chat.go
│       └── render.go
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
- execute_command 默认需要用户确认（y/n/always）
- write_file 覆盖已有文件需确认（显示行数变化）
- edit_file 替换内容需确认（显示变更对比）
- 权限记忆存储在 ~/.axe/permissions.yaml，支持 always allow
- 不自动执行 rm、sudo 等危险命令
- API key 不硬编码，从配置文件或环境变量读取
