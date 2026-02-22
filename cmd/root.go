package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	"github.com/Lewis-404/axe/internal/skills"
	"github.com/Lewis-404/axe/internal/tools"
	"github.com/Lewis-404/axe/internal/ui"
)

var Version = "dev"

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

func setupRegistry(perms *permissions.Store, printMode, autoMode bool) *tools.Registry {
	var opts tools.RegistryOpts
	if printMode || autoMode {
		opts = tools.RegistryOpts{
			Confirm:          func(string) bool { return true },
			ConfirmOverwrite: func(string, int, int) bool { return true },
			ConfirmEdit:      func(string, string, string) bool { return true },
		}
	} else {
		opts = tools.RegistryOpts{
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

	registry := tools.NewRegistry(opts)

	if !printMode && !autoMode {
		registry.SetBatchConfirm(func(toolName string, items []tools.BatchConfirmItem) bool {
			if allowed, found := perms.Check(toolName, "*"); found && allowed {
				return true
			}
			emoji := map[string]string{"write_file": "📝", "edit_file": "✏️", "execute_command": "⚡", "bg_command": "⚡"}
			icon := emoji[toolName]
			if icon == "" {
				icon = "🔧"
			}
			fmt.Printf("\n%s 即将批量执行 %d 个 %s:\n", icon, len(items), toolName)
			for _, item := range items {
				var p struct {
					Path    string `json:"path"`
					Command string `json:"command"`
				}
				json.Unmarshal(item.Input, &p)
				label := p.Path
				if label == "" {
					label = p.Command
				}
				fmt.Printf("  - %s\n", label)
			}
			answer := ui.ReadLine("Allow all? [y/N/A(lways)] ")
			switch strings.ToLower(answer) {
			case "a", "always":
				perms.AddAllow(toolName, "*")
				fmt.Printf("  ✅ 已记住: 始终允许 %s\n", toolName)
				return true
			case "y":
				return true
			default:
				return false
			}
		})
	}

	return registry
}

func setupAutoVerify(registry *tools.Registry, cfg *config.Config) {
	if cfg.AutoVerify != nil && !*cfg.AutoVerify {
		return
	}
	registry.SetPostExecHook(func(name string, input json.RawMessage, result string) string {
		if name != "write_file" && name != "edit_file" {
			return ""
		}
		var params struct{ Path string }
		if json.Unmarshal(input, &params) != nil || params.Path == "" {
			return ""
		}
		ext := filepath.Ext(params.Path)
		fileDir := filepath.Dir(params.Path)

		switch ext {
		case ".go":
			buildDir := findProjectRoot(fileDir, "go.mod")
			cmd := exec.Command("go", "build", "./...")
			cmd.Dir = buildDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Sprintf("[Auto-verify] go build FAILED:\n%s", string(out))
			}
			return "[Auto-verify] go build OK"
		case ".py":
			cmd := exec.Command("python3", "-m", "py_compile", params.Path)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Sprintf("[Auto-verify] python compile FAILED:\n%s", string(out))
			}
			return "[Auto-verify] python syntax OK"
		case ".rs":
			buildDir := findProjectRoot(fileDir, "Cargo.toml")
			cmd := exec.Command("cargo", "check", "--quiet")
			cmd.Dir = buildDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Sprintf("[Auto-verify] cargo check FAILED:\n%s", string(out))
			}
			return "[Auto-verify] cargo check OK"
		case ".ts", ".tsx":
			buildDir := findProjectRoot(fileDir, "tsconfig.json")
			cmd := exec.Command("npx", "tsc", "--noEmit")
			cmd.Dir = buildDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Sprintf("[Auto-verify] tsc FAILED:\n%s", string(out))
			}
			return "[Auto-verify] tsc OK"
		}
		return ""
	})
}

// findProjectRoot walks up from dir looking for a marker file (e.g. go.mod).
func findProjectRoot(dir, marker string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, marker)); err == nil {
			return d
		}
		if d == filepath.Dir(d) {
			break
		}
	}
	return dir
}

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
		fmt.Printf("axe %s\n", Version)
		return
	}

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

	printMode := false
	autoMode := false
	for i := len(args) - 1; i >= 0; i-- {
		switch args[i] {
		case "--print", "-p":
			printMode = true
			args = append(args[:i], args[i+1:]...)
		case "--auto":
			autoMode = true
			args = append(args[:i], args[i+1:]...)
		}
	}

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

	if pc := config.LoadProjectConfig(dir); pc != nil {
		cfg.Merge(pc)
	}

	ctx := context.Collect(dir)
	sys := fmt.Sprintf(systemPrompt, ctx)

	perms := permissions.Load()
	registry := setupRegistry(perms, printMode, autoMode)

	// start MCP servers
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

	// load skills
	home, _ := os.UserHomeDir()
	loadedSkills := skills.LoadSkills(filepath.Join(home, ".axe", "skills"), filepath.Join(dir, ".axe", "skills"))
	pkgSkills = loadedSkills
	var skillNames, skillDescs []string
	for _, s := range loadedSkills {
		skillNames = append(skillNames, s.Name)
		skillDescs = append(skillDescs, s.Description)
	}
	ui.RegisterSkillCommands(skillNames, skillDescs)
	if catalog := skills.SkillCatalog(loadedSkills); catalog != "" {
		sys += "\n\n" + catalog
	}

	setupAutoVerify(registry, cfg)

	client := llm.NewClient(cfg.Models, registry.Definitions())
	ag := agent.New(client, registry, sys)

	if printMode {
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
					ui.FmtTokens(roundIn), ui.FmtTokens(roundOut), roundCost,
					ui.FmtTokens(totalIn), ui.FmtTokens(totalOut), totalCost)
			} else {
				ui.PrintUsage(roundIn, roundOut, totalIn, totalOut)
			}
		})
		ag.OnCompact(func(before, after int) {
			fmt.Printf("🗜️ 上下文已压缩: ~%dk → ~%dk tokens\n", before/1000, after/1000)
		})
	}

	// --resume
	var savePath string
	resume := len(args) > 0 && args[0] == "--resume"
	if resume {
		p, msgs, err := history.LoadLatest()
		if err != nil {
			ui.PrintError(err)
			os.Exit(1)
		}
		resumeConversation(ag, p, msgs, &savePath, "已恢复对话并刷新项目上下文")
		args = args[1:]
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
	fmt.Printf("🪓 Axe %s — vibe coding agent\n", Version)
	fmt.Printf("   📁 %s | 🤖 %s | 🔧 %d tools | 📦 %d skills\n", filepath.Base(dir), client.ModelName(), len(registry.Definitions()), len(pkgSkills))
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
			cmdName := strings.TrimPrefix(strings.Fields(input)[0], "/")
			if s := skills.FindSkill(pkgSkills, cmdName); s != nil {
				content, err := skills.ReadSkillContent(*s)
				if err != nil {
					ui.PrintError(err)
				} else {
					ag.InjectContext(fmt.Sprintf("[Skill: %s]\n%s", s.Name, content))
					fmt.Printf("🧩 已激活技能: %s\n", s.Name)
					rest := strings.TrimSpace(strings.TrimPrefix(input, "/"+cmdName))
					if rest == "" {
						rest = strings.TrimSpace(strings.TrimPrefix(input, "/"+s.Name))
					}
					if rest != "" {
						if err := ag.Run(rest); err != nil {
							ui.PrintError(err)
						}
						autoCommit(rest)
						autoSave()
					}
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
