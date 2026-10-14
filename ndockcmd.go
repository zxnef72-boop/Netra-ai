// ndockcmd.go — command handler /ndock di TUI.
// Wrapper tipis: parse subcommand + args, dispatch ke fungsi di ndock.go.
package main

import (
	"fmt"
	"strings"
)

func ndockHelp() string {
	return "**NetraDock** — Docker-like pakai proot-distro\n\n" +
		"Pakai:\n" +
		"- `/ndock` — list image\n" +
		"- `/ndock pull <name> <url|distro>` — pull image\n" +
		"- `/ndock run <name> <command>` — jalanin command di container\n" +
		"- `/ndock ps` — container yang jalan\n" +
		"- `/ndock logs <name>` — log container\n" +
		"- `/ndock inspect <name>` — detail container\n" +
		"- `/ndock rm <name>` — hapus container\n"
}

// handleNdock — dispatch "/ndock <sub> [args]".
func (m *Model) handleNdock(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ndockHelp()})
		return
	}
	sub := strings.ToLower(args[1])
	rest := args[2:]

	switch sub {
	case "help", "-h", "--help":
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ndockHelp()})

	case "ls", "images", "image", "list":
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ndockImages()})

	case "pull":
		if len(rest) < 2 {
			m.messages = append(m.messages, ChatMsg{Role: "error",
				Text: "Pakai: `/ndock pull <name> <url|distro>`\nContoh: `/ndock pull alpine alpine`"})
			return
		}
		name := rest[0]
		url := strings.Join(rest[1:], " ")
		out, err := ndockPull(name, url)
		if err != nil {
			m.messages = append(m.messages, ChatMsg{Role: "error",
				Text: fmt.Sprintf("Pull gagal: %v\n%s", err, out)})
			return
		}
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: out})

	case "run", "exec":
		if len(rest) < 2 {
			m.messages = append(m.messages, ChatMsg{Role: "error",
				Text: "Pakai: `/ndock run <name> <command>`\nContoh: `/ndock run alpine echo hello`"})
			return
		}
		name := rest[0]
		cmdStr := strings.Join(rest[1:], " ")
		out, err := ndockRun(name, cmdStr)
		if err != nil {
			m.messages = append(m.messages, ChatMsg{Role: "error",
				Text: fmt.Sprintf("Run gagal: %v\n%s", err, out)})
			return
		}
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: out})

	case "ps":
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ndockPS()})

	case "logs", "log":
		if len(rest) < 1 {
			m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Pakai: `/ndock logs <name>`"})
			return
		}
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ndockLogs(rest[0])})

	case "inspect", "info":
		if len(rest) < 1 {
			m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Pakai: `/ndock inspect <name>`"})
			return
		}
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ndockInspect(rest[0])})

	case "rm", "remove", "delete":
		if len(rest) < 1 {
			m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Pakai: `/ndock rm <name>`"})
			return
		}
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ndockRemove(rest[0])})

	default:
		m.messages = append(m.messages, ChatMsg{Role: "error",
			Text: fmt.Sprintf("Subcommand gak dikenal: `%s`\n\n%s", sub, ndockHelp())})
	}
}
