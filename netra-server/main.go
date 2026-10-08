package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const TOKEN = "rahasia-netra-2026"
const WORKDIR = "/data/data/com.termux/files/home/netra-ai"

func auth(r *http.Request) bool {
	return r.Header.Get("X-Netra-Token") == TOKEN
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func filesHandler(w http.ResponseWriter, r *http.Request) {
	if !auth(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	entries, err := os.ReadDir(WORKDIR)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	allowed := map[string]bool{
		".go": true, ".lua": true, ".py": true, ".html": true, ".htm": true,
		".css": true, ".js": true, ".java": true, ".sh": true, ".bash": true,
		".c": true, ".cpp": true, ".h": true, ".hpp": true, ".rs": true,
		".rb": true, ".php": true, ".ts": true, ".jsx": true, ".tsx": true,
		".json": true, ".md": true, ".txt": true, ".xml": true,
		".yaml": true, ".yml": true, ".toml": true, ".ini": true, ".conf": true,
	}

	type FileInfo struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
		Ext  string `json:"ext"`
	}

	var files []FileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !allowed[ext] {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		files = append(files, FileInfo{Name: e.Name(), Size: info.Size(), Ext: ext})
	}
	writeJSON(w, map[string]interface{}{"files": files, "count": len(files)})
}

func readHandler(w http.ResponseWriter, r *http.Request) {
	if !auth(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	name := r.URL.Query().Get("p")
	if name == "" {
		writeJSON(w, map[string]string{"error": "missing param p"})
		return
	}
	// Cegah path traversal
	if filepath.Base(name) != name {
		writeJSON(w, map[string]string{"error": "invalid path"})
		return
	}
	path := filepath.Join(WORKDIR, name)
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"content": string(data)})
}

func writeHandler(w http.ResponseWriter, r *http.Request) {
	if !auth(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	if filepath.Base(body.Name) != body.Name {
		writeJSON(w, map[string]string{"error": "invalid path"})
		return
	}
	path := filepath.Join(WORKDIR, body.Name)
	if err := os.WriteFile(path, []byte(body.Content), 0644); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	if !auth(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Cmd string `json:"cmd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	// Whitelist command
	allowed := map[string]bool{"go": true, "lua": true, "gofmt": true}
	parts := []string{}
	for _, p := range []string{} {
		_ = p
	}
	// Cek command pertama
	cmdParts := splitArgs(body.Cmd)
	if len(cmdParts) == 0 || !allowed[cmdParts[0]] {
		writeJSON(w, map[string]string{"error": "command not allowed"})
		return
	}
	parts = cmdParts
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = WORKDIR
	out, err := cmd.CombinedOutput()
	writeJSON(w, map[string]interface{}{
		"output": string(out),
		"error":  errString(err),
	})
}

func splitArgs(s string) []string {
	var args []string
	var cur string
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ' ' && !inQuote:
			if cur != "" {
				args = append(args, cur)
				cur = ""
			}
		default:
			cur += string(r)
		}
	}
	if cur != "" {
		args = append(args, cur)
	}
	return args
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func mkdirHandler(w http.ResponseWriter, r *http.Request) {
	if !auth(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	if body.Name == "" {
		writeJSON(w, map[string]string{"error": "nama kosong"})
		return
	}
	// Tolak path naik keluar
	if filepath.IsAbs(body.Name) || filepath.Base(body.Name) != body.Name && !validRelPath(body.Name) {
		writeJSON(w, map[string]string{"error": "invalid path"})
		return
	}
	path := filepath.Join(WORKDIR, body.Name)
	if err := os.MkdirAll(path, 0755); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "path": body.Name})
}

func newfileHandler(w http.ResponseWriter, r *http.Request) {
	if !auth(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	if body.Name == "" {
		writeJSON(w, map[string]string{"error": "nama kosong"})
		return
	}
	path := filepath.Join(WORKDIR, body.Name)
	// Bikin parent folder kalau belum ada
	if dir := filepath.Dir(path); dir != WORKDIR {
		if err := os.MkdirAll(dir, 0755); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
	}
	if _, err := os.Stat(path); err == nil {
		writeJSON(w, map[string]string{"error": "file udah ada"})
		return
	}
	if err := os.WriteFile(path, []byte(body.Content), 0644); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "path": body.Name})
}

func validRelPath(p string) bool {
	// Cegah ../
	return !strings.Contains(p, "..")
}

func main() {
	http.HandleFunc("/api/files", filesHandler)
	http.HandleFunc("/api/read", readHandler)
	http.HandleFunc("/api/write", writeHandler)
	http.HandleFunc("/api/run", runHandler)
	http.HandleFunc("/api/mkdir", mkdirHandler)
	http.HandleFunc("/api/newfile", newfileHandler)
	println("Netra server listening on 127.0.0.1:8080")
	http.ListenAndServe("0.0.0.0:8080", nil)
}
