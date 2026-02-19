# 🪓 Axe — Vibe Coding Agent

Go 写的 CLI vibe coding agent。用自然语言描述需求，axe 自动读取项目上下文、调用 LLM 生成代码、创建/修改文件、执行命令。

## 特性

- 🌊 **流式输出** — SSE streaming 逐字打印，实时看到 AI 思考过程
- 💾 **对话历史** — 自动保存，支持恢复上次对话
- 📊 **Token 用量** — 每轮显示本轮和累计 token 消耗
- 🤖 **多模型支持** — Anthropic Claude + OpenAI 兼容接口
- 📝 **项目感知** — 自动读取 CLAUDE.md 项目指令、.axeignore 忽略规则、智能检测项目类型
- ✏️ **diff 预览** — 文件修改前显示变更对比，需确认才执行
- 📦 **自动 commit** — 每轮完成后自动 git commit，方便回滚
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
provider: "anthropic"  # 或 "openai"
api_key: "your-api-key"
base_url: "https://api.anthropic.com"  # 支持中转
model: "claude-sonnet-4-20250514"
max_tokens: 8192
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
| `/clear` | 清空对话上下文 |
| `/model` | 查看当前模型 |
| `/model <name>` | 切换模型 |
| `/cost` | 查看累计 token 用量 |
| `/help` | 显示命令列表 |

## 工具

axe 内置 6 个工具供 LLM 调用：

| 工具 | 功能 |
|------|------|
| `read_file` | 读取文件内容 |
| `write_file` | 创建/覆盖文件（已有文件需确认） |
| `edit_file` | 精确替换文件内容（需确认） |
| `list_directory` | 列出目录结构 |
| `execute_command` | 执行 shell 命令（需确认） |
| `search_files` | grep 搜索文件 |

## 项目感知

axe 会自动读取项目根目录的以下文件：

- **CLAUDE.md / .axe.md** — 项目指令，作为额外 system prompt
- **.axeignore** — 忽略规则，格式同 .gitignore
- **.gitignore** — 自动跳过匹配文件

智能检测项目类型（Go/Python/Node/Rust），自动读取对应关键文件作为上下文。

## License

MIT
