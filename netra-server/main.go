package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultPort    = "8080"
	termuxWorkdir  = "/data/data/com.termux/files/home/netra-ai"
	maxReadBytes   = 2 << 20 // 2 MB
	maxWriteBytes  = 8 << 20 // 8 MB
	maxRunOutput   = 64 << 10
	runTimeout     = 60 * time.Second
	tokenHeader    = "X-Netra-Token"
	tokenMarkerStr = "__NETRA_TOKEN__"
)

var (
	token    string
	workDir  string // path asli folder kerja (symlink sudah di-resolve)
	hostOK   map[string]bool
	allowed  = map[string]bool{}
	runAllow = map[string]bool{"go": true, "lua": true, "gofmt": true}
)

func init() {
	exts := []string{
		".go", ".lua", ".py", ".html", ".htm", ".css", ".js", ".java", ".sh", ".bash",
		".c", ".cpp", ".h", ".hpp", ".rs", ".rb", ".php", ".ts", ".jsx", ".tsx",
		".json", ".md", ".txt", ".xml", ".yaml", ".yml", ".toml", ".ini", ".conf",
		".mod", ".sum", ".gitignore", ".csv", ".sql",
	}
	for _, e := range exts {
		allowed[e] = true
	}
}

// ---------- util ----------

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func newToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// resolve mengubah path relatif dari UI jadi path absolut di dalam workDir.
// ".." dibuang secara leksikal, symlink yang keluar dari workDir ditolak,
// dan folder .git tidak boleh disentuh.
func resolve(rel string) (string, error) {
	if strings.ContainsRune(rel, 0) {
		return "", errors.New("path tidak valid")
	}
	clean := filepath.Join(string(filepath.Separator), filepath.FromSlash(rel))
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part == ".git" {
			return "", errors.New("folder .git tidak boleh diakses")
		}
	}
	full := filepath.Join(workDir, clean)

	probe := full
	for {
		if _, err := os.Lstat(probe); err == nil {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			break
		}
		probe = parent
	}
	real, err := filepath.EvalSymlinks(probe)
	if err != nil {
		return "", err
	}
	if real != workDir && !strings.HasPrefix(real, workDir+string(filepath.Separator)) {
		return "", errors.New("path di luar folder project")
	}
	return full, nil
}

func extOK(name string) bool {
	return allowed[strings.ToLower(filepath.Ext(name))]
}

// ---------- middleware ----------

func guard(next http.HandlerFunc, method string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hostOK[r.Host] {
			writeErr(w, http.StatusForbidden, "host tidak diizinkan")
			return
		}
		got := r.Header.Get(tokenHeader)
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			writeErr(w, http.StatusUnauthorized, "token salah atau kadaluarsa, muat ulang halaman")
			return
		}
		if method != "" && r.Method != method {
			writeErr(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
			return
		}
		next(w, r)
	}
}

// ---------- handlers ----------

type entry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
	Ext  string `json:"ext"`
}

func filesHandler(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("d")
	dir, err := resolve(rel)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	out := []entry{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			if name == ".git" {
				continue
			}
			out = append(out, entry{Name: name, Dir: true})
			continue
		}
		if !extOK(name) {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		out = append(out, entry{Name: name, Size: info.Size(), Ext: strings.ToLower(filepath.Ext(name))})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"dir":     filepath.ToSlash(strings.TrimPrefix(rel, "/")),
		"root":    filepath.Base(workDir),
		"entries": out,
	})
}

func readHandler(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("p")
	if rel == "" {
		writeErr(w, http.StatusBadRequest, "parameter p kosong")
		return
	}
	if !extOK(rel) {
		writeErr(w, http.StatusBadRequest, "tipe file tidak didukung")
		return
	}
	path, err := resolve(rel)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if info.IsDir() {
		writeErr(w, http.StatusBadRequest, "ini folder, bukan file")
		return
	}
	if info.Size() > maxReadBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "file terlalu besar buat dibuka di editor (maks 2 MB)")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"content": string(data)})
}

func writeHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxWriteBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Name == "" || !extOK(body.Name) {
		writeErr(w, http.StatusBadRequest, "nama atau tipe file tidak valid")
		return
	}
	path, err := resolve(body.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.WriteFile(path, []byte(body.Content), 0644); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func newfileHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxWriteBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "nama kosong")
		return
	}
	if !extOK(body.Name) {
		writeErr(w, http.StatusBadRequest, "ekstensi tidak didukung, contoh: baru.go, catatan.md")
		return
	}
	path, err := resolve(body.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := os.Stat(path); err == nil {
		writeErr(w, http.StatusConflict, "file udah ada")
		return
	}
	if err := os.WriteFile(path, []byte(body.Content), 0644); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "path": body.Name})
}

func mkdirHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeErr(w, http.StatusBadRequest, "nama kosong")
		return
	}
	path, err := resolve(body.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "path": body.Name})
}

type capBuf struct {
	buf   bytes.Buffer
	max   int
	trunc bool
}

func (c *capBuf) Write(p []byte) (int, error) {
	room := c.max - c.buf.Len()
	if room <= 0 {
		c.trunc = true
		return len(p), nil
	}
	if len(p) > room {
		c.buf.Write(p[:room])
		c.trunc = true
		return len(p), nil
	}
	return c.buf.Write(p)
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Cmd string `json:"cmd"`
		Dir string `json:"dir"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	parts := splitArgs(body.Cmd)
	if len(parts) == 0 || !runAllow[parts[0]] {
		writeErr(w, http.StatusForbidden, "command tidak diizinkan, cuma: go, lua, gofmt")
		return
	}
	dir, err := resolve(body.Dir)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = dir
	cmd.WaitDelay = 2 * time.Second
	cb := &capBuf{max: maxRunOutput}
	cmd.Stdout = cb
	cmd.Stderr = cb
	runErr := cmd.Run()

	out := cb.buf.String()
	if cb.trunc {
		out += "\n[output dipotong di 64 KB]"
	}
	msg := ""
	if runErr != nil {
		msg = runErr.Error()
		if ctx.Err() == context.DeadlineExceeded {
			msg = "dihentikan: lebih dari 60 detik"
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"output": out, "error": msg})
}

func splitArgs(s string) []string {
	var args []string
	var cur strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t') && !inQuote:
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !hostOK[r.Host] {
		http.Error(w, "host tidak diizinkan", http.StatusForbidden)
		return
	}
	page := bytes.Replace(uiHTML, []byte(tokenMarkerStr), []byte(token), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Write(page)
}

func main() {
	port := getenv("NETRA_PORT", defaultPort)
	addr := "127.0.0.1:" + port
	hostOK = map[string]bool{
		"127.0.0.1:" + port: true,
		"localhost:" + port: true,
	}

	token = getenv("NETRA_TOKEN", "")
	if token == "" {
		token = newToken()
	}

	wd := getenv("NETRA_WORKDIR", "")
	if wd == "" {
		if st, err := os.Stat(termuxWorkdir); err == nil && st.IsDir() {
			wd = termuxWorkdir
		} else {
			wd, _ = os.Getwd()
		}
	}
	real, err := filepath.EvalSymlinks(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "folder kerja tidak bisa dibuka:", err)
		os.Exit(1)
	}
	workDir = real

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/api/files", guard(filesHandler, http.MethodGet))
	mux.HandleFunc("/api/read", guard(readHandler, http.MethodGet))
	mux.HandleFunc("/api/write", guard(writeHandler, http.MethodPost))
	mux.HandleFunc("/api/newfile", guard(newfileHandler, http.MethodPost))
	mux.HandleFunc("/api/mkdir", guard(mkdirHandler, http.MethodPost))
	mux.HandleFunc("/api/run", guard(runHandler, http.MethodPost))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
	}

	fmt.Println("Netra server jalan di http://" + addr)
	fmt.Println("Folder kerja:", workDir)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, "server berhenti:", err)
		os.Exit(1)
	}
}
