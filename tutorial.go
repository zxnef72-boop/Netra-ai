// tutorial.go — /tutorial <nama> — panggil tutorial langkah demi langkah
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func tutorialBaseDir() string {
	dirs := []string{
		"tutorials",
		filepath.Join(filepath.Dir(os.Args[0]), "tutorials"),
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "netra-ai", "tutorials"))
	}
	for _, d := range dirs {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
	}
	return ""
}

func tutorialList() []string {
	dir := tutorialBaseDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".txt") {
			names = append(names, strings.TrimSuffix(name, ".txt"))
		}
	}
	sort.Strings(names)
	return names
}

func tutorialFind(query string) (string, string) {
	dir := tutorialBaseDir()
	if dir == "" {
		return "", ""
	}
	low := strings.ToLower(strings.TrimSpace(query))
	normalized := strings.ReplaceAll(low, " ", "-")

	candidates := []string{
		normalized + ".txt",
		low + ".txt",
	}
	for _, c := range candidates {
		full := filepath.Join(dir, c)
		if _, err := os.Stat(full); err == nil {
			data, _ := os.ReadFile(full)
			return c, string(data)
		}
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		nameLow := strings.ToLower(strings.TrimSuffix(name, ".txt"))
		nameNorm := strings.ReplaceAll(nameLow, "-", " ")

		if strings.Contains(nameNorm, low) || strings.Contains(low, nameNorm) {
			full := filepath.Join(dir, name)
			data, _ := os.ReadFile(full)
			return name, string(data)
		}
	}

	return "", ""
}

func (m *Model) handleTutorial(args []string) {
	dir := tutorialBaseDir()
	if dir == "" {
		m.messages = append(m.messages, ChatMsg{Role: "error",
			Text: "Folder `tutorials/` gak ketemu. Bikin di `~/netra-ai/tutorials/`"})
		return
	}

	if len(args) < 2 {
		names := tutorialList()
		if len(names) == 0 {
			m.messages = append(m.messages, ChatMsg{Role: "assistant",
				Text: "Belum ada tutorial. Taruh file `.txt` di `" + dir + "`"})
			return
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**%d tutorial tersedia:**\n\n", len(names)))
		for _, n := range names {
			display := strings.ReplaceAll(n, "-", " ")
			sb.WriteString(fmt.Sprintf("- `%s`\n", display))
		}
		sb.WriteString("\nPakai: `/tutorial <nama>`")
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: sb.String()})
		return
	}

	query := strings.Join(args[1:], " ")
	fname, content := tutorialFind(query)
	if content == "" {
		names := tutorialList()
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Tutorial `%s` gak ketemu.\n\n", query))
		if len(names) > 0 {
			sb.WriteString("Yang ada:\n")
			for _, n := range names {
				sb.WriteString(fmt.Sprintf("- `%s`\n", strings.ReplaceAll(n, "-", " ")))
			}
		}
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: sb.String()})
		return
	}

	display := strings.TrimSuffix(fname, ".txt")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📖 **Tutorial: %s**\n\n", strings.ReplaceAll(display, "-", " ")))
	sb.WriteString("```\n")
	sb.WriteString(strings.TrimRight(content, "\n"))
	sb.WriteString("\n```")

	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: sb.String()})
}
