// edit.go — /edit <file> [instruction]
// AI baca file → output versi baru → auto-backup → tulis file
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// handleEdit — entry point dari handleSlash
func (m *Model) handleEdit(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "**Cara pakai:** `/edit <file> [instruksi]`\n\n" +
			"**Contoh:**\n" +
			"- `/edit port.py fix exception handling`\n" +
			"- `/edit main.go tambahin logging`\n" +
			"- `/edit index.html bikin responsive`\n\n" +
			"**Cara kerja:**\n" +
			"1. AI baca file lu\n" +
			"2. AI kasih versi baru\n" +
			"3. File lama di-backup → `<file>.bak-edit-<timestamp>`\n" +
			"4. File ditimpa dengan versi baru\n\n" +
			"**Limit:** file max 32KB. Kalo lebih, pakai `/attach` aja."})
		return
	}

	path := args[1]
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.currentDir, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "File gak ketemu: " + path})
		return
	}
	if info.Size() > 32*1024 {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "File > 32KB. Terlalu gede buat auto-edit."})
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Baca file gagal: " + err.Error()})
		return
	}

	instruction := "perbaiki dan rapihin kode ini"
	if len(args) >= 3 {
		instruction = strings.Join(args[2:], " ")
	}

	lang := langFromPath(path)
	base := filepath.Base(path)

	// Prompt edit — kunci: minta FULL file dalam 1 code block
	editMsg := fmt.Sprintf(
		"Kamu adalah editor kode. TUGAS: edit file sesuai instruksi user.\n\n"+
			"**File:** `%s`\n"+
			"**Instruksi:** %s\n\n"+
			"**Isi file saat ini:**\n"+
			"```%s\n%s\n```\n\n"+
			"**ATURAN OUTPUT (WAJIB):**\n"+
			"1. Balikin FULL file yang udah diedit dalam SATU code block ```%s ... ```\n"+
			"2. JANGAN ada penjelasan sebelum/sesudah code block\n"+
			"3. JANGAN ada teks apapun di luar code block\n"+
			"4. Langsung mulai dari ```%s\n\n"+
			"Sekarang output full file-nya:",
		base, instruction, lang, string(data), lang, lang,
	)

	// Set edit mode (buat di-detect di chunkMsg Done)
	m.editActive = true
	m.editPath = path

	// Kirim ke chat (visible)
	m.messages = append(m.messages, ChatMsg{Role: "user",
		Text: fmt.Sprintf("`/edit %s` — %s", base, instruction)})
	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ""})

	m.input = ""
	m.inputCursor = 0
	m.streaming = true
	m.chatScroll = 0

	// History khusus edit — gak bawa chat lama biar fokus
	history := []Message{
		{Role: "system", Content: m.systemPrompt},
		{Role: "user", Content: editMsg},
	}

	ch := make(chan StreamChunk, 64)
	m.streamCh = ch
	go streamChat(m.provider, history, ch)
}

// completeEdit — dipanggil dari chunkMsg Done kalo editActive true
func (m *Model) completeEdit() {
	defer func() {
		m.editActive = false
		m.editPath = ""
	}()

	if len(m.messages) == 0 {
		return
	}

	// Ambil assistant message terakhir
	last := m.messages[len(m.messages)-1]
	if last.Role != "assistant" {
		return
	}

	// Extract code block pertama
	segs := splitCodeBlocks(last.Text)
	var newCode string
	for _, seg := range segs {
		if seg.IsCode && strings.TrimSpace(seg.Text) != "" {
			newCode = seg.Text
			break
		}
	}

	if newCode == "" {
		m.messages = append(m.messages, ChatMsg{Role: "error",
			Text: "AI gak balikin code block. File gak diubah."})
		return
	}

	// Backup file lama
	ts := time.Now().Format("20060102-150405")
	bakPath := m.editPath + ".bak-edit-" + ts

	oldData, err := os.ReadFile(m.editPath)
	if err == nil {
		if err := os.WriteFile(bakPath, oldData, 0644); err != nil {
			m.messages = append(m.messages, ChatMsg{Role: "error",
				Text: "Backup gagal: " + err.Error()})
			return
		}
	}

	// Tulis file baru
	if err := os.WriteFile(m.editPath, []byte(newCode), 0644); err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error",
			Text: "Tulis file gagal: " + err.Error()})
		return
	}

	// Notif sukses
	m.messages = append(m.messages, ChatMsg{Role: "assistant",
		Text: fmt.Sprintf("✓ **%s** diupdate (%d byte)\n\n"+
			"Backup: `%s`\n\n"+
			"Kalo hasil jelek, revert:\n"+
			"```bash\nmv %s %s\n```",
			filepath.Base(m.editPath), len(newCode),
			filepath.Base(bakPath),
			filepath.Base(bakPath), filepath.Base(m.editPath))})

	// Refresh file manager
	m.refreshFiles()
}
