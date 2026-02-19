package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Lewis-404/axe/internal/agent"
	"github.com/Lewis-404/axe/internal/config"
	"github.com/Lewis-404/axe/internal/context"
	"github.com/Lewis-404/axe/internal/git"
	"github.com/Lewis-404/axe/internal/history"
	"github.com/Lewis-404/axe/internal/llm"
	"github.com/Lewis-404/axe/internal/tools"
	"github.com/Lewis-404/axe/internal/ui"
)

const systemPrompt = `You are Axe, a vibe coding agent. You help users build software by reading, writing, and editing code files, executing commands, and searching codebases.

Rules:
- Be concise and direct
- Write clean, idiomatic code
- When modifying files, use edit_file for surgical changes, write_file for new files
- Always verify your changes compile/work by running appropriate commands
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
		fmt.Println("axe v0.1.0")
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

	cfg, err := config.Load()
	if err != nil {
		ui.PrintError(err)
		os.Exit(1)
	}

	dir, _ := os.Getwd()
	ctx := context.Collect(dir)
	sys := fmt.Sprintf(systemPrompt, ctx)

	registry := tools.NewRegistry(tools.RegistryOpts{
		Confirm:          ui.Confirm,
		ConfirmOverwrite: ui.ConfirmOverwrite,
		ConfirmEdit:      ui.ConfirmEdit,
	})
	client := llm.NewClient(cfg.Models, registry.Definitions())
	ag := agent.New(client, registry, sys)
	ag.OnTextDelta(ui.PrintTextDelta)
	ag.OnBlockDone(ui.PrintBlockDone)
	ag.OnTool(ui.PrintTool)
	ag.OnUsage(ui.PrintUsage)

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

	// interactive mode
	fmt.Println("🪓 Axe v0.1.0 — vibe coding agent")
	fmt.Println("   Type your request. /help for commands.")
	fmt.Println()

	for {
		input := ui.ReadLine("you> ")
		if input == "" {
			continue
		}
		if strings.HasPrefix(input, "/") {
			if input == "/exit" || input == "/quit" {
				autoSave()
				fmt.Println("👋")
				return
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
		} else {
			p, msgs, err := history.LoadLatest()
			if err != nil {
				ui.PrintError(err)
				return
			}
			ag.SetMessages(msgs)
			*savePath = p
			fmt.Printf("📂 已恢复最近对话（%d 条消息）\n", len(msgs))
		}
	case "/cost":
		in, out := ag.TotalUsage()
		ui.PrintTotalUsage(in, out)
	case "/help":
		fmt.Println("可用命令:")
		fmt.Println("  /clear          清空对话上下文")
		fmt.Println("  /list           查看最近对话记录")
		fmt.Println("  /resume         恢复最近一次对话")
		fmt.Println("  /resume <编号>  恢复指定对话（编号从 /list 获取）")
		fmt.Println("  /model          显示当前和可用模型")
		fmt.Println("  /model <name>   切换模型")
		fmt.Println("  /cost           显示累计 token 用量")
		fmt.Println("  /exit           退出 Axe")
		fmt.Println("  /help           显示此帮助")
	default:
		fmt.Printf("未知命令: %s（输入 /help 查看可用命令）\n", cmd)
	}
}
