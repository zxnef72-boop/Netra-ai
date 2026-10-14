// lua_rules.go — Lua Rules Engine buat Eliza
// File .lua di ~/.netra-ai/lua-rules/ bisa override reply Eliza
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// State store persist antar panggilan rule
var (
	luaStateStore   = make(map[string]string)
	luaStateStoreMu sync.Mutex
)

// LuaRule — 1 file rule
type LuaRule struct {
	Name string
	Path string
}

var luaRules []LuaRule

// luaRulesDir — lokasi folder rules
func luaRulesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".netra-ai", "lua-rules")
}

// loadLuaRules — scan folder, cache daftar file
func loadLuaRules() int {
	dir := luaRulesDir()
	if dir == "" {
		return 0
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	luaRules = nil
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".lua") {
			continue
		}
		luaRules = append(luaRules, LuaRule{
			Name: strings.TrimSuffix(e.Name(), ".lua"),
			Path: filepath.Join(dir, e.Name()),
		})
	}
	return len(luaRules)
}

// matchLuaRules — iterasi semua rules, cek match()
// Return (reply, true) kalo ada yang match
func matchLuaRules(input string, inputRaw string) (string, bool) {
	if len(luaRules) == 0 {
		return "", false
	}

	for _, rule := range luaRules {
		reply, matched := evalLuaRule(rule, input, inputRaw)
		if matched {
			return reply, true
		}
	}
	return "", false
}

