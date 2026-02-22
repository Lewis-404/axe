package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Lewis-404/axe/internal/agent"
	"github.com/Lewis-404/axe/internal/commands"
	"github.com/Lewis-404/axe/internal/context"
	"github.com/Lewis-404/axe/internal/git"
	"github.com/Lewis-404/axe/internal/history"
	"github.com/Lewis-404/axe/internal/llm"
	"github.com/Lewis-404/axe/internal/pricing"
	"github.com/Lewis-404/axe/internal/skills"
	"github.com/Lewis-404/axe/internal/ui"
)

var pkgCustomCmds []commands.CustomCommand
var pkgSkills []skills.Skill

// resumeConversation restores a conversation and refreshes project context.
func resumeConversation(ag *agent.Agent, path string, msgs []llm.Message, savePath *string, label string) {
	ag.SetMessages(msgs)
	dir, _ := os.Getwd()
	ag.RefreshSystem(fmt.Sprintf(systemPrompt, context.Collect(dir)))
	*savePath = path
	fmt.Printf("🔄 %s（%d 条消息）\n", label, len(msgs))
	ui.PrintHistory(msgs)
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
			resumeConversation(ag, p, msgs, savePath, fmt.Sprintf("已恢复对话并刷新项目上下文 [%d]", idx))
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
			resumeConversation(ag, p, msgs, savePath, fmt.Sprintf("已恢复对话并刷新项目上下文 [%d]", idx))
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
			fmt.Printf("📊 累计: ↑%s ↓%s | 💰 $%.4f\n", ui.FmtTokens(in), ui.FmtTokens(out), cost)
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
	case "/diff":
		dir, _ := os.Getwd()
		if !git.IsRepo(dir) {
			fmt.Println("⚠️ 当前目录不是 git 仓库")
		} else {
			out, err := git.Diff(dir)
			if err != nil {
				ui.PrintError(err)
			} else if out == "" {
				fmt.Println("✅ 没有未提交的变更")
			} else {
				fmt.Println(out)
			}
		}
	case "/retry":
		if last := ag.PopLastRound(); last == "" {
			fmt.Println("⚠️ 没有可重试的对话")
		} else {
			fmt.Println("🔄 重试上一轮...")
			if err := ag.Run(last); err != nil {
				ui.PrintError(err)
			}
		}
	case "/export":
		msgs := ag.Messages()
		if len(msgs) == 0 {
			fmt.Println("⚠️ 当前没有对话内容")
		} else {
			var sb strings.Builder
			sb.WriteString("# Axe 对话导出\n\n")
			for _, m := range msgs {
				for _, b := range m.Content {
					if b.Type == "text" && b.Text != "" {
						if m.Role == llm.RoleUser {
							sb.WriteString("## 🧑‍💻 User\n\n")
						} else {
							sb.WriteString("## 🪓 Axe\n\n")
						}
						sb.WriteString(b.Text)
						sb.WriteString("\n\n")
					}
				}
			}
			outPath := "axe-export.md"
			if len(parts) > 1 {
				outPath = parts[1]
			}
			if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
				ui.PrintError(err)
			} else {
				fmt.Printf("📄 已导出到 %s\n", outPath)
			}
		}
	case "/init":
	case "/git":
		dir, _ := os.Getwd()
		if !git.IsRepo(dir) {
			fmt.Println("⚠️ 当前目录不是 git 仓库")
		} else {
			sub := "status"
			if len(parts) > 1 {
				sub = parts[1]
			}
			var gitArgs []string
			switch sub {
			case "status", "s":
				gitArgs = []string{"status", "--short"}
			case "log", "l":
				gitArgs = []string{"log", "--oneline", "-10"}
			case "branch", "b":
				gitArgs = []string{"branch", "-a"}
			case "stash":
				gitArgs = []string{"stash", "list"}
			default:
				gitArgs = parts[1:]
			}
			c := exec.Command("git", gitArgs...)
			c.Dir = dir
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Run()
		}
	case "/context":
		in, out := ag.TotalUsage()
		msgs := ag.Messages()
		fmt.Printf("📊 上下文: %d 条消息, ↑%s ↓%s\n", len(msgs), ui.FmtTokens(in), ui.FmtTokens(out))
	case "/skills":
		if len(pkgSkills) == 0 {
			fmt.Println("📦 没有已加载的技能")
		} else {
			fmt.Printf("📦 已加载 %d 个技能 (使用 /skill <name> 激活):\n", len(pkgSkills))
			for _, s := range pkgSkills {
				fmt.Printf("  • %s — %s\n", s.Name, s.Description)
			}
		}
	case "/skill":
		if len(parts) < 2 {
			fmt.Println("用法: /skill <name>")
			return
		}
		s := skills.FindSkill(pkgSkills, parts[1])
		if s == nil {
			fmt.Printf("❌ 未找到技能: %s\n", parts[1])
			return
		}
		content, err := skills.ReadSkillContent(*s)
		if err != nil {
			ui.PrintError(err)
			return
		}
		ag.InjectContext(fmt.Sprintf("[Skill: %s]\n%s", s.Name, content))
		fmt.Printf("🧩 已激活技能: %s\n", s.Name)
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
		fmt.Println("  /diff           查看未提交的变更")
		fmt.Println("  /retry          重试上一轮对话")
		fmt.Println("  /export [file]  导出对话为 Markdown")
		fmt.Println("  /git [cmd]      快捷 git (status/log/branch)")
		fmt.Println("  /context        查看上下文 token 用量")
		fmt.Println("  /budget <$>     设置费用上限 (off 关闭)")
		fmt.Println("  /cost           显示累计 token 用量和费用")
		fmt.Println("  /skills         列出已加载的技能")
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
