// agent.go — AI agent yang bisa baca/edit file + jalanin command.
// Terpisah dari Eliza. Butuh LLM yang support tool calling.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
			Name:        "write_file",
			Description: "Tulis konten ke file (overwrite kalau ada).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string"},
					"content": map[string]interface{}{"type": "string"},
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
	}
}

// agentAllowWrite — false secara default. Harus di-set true eksplisit.
var agentAllowWrite = false

// agentWorkspace — folder yang boleh diakses agent. Default /sdcard/AiNef.
var agentWorkspace = "/sdcard/AiNef"

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

func agentExecTool(name string, args map[string]interface{}, cwd string) string {
	// Safety: tool bahaya butuh izin
	if (name == "write_file" || name == "run_command") && !agentAllowWrite {
		return "[DIBLOKIR: tool '" + name + "' butuh izin. Jalankan /agent-unsafe on dulu]"
	}

	// Cek path restriction untuk read/write/list
	if name == "read_file" || name == "write_file" || name == "list_dir" {
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

	case "write_file":
		path, _ := args["path"].(string)
		// Handle content: bisa string, atau object {value: "..."}
		var content string
		switch v := args["content"].(type) {
		case string:
			content = v
		case map[string]interface{}:
			// Model kadang kirim schema, ambil value-nya
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
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Sprintf("[error tulis file: %v]", err)
		}
		return fmt.Sprintf("OK: file ditulis %s (%d byte)", path, len(content))

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

func cmdAgentRun(userPrompt string, cwd string, provider Provider) (string, error) {
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
		{Role: "system", Content: "Kamu adalah Netra Agent. Bisa baca file (read_file), edit file (write_file), list folder (list_dir), dan jalanin command (run_command). Pakai tools kalau perlu. Kalau sudah cukup, jawab langsung tanpa tools. Jawab singkat dalam Bahasa Indonesia."},
		{Role: "user", Content: userPrompt},
	}

	client := &http.Client{Timeout: 3 * time.Minute}
	maxSteps := 8

	// Loop detection: simpen signature tool call terakhir
	lastCallSig := ""
	repeatCount := 0

	for step := 1; step <= maxSteps; step++ {
		reqBody := map[string]interface{}{
			"model":       provider.Model,
			"messages":    messages,
			"tools":       toolsJSON,
			"temperature": 0.3,
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

		if msg.Content != "" {
			log.WriteString(fmt.Sprintf("**[Langkah %d]** %s\n\n", step, msg.Content))
		}

		if len(msg.ToolCalls) == 0 {
			if msg.Content == "" {
				log.WriteString("_(selesai tanpa output)_")
			}
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
			if repeatCount >= 3 {
				log.WriteString("\n**Loop terdeteksi — agent panggil tool yang sama 3x.**\n")
				log.WriteString("Model mungkin gak bisa selesai. Coba model lebih gede.\n")
				return log.String(), nil
			}
		} else {
			lastCallSig = callSig
			repeatCount = 0
		}

		for _, tc := range msg.ToolCalls {
			var args map[string]interface{}
			json.Unmarshal([]byte(tc.Function.Arguments), &args)

			shortArgs := tc.Function.Arguments
			if len(shortArgs) > 60 {
				shortArgs = shortArgs[:60] + "..."
			}

			result := agentExecTool(tc.Function.Name, args, cwd)

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

	log.WriteString("\n**Max langkah tercapai (8).**")
	return log.String(), nil
}
