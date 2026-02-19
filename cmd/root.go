package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Lewis-404/axe/internal/agent"
	"github.com/Lewis-404/axe/internal/commands"
	"github.com/Lewis-404/axe/internal/config"
	"github.com/Lewis-404/axe/internal/context"
	"github.com/Lewis-404/axe/internal/git"
	"github.com/Lewis-404/axe/internal/history"
	"github.com/Lewis-404/axe/internal/llm"
	"github.com/Lewis-404/axe/internal/mcp"
	"github.com/Lewis-404/axe/internal/permissions"
	"github.com/Lewis-404/axe/internal/pricing"
	"github.com/Lewis-404/axe/internal/tools"
	"github.com/Lewis-404/axe/internal/ui"
)

const systemPrompt = `You are Axe, a vibe coding agent. You help users build software by reading, writing, and editing code files, executing commands, and searching codebases.

Rules:
- For complex tasks (multi-file changes, refactoring, new features), use the think tool FIRST to plan your approach step by step
- Be concise and direct
- Write clean, idiomatic code
- When modifying files, use edit_file for surgical changes, write_file for new files
- If a tool call fails, read the error carefully, fix the issue, and retry (max 3 retries per step)
- After modifying code files, check compilation results in the tool output — fix any errors before moving on
- Explain what you're doing briefly before doing it

Project context:
%s`

