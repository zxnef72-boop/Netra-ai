// agent.go — AI agent yang bisa baca/edit file + jalanin command.
// Terpisah dari Eliza. Butuh LLM yang support tool calling.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type agentToolDef struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
}

func agentToolList() []agentToolDef {
	return []agentToolDef{
		{
			Name:        "read_file",
			Description: "Baca isi file. Path relatif ke cwd atau absolute.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Path file"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "edit_file",
			Description: "Edit file yang sudah ada dengan penggantian teks persis. old_text wajib ditemukan tepat satu kali. Backup dibuat sebelum perubahan.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":     map[string]interface{}{"type": "string"},
					"old_text": map[string]interface{}{"type": "string"},
					"new_text": map[string]interface{}{"type": "string"},
				},
				"required": []string{"path", "old_text", "new_text"},
			},
		},
		{
			Name:        "write_file",
			Description: "Untuk file kecil dan menengah, tulis SELURUH isi dalam SATU panggilan: append=false, complete=true. Jangan membagi HTML atau game sederhana menjadi potongan. Hanya untuk file yang terlalu panjang: potongan pertama append=false, complete=false; potongan berikutnya append=true, complete=false; potongan terakhir append=true, complete=true. Jangan mengulang potongan yang sudah berhasil. File tujuan diterbitkan setelah validasi.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":     map[string]interface{}{"type": "string"},
					"content":  map[string]interface{}{"type": "string"},
					"append":   map[string]interface{}{"type": "boolean"},
					"complete": map[string]interface{}{"type": "boolean"},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "list_dir",
			Description: "List isi folder. Pakai '.' untuk current dir.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "run_command",
			Description: "Jalanin shell command di cwd. Output dibatasi 5KB.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cmd": map[string]interface{}{"type": "string"},
				},
				"required": []string{"cmd"},
			},
		},
		{
			Name:        "run_in_container",
			Description: "Jalanin shell command di dalam container Ndock (proot-distro). Terisolasi dari Termux. Butuh izin /agent-unsafe on.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"container": map[string]interface{}{"type": "string", "description": "Nama container Ndock"},
					"command":   map[string]interface{}{"type": "string", "description": "Command di container"},
				},
				"required": []string{"container", "command"},
			},
		},
	}
}

// agentAllowWrite — false secara default. Harus di-set true eksplisit.
var agentAllowWrite = false

// agentWorkspace — folder yang boleh diakses agent. Termux: /sdcard/AiNef, lainnya: ~/netra-projects (override env NETRA_WORKSPACE).
var agentWorkspace = defaultAgentWorkspace()

// agentPathAllowed — cek apakah path diizinkan (dalam workspace).
func agentPathAllowed(path string) bool {
	if agentWorkspace == "" {
		return true // no restriction
	}
	// Resolve path ke absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absWs, err := filepath.Abs(agentWorkspace)
	if err != nil {
		return false
	}
	// Cek apakah path di dalam workspace
	rel, err := filepath.Rel(absWs, absPath)
	if err != nil {
		return false
	}
	// Kalau rel diawali "..", berarti keluar workspace
	if strings.HasPrefix(rel, "..") {
		return false
	}
	return true
}

