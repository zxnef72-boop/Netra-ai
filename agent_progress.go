// agent_progress.go — event progres agent (plan/start/ok/fail) supaya TUI bisa
// nampilin checklist langkah secara live, bukan cuma log di akhir.
package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// agentHook di-set sama handleAgent selama agent jalan. nil = gak ada yang dengerin.
var agentHook func(kind, tool, label, detail string)

func agentEmit(kind, tool, label, detail string) {
	if agentHook != nil {
		agentHook(kind, tool, label, detail)
	}
}

const (
	stepPending = 0
	stepRunning = 1
	stepOK      = 2
	stepFail    = 3
)

type agentStep struct {
	Tool   string
	Label  string
	Detail string
	Status int
}

// agentLabel — teks singkat yang menjelaskan apa yang dikerjain tool call ini.
func agentLabel(tool string, args map[string]interface{}, cwd string) string {
	var s string
	switch tool {
	case "read_file", "write_file", "edit_file", "list_dir":
		s, _ = args["path"].(string)
		if s != "" && filepath.IsAbs(s) {
			if rel, err := filepath.Rel(cwd, s); err == nil && !strings.HasPrefix(rel, "..") {
				s = rel
			}
		}
	case "run_command":
		s, _ = args["cmd"].(string)
	case "run_in_container":
		s, _ = args["command"].(string)
	}
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		s = "."
	}
	return s
}

func agentVerb(tool string) string {
	switch tool {
	case "read_file":
		return "baca"
	case "write_file":
		return "tulis"
	case "edit_file":
		return "edit"
	case "list_dir":
		return "lihat"
	case "run_command":
		return "jalan"
	case "run_in_container":
		return "ndock"
	}
	return "tool"
}

func agentSize(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f KB", float64(n)/1024)
}

var agentByteRe = regexp.MustCompile(`\((\d+) byte\)`)

// agentOutcome — berhasil atau gagal, plus keterangan pendek dari hasil tool.
func agentOutcome(tool, result string) (bool, string) {
	r := strings.TrimSpace(result)
	if strings.HasPrefix(r, "[error") || strings.HasPrefix(r, "[DIBLOKIR") || strings.HasPrefix(r, "[exit error") {
		first := strings.TrimSpace(strings.SplitN(r, "\n", 2)[0])
		return false, strings.Trim(first, "[]")
	}
	switch tool {
	case "write_file", "edit_file":
		if mm := agentByteRe.FindStringSubmatch(r); mm != nil {
			n, _ := strconv.Atoi(mm[1])
			return true, agentSize(n)
		}
	case "read_file":
		return true, fmt.Sprintf("%d baris", strings.Count(r, "\n")+1)
	case "list_dir":
		n := 0
		for _, ln := range strings.Split(r, "\n") {
			if strings.TrimSpace(ln) != "" {
				n++
			}
		}
		return true, fmt.Sprintf("%d item", n)
	}
	return true, ""
}

// agentEncode — bungkus event jadi satu string (dipisah tab) buat lewat StreamChunk.
func agentEncode(kind, tool, label, detail string) string {
	clean := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")
	return strings.Join([]string{kind, clean.Replace(tool), clean.Replace(label), clean.Replace(detail)}, "\t")
}

// agentApply — terapkan satu event ke state model.
func (m *Model) agentApply(text string) {
	p := strings.SplitN(text, "\t", 4)
	for len(p) < 4 {
		p = append(p, "")
	}
	switch p[0] {
	case "begin":
		m.agentSteps = nil
		m.agentPhase = ""
	case "think":
		m.agentPhase = p[2]
	case "plan":
		m.agentPhase = ""
		m.agentSteps = append(m.agentSteps, agentStep{Tool: p[1], Label: p[2], Status: stepPending})
	case "start":
		m.agentPhase = ""
		for i := range m.agentSteps {
			s := &m.agentSteps[i]
			if s.Status == stepPending && s.Tool == p[1] && s.Label == p[2] {
				s.Status = stepRunning
				return
			}
		}
		m.agentSteps = append(m.agentSteps, agentStep{Tool: p[1], Label: p[2], Status: stepRunning})
	case "ok", "fail":
		status := stepOK
		if p[0] == "fail" {
			status = stepFail
		}
		for i := len(m.agentSteps) - 1; i >= 0; i-- {
			s := &m.agentSteps[i]
			if s.Status == stepRunning && s.Tool == p[1] && s.Label == p[2] {
				s.Status = status
				s.Detail = p[3]
				return
			}
		}
	}
}