func Run(args []string) {
	if len(args) > 0 && args[0] == "init" {
		if err := config.Init(); err != nil {
			ui.PrintError(err)
			os.Exit(1)
		}
		fmt.Println("✅ Config created at ~/.axe/config.yaml")
		fmt.Println("   Edit it to add your API key.")
		return
	}

	if len(args) > 0 && args[0] == "version" {
		fmt.Println("axe v0.6.0")
		return
	}

	// --list: show recent conversations
	if len(args) > 0 && args[0] == "--list" {
		lines, err := history.ListRecentIndexed(10)
		if err != nil {
			ui.PrintError(err)
			os.Exit(1)
		}
		fmt.Println("Recent conversations:")
		for _, l := range lines {
			fmt.Println(l)
		}
		return
	}

	// --print: non-interactive mode (output only text, auto-allow all tools)
	printMode := false
	for i, a := range args {
		if a == "--print" || a == "-p" {
			printMode = true
			args = append(args[:i], args[i+1:]...)
			break
		}
	}

	// stdin pipe: read prompt from stdin if not a terminal
	if !printMode {
		if stat, _ := os.Stdin.Stat(); stat.Mode()&os.ModeCharDevice == 0 {
			data, _ := io.ReadAll(bufio.NewReader(os.Stdin))
			if len(data) > 0 {
				args = append(args, strings.TrimSpace(string(data)))
				printMode = true
			}
		}
	}

	cfg, err := config.Load()
	if err != nil {
		ui.PrintError(err)
		os.Exit(1)
	}

	dir, _ := os.Getwd()
	history.SetProjectDir(dir)

	// merge project-level config
	if pc := config.LoadProjectConfig(dir); pc != nil {
		cfg.Merge(pc)
	}

	ctx := context.Collect(dir)
	sys := fmt.Sprintf(systemPrompt, ctx)

	perms := permissions.Load()

	var registryOpts tools.RegistryOpts
	if printMode {
		// auto-allow everything in print mode
		registryOpts = tools.RegistryOpts{
			Confirm:          func(string) bool { return true },
			ConfirmOverwrite: func(string, int, int) bool { return true },
			ConfirmEdit:      func(string, string, string) bool { return true },
		}
	} else {
		registryOpts = tools.RegistryOpts{
		Confirm: func(cmd string) bool {
			if allowed, found := perms.Check("execute_command", cmd); found {
				if allowed {
					fmt.Printf("\n⚡ Execute: %s \033[90m(auto-allowed)\033[0m\n", cmd)
				}
				return allowed
			}
			fmt.Printf("\n⚡ Execute: %s\n", cmd)
			answer := ui.ReadLine("Allow? [y/N/A(lways)] ")
			switch strings.ToLower(answer) {
			case "a", "always":
				// extract command prefix (first word)
				prefix := strings.Fields(cmd)[0]
				perms.AddAllow("execute_command", prefix)
				fmt.Printf("  ✅ 已记住: 始终允许 %s 命令\n", prefix)
				return true
			case "y":
				return true
			default:
				return false
			}
		},
		ConfirmOverwrite: func(path string, oldLines, newLines int) bool {
			if allowed, found := perms.Check("write_file", path); found {
				if allowed {
					fmt.Printf("\n📝 覆盖 %s (原 %d 行 → 新 %d 行) \033[90m(auto-allowed)\033[0m\n", path, oldLines, newLines)
				}
				return allowed
			}
			fmt.Printf("\n📝 覆盖 %s (原 %d 行 → 新 %d 行)\n", path, oldLines, newLines)
			answer := ui.ReadLine("Allow? [y/N/A(lways)] ")
			switch strings.ToLower(answer) {
			case "a", "always":
				perms.AddAllow("write_file", "*")
				fmt.Println("  ✅ 已记住: 始终允许文件写入")
				return true
			case "y":
				return true
			default:
				return false
			}
		},
		ConfirmEdit: func(path, oldText, newText string) bool {
			if allowed, found := perms.Check("edit_file", path); found {
				if allowed {
					fmt.Printf("\n✏️ 编辑 %s \033[90m(auto-allowed)\033[0m\n", path)
				}
				return allowed
			}
			fmt.Printf("\n✏️ 编辑 %s:\n", path)
			ui.PrintDiff(path, oldText, newText)
			answer := ui.ReadLine("Allow? [y/N/A(lways)] ")
			switch strings.ToLower(answer) {
			case "a", "always":
				perms.AddAllow("edit_file", "*")
				fmt.Println("  ✅ 已记住: 始终允许文件编辑")
				return true
			case "y":
				return true
			default:
				return false
			}
		},
		}
	}
	registry := tools.NewRegistry(registryOpts)

	// start MCP servers and register their tools
	var mcpClients []*mcp.Client
	for name, srv := range cfg.MCPServers {
		mc, err := mcp.NewClient(srv.Command, srv.Args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ MCP server %q failed: %s\n", name, err)
			continue
		}
		mcpClients = append(mcpClients, mc)
		for _, t := range mc.Tools() {
			t := t
			registry.Register(&t)
		}
	}
	defer func() {
		for _, mc := range mcpClients {
			mc.Close()
		}
	}()

	// Auto-verify: run build check after file modifications
	registry.SetPostExecHook(func(name string, input json.RawMessage, result string) string {
		if name != "write_file" && name != "edit_file" {
			return ""
		}
		var params struct{ Path string }
		if json.Unmarshal(input, &params) != nil || params.Path == "" {
			return ""
		}
		if filepath.Ext(params.Path) != ".go" {
			return ""
		}
		// Find the module root (directory containing go.mod)
		buildDir := filepath.Dir(params.Path)
		for d := buildDir; ; d = filepath.Dir(d) {
			if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
				buildDir = d
				break
			}
			if d == filepath.Dir(d) {
				break
			}
		}
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = buildDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("[Auto-verify] go build FAILED:\n%s", string(out))
		}
		return "[Auto-verify] go build OK"
	})
	client := llm.NewClient(cfg.Models, registry.Definitions())
	ag := agent.New(client, registry, sys)

	if printMode {
		// minimal output: only final text
		var output strings.Builder
		ag.OnTextDelta(func(s string) { output.WriteString(s) })
		ag.OnBlockDone(func() {
			fmt.Print(output.String())
			output.Reset()
		})
	} else {
		ag.OnTextDelta(ui.PrintTextDelta)
		ag.OnBlockDone(ui.PrintBlockDone)
		ag.OnTool(ui.PrintTool)
		ag.OnUsage(func(roundIn, roundOut, totalIn, totalOut int) {
			model := client.ModelName()
			roundCost := pricing.Cost(model, roundIn, roundOut)
			totalCost := pricing.Cost(model, totalIn, totalOut)
			if totalCost > 0 {
				fmt.Printf("📊 本轮: ↑%s ↓%s ($%.4f) | 累计: ↑%s ↓%s ($%.4f)\n",
					fmtTokens(roundIn), fmtTokens(roundOut), roundCost,
					fmtTokens(totalIn), fmtTokens(totalOut), totalCost)
			} else {
				ui.PrintUsage(roundIn, roundOut, totalIn, totalOut)
			}
		})
		ag.OnCompact(func(before, after int) {
			fmt.Printf("🗜️ 上下文已压缩: ~%dk → ~%dk tokens\n", before/1000, after/1000)
		})
	}

	// --resume: restore latest conversation
	var savePath string
	resume := len(args) > 0 && args[0] == "--resume"
	if resume {
		p, msgs, err := history.LoadLatest()
		if err != nil {
			ui.PrintError(err)
			os.Exit(1)
		}
		ag.SetMessages(msgs)
		savePath = p
		args = args[1:]
		fmt.Println("📂 Resumed previous conversation")
		ui.PrintHistory(msgs)
	} else {
		savePath = history.NewFilePath()
	}

	autoSave := func() {
		if msgs := ag.Messages(); len(msgs) > 0 {
			if err := history.SaveTo(savePath, msgs); err != nil {
				ui.PrintError(fmt.Errorf("save history: %w", err))
			}
		}
	}

	autoCommit := func(input string) {
		if git.IsRepo(dir) && git.HasChanges(dir) {
			if hash, err := git.AutoCommit(dir, input); err == nil {
				fmt.Printf("\n📦 Auto-commit: %s\n", hash)
			}
		}
	}

	// single-shot mode
	if len(args) > 0 {
		prompt := strings.Join(args, " ")
		if err := ag.Run(prompt); err != nil {
			ui.PrintError(err)
			os.Exit(1)
		}
		autoCommit(prompt)
		autoSave()
		return
	}

	// load custom project commands
	customCmds := commands.LoadProjectCommands(dir)
	pkgCustomCmds = customCmds

	// interactive mode
	fmt.Println("🪓 Axe v0.6.0 — vibe coding agent")
	fmt.Println("    Type your request. /help for commands.")
	fmt.Println()

	for {
		input := ui.ReadLine("\033[36m❯\033[0m ")
		if input == "" {
			continue
		}
		if strings.HasPrefix(input, "/") {
			if input == "/exit" || input == "/quit" {
				autoSave()
				fmt.Println("👋")
				return
			}
			// handle /project:xxx custom commands
			if strings.HasPrefix(input, "/project:") {
				cmdName := strings.TrimPrefix(strings.Fields(input)[0], "/project:")
				found := false
				for _, c := range customCmds {
					if c.Name == cmdName {
						found = true
						fmt.Printf("🔧 执行项目命令: %s\n", cmdName)
						if err := ag.Run(c.Content); err != nil {
							ui.PrintError(err)
						}
						autoCommit(c.Content)
						autoSave()
						break
					}
				}
				if !found {
					fmt.Printf("❌ 未找到项目命令: %s\n", cmdName)
				}
				continue
			}
			handleSlashCommand(input, ag, client, &savePath)
			continue
		}
		if err := ag.Run(input); err != nil {
			ui.PrintError(err)
		}
		autoCommit(input)
		autoSave()
		fmt.Println()
	}
}

