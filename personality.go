// personality.go — sistem personality buat Eliza.
// Tiap personality = 1 file .lua di ~/.netra-ai/personalities/
// Command: /persona <nama> buat ganti, /persona buat list.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type Personality struct {
	Name      string
	Desc      string
	L         *lua.LState
	transform *lua.LFunction
	greeting  *lua.LFunction
}

var activePersonality *Personality

func personalityDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".netra-ai", "personalities")
}

func loadPersonality(name string) (*Personality, error) {
	dir := personalityDir()
	path := filepath.Join(dir, name+".lua")
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("personality '%s' gak ada", name)
	}

	L := lua.NewState()

	if err := L.DoFile(path); err != nil {
		L.Close()
		return nil, fmt.Errorf("gagal load: %w", err)
	}

	p := &Personality{Name: name, L: L}
	if v := L.GetGlobal("personality_name"); v.Type() == lua.LTString {
		p.Name = v.String()
	}
	if v := L.GetGlobal("personality_desc"); v.Type() == lua.LTString {
		p.Desc = v.String()
	}
	if v := L.GetGlobal("transform_reply"); v.Type() == lua.LTFunction {
		p.transform = v.(*lua.LFunction)
	}
	if v := L.GetGlobal("greeting"); v.Type() == lua.LTFunction {
		p.greeting = v.(*lua.LFunction)
	}
	return p, nil
}

func (p *Personality) Transform(input, reply string) string {
	if p.transform == nil {
		return ""
	}
	L := p.L
	L.Push(p.transform)
	L.Push(lua.LString(input))
	L.Push(lua.LString(reply))
	if err := L.PCall(2, 1, nil); err != nil {
		return ""
	}
	ret := L.Get(-1)
	L.Pop(1)
	if ret.Type() == lua.LTString {
		return ret.String()
	}
	return ""
}

func (p *Personality) Greeting() string {
	if p.greeting == nil {
		return ""
	}
	L := p.L
	L.Push(p.greeting)
	if err := L.PCall(0, 1, nil); err != nil {
		return ""
	}
	ret := L.Get(-1)
	L.Pop(1)
	if ret.Type() == lua.LTString {
		return ret.String()
	}
	return ""
}

func (p *Personality) Close() {
	if p.L != nil {
		p.L.Close()
	}
}

func listPersonalities() []string {
	entries, err := os.ReadDir(personalityDir())
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".lua") {
			out = append(out, strings.TrimSuffix(e.Name(), ".lua"))
		}
	}
	return out
}

func setPersonality(name string) error {
	if name == "" || name == "default" || name == "off" {
		if activePersonality != nil {
			activePersonality.Close()
		}
		activePersonality = nil
		return nil
	}
	p, err := loadPersonality(name)
	if err != nil {
		return err
	}
	if activePersonality != nil {
		activePersonality.Close()
	}
	activePersonality = p
	return nil
}