func agentEditFile(path, oldText, newText, cwd string) string {
	if path == "" || oldText == "" {
		return "[error: path dan old_text wajib diisi]"
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Sprintf("[error path: %v]", err)
	}

	info, err := os.Lstat(absPath)
	if err != nil {
		return fmt.Sprintf("[error memeriksa file: %v]", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "[error: hanya file biasa yang boleh diedit]"
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return fmt.Sprintf("[error resolve path: %v]", err)
	}
	if !agentPathAllowed(realPath) {
		return "[DIBLOKIR: file berada di luar workspace]"
	}
	if info.Size() > 1024*1024 {
		return "[error: ukuran file maksimal 1 MiB]"
	}

	oldData, err := os.ReadFile(realPath)
	if err != nil {
		return fmt.Sprintf("[error membaca file: %v]", err)
	}
	if strings.Count(string(oldData), oldText) != 1 {
		return "[error: old_text harus ditemukan tepat satu kali. File tidak diubah]"
	}

	newData := []byte(strings.Replace(string(oldData), oldText, newText, 1))

	backupPath := realPath + ".bak-agent-" +
		time.Now().Format("20060102-150405.000000000")
	backup, err := os.OpenFile(
		backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600,
	)
	if err != nil {
		return fmt.Sprintf("[error membuat backup: %v]", err)
	}
	if _, err := backup.Write(oldData); err != nil {
		backup.Close()
		_ = os.Remove(backupPath)
		return fmt.Sprintf("[error menulis backup: %v]", err)
	}
	if err := backup.Sync(); err != nil {
		backup.Close()
		_ = os.Remove(backupPath)
		return fmt.Sprintf("[error sync backup: %v]", err)
	}
	if err := backup.Close(); err != nil {
		_ = os.Remove(backupPath)
		return fmt.Sprintf("[error menutup backup: %v]", err)
	}

	// Batalkan jika isi file berubah sejak dibaca.
	current, err := os.ReadFile(realPath)
	if err != nil || !bytes.Equal(current, oldData) {
		return "[error: file berubah selama proses edit. Perubahan dibatalkan]"
	}

	temp, err := os.CreateTemp(filepath.Dir(realPath), ".netra-edit-*")
	if err != nil {
		return fmt.Sprintf("[error membuat file sementara: %v]", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(info.Mode().Perm()); err != nil {
		temp.Close()
		return fmt.Sprintf("[error permission file sementara: %v]", err)
	}
	if _, err := temp.Write(newData); err != nil {
		temp.Close()
		return fmt.Sprintf("[error menulis file sementara: %v]", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Sprintf("[error sync file sementara: %v]", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Sprintf("[error menutup file sementara: %v]", err)
	}

	current, err = os.ReadFile(realPath)
	if err != nil || !bytes.Equal(current, oldData) {
		return "[error: file berubah sebelum publikasi. Perubahan dibatalkan]"
	}

	if err := os.Rename(tempPath, realPath); err != nil {
		return fmt.Sprintf("[error mengganti file: %v. Backup: %s]", err, backupPath)
	}

	verified, err := os.ReadFile(realPath)
	if err != nil || !bytes.Equal(verified, newData) {
		return fmt.Sprintf("[error verifikasi hasil edit. Backup: %s]", backupPath)
	}

	return fmt.Sprintf(
		"OK: file berhasil diedit dan diverifikasi: %s (%d byte). Backup: %s",
		realPath, len(newData), backupPath,
	)
}

func agentNormalizeToolName(name string) string {
	name = strings.TrimSpace(name)

	if strings.HasPrefix(name, "repo_browser.") {
		alias := strings.TrimPrefix(name, "repo_browser.")
		switch alias {
		case "list_dir", "read_file", "edit_file", "write_file":
			return alias
		}
	}

	return name
}

func agentExecTool(name string, args map[string]interface{}, cwd string) string {
	// Safety: tool bahaya butuh izin
	if (name == "write_file" || name == "edit_file" || name == "run_command" || name == "run_in_container") && !agentAllowWrite {
		return "[DIBLOKIR: tool '" + name + "' butuh izin. Jalankan /agent-unsafe on dulu]"
	}

	// Cek path restriction untuk read/write/list
	if name == "read_file" || name == "write_file" || name == "edit_file" || name == "list_dir" {
		if pathArg, ok := args["path"].(string); ok {
			if !filepath.IsAbs(pathArg) {
				pathArg = filepath.Join(cwd, pathArg)
			}
			if !agentPathAllowed(pathArg) {
				return "[DIBLOKIR: path di luar workspace agent. Workspace: " + agentWorkspace + "]"
			}
		}
	}

	switch name {
	case "read_file":
		path, _ := args["path"].(string)
		if path == "" {
			return "[error: path kosong]"
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Sprintf("[error baca file: %v]", err)
		}
		out := string(data)
		if len(out) > 8000 {
			out = out[:8000] + "\n... (terpotong)"
		}
		return out

	case "edit_file":
		path, _ := args["path"].(string)
		oldText, _ := args["old_text"].(string)
		newText, _ := args["new_text"].(string)
		return agentEditFile(path, oldText, newText, cwd)

	case "write_file":
		path, _ := args["path"].(string)

		var content string
		switch v := args["content"].(type) {
		case string:
			content = v
		case map[string]interface{}:
			if val, ok := v["value"].(string); ok {
				content = val
			} else if desc, ok := v["description"].(string); ok {
				content = desc
			}
		}

		if path == "" {
			return "[error: path kosong]"
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}

		appendMode, _ := args["append"].(bool)
		complete, hasComplete := args["complete"].(bool)
		if !hasComplete {
			complete = true
		}

		ext := strings.ToLower(filepath.Ext(path))
		isHTML := ext == ".html" || ext == ".htm"

		// HTML wajib dikirim utuh dalam satu panggilan.
		if isHTML && (appendMode || !complete) {
			return "[error: HTML harus ditulis utuh dalam SATU panggilan write_file dengan append=false dan complete=true. Kirim ulang seluruh isi file, jangan gunakan potongan.]"
		}

		tempPath := path + ".netra-part"

		// Penulisan baru membersihkan staging yang tertinggal
		// dari percobaan sebelumnya yang gagal divalidasi.
		if !appendMode {
			if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
				return fmt.Sprintf("[error membersihkan staging lama: %v]", err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Sprintf("[error membuat folder: %v]", err)
		}

		flags := os.O_WRONLY | os.O_CREATE
		if appendMode {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_EXCL
		}

		f, err := os.OpenFile(tempPath, flags, 0644)
		if err != nil {
			return fmt.Sprintf("[error file sementara: %v]", err)
		}
		if _, err := f.WriteString(content); err != nil {
			f.Close()
			return fmt.Sprintf("[error menulis potongan: %v]", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Sprintf("[error menutup file sementara: %v]", err)
		}

		if !complete {
			return fmt.Sprintf("OK: potongan tersimpan (%d byte). Lanjutkan dengan append=true.", len(content))
		}

		data, err := os.ReadFile(tempPath)
		if err != nil {
			return fmt.Sprintf("[error membaca file sementara: %v]", err)
		}

		if isHTML {
			html := strings.ToLower(string(data))
			if !strings.Contains(html, "<html") ||
				!strings.Contains(html, "</html>") {
				_ = os.Remove(tempPath)
				return "[error: HTML belum lengkap; staging dibersihkan dan file tujuan belum diterbitkan]"
			}
			if strings.Contains(html, "<body") &&
				!strings.Contains(html, "</body>") {
				_ = os.Remove(tempPath)
				return "[error: body belum ditutup; staging dibersihkan dan file tujuan belum diterbitkan]"
			}
			if strings.Count(html, "<script") >
				strings.Count(html, "</script>") {
				_ = os.Remove(tempPath)
				return "[error: script belum ditutup; staging dibersihkan dan file tujuan belum diterbitkan]"
			}
		}

		// Hard link create-only: tidak menimpa file tujuan yang sudah ada.
		if err := os.Link(tempPath, path); err != nil {
			return fmt.Sprintf("[error menerbitkan file; tujuan mungkin sudah ada: %v]", err)
		}
		if err := os.Remove(tempPath); err != nil {
			return fmt.Sprintf("OK: file diterbitkan, tetapi staging gagal dihapus: %v", err)
		}

		return fmt.Sprintf("OK: file selesai dan tervalidasi: %s (%d byte)", path, len(data))
	case "list_dir":
		path, _ := args["path"].(string)
		if path == "" || path == "." {
			path = cwd
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return fmt.Sprintf("[error list dir: %v]", err)
		}
		var sb strings.Builder
		for _, e := range entries {
			mark := "  F "
			if e.IsDir() {
				mark = "  D "
			}
			sb.WriteString(mark + e.Name() + "\n")
		}
		return sb.String()

	case "run_command":
		cmdStr, _ := args["cmd"].(string)
		if cmdStr == "" {
			return "[error: cmd kosong]"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		result := string(out)
		if len(result) > 5000 {
			result = result[:5000] + "\n... (terpotong)"
		}
		if err != nil {
			return fmt.Sprintf("[exit error: %v]\n%s", err, result)
		}
		return result
	case "run_in_container":
		container, _ := args["container"].(string)
		command, _ := args["command"].(string)
		if container == "" || command == "" {
			return "[error: container/command kosong]"
		}
		out, err := ndockRun(container, command)
		if err != nil {
			return fmt.Sprintf("[exit error: %v]\n%s", err, out)
		}
		if len(out) > 5000 {
			out = out[:5000] + "\n... (terpotong)"
		}
		return out
	}
	return fmt.Sprintf("[error: tool tidak dikenal: %s]", name)
}

type agentMsg struct {
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	ToolCalls  []agentToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

type agentToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func agentHistoryMessages(history []ChatMsg) []agentMsg {
	start := len(history) - 8
	if start < 0 {
		start = 0
	}

	out := make([]agentMsg, 0, len(history)-start)
	for _, cm := range history[start:] {
		if cm.Role != "user" && cm.Role != "assistant" {
			continue
		}

		content := cm.Text
		if cm.APIContent != "" {
			content = cm.APIContent
		}
		if content == "" {
			continue
		}

		runes := []rune(content)
		if len(runes) > 2500 {
			content = string(runes[len(runes)-2500:]) + "\n[riwayat dipotong]"
		}
		out = append(out, agentMsg{Role: cm.Role, Content: content})
	}
	return out
}

func cmdAgentRun(userPrompt string, cwd string, provider Provider, history []ChatMsg) (string, error) {
	var log strings.Builder

	if strings.HasPrefix(strings.ToLower(provider.Name), "eliza") {
		return "", fmt.Errorf("Agent butuh LLM. Pilih dulu: /vendor 1 (Ollama) atau /vendor 2 (Gemini)")
	}
	if provider.BaseURL == "" || provider.Model == "" {
		return "", fmt.Errorf("Provider %s gak punya base_url/model", provider.Name)
	}

	log.WriteString(fmt.Sprintf("**Agent Run** — `%s` / `%s`\n\n", provider.Name, provider.Model))
	log.WriteString(fmt.Sprintf("Prompt: `%s`\n\n", userPrompt))

	// Build tools JSON
	toolsJSON := []map[string]interface{}{}
	for _, t := range agentToolList() {
		toolsJSON = append(toolsJSON, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		})
	}

	messages := []agentMsg{
		{Role: "system", Content: `Kamu adalah Netra Agent, coding agent otonom.
Pahami tujuan pengguna dan konteks percakapan sebelum bertindak.
Gunakan read_file dan list_dir untuk memeriksa workspace sebelum mengubah file.
Saat diminta membuat aplikasi atau game, hasilkan implementasi lengkap dan fungsional, bukan kerangka atau placeholder.
Jika pengguna meminta file dibuat, WAJIB gunakan write_file. Jangan hanya mencetak source code di chat.

STRATEGI PENULISAN FILE:
Untuk file kecil dan menengah, tulis seluruh isi dalam SATU panggilan write_file dengan append=false dan complete=true.
Jangan menggunakan chunk untuk file HTML sederhana atau game sederhana.
Hanya gunakan chunk jika isi file benar-benar terlalu panjang untuk satu panggilan.
Jika harus menggunakan chunk, buat potongan besar dan berurutan, sekitar 3000-5000 karakter per potongan:
1. Pertama append=false, complete=false.
2. Potongan berikutnya append=true, complete=false.
3. Terakhir append=true, complete=true.
Jangan mengulang potongan yang sudah berhasil disimpan.
Setelah write_file berhasil, jangan tulis ulang file yang sama. Lanjutkan ke verifikasi. 
Jika mendapat error, baca pesannya dan perbaiki langkah berikutnya.
Jangan menimpa file yang sudah ada. Periksa folder dan gunakan nama file baru jika perlu.
Setelah file selesai, baca kembali dan verifikasi hasilnya.
Jangan mengklaim selesai sebelum hasil benar-benar tersedia.
Jalankan validasi yang relevan jika memungkinkan.
Jawab dalam Bahasa Indonesia.`},
	}
	messages = append(messages, agentHistoryMessages(history)...)
	messages = append(messages, agentMsg{Role: "user", Content: userPrompt})
	client := &http.Client{Timeout: 8 * time.Minute}
	maxSteps := 10
	noToolRetry := 0

	promptLower := strings.ToLower(userPrompt)
	requiresFileTask := false
	for _, keyword := range []string{
		"buat", "bikin", "game", "website", "html",
		"file", "aplikasi", "program", "script",
	} {
		if strings.Contains(promptLower, keyword) {
			requiresFileTask = true
			break
		}
	}

	// Catat file yang berhasil dibuat dan diverifikasi.
	writtenFiles := make(map[string]bool)
	verifiedFiles := make(map[string]bool)
	readFiles := make(map[string]bool)

	createdFiles := make(map[string]bool)
	modifiedFiles := make(map[string]bool)
	inspectedFiles := make(map[string]bool)

	writtenOrder := []string{}
	createdOrder := []string{}
	modifiedOrder := []string{}
	inspectedOrder := []string{}

	appendTracked := func(order *[]string, seen map[string]bool, path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			*order = append(*order, path)
		}
	}

	resolveTrackedPath := func(raw string) string {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return ""
		}
		if !filepath.IsAbs(raw) {
			raw = filepath.Join(cwd, raw)
		}
		if abs, err := filepath.Abs(raw); err == nil {
			raw = abs
		}
		return filepath.Clean(raw)
	}

	verifyPendingFiles := func(reason string) {
		pending := []string{}
		for _, path := range writtenOrder {
			if writtenFiles[path] && !verifiedFiles[path] {
				pending = append(pending, path)
			}
		}
		if len(pending) == 0 {
			return
		}

		log.WriteString(fmt.Sprintf("\n**Verifikasi otomatis (%s):**\n", reason))
		for _, path := range pending {
			result := agentExecTool("read_file", map[string]interface{}{
				"path": path,
			}, cwd)

			if strings.HasPrefix(strings.TrimSpace(result), "[error") {
				log.WriteString(fmt.Sprintf("  GAGAL membaca `%s`: %s\n", path, strings.TrimSpace(result)))
				continue
			}

			appendTracked(&inspectedOrder, inspectedFiles, path)
			verifiedFiles[path] = true
			lines := strings.Count(result, "\n") + 1
			log.WriteString(fmt.Sprintf("  OK `%s` — %d baris, %d byte\n", path, lines, len(result)))
		}
	}
	writeAgentFileSummary := func() {
		displayPath := func(path string) string {
			rel, err := filepath.Rel(cwd, path)
			if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
				return filepath.ToSlash(rel)
			}
			return path
		}

		writeSection := func(title string, paths []string, include func(string) bool) {
			items := []string{}
			for _, path := range paths {
				if include == nil || include(path) {
					items = append(items, path)
				}
			}
			if len(items) == 0 {
				return
			}
			log.WriteString("\n**" + title + ":**\n")
			for _, path := range items {
				log.WriteString("  - `" + displayPath(path) + "`\n")
			}
		}

		log.WriteString("\n---\n**RINGKASAN FILE AGENT**\n")
		writeSection("Created", createdOrder, nil)
		writeSection("Modified", modifiedOrder, nil)
		writeSection("Inspected", inspectedOrder, nil)
		writeSection("Unchanged", inspectedOrder, func(path string) bool {
			return !createdFiles[path] && !modifiedFiles[path]
		})
		writeSection("Verified", writtenOrder, func(path string) bool {
			return verifiedFiles[path]
		})

		if len(createdOrder)+len(modifiedOrder)+len(inspectedOrder) == 0 {
			log.WriteString("\nTidak ada perubahan atau pemeriksaan file yang tercatat.\n")
		}
	}
	// Loop detection: simpen signature tool call terakhir
	lastCallSig := ""
	repeatCount := 0

	for step := 1; step <= maxSteps; step++ {
		agentEmit("think", "", fmt.Sprintf("model mikir (langkah %d) [%s/%s]", step, provider.Name, provider.Model), "")
		reqBody := map[string]interface{}{
			"model":       provider.Model,
			"messages":    messages,
			"tools":       toolsJSON,
			"temperature": 0.3,
			"max_tokens":  6000,
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req, err := http.NewRequest("POST", provider.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
		if err != nil {
			return log.String(), err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)

		resp, err := client.Do(req)
		if err != nil {
			return log.String(), fmt.Errorf("HTTP: %w", err)
		}

		// Cek status code SEBELUM decode
		for try := 0; try < 2 && resp.StatusCode == 429; try++ {
			bodyRetry, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			wait, ok := agentRetryWait(resp.Header.Get("Retry-After"), string(bodyRetry))
			if !ok || wait > 45*time.Second {
				return log.String(), fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyRetry)))
			}
			agentEmit("think", "", fmt.Sprintf("kena rate limit, tunggu %ds (langkah %d)", int(wait.Seconds()), step), "")
			time.Sleep(wait)
			reqRetry, errRetry := http.NewRequest("POST", provider.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
			if errRetry != nil {
				return log.String(), errRetry
			}
			reqRetry.Header.Set("Content-Type", "application/json")
			reqRetry.Header.Set("Authorization", "Bearer "+provider.APIKey)
			resp, err = client.Do(reqRetry)
			if err != nil {
				return log.String(), fmt.Errorf("HTTP: %w", err)
			}
		}
		if resp.StatusCode != 200 {
			bodyErr, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return log.String(), fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyErr)))
		}

		var parsed struct {
			Choices []struct {
				Message struct {
					Role      string          `json:"role"`
					Content   string          `json:"content"`
					ToolCalls []agentToolCall `json:"tool_calls"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		err = json.NewDecoder(resp.Body).Decode(&parsed)
		resp.Body.Close()
		if err != nil {
			return log.String(), fmt.Errorf("decode: %w", err)
		}
		if len(parsed.Choices) == 0 {
			return log.String(), fmt.Errorf("choices kosong")
		}

		msg := parsed.Choices[0].Message
		finishReason := parsed.Choices[0].FinishReason
		if finishReason == "length" {
			log.WriteString(fmt.Sprintf("\\n**[Langkah %d] Output atau tool call terpotong karena batas token provider.**\\n", step))
			if msg.Content != "" {
				log.WriteString(msg.Content + "\\n")
			}
			log.WriteString("Tool tidak dijalankan karena output belum lengkap.\\n")
			writeAgentFileSummary()
			return log.String(), nil
		}

		// Kalau model gak isi tool_calls, tapi content isinya JSON tool call
		// (qwen2.5-coder kadang gini), kita parse manual.
		if len(msg.ToolCalls) == 0 && msg.Content != "" {
			trimmed := strings.TrimSpace(msg.Content)
			// Cari JSON object di content
			var tc struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			if strings.HasPrefix(trimmed, "{") && json.Unmarshal([]byte(trimmed), &tc) == nil && tc.Name != "" {
				argsJSON, _ := json.Marshal(tc.Arguments)
				msg.ToolCalls = []agentToolCall{{
					ID:   "call_manual_" + fmt.Sprint(step),
					Type: "function",
				}}
				msg.ToolCalls[0].Function.Name = tc.Name
				msg.ToolCalls[0].Function.Arguments = string(argsJSON)
				msg.Content = "" // biar gak di-print sebagai teks
			}
		}

		if len(msg.ToolCalls) == 0 && requiresFileTask && noToolRetry < 2 {
			pendingVerification := []string{}
			for _, path := range writtenOrder {
				if !verifiedFiles[path] {
					pendingVerification = append(pendingVerification, path)
				}
			}

			retryInstruction := ""
			action := ""
			if len(writtenFiles) == 0 && len(readFiles) == 0 {
				action = "pembuatan file"
				retryInstruction = "Belum ada file yang berhasil dibuat. Gunakan write_file untuk membuat file yang diminta di workspace. Jangan tampilkan source code di chat. Untuk HTML, kirim seluruh isi dalam satu panggilan dengan append=false dan complete=true."
			} else if len(pendingVerification) > 0 {
				action = "verifikasi file"
				retryInstruction = "File berikut sudah berhasil dibuat. JANGAN membuat ulang, mengganti nama, atau menduplikasinya. Baca setiap file berikut menggunakan read_file untuk verifikasi:\n" + strings.Join(pendingVerification, "\n")
			}

			if retryInstruction != "" {
				noToolRetry++
				retryContent := msg.Content
				runes := []rune(retryContent)
				if len(runes) > 3000 {
					retryContent = string(runes[:3000]) + "\n[respons dipotong]"
				}
				messages = append(messages,
					agentMsg{Role: "assistant", Content: retryContent},
					agentMsg{Role: "user", Content: retryInstruction},
				)
				log.WriteString(fmt.Sprintf("**[Langkah %d]** Meminta %s (%d/2).\n", step, action, noToolRetry))
				continue
			}
		}

		if msg.Content != "" {
			log.WriteString(fmt.Sprintf("**[Langkah %d]** %s\n\n", step, msg.Content))
		}

		if len(msg.ToolCalls) == 0 {
			if msg.Content == "" {
				log.WriteString("_(selesai tanpa output)_")
			}
			verifyPendingFiles("Agent selesai")
			writeAgentFileSummary()
			return log.String(), nil
		}

		messages = append(messages, agentMsg{
			Role:      "assistant",
			Content:   msg.Content,
			ToolCalls: msg.ToolCalls,
		})

		// Loop detection — kalau 3x panggil tool sama persis, stop
		callSig := ""
		for _, tc := range msg.ToolCalls {
			callSig += tc.Function.Name + ":" + tc.Function.Arguments + "|"
		}
		if callSig == lastCallSig {
			repeatCount++
			if repeatCount >= 1 {
				log.WriteString("\n**Loop terdeteksi — agent mengulang tool yang sama.**\n")
				log.WriteString("Model mungkin gak bisa selesai. Coba model lebih gede.\n")
				verifyPendingFiles("loop terdeteksi")
				writeAgentFileSummary()
				return log.String(), nil
			}
		} else {
			lastCallSig = callSig
			repeatCount = 0
		}

		// Normalisasi alias tool sebelum progress dan eksekusi.
		for i := range msg.ToolCalls {
			msg.ToolCalls[i].Function.Name = agentNormalizeToolName(
				msg.ToolCalls[i].Function.Name,
			)
		}

		// daftar semua langkah di giliran ini dulu (biar checklist langsung penuh)
		for _, tc0 := range msg.ToolCalls {
			var a0 map[string]interface{}
			json.Unmarshal([]byte(tc0.Function.Arguments), &a0)
			agentEmit("plan", tc0.Function.Name, agentLabel(tc0.Function.Name, a0, cwd), "")
		}

		for _, tc := range msg.ToolCalls {
			var args map[string]interface{}
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			stepLabel := agentLabel(tc.Function.Name, args, cwd)
			agentEmit("start", tc.Function.Name, stepLabel, "")

			shortArgs := tc.Function.Arguments
			if len(shortArgs) > 60 {
				shortArgs = shortArgs[:60] + "..."
			}

			result := agentExecTool(tc.Function.Name, args, cwd)
			if rawPath, ok := args["path"].(string); ok {
				trackedPath := resolveTrackedPath(rawPath)
				if trackedPath != "" && !strings.HasPrefix(strings.TrimSpace(result), "[error") {
					switch tc.Function.Name {
					case "write_file":
						if strings.HasPrefix(result, "OK: file selesai dan tervalidasi:") {
							appendTracked(&writtenOrder, writtenFiles, trackedPath)
							appendTracked(&createdOrder, createdFiles, trackedPath)
							writtenFiles[trackedPath] = true
						}
					case "edit_file":
						if strings.HasPrefix(result, "OK: file berhasil diedit dan diverifikasi:") {
							appendTracked(&writtenOrder, writtenFiles, trackedPath)
							appendTracked(&modifiedOrder, modifiedFiles, trackedPath)
							writtenFiles[trackedPath] = true
							verifiedFiles[trackedPath] = true
						}
					case "read_file":
						readFiles[trackedPath] = true
						appendTracked(&inspectedOrder, inspectedFiles, trackedPath)
						if writtenFiles[trackedPath] {
							verifiedFiles[trackedPath] = true
						}
					}
				}
			}
			if okStep, det := agentOutcome(tc.Function.Name, result); okStep {
				agentEmit("ok", tc.Function.Name, stepLabel, det)
			} else {
				agentEmit("fail", tc.Function.Name, stepLabel, det)
			}

			// Compact summary per tool (Codex-style)
			var summary string
			switch tc.Function.Name {
			case "read_file":
				lines := strings.Count(result, "\n") + 1
				if strings.HasPrefix(result, "[error") {
					summary = result
				} else {
					summary = fmt.Sprintf("%d baris, %d byte", lines, len(result))
				}
			case "write_file":
				summary = result
			case "list_dir":
				if strings.HasPrefix(result, "[error") {
					summary = result
				} else {
					n := 0
					for _, ln := range strings.Split(result, "\n") {
						if strings.TrimSpace(ln) != "" {
							n++
						}
					}
					summary = fmt.Sprintf("%d item", n)
				}
			case "run_command":
				if len(result) > 200 {
					summary = result[:200] + "..."
				} else {
					summary = result
				}
			case "run_in_container":
				if len(result) > 200 {
					summary = result[:200] + "..."
				} else {
					summary = result
				}
			default:
				summary = result
			}

			summary = strings.TrimSpace(summary)
			log.WriteString(fmt.Sprintf("  `%s` %s\n", tc.Function.Name, summary))

			messages = append(messages, agentMsg{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			})
		}
	}

	verifyPendingFiles("batas langkah tercapai")
	log.WriteString(fmt.Sprintf("\n**Max langkah tercapai (%d).**", maxSteps))
	writeAgentFileSummary()
	return log.String(), nil
}
