# 🪓 Axe — Vibe Coding Agent

Go 写的 CLI vibe coding agent。用自然语言描述需求，axe 自动读取项目上下文、调用 Claude 生成代码、创建/修改文件、执行命令。

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
api_key: "your-anthropic-api-key"
base_url: "https://api.anthropic.com"  # 支持中转
model: "claude-sonnet-4-20250514"
max_tokens: 8192
```

也支持环境变量：

```bash
export ANTHROPIC_API_KEY="sk-xxx"
export ANTHROPIC_BASE_URL="https://your-proxy.com"
```

## 使用

```bash
# 交互模式
axe

# 单次执行
axe "帮我写一个 HTTP server"

# 查看版本
axe version
```

## 工具

axe 内置 6 个工具供 Claude 调用：

| 工具 | 功能 |
|------|------|
| `read_file` | 读取文件内容 |
| `write_file` | 创建/覆盖文件 |
| `edit_file` | 精确替换文件内容 |
| `list_directory` | 列出目录结构 |
| `execute_command` | 执行 shell 命令（需确认） |
| `search_files` | grep 搜索文件 |

## License

MIT
