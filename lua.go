// lua.go — execute Lua scripts dari netra-ai TUI
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func luaHTTPGet(url string) (string, int, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	return string(body), resp.StatusCode, nil
}

func (m *Model) handleLua(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "**Cara pakai:** `/lua <file.lua> [args...]`\n\n**Contoh:**\n- `/lua script.lua`\n- `/lua backup.lua arg1 arg2`\n\n**Yang tersedia:**\n- `print(...)` — output ke chat\n- `args` — table argumen\n- `cwd` — current directory\n- `http_get(url)` — HTTP GET, return (body, status)\n- `get_env(name)` — env var\n- `file_read(path)` — baca file (max 1MB)\n\nLua 5.1 syntax (kompatibel Love2D)."})
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
	if info.Size() > 512*1024 {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "File terlalu gede (>512KB)"})
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Baca file gagal: " + err.Error()})
		return
	}

	L := lua.NewState()
	defer L.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	L.SetContext(ctx)

	var output strings.Builder

	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		for i := 1; i <= n; i++ {
			if i > 1 {
				output.WriteString("\t")
			}
			output.WriteString(L.ToStringMeta(L.Get(i)).String())
		}
		output.WriteString("\n")
		return 0
	}))

	argsTable := L.NewTable()
	for i := 2; i < len(args); i++ {
		argsTable.Append(lua.LString(args[i]))
	}
	L.SetGlobal("args", argsTable)
	L.SetGlobal("cwd", lua.LString(m.currentDir))

	L.SetGlobal("http_get", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)
		body, status, err := luaHTTPGet(url)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LNumber(0))
			L.Push(lua.LString(err.Error()))
			return 3
		}
		L.Push(lua.LString(body))
		L.Push(lua.LNumber(status))
		return 2
	}))

	L.SetGlobal("get_env", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		L.Push(lua.LString(os.Getenv(name)))
		return 1
	}))

	L.SetGlobal("file_read", L.NewFunction(func(L *lua.LState) int {
		p := L.CheckString(1)
		if !filepath.IsAbs(p) {
			p = filepath.Join(m.currentDir, p)
		}
		d, err := os.ReadFile(p)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(d)))
		L.Push(lua.LNil)
		return 2
	}))

	// http_status_many — cek status HTTP paralel (max 25 concurrent).
	// Input: table of URLs. Output: table of status codes (0=error).
	L.SetGlobal("http_status_many", L.NewFunction(func(L *lua.LState) int {
		urlsTable := L.CheckTable(1)
		var urls []string
		urlsTable.ForEach(func(_, v lua.LValue) {
			urls = append(urls, v.String())
		})

		results := make([]int, len(urls))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 25)
		client := &http.Client{Timeout: 12 * time.Second}

		for i, u := range urls {
			wg.Add(1)
			go func(i int, u string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				req, err := http.NewRequest("GET", u, nil)
				if err != nil {
					results[i] = 0
					return
				}
				req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13; SM-S908B) AppleWebKit/537.36")
				resp, err := client.Do(req)
				if err != nil {
					results[i] = 0
					return
				}
				defer resp.Body.Close()
				io.Copy(io.Discard, resp.Body)
				results[i] = resp.StatusCode
			}(i, u)
		}
		wg.Wait()

		tbl := L.NewTable()
		for i, s := range results {
			tbl.RawSetInt(i+1, lua.LNumber(s))
		}
		L.Push(tbl)
		return 1
	}))

	// http_probe_many — cek HTTP paralel, return status + title + size.
	L.SetGlobal("http_probe_many", L.NewFunction(func(L *lua.LState) int {
		urlsTable := L.CheckTable(1)
		var urls []string
		urlsTable.ForEach(func(_, v lua.LValue) {
			urls = append(urls, v.String())
		})

		type Probe struct {
			Status  int
			Title   string
			OgTitle string
			Size    int
			NegHits int
		}
		results := make([]Probe, len(urls))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 20)
		client := &http.Client{Timeout: 15 * time.Second}
		titleRe := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
		tagRe := regexp.MustCompile(`<[^>]+>`)
		ogRe := regexp.MustCompile(`(?is)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)["']`)
		ogRe2 := regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:title["']`)

		negWords := []string{
			"user not found",
			"page not found",
			"profile not found",
			"account not found",
			"not found",
			"doesn't exist",
			"does not exist",
			"page doesn't exist",
			"page unavailable",
			"sorry, this page",
			"no such user",
			"no user",
			"user doesn't exist",
			"account suspended",
			"profile unavailable",
			"this account doesn't exist",
		}

		for i, u := range urls {
			wg.Add(1)
			go func(i int, u string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				req, err := http.NewRequest("GET", u, nil)
				if err != nil {
					results[i] = Probe{Status: 0}
					return
				}
				req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13; SM-S908B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36")
				req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
				req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")

				resp, err := client.Do(req)
				if err != nil {
					results[i] = Probe{Status: 0}
					return
				}
				defer resp.Body.Close()

				body, _ := io.ReadAll(io.LimitReader(resp.Body, 100*1024))
				title := ""
				if m := titleRe.FindSubmatch(body); len(m) > 1 {
					t := tagRe.ReplaceAllString(string(m[1]), "")
					t = strings.TrimSpace(t)
					t = strings.ReplaceAll(t, "&amp;", "&")
					t = strings.ReplaceAll(t, "&#39;", "'")
					t = strings.ReplaceAll(t, "&quot;", "\"")
					t = strings.ReplaceAll(t, "&lt;", "<")
					t = strings.ReplaceAll(t, "&gt;", ">")
					if len(t) > 200 {
						t = t[:200]
					}
					title = t
				}
				ogTitle := ""
				if m := ogRe.FindSubmatch(body); len(m) > 1 {
					ogTitle = strings.TrimSpace(string(m[1]))
				} else if m := ogRe2.FindSubmatch(body); len(m) > 1 {
					ogTitle = strings.TrimSpace(string(m[1]))
				}
				lowBody := strings.ToLower(string(body))
				negHits := 0
				for _, w := range negWords {
					if strings.Contains(lowBody, w) {
						negHits++
					}
				}
				results[i] = Probe{
					Status:  resp.StatusCode,
					Title:   title,
					OgTitle: ogTitle,
					Size:    len(body),
					NegHits: negHits,
				}
			}(i, u)
		}
		wg.Wait()

		tbl := L.NewTable()
		for i, r := range results {
			item := L.NewTable()
			item.RawSetString("status", lua.LNumber(r.Status))
			item.RawSetString("title", lua.LString(r.Title))
			item.RawSetString("og_title", lua.LString(r.OgTitle))
			item.RawSetString("size", lua.LNumber(r.Size))
			item.RawSetString("neg_hits", lua.LNumber(r.NegHits))
			tbl.RawSetInt(i+1, item)
		}
		L.Push(tbl)
		return 1
	}))

	// ===== PROOT-DOCKER (Docker-like untuk Termux) =====

	prootExec := func(args ...string) (string, int) {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "proot-distro", args...)
		out, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				code = -1
			}
		}
		if ctx.Err() == context.DeadlineExceeded {
			code = 124
		}
		return string(out), code
	}

	// proot_run(distro, command) -> output, exit_code
	L.SetGlobal("proot_run", L.NewFunction(func(L *lua.LState) int {
		distro := L.CheckString(1)
		command := L.CheckString(2)
		out, code := prootExec("login", distro, "--", "bash", "-c", command)
		L.Push(lua.LString(out))
		L.Push(lua.LNumber(code))
		return 2
	}))

	// proot_install(distro) -> output, exit_code
	L.SetGlobal("proot_install", L.NewFunction(func(L *lua.LState) int {
		distro := L.CheckString(1)
		out, code := prootExec("install", distro)
		L.Push(lua.LString(out))
		L.Push(lua.LNumber(code))
		return 2
	}))

	// proot_remove(distro) -> output, exit_code
	L.SetGlobal("proot_remove", L.NewFunction(func(L *lua.LState) int {
		distro := L.CheckString(1)
		out, code := prootExec("remove", "--force", distro)
		L.Push(lua.LString(out))
		L.Push(lua.LNumber(code))
		return 2
	}))

	// proot_list() -> table of distro names
	L.SetGlobal("proot_list", L.NewFunction(func(L *lua.LState) int {
		prefix := os.Getenv("PREFIX")
		if prefix == "" {
			prefix = "/data/data/com.termux/files/usr"
		}
		containersDir := filepath.Join(prefix, "var", "lib", "proot-distro", "containers")
		entries, err := os.ReadDir(containersDir)
		tbl := L.NewTable()
		if err != nil {
			L.Push(tbl)
			return 1
		}
		i := 1
		for _, e := range entries {
			if e.IsDir() {
				tbl.RawSetInt(i, lua.LString(e.Name()))
				i++
			}
		}
		L.Push(tbl)
		return 1
	}))

	// ===== TCP SOCKET functions (buat port scan / banner grab) =====

	// tcp_connect(host, port, timeout_ms) → boolean
	L.SetGlobal("tcp_connect", L.NewFunction(func(L *lua.LState) int {
		host := L.CheckString(1)
		port := L.CheckInt(2)
		timeoutMs := L.OptInt(3, 1000)

		addr := net.JoinHostPort(host, strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, time.Duration(timeoutMs)*time.Millisecond)
		if err != nil {
			L.Push(lua.LFalse)
			return 1
		}
		conn.Close()
		L.Push(lua.LTrue)
		return 1
	}))

	// tcp_send(host, port, data, timeout_ms) → (response_string, error_string)
	L.SetGlobal("tcp_send", L.NewFunction(func(L *lua.LState) int {
		host := L.CheckString(1)
		port := L.CheckInt(2)
		data := L.CheckString(3)
		timeoutMs := L.OptInt(4, 2000)

		addr := net.JoinHostPort(host, strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, time.Duration(timeoutMs)*time.Millisecond)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))

		if _, err := conn.Write([]byte(data)); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil && n == 0 {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(buf[:n])))
		L.Push(lua.LNil)
		return 2
	}))

	// tcp_grab(host, port, timeout_ms) → banner_string (atau nil)
	L.SetGlobal("tcp_grab", L.NewFunction(func(L *lua.LState) int {
		host := L.CheckString(1)
		port := L.CheckInt(2)
		timeoutMs := L.OptInt(3, 2000)

		addr := net.JoinHostPort(host, strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, time.Duration(timeoutMs)*time.Millisecond)
		if err != nil {
			L.Push(lua.LNil)
			return 1
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil && n == 0 {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LString(string(buf[:n])))
		return 1
	}))

	// http_request(method, url, headers_table, body, timeout_ms) → (status, headers_string, body_string)
	L.SetGlobal("http_request", L.NewFunction(func(L *lua.LState) int {
		method := L.CheckString(1)
		urlStr := L.CheckString(2)
		headersTable := L.OptTable(3, nil)
		body := L.OptString(4, "")
		timeoutMs := L.OptInt(5, 10000)

		var bodyReader io.Reader
		if body != "" {
			bodyReader = strings.NewReader(body)
		}
		req, err := http.NewRequest(method, urlStr, bodyReader)
		if err != nil {
			L.Push(lua.LNumber(0))
			L.Push(lua.LString(""))
			L.Push(lua.LString(err.Error()))
			return 3
		}
		if headersTable != nil {
			headersTable.ForEach(func(k, v lua.LValue) {
				req.Header.Set(k.String(), v.String())
			})
		}
		client := &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}
		resp, err := client.Do(req)
		if err != nil {
			L.Push(lua.LNumber(0))
			L.Push(lua.LString(""))
			L.Push(lua.LString(err.Error()))
			return 3
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		headersStr := ""
		for k, vals := range resp.Header {
			for _, v := range vals {
				headersStr += k + ": " + v + "\n"
			}
		}
		L.Push(lua.LNumber(resp.StatusCode))
		L.Push(lua.LString(headersStr))
		L.Push(lua.LString(string(respBody)))
		return 3
	}))

	// sleep(ms) — jeda
	L.SetGlobal("sleep", L.NewFunction(func(L *lua.LState) int {
		ms := L.CheckInt(1)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return 0
	}))

	// scan_ports(host, start, end, workers?, timeout_ms?) — PARALEL via goroutine
	L.SetGlobal("scan_ports", L.NewFunction(func(L *lua.LState) int {
		host := L.CheckString(1)
		start := L.CheckInt(2)
		endp := L.CheckInt(3)
		workers := L.OptInt(4, 100)
		timeoutMs := L.OptInt(5, 500)

		if start < 1 {
			start = 1
		}
		if endp > 65535 {
			endp = 65535
		}
		if start > endp {
			start, endp = endp, start
		}
		if workers < 1 {
			workers = 100
		}
		if workers > 500 {
			workers = 500
		}

		portsChan := make(chan int, workers)
		var openPorts []int
		var mu sync.Mutex
		var wg sync.WaitGroup
		timeout := time.Duration(timeoutMs) * time.Millisecond

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for port := range portsChan {
					addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
					conn, err := net.DialTimeout("tcp", addr, timeout)
					if err != nil {
						continue
					}
					conn.Close()
					mu.Lock()
					openPorts = append(openPorts, port)
					mu.Unlock()
				}
			}()
		}

		for prt := start; prt <= endp; prt++ {
			portsChan <- prt
		}
		close(portsChan)
		wg.Wait()

		sort.Ints(openPorts)

		tbl := L.NewTable()
		for i, prt := range openPorts {
			tbl.RawSetInt(i+1, lua.LNumber(prt))
		}
		L.Push(tbl)
		return 1
	}))

	// Hardening dasar
	L.SetGlobal("dofile", lua.LNil)
	L.SetGlobal("loadfile", lua.LNil)
	if osLib, ok := L.GetGlobal("os").(*lua.LTable); ok {
		osLib.RawSetString("execute", lua.LNil)
		osLib.RawSetString("exit", lua.LNil)
		osLib.RawSetString("remove", lua.LNil)
		osLib.RawSetString("rename", lua.LNil)
	}

	start := time.Now()
	err = L.DoString(string(data))
	elapsed := time.Since(start)

	var result strings.Builder
	result.WriteString(fmt.Sprintf("**Lua** `%s` (%.2fs)\n\n", filepath.Base(path), elapsed.Seconds()))

	if err != nil {
		result.WriteString("**Error:**\n```\n")
		result.WriteString(err.Error())
		result.WriteString("\n```\n\n")
	}

	out := strings.TrimRight(output.String(), "\n")
	if out != "" {
		result.WriteString("**Output:**\n```\n")
		result.WriteString(out)
		result.WriteString("\n```")
	} else if err == nil {
		result.WriteString("_(tidak ada output)_")
	}

	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: result.String()})
}
