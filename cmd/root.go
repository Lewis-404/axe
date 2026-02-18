package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Lewis-404/axe/internal/agent"
	"github.com/Lewis-404/axe/internal/config"
	"github.com/Lewis-404/axe/internal/context"
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
		lines, err := history.ListRecent(10)
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

	registry := tools.NewRegistry(ui.Confirm)
	client := llm.NewClient(cfg, registry.Definitions())
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

	// single-shot mode
	if len(args) > 0 {
		prompt := strings.Join(args, " ")
		if err := ag.Run(prompt); err != nil {
			ui.PrintError(err)
			os.Exit(1)
		}
		autoSave()
		return
	}

	// interactive mode
	fmt.Println("🪓 Axe v0.1.0 — vibe coding agent")
	fmt.Println("   Type your request, or 'quit' to exit.\n")

	for {
		input := ui.ReadLine("you> ")
		if input == "" {
			continue
		}
		if input == "quit" || input == "exit" {
			autoSave()
			fmt.Println("👋")
			return
		}
		if err := ag.Run(input); err != nil {
			ui.PrintError(err)
		}
		autoSave()
		fmt.Println()
	}
}
