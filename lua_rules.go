// lua_rules.go — Lua Rules Engine buat Eliza
// File .lua di ~/.netra-ai/lua-rules/ bisa override reply Eliza
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
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
func matchLuaRules(input string) (string, bool) {
	if len(luaRules) == 0 {
		return "", false
	}

	for _, rule := range luaRules {
		reply, matched := evalLuaRule(rule, input)
		if matched {
			return reply, true
		}
	}
	return "", false
}

// evalLuaRule — load 1 rule, panggil match() + reply()
func evalLuaRule(rule LuaRule, input string) (string, bool) {
	L := lua.NewState()
	defer L.Close()

	// Timeout via context (max 2 detik per rule)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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

	// Load file
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
