// plugins.go — routing natural language ke file .lua plugin.
// Contoh: "check crypto btc" -> jalankan crypto.lua dengan arg "btc".
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// Plugin — definisi 1 plugin Lua.
type Plugin struct {
	Name     string   // identifier
	File     string   // nama file .lua
	Triggers []string // frasa pemicu (lowercase)
	Usage    string   // contoh cara pakai
}

// Daftar plugin yang aktif. Tambah di sini kalau bikin plugin baru.
var pluginRegistry = []Plugin{
	{
		Name:     "crypto",
		File:     "crypto.lua",
		Triggers: []string{"check crypto", "cek crypto", "harga crypto", "harga btc", "harga bitcoin", "harga ethereum", "harga eth"},
		Usage:    "check crypto <btc|eth|sol|...>",
	},
	{
		Name:     "cuaca",
		File:     "cuaca.lua",
		Triggers: []string{"cek cuaca", "check cuaca", "cuaca sekarang"},
		Usage:    "cek cuaca <kota>",
	},
	{
		Name:     "gempa",
		File:     "gempa.lua",
		Triggers: []string{"cek gempa", "info gempa", "gempa terbaru"},
		Usage:    "cek gempa",
	},
	{
		Name:     "ipinfo",
		File:     "ipinfo.lua",
		Triggers: []string{"cek ip", "info ip", "lokasi ip"},
		Usage:    "cek ip <ip>",
	},
	{
		Name:     "github",
		File:     "github.lua",
		Triggers: []string{"cek github", "github user", "profil github"},
		Usage:    "cek github <username>",
	},
}

// hasPluginMatch — cek apakah input match trigger plugin (tanpa eksekusi).
// Dipakai buat mencegah search engine rebutan sama plugin.
func hasPluginMatch(input string) bool {
	low := strings.ToLower(strings.TrimSpace(input))
	for _, p := range pluginRegistry {
		for _, trig := range p.Triggers {
			idx := strings.Index(low, trig)
			if idx < 0 {
				continue
			}
			rest := strings.TrimSpace(input[idx+len(trig):])
			multiWord := strings.Contains(trig, " ")
			if !multiWord && rest == "" {
				continue
			}
			// File plugin harus ada
			home, _ := os.UserHomeDir()
			path := filepath.Join(home, "netra-ai", p.File)
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}
	}
	return false
}

// tryPlugin — deteksi apakah input match salah satu plugin.
// Return (output, true) kalau match, ("", false) kalau tidak.
func tryPlugin(input string) (string, bool) {
	low := strings.ToLower(strings.TrimSpace(input))

	for _, p := range pluginRegistry {
		for _, trig := range p.Triggers {
			idx := strings.Index(low, trig)
			if idx < 0 {
				continue
			}

			// Ambil rest setelah trigger
			rest := strings.TrimSpace(input[idx+len(trig):])

			// Aturan: trigger 1 kata butuh argumen, trigger multi-kata bebas
			multiWord := strings.Contains(trig, " ")
			if !multiWord && rest == "" {
				continue // biar gak false positive (e.g. "aku suka crypto")
			}

			// Bangun args
			var pluginArgs []string
			if rest != "" {
				pluginArgs = strings.Fields(rest)
			}

			// Cari file plugin di folder project
			home, _ := os.UserHomeDir()
			path := filepath.Join(home, "netra-ai", p.File)
			if _, err := os.Stat(path); err != nil {
				continue
			}

			output, err := runPluginFile(path, pluginArgs)
			if err != nil {
				return fmt.Sprintf("Plugin `%s` error: %v", p.Name, err), true
			}
			if strings.TrimSpace(output) == "" {
				output = "(plugin gak ada output)"
			}
			return output, true
		}
	}
	return "", false
}

// runPluginFile — eksekusi file Lua, return output sebagai string.
// Environment: print, args, cwd, http_get, get_env, file_read.
func runPluginFile(path string, args []string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	L := lua.NewState()
	defer L.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	for _, a := range args {
		argsTable.Append(lua.LString(a))
	}
	L.SetGlobal("args", argsTable)

	home, _ := os.UserHomeDir()
	L.SetGlobal("cwd", lua.LString(home))

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

	fn, err := L.LoadString(string(data))
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}
	L.Push(fn)
	if err := L.PCall(0, lua.MultRet, nil); err != nil {
		return "", fmt.Errorf("exec error: %w", err)
	}

	return output.String(), nil
}
