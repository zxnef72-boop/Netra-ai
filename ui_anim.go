// ui_anim.go — animasi loading: spinner, garis panel "bernapas", kilau teks,
// kursor kedip. Tick cuma jalan selama streaming.
package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ocTickMsg struct{}

const ocTickEvery = 90 * time.Millisecond

func ocTick() tea.Cmd {
	return tea.Tick(ocTickEvery, func(time.Time) tea.Msg { return ocTickMsg{} })
}

// Update — pembungkus update0 (logika asli di main.go). Nyalain tick waktu
// streaming mulai, dan berhenti sendiri waktu streaming selesai.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(ocTickMsg); ok {
		if !m.streaming {
			m.ticking = false
			m.streamStart = time.Time{}
			return m, nil
		}
		m.frame++
		return m, ocTick()
	}

	if cm, ok := msg.(chunkMsg); ok && cm.Provider == "agent-step" {
		m.agentApply(cm.Text)
		if m.streamCh != nil {
			return m, waitForChunk(m.streamCh)
		}
		return m, nil
	}

	nm, cmd := m.update0(msg)
	mm, ok := nm.(Model)
	if !ok {
		return nm, cmd
	}
	if mm.streaming {
		if mm.streamStart.IsZero() {
			mm.streamStart = time.Now()
			mm.frame = 0
		}
		if !mm.ticking {
			mm.ticking = true
			return mm, tea.Batch(cmd, ocTick())
		}
	} else {
		mm.streamStart = time.Time{}
		mm.agentSteps = nil
		mm.agentPhase = ""
	}
	return mm, cmd
}

var ocSpinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func ocSpin(frame int) string { return ocSpinner[frame%len(ocSpinner)] }

func ocHex(s string) (int, int, int) {
	v, _ := strconv.ParseUint(strings.TrimPrefix(s, "#"), 16, 32)
	return int(v >> 16 & 0xff), int(v >> 8 & 0xff), int(v & 0xff)
}

// ocLerp — campur dua warna hex, t = 0..1.
func ocLerp(a, b string, t float64) lipgloss.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ar, ag, ab := ocHex(a)
	br, bg, bb := ocHex(b)
	mix := func(x, y int) int { return x + int(math.Round(float64(y-x)*t)) }
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", mix(ar, br), mix(ag, bg), mix(ab, bb)))
}

// ocBreath — gelombang segitiga 0..1 dengan periode `period` frame.
func ocBreath(frame, period int) float64 {
	if period < 2 {
		return 0
	}
	f := float64(frame % period)
	half := float64(period) / 2
	if f > half {
		f = float64(period) - f
	}
	return f / half
}

// ocShimmer — teks dengan kilau yang bergerak dari kiri ke kanan.
// bg diisi kalau teks ditaruh di atas panel berwarna.
func ocShimmer(text string, frame int, bg string) string {
	r := []rune(text)
	n := len(r)
	if n == 0 {
		return ""
	}
	pos := (frame/2)%(n+8) - 4
	var b strings.Builder
	for i, ch := range r {
		d := i - pos
		if d < 0 {
			d = -d
		}
		t := 1 - float64(d)/4
		st := lipgloss.NewStyle().Foreground(ocLerp("#565f89", "#c0caf5", t)).Italic(true)
		if bg != "" {
			st = st.Background(lipgloss.Color(bg))
		}
		b.WriteString(st.Render(string(ch)))
	}
	return b.String()
}

// ocDots — "", ".", "..", "..." berganti pelan, lebar tetap 3 biar gak goyang.
func ocDots(frame int) string {
	return fmt.Sprintf("%-3s", strings.Repeat(".", (frame/5)%4))
}

func (m Model) ocElapsed() string {
	if m.streamStart.IsZero() {
		return ""
	}
	s := int(time.Since(m.streamStart).Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	return fmt.Sprintf("%dm%02ds", s/60, s%60)
}
