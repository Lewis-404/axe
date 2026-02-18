package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Lewis-404/axe/internal/agent"
	"github.com/Lewis-404/axe/internal/config"
	"github.com/Lewis-404/axe/internal/context"
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
	ag.OnText(ui.PrintAssistant)
	ag.OnTool(ui.PrintTool)

	// single-shot mode
	if len(args) > 0 {
		prompt := strings.Join(args, " ")
		if err := ag.Run(prompt); err != nil {
			ui.PrintError(err)
			os.Exit(1)
		}
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
			fmt.Println("👋")
			return
		}
		if err := ag.Run(input); err != nil {
			ui.PrintError(err)
		}
		fmt.Println()
	}
}