var pkgCustomCmds []commands.CustomCommand

func handleSlashCommand(input string, ag *agent.Agent, client *llm.Client, savePath *string) {
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/clear":
		ag.Reset()
		ui.ClearScreen()
		fmt.Println("🧹 上下文已清空，开始新对话")
	case "/model":
		if len(parts) > 1 {
			if client.SwitchModel(parts[1]) {
				fmt.Printf("✅ 模型已切换为: %s\n", parts[1])
			} else {
				fmt.Printf("❌ 未找到模型: %s\n", parts[1])
				fmt.Printf("   可用模型: %s\n", strings.Join(client.ListModels(), ", "))
			}
		} else {
			fmt.Printf("当前模型: %s\n", client.ModelName())
			fmt.Printf("可用模型: %s\n", strings.Join(client.ListModels(), ", "))
		}
	case "/list":
		lines, err := history.ListRecentIndexed(10)
		if err != nil {
			ui.PrintError(err)
			return
		}
		fmt.Println("最近对话:")
		for _, l := range lines {
			fmt.Println(l)
		}
		fmt.Println("  输入 /resume <编号> 恢复对话")
	case "/resume":
		if len(parts) > 1 {
			idx, err := strconv.Atoi(parts[1])
			if err != nil {
				fmt.Println("❌ 请输入数字编号，如: /resume 3")
				return
			}
			p, msgs, err := history.LoadByIndex(idx)
			if err != nil {
				ui.PrintError(err)
				return
			}
			ag.SetMessages(msgs)
			*savePath = p
			fmt.Printf("📂 已恢复对话 [%d]（%d 条消息）\n", idx, len(msgs))
			ui.PrintHistory(msgs)
		} else {
			lines, err := history.ListRecentIndexed(10)
			if err != nil {
				ui.PrintError(err)
				return
			}
			if len(lines) == 0 {
				fmt.Println("📭 没有历史对话")
				return
			}
			fmt.Println("最近对话:")
			for _, l := range lines {
				fmt.Println(l)
			}
			answer := ui.ReadLine("输入编号恢复 (0 取消): ")
			if answer == "" || answer == "0" {
				return
			}
			idx, err := strconv.Atoi(answer)
			if err != nil {
				fmt.Println("❌ 请输入数字编号")
				return
			}
			p, msgs, err := history.LoadByIndex(idx)
			if err != nil {
				ui.PrintError(err)
				return
			}
			ag.SetMessages(msgs)
			*savePath = p
			fmt.Printf("📂 已恢复对话 [%d]（%d 条消息）\n", idx, len(msgs))
			ui.PrintHistory(msgs)
		}
	case "/compact":
		hint := ""
		if len(parts) > 1 {
			hint = strings.Join(parts[1:], " ")
		}
		if err := ag.Compact(hint); err != nil {
			ui.PrintError(err)
		} else {
			fmt.Println("🗜️ 对话上下文已压缩")
		}
	case "/cost":
		in, out := ag.TotalUsage()
		cost := pricing.Cost(client.ModelName(), in, out)
		if cost > 0 {
			fmt.Printf("📊 累计: ↑%s ↓%s | 💰 $%.4f\n", fmtTokens(in), fmtTokens(out), cost)
		} else {
			ui.PrintTotalUsage(in, out)
		}
	case "/fork":
		newPath := history.NewFilePath()
		if msgs := ag.Messages(); len(msgs) > 0 {
			if err := history.SaveTo(newPath, msgs); err != nil {
				ui.PrintError(err)
			} else {
				*savePath = newPath
				fmt.Printf("🔀 对话已分支，新路径: %s\n", filepath.Base(newPath))
			}
		} else {
			fmt.Println("⚠️ 当前没有对话内容")
		}
	case "/undo":
		dir, _ := os.Getwd()
		if !git.IsRepo(dir) {
			fmt.Println("⚠️ 当前目录不是 git 仓库")
		} else if !git.HasCommits(dir) {
			fmt.Println("⚠️ 没有可撤销的 commit")
		} else {
			out, err := git.Undo(dir)
			if err != nil {
				ui.PrintError(err)
			} else {
				fmt.Printf("⏪ 已撤销: %s\n", out)
			}
		}
	case "/search":
		if len(parts) < 2 {
			fmt.Println("用法: /search <关键词>")
		} else {
			keyword := strings.Join(parts[1:], " ")
			results, err := history.Search(keyword, 10)
			if err != nil {
				ui.PrintError(err)
			} else if len(results) == 0 {
				fmt.Printf("🔍 未找到包含 \"%s\" 的对话\n", keyword)
			} else {
				fmt.Printf("🔍 搜索 \"%s\" 结果:\n", keyword)
				for _, r := range results {
					fmt.Println(r)
				}
			}
		}
	case "/ask":
		if len(parts) < 3 {
			fmt.Println("用法: /ask <model> <prompt>")
		} else {
			modelName := parts[1]
			prompt := strings.Join(parts[2:], " ")
			origModel := client.ModelName()
			if !client.SwitchModel(modelName) {
				fmt.Printf("❌ 未找到模型: %s\n", modelName)
			} else {
				fmt.Printf("🔄 临时使用 %s\n", modelName)
				if err := ag.Run(prompt); err != nil {
					ui.PrintError(err)
				}
				client.SwitchModel(origModel)
			}
		}
	case "/budget":
		if len(parts) < 2 {
			fmt.Println("用法: /budget <美元金额>  (如 /budget 0.5)")
			fmt.Println("      /budget off  关闭预算限制")
		} else if parts[1] == "off" {
			ag.SetBudget(0, nil)
			fmt.Println("💰 预算限制已关闭")
		} else {
			val, err := strconv.ParseFloat(parts[1], 64)
			if err != nil || val <= 0 {
				fmt.Println("❌ 请输入有效金额")
			} else {
				model := client.ModelName()
				ag.SetBudget(val, func(in, out int) float64 {
					return pricing.Cost(model, in, out)
				})
				fmt.Printf("💰 预算已设为 $%.2f\n", val)
			}
		}
	case "/init":
		dir, _ := os.Getwd()
		target := filepath.Join(dir, "CLAUDE.md")
		if _, err := os.Stat(target); err == nil {
			fmt.Println("⚠️  CLAUDE.md 已存在，跳过生成")
		} else {
			content := context.GenerateCLAUDEMD(dir)
			if err := os.WriteFile(target, []byte(content), 0644); err != nil {
				ui.PrintError(err)
			} else {
				fmt.Println("✅ 已生成 CLAUDE.md，请根据项目实际情况编辑完善")
			}
		}
	case "/help":
		fmt.Println("可用命令:")
		fmt.Println("  /clear          清空对话上下文")
		fmt.Println("  /compact [hint]  压缩对话上下文")
		fmt.Println("  /fork           从当前对话创建分支")
		fmt.Println("  /init           为当前项目生成 CLAUDE.md")
		fmt.Println("  /list           查看最近对话记录")
		fmt.Println("  /resume         选择并恢复对话")
		fmt.Println("  /model          显示当前和可用模型")
		fmt.Println("  /model <name>   切换模型")
		fmt.Println("  /ask <m> <p>    临时用另一个模型回答")
		fmt.Println("  /search <kw>    搜索历史对话")
		fmt.Println("  /undo           撤销上一次 git commit")
		fmt.Println("  /budget <$>     设置费用上限 (off 关闭)")
		fmt.Println("  /cost           显示累计 token 用量和费用")
		fmt.Println("  /exit           退出 Axe")
		fmt.Println("  /help           显示此帮助")
		fmt.Println("  💡 支持图片: 在 prompt 中直接写图片路径")
		if h := commands.FormatHelp(pkgCustomCmds); h != "" {
			fmt.Print(h)
		}
	default:
		fmt.Printf("未知命令: %s（输入 /help 查看可用命令）\n", cmd)
	}
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func fmtTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
