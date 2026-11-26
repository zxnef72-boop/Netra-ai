package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func GetLuaStatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".netra-ai", "lua_state.json")
}

func LoadLuaState() {
	data, err := os.ReadFile(GetLuaStatePath())
	if err != nil {
		return
	}
	luaStateStoreMu.Lock()
	defer luaStateStoreMu.Unlock()
	json.Unmarshal(data, &luaStateStore)
}

func SaveLuaState() {
	data, err := json.MarshalIndent(luaStateStore, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(GetLuaStatePath(), data, 0644)
}
