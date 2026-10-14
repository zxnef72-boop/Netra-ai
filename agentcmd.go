// agentcmd.go — command /agent dan /agent-unsafe di TUI.
package main

import (
	"fmt"
	"strings"
)

// handleAgent — jalanin agent otonom dengan prompt user.
func (m *Model) handleAgent(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant",
			Text: "**Cara pakai:** `/agent <prompt>`\n\n" +
				"**Contoh:**\n" +
				"- `/agent bikin file hello.txt isinya halo`\n" +
				"- `/agent list semua file .go di folder ini`\n\n" +
				"Agent bakal pakai tools buat nyelesain task.\n" +
				"Butuh `/agent-unsafe on` buat write_file, run_command, dan run_in_container."})
		return
	}

	prompt := strings.Join(args[1:], " ")
	m.messages = append(m.messages, ChatMsg{Role: "user", Text: "AGENT: " + prompt})
	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ""})
	m.streaming = true
	m.chatScroll = 0

	ch := make(chan StreamChunk, 64)
	m.streamCh = ch

	cwd := m.currentDir
	provider := m.provider

	go func() {
		defer close(ch)
		out, err := cmdAgentRun(prompt, cwd, provider)
		if err != nil {
			ch <- StreamChunk{Text: fmt.Sprintf("[agent error: %v]", err), Provider: "agent"}
		} else {
			ch <- StreamChunk{Text: out, Provider: "agent"}
		}
		ch <- StreamChunk{Done: true, Provider: "agent"}
	}()
}

// handleAgentUnsafe — toggle izin tool bahaya (write_file, run_command, run_in_container).
func (m *Model) handleAgentUnsafe(args []string) {
	if len(args) >= 2 {
		switch strings.ToLower(args[1]) {
		case "on", "true", "1":
			agentAllowWrite = true
		case "off", "false", "0":
			agentAllowWrite = false
		default:
			m.messages = append(m.messages, ChatMsg{Role: "error",
				Text: "Pakai: `/agent-unsafe on` atau `/agent-unsafe off`"})
			return
		}
	} else {
		agentAllowWrite = !agentAllowWrite
	}
	status := "OFF"
	if agentAllowWrite {
		status = "ON"
	}
	m.messages = append(m.messages, ChatMsg{Role: "assistant",
		Text: fmt.Sprintf("Agent unsafe mode: **%s**\nKalo ON, agent bisa write_file, run_command, dan run_in_container.", status)})
}
