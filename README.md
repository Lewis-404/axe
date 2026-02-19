# 🪓 Axe — Vibe Coding Agent

Go 写的 CLI vibe coding agent。用自然语言描述需求，axe 自动读取项目上下文、调用 LLM 生成代码、创建/修改文件、执行命令。

## 特性

- 🌊 **流式输出** — SSE streaming 逐字打印，实时看到 AI 思考过程
- 💾 **对话历史** — 自动保存（按项目维度），支持恢复上次对话
- 📊 **Token 用量 + 费用** — 每轮显示 token 消耗和美元费用估算
- 🤖 **多模型支持** — Anthropic Claude + OpenAI 兼容接口，自动 fallback
- 📝 **项目感知** — 自动读取 CLAUDE.md 项目指令、.axeignore 忽略规则、智能检测项目类型
- ✏️ **diff 预览** — 文件修改前显示变更对比，需确认才执行
- 📦 **自动 commit** — 每轮完成后自动 git commit，方便回滚
- 🔐 **权限记忆** — 支持 "Always allow" 记住授权决策，避免重复确认
- 🗜️ **上下文压缩** — 长对话自动压缩，防止超出 token 限制
- 🎨 **Markdown 渲染** — 终端中渲染代码高亮、表格、列表等
- ⌨️ **中文友好** — 完整的 CJK 输入支持

## 安装

```bash
go install github.com/Lewis-404/axe@latest
```

或本地编译：

```bash
git clone https://github.com/Lewis-404/axe.git
cd axe
go build -o axe .
```

## 配置

```bash
axe init
```

编辑 `~/.axe/config.yaml`：

```yaml
# 至少配置一个模型，支持多个模型自动 fallback
models:
  - provider: anthropic          # anthropic 或 openai
    api_key: "your-api-key"
    base_url: "https://api.anthropic.com"
    model: "claude-sonnet-4-20250514"
    max_tokens: 8192

  # 备用模型（可选，第一个失败时自动切换）
  # - provider: openai
  #   api_key: "sk-xxx"
  #   base_url: "https://api.openai.com"
  #   model: "gpt-4o"
  #   max_tokens: 8192
```

也支持环境变量：

```bash
# Anthropic
export ANTHROPIC_API_KEY="sk-xxx"
export ANTHROPIC_BASE_URL="https://your-proxy.com"

# OpenAI
export OPENAI_API_KEY="sk-xxx"
export OPENAI_BASE_URL="https://api.openai.com"
```

## 使用

```bash
# 交互模式
axe

# 单次执行
axe "帮我写一个 HTTP server"

# 恢复上次对话
axe --resume

# 列出最近对话
axe --list

# 初始化配置
axe init

# 查看版本
axe version
```

### 交互命令

在交互模式中支持 `/` 命令：

| 命令 | 功能 |
|------|------|
| `/clear` | 清空对话上下文并清屏 |
| `/compact [hint]` | 压缩对话上下文（可带提示指导方向） |
| `/init` | 为当前项目生成 CLAUDE.md |
| `/list` | 查看最近对话记录（带编号） |
| `/resume` | 列出最近对话，选择并恢复（展示完整历史） |
| `/resume <编号>` | 恢复指定对话（编号从 `/list` 获取） |
| `/model` | 查看当前和可用模型 |
| `/model <name>` | 切换模型 |
| `/cost` | 查看累计 token 用量和费用 |
| `/help` | 显示命令列表 |

## 工具

axe 内置 9 个工具供 LLM 调用：

| 工具 | 功能 |
|------|------|
| `read_file` | 读取文件内容 |
| `write_file` | 创建/覆盖文件（已有文件需确认） |
| `edit_file` | 精确替换文件内容（需确认） |
| `list_directory` | 列出目录结构 |
| `execute_command` | 执行 shell 命令（需确认） |
| `search_files` | grep 搜索文件内容 |
| `glob` | 按文件名模式搜索（如 `**/*.go`） |
| `bg_command` | 后台进程管理（启动/状态/停止/日志） |
| `think` | 内部思考，用于任务规划 |

### 权限记忆

工具确认时输入 `A` (Always) 可记住授权决策：

- 命令执行：按命令前缀记忆（如允许所有 `go` 命令）
- 文件写入/编辑：可设为始终允许

权限存储在 `~/.axe/permissions.yaml`，可手动编辑。

## 项目感知

axe 会自动读取项目根目录的以下文件：

- **CLAUDE.md / .axe.md** — 项目指令，作为额外 system prompt
- **.axeignore** — 忽略规则，格式同 .gitignore
- **.gitignore** — 自动跳过匹配文件

智能检测项目类型（Go/Python/Node/Rust），自动读取对应关键文件作为上下文。

### 对话历史

对话按项目维度存储在 `~/.axe/history/<project>/` 下，不同项目的历史互不干扰。

## License

MIT