// evalLuaRule — load 1 rule, panggil match() + reply()
func evalLuaRule(rule LuaRule, input string, inputRaw string) (string, bool) {
	if inputRaw == "" {
		inputRaw = input
	}
	L := lua.NewState()
	defer L.Close()

	// Timeout via context (max 2 detik per rule)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	L.SetContext(ctx)

	// Sandbox dasar
	L.SetGlobal("dofile", lua.LNil)
	L.SetGlobal("loadfile", lua.LNil)
	L.SetGlobal("require", lua.LNil)

	// Set context global
	now := time.Now()
	greeting := "malam"
	switch {
	case now.Hour() >= 4 && now.Hour() < 11:
		greeting = "pagi"
	case now.Hour() >= 11 && now.Hour() < 15:
		greeting = "siang"
	case now.Hour() >= 15 && now.Hour() < 19:
		greeting = "sore"
	}

	ctxTable := L.NewTable()
	ctxTable.RawSetString("last_entity", lua.LString(getLastEntity()))
	ctxTable.RawSetString("last_entity_type", lua.LString(getLastEntityType()))
	ctxTable.RawSetString("last_topic", lua.LString(elizaMemory.LastTopic))
	ctxTable.RawSetString("mood", lua.LString(elizaMemory.Mood))
	ctxTable.RawSetString("nama_user", lua.LString(elizaMemory.Nama))
	ctxTable.RawSetString("hour", lua.LNumber(now.Hour()))
	ctxTable.RawSetString("greeting", lua.LString(greeting))
	L.SetGlobal("ctx", ctxTable)

	// Set input
	L.SetGlobal("input", lua.LString(input))
	L.SetGlobal("input_lower", lua.LString(strings.ToLower(input)))
	L.SetGlobal("input_raw", lua.LString(inputRaw))
	L.SetGlobal("input_raw_lower", lua.LString(strings.ToLower(inputRaw)))

	// Load file
	// http_get(url) -> body, status, err
	L.SetGlobal("http_get", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LNumber(0))
			L.Push(lua.LString(err.Error()))
			return 3
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		L.Push(lua.LString(string(body)))
		L.Push(lua.LNumber(resp.StatusCode))
		L.Push(lua.LNil)
		return 3
	}))

	// http_post(url, body, headers_tbl) -> body, status, err
	L.SetGlobal("http_post", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)
		body := L.OptString(2, "")
		headersTbl := L.OptTable(3, nil)

		req, err := http.NewRequest("POST", url, bytes.NewBufferString(body))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LNumber(0))
			L.Push(lua.LString(err.Error()))
			return 3
		}
		req.Header.Set("Content-Type", "application/json")
		if headersTbl != nil {
			headersTbl.ForEach(func(k, v lua.LValue) {
				req.Header.Set(k.String(), v.String())
			})
		}
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LNumber(0))
			L.Push(lua.LString(err.Error()))
			return 3
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		L.Push(lua.LString(string(respBody)))
		L.Push(lua.LNumber(resp.StatusCode))
		L.Push(lua.LNil)
		return 3
	}))

	// json_parse(str) -> table, err
	L.SetGlobal("json_parse", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		var v interface{}
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(goValueToLua(L, v))
		L.Push(lua.LNil)
		return 2
	}))

	// json_encode(tbl) -> string
	L.SetGlobal("json_encode", L.NewFunction(func(L *lua.LState) int {
		v := L.Get(1)
		goV := luaValueToGo(v)
		b, err := json.Marshal(goV)
		if err != nil {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(string(b)))
		return 1
	}))

	// file_read(path) -> content, err
	L.SetGlobal("file_read", L.NewFunction(func(L *lua.LState) int {
		p := L.CheckString(1)
		home, _ := os.UserHomeDir()
		if !filepath.IsAbs(p) {
			p = filepath.Join(home, ".netra-ai", p)
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

	// file_write(path, content) -> ok_bool, err
	L.SetGlobal("file_write", L.NewFunction(func(L *lua.LState) int {
		p := L.CheckString(1)
		c := L.CheckString(2)
		home, _ := os.UserHomeDir()
		if !filepath.IsAbs(p) {
			p = filepath.Join(home, ".netra-ai", p)
		}
		os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, []byte(c), 0644); err != nil {
			L.Push(lua.LFalse)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		L.Push(lua.LNil)
		return 2
	}))

	// file_append(path, content) -> ok_bool, err
	L.SetGlobal("file_append", L.NewFunction(func(L *lua.LState) int {
		p := L.CheckString(1)
		c := L.CheckString(2)
		home, _ := os.UserHomeDir()
		if !filepath.IsAbs(p) {
			p = filepath.Join(home, ".netra-ai", p)
		}
		os.MkdirAll(filepath.Dir(p), 0755)
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			L.Push(lua.LFalse)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		defer f.Close()
		if _, err := f.WriteString(c); err != nil {
			L.Push(lua.LFalse)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		L.Push(lua.LNil)
		return 2
	}))

	// date_today() -> "2026-10-14"
	// time_hm() -> "13:45"
	L.SetGlobal("date_today", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(time.Now().Format("2006-01-02")))
		return 1
	}))
	L.SetGlobal("time_hm", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(time.Now().Format("15:04")))
		return 1
	}))

	// file_exists(path) -> bool
	L.SetGlobal("file_exists", L.NewFunction(func(L *lua.LState) int {
		p := L.CheckString(1)
		home, _ := os.UserHomeDir()
		if !filepath.IsAbs(p) {
			p = filepath.Join(home, ".netra-ai", p)
		}
		if _, err := os.Stat(p); err == nil {
			L.Push(lua.LTrue)
		} else {
			L.Push(lua.LFalse)
		}
		return 1
	}))

	// state_get(key) -> string ("" kalau gak ada)
	L.SetGlobal("state_get", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		luaStateStoreMu.Lock()
		v := luaStateStore[rule.Name+":"+key]
		luaStateStoreMu.Unlock()
		L.Push(lua.LString(v))
		return 1
	}))

	// state_set(key, value)
	L.SetGlobal("state_set", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		val := L.CheckString(2)
		luaStateStoreMu.Lock()
		luaStateStore[rule.Name+":"+key] = val
		luaStateStoreMu.Unlock()
		return 0
	}))

	// state_del(key)
	L.SetGlobal("state_del", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		luaStateStoreMu.Lock()
		delete(luaStateStore, rule.Name+":"+key)
		luaStateStoreMu.Unlock()
		return 0
	}))

	if err := L.DoFile(rule.Path); err != nil {
		return "", false
	}

	// Panggil match(input)
	matchFn := L.GetGlobal("match")
	if matchFn.Type() != lua.LTFunction {
		return "", false
	}
	if err := L.CallByParam(lua.P{
		Fn:      matchFn,
		NRet:    1,
		Protect: true,
	}, lua.LString(input)); err != nil {
		return "", false
	}
	matchRet := L.Get(-1)
	L.Pop(1)
	if matchRet.Type() != lua.LTBool || !lua.LVAsBool(matchRet) {
		return "", false
	}

	// Panggil reply(input, ctx)
	replyFn := L.GetGlobal("reply")
	if replyFn.Type() != lua.LTFunction {
		return "", false
	}
	if err := L.CallByParam(lua.P{
		Fn:      replyFn,
		NRet:    1,
		Protect: true,
	}, lua.LString(input), ctxTable); err != nil {
		return "", false
	}
	replyRet := L.Get(-1)
	L.Pop(1)
	if replyRet.Type() != lua.LTString {
		return "", false
	}

	return replyRet.String(), true
}
