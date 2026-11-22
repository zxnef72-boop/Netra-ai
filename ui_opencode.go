// ui_opencode.go — tampilan ala opencode: panel dengan bar kiri, blok pesan,
// header sesi, dan footer hint. Cuma urusan render; logika tombol ada di main.go.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	ocPanel   = lipgloss.Color("#16161e") // background panel input / blok user
	ocPanelHi = lipgloss.Color("#24283b") // baris terpilih
	ocBarUser = lipgloss.Color("#7aa2f7")
	ocBarErr  = lipgloss.Color("#f7768e")
	ocBarCode = lipgloss.Color("#3b4261")
)

// ---------- helper ----------

// ocRow — satu baris panel. Tiap potongan teks dikasih background sendiri,
// jadi warna panel gak putus gara-gara reset ANSI di tengah baris.
type ocRow struct {
	bg    lipgloss.Color
	parts []string
}

func newOcRow(bg lipgloss.Color) *ocRow { return &ocRow{bg: bg} }

func (r *ocRow) add(fg lipgloss.Color, text string) *ocRow {
	if text == "" {
		return r
	}
	r.parts = append(r.parts, lipgloss.NewStyle().Foreground(fg).Background(r.bg).Render(text))
	return r
}

func (r *ocRow) addBold(fg lipgloss.Color, text string) *ocRow {
	if text == "" {
		return r
	}
	r.parts = append(r.parts, lipgloss.NewStyle().Foreground(fg).Background(r.bg).Bold(true).Render(text))
	return r
}

// addRaw — sisipkan potongan yang sudah di-render sendiri (harus sudah punya background).
func (r *ocRow) addRaw(s string) *ocRow {
	if s != "" {
		r.parts = append(r.parts, s)
	}
	return r
}

func (r *ocRow) body() string { return strings.Join(r.parts, "") }

// render — bar kiri 1 kolom + isi, diisi background sampai lebar w.
func (r *ocRow) render(w int, bar lipgloss.Color) string {
	body := r.body()
	pad := w - 1 - lipgloss.Width(body)
	if pad > 0 {
		body += lipgloss.NewStyle().Background(r.bg).Render(strings.Repeat(" ", pad))
	}
	barStr := lipgloss.NewStyle().Foreground(bar).Background(r.bg).Render("┃")
	return barStr + body
}

func ocTrunc(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// ocTail — potong dari depan, sisakan ujung belakang (buat path panjang).
func ocTail(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return "…" + string(r[len(r)-(max-1):])
}

// ocHardWrap — pecah baris yang masih kepanjangan (URL, token panjang).
func ocHardWrap(lines []string, max int) []string {
	if max < 4 {
		max = 4
	}
	var out []string
	for _, l := range lines {
		r := []rune(l)
		for len(r) > max {
			out = append(out, string(r[:max]))
			r = r[max:]
		}
		out = append(out, string(r))
	}
	return out
}

// ocWindow — ambil bagian input yang muat di layar, kursor tetap kelihatan.
func ocWindow(before, after string, avail int) (string, string) {
	br, ar := []rune(before), []rune(after)
	room := avail - 1 // sisakan 1 kolom buat kursor
	if room < 4 {
		room = 4
	}
	if len(br)+len(ar) <= room {
		return before, after
	}
	maxAfter := room / 3
	if len(ar) > maxAfter {
		ar = ar[:maxAfter]
	}
	maxBefore := room - len(ar)
	if len(br) > maxBefore {
		br = br[len(br)-maxBefore:]
	}
	return string(br), string(ar)
}

func ocShortPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func ocKey(k, d string) string {
	return styleMuted.Render(k) + " " + styleDim.Render(d)
}

// ---------- view utama ----------

func (m Model) ocView() string {
	if !m.ready {
		return ""
	}
	w, h := m.width, m.height
	if w < 40 {
		w = 40
	}
	if h < 10 {
		h = 10
	}

	if m.showHistory {
		return m.renderHistory(w, h)
	}
	if m.isWelcome() && !m.showFiles {
		return m.ocHome(w, h)
	}
	if m.showFiles {
		return m.renderFileBrowser(w, h)
	}

	header := m.ocHeader(w)
	input := m.ocInputPanel(w, false)
	footer := m.ocFooter(w)

	chatH := h - lipgloss.Height(header) - lipgloss.Height(input) - lipgloss.Height(footer)
	if chatH < 3 {
		chatH = 3
	}
	chat := m.ocChat(w, chatH)
	return header + "\n" + chat + "\n" + input + "\n" + footer
}

// ---------- header sesi ----------

func (m Model) ocHeader(w int) string {
	title := strings.TrimSpace(strings.ReplaceAll(m.currentTitle, "\n", " "))
	if title == "" {
		title = "Chat baru"
	}
	bg := ocPanel
	right := newOcRow(bg)
	right.add(clrDim, fmt.Sprintf("%d pesan  ", len(m.messages)))
	maxTitle := w - lipgloss.Width(right.body()) - 6
	if maxTitle < 8 {
		maxTitle = 8
	}
	left := newOcRow(bg)
	left.add(clrText, " ").addBold(clrAccent2, "# ").addBold(clrText, ocTrunc(title, maxTitle))
	pad := w - lipgloss.Width(left.body()) - lipgloss.Width(right.body())
	if pad < 1 {
		pad = 1
	}
	return left.body() + lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad)) + right.body()
}

// ---------- panel input (home dan chat) ----------

func (m Model) ocInputPanel(w int, roomy bool) string {
	bg := ocPanel
	bar := clrAccent2
	if m.streaming {
		// garis kiri "bernapas" antara ungu dan biru
		bar = ocLerp("#bb9af7", "#7aa2f7", ocBreath(m.frame, 28))
	}
	textW := w - 5
	if textW < 10 {
		textW = 10
	}
	blank := func() string { return newOcRow(bg).render(w, bar) }

	rows := []string{blank()}

	// picker pilihan A/B/C dari AI
	if m.choiceActive && len(m.choices) > 0 {
		for i, c := range m.choices {
			rbg := bg
			if i == m.choiceCursor {
				rbg = ocPanelHi
			}
			r := newOcRow(rbg)
			r.add(clrText, "  ")
			letter := string(rune('A' + i))
			if i == m.choiceCursor {
				r.addBold(clrAccent2, "> "+letter+"  ")
				r.add(clrText, ocTrunc(c, textW-5))
			} else {
				r.add(clrDim, "  "+letter+"  ")
				r.add(clrMuted, ocTrunc(c, textW-5))
			}
			rows = append(rows, r.render(w, bar))
		}
		hint := newOcRow(bg)
		hint.add(clrText, "  ").add(clrDim, ocTrunc("↑↓ pilih  enter kirim  1-9 cepat  esc batal", textW))
		rows = append(rows, blank(), hint.render(w, bar))
		return strings.Join(rows, "\n")
	}

	if len(m.attached) > 0 {
		var names []string
		for _, af := range m.attached {
			names = append(names, af.Name)
		}
		r := newOcRow(bg)
		r.add(clrText, "  ").add(clrAccent2, ocTrunc("+ "+strings.Join(names, ", ")+"  ctrl+r", textW))
		rows = append(rows, r.render(w, bar))
	}

	before, after := m.input, ""
	if m.inputCursor >= 0 && m.inputCursor <= len(m.input) {
		before, after = m.input[:m.inputCursor], m.input[m.inputCursor:]
	}
	before, after = ocWindow(before, after, textW)
	in := newOcRow(bg)
	in.add(clrText, "  ").add(clrText, before).add(clrAccent2, "█")
	if m.input == "" {
		in.add(clrDim, "tulis pesan...")
	} else {
		in.add(clrText, after)
	}
	rows = append(rows, in.render(w, bar))
	if roomy {
		rows = append(rows, blank())
	}

	mode := newOcRow(bg)
	mode.add(clrText, "  ")
	if m.streaming {
		label := "berpikir"
		if len(m.agentSteps) > 0 {
			label = "bekerja"
		}
		if n := len(m.messages); n > 0 && m.messages[n-1].Role != "user" && strings.TrimSpace(m.messages[n-1].Text) != "" {
			label = "menulis"
		}
		mode.addBold(bar, ocSpin(m.frame)).add(clrText, " ")
		mode.addRaw(ocShimmer(label, m.frame, string(bg)))
		mode.add(clrDim, ocDots(m.frame))
		if el := m.ocElapsed(); el != "" {
			mode.add(clrDim, "  "+el)
		}
	} else {
		name := ocTrunc(m.provider.Name, textW-8)
		mode.addBold(clrAccent2, "Chat").add(clrDim, "  ").add(clrMuted, name)
		left := textW - 6 - len([]rune(name)) - 2
		if left > 3 {
			mode.add(clrDim, "  "+ocTrunc(m.provider.Model, left))
		}
	}
	rows = append(rows, mode.render(w, bar))
	if roomy {
		rows = append(rows, blank())
	}
	return strings.Join(rows, "\n")
}

// ---------- footer ----------

func (m Model) ocFooter(w int) string {
	var left string
	switch {
	case m.fileFocus:
		left = "  " + lipgloss.NewStyle().Foreground(clrUser).Render("● files") + styleDim.Render("  tab  esc")
	default:
		left = "  " + ocKey("enter", "kirim") + "   " + ocKey("ctrl+b", "files")
		if w >= 56 {
			left += "   " + ocKey("ctrl+s", "sessions")
		}
	}
	room := w - lipgloss.Width(left) - 4
	right := ""
	if room > 4 {
		right = styleDim.Render(ocTrunc(m.provider.Model, room)) + "  "
	}
	pad := w - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

// ---------- home ----------

func (m Model) ocHome(w, h int) string {
	boxW := w - 6
	if boxW > 84 {
		boxW = 84
	}
	if boxW < 36 {
		boxW = 36
	}

	full := h >= 20 && w >= 46
	var logoBlock string
	if full {
		logo := []string{
			"███╗   ██╗███████╗████████╗██████╗  █████╗ ",
			"████╗  ██║██╔════╝╚══██╔══╝██╔══██╗██╔══██╗",
			"██╔██╗ ██║█████╗     ██║   ██████╔╝███████║",
			"██║╚██╗██║███████╗   ██║   ██╔══██╗██║  ██║",
			"╚═╝ ╚═╝╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝",
		}
		gradient := []lipgloss.Color{
			lipgloss.Color("#c4b5fd"), lipgloss.Color("#a78bfa"),
			lipgloss.Color("#8b5cf6"), lipgloss.Color("#7aa2f7"),
			lipgloss.Color("#5b8def"),
		}
		var ls []string
		for i, l := range logo {
			ls = append(ls, lipgloss.NewStyle().Foreground(gradient[i]).Bold(true).Render(l))
		}
		logoBlock = strings.Join(ls, "\n")
	} else {
		logoBlock = styleAccent.Render("N E T R A")
	}

	center := func(s string) string { return lipgloss.PlaceHorizontal(w, lipgloss.Center, s) }

	panel := m.ocInputPanel(boxW, true)

	hintText := ocKey("ctrl+b", "files") + "   " + ocKey("ctrl+s", "sessions") + "   " + ocKey("esc", "quit")
	if boxW < 46 {
		hintText = ocKey("ctrl+b", "files") + "   " + ocKey("esc", "quit")
	}
	hintW := lipgloss.Width(hintText)
	boxLeft := (w - boxW) / 2
	margin := boxLeft + boxW - hintW
	if margin < boxLeft {
		margin = boxLeft
	}
	hintLine := strings.Repeat(" ", margin) + hintText

	tip := lipgloss.NewStyle().Foreground(clrCode).Render("●") + " " +
		lipgloss.NewStyle().Foreground(clrCode).Render("Tip") + "  " +
		styleMuted.Render("ketik /help buat daftar command")

	var lines []string
	lines = append(lines, center(logoBlock))
	if full {
		lines = append(lines, "", center(styleDim.Render("netra-ai · v0.1.0")))
	}
	lines = append(lines, "", center(panel), hintLine, "", center(tip))

	content := strings.Join(lines, "\n")
	ch := strings.Count(content, "\n") + 1
	top := (h - 1 - ch) / 2
	if top < 0 {
		top = 0
	}
	body := strings.Repeat("\n", top) + content
	bl := strings.Count(body, "\n") + 1
	if bl > h-1 {
		body = strings.Join(strings.Split(body, "\n")[:h-1], "\n")
	} else if bl < h-1 {
		body += strings.Repeat("\n", h-1-bl)
	}

	version := styleDim.Render("v0.1.0") + "  "
	maxPath := w - lipgloss.Width(version) - 6
	left := "  " + styleDim.Render(ocTail(ocShortPath(m.currentDir), maxPath))
	pad := w - lipgloss.Width(left) - lipgloss.Width(version)
	if pad < 1 {
		pad = 1
	}
	return body + "\n" + left + strings.Repeat(" ", pad) + version
}

// ---------- chat ----------

func (m Model) ocChat(w, h int) string {
	textW := w - 5
	if textW < 20 {
		textW = 20
	}
	last := len(m.messages) - 1
	var lines []string

	for idx, msg := range m.messages {
		lines = append(lines, "")

		switch msg.Role {
		case "user", "error":
			bar, fg := ocBarUser, clrText
			if msg.Role == "error" {
				bar, fg = ocBarErr, clrErr
			}
			bg := ocPanel
			blank := newOcRow(bg).render(w, bar)
			lines = append(lines, blank)
			for _, name := range msg.Attachments {
				label := "+ " + name
				if info, err := os.Stat(name); err == nil {
					label += fmt.Sprintf(" (%.1f KB)", float64(info.Size())/1024)
				}
				r := newOcRow(bg)
				r.add(clrText, "  ").add(clrAccent2, ocTrunc(label, textW))
				lines = append(lines, r.render(w, bar))
			}
			for _, wl := range ocHardWrap(wrapText(msg.Text, textW), textW) {
				r := newOcRow(bg)
				r.add(clrText, "  ").add(fg, wl)
				lines = append(lines, r.render(w, bar))
			}
			lines = append(lines, blank)

		default:
			msgStart := len(lines)
			segments := splitCodeBlocks(msg.Text)
			anyContent := false
			for _, seg := range segments {
				if seg.IsCode {
					anyContent = true
					label := seg.Lang
					if label == "" {
						label = "code"
					}
					lines = append(lines, "   "+styleDim.Render(label))
					barS := lipgloss.NewStyle().Foreground(ocBarCode).Render("┃")
					codeMaxW := w - 13
					if codeMaxW < 16 {
						codeMaxW = 16
					}
					inStyle, inScript := false, false
					baseLang := strings.ToLower(seg.Lang)

					for i, cl := range strings.Split(seg.Text, "\n") {
						if baseLang == "html" || baseLang == "htm" {
							trimmed := strings.TrimSpace(cl)
							if strings.Contains(trimmed, "<style") && !strings.Contains(trimmed, "</style") {
								inStyle = true
							}
							if strings.Contains(trimmed, "<script") && !strings.Contains(trimmed, "</script") {
								inScript = true
							}
						}
						lineLang := seg.Lang
						if inStyle {
							lineLang = "css"
						} else if inScript {
							lineLang = "javascript"
						}

						wrapped := wrapCodeLine(cl, codeMaxW)
						if len(wrapped) == 0 {
							wrapped = []string{""}
						}
						for k, wl := range wrapped {
							num := "    "
							if k == 0 {
								num = fmt.Sprintf("%3d ", i+1)
							}
							lines = append(lines, "   "+barS+" "+styleCodeNum.Render(num)+indentAndHighlight(wl, lineLang))
						}
					}
					lines = append(lines, "")
				} else {
					clean := strings.ReplaceAll(seg.Text, "**", "")
					for _, wl := range ocHardWrap(wrapText(clean, textW), textW) {
						if strings.TrimSpace(wl) == "" {
							lines = append(lines, "")
							continue
						}
						anyContent = true
						lines = append(lines, "   "+styleText.Render(wl))
					}
				}
			}

			streamingThis := m.streaming && idx == last
			if streamingThis && len(m.agentSteps) > 0 {
				lines = append(lines, m.ocAgentChecklist(textW)...)
			} else if !anyContent && streamingThis {
				spin := lipgloss.NewStyle().Foreground(ocLerp("#bb9af7", "#7aa2f7", ocBreath(m.frame, 28))).Render(ocSpin(m.frame))
				lines = append(lines, "   "+spin+" "+ocShimmer("berpikir", m.frame, "")+styleDim.Render(ocDots(m.frame)))
			} else if anyContent && streamingThis {
				// kursor kedip di ujung teks yang lagi diketik
				i := len(lines) - 1
				for i >= msgStart && strings.TrimSpace(lines[i]) == "" {
					i--
				}
				if i >= msgStart && (m.frame/5)%2 == 0 {
					lines[i] += lipgloss.NewStyle().Foreground(clrAccent).Render("▍")
				}
			} else if anyContent && !streamingThis {
				meta := "   " + lipgloss.NewStyle().Foreground(clrAccent).Render("▣") + styleDim.Render(" netra")
				if idx == last {
					meta += styleDim.Render(" · " + m.provider.Model)
				}
				lines = append(lines, "", meta)
			}
		}
	}

	total := len(lines)
	avail := h
	maxScroll := 0
	if total > avail {
		maxScroll = total - avail
	}
	cs := m.chatScroll
	if cs > maxScroll {
		cs = maxScroll
	}
	if cs < 0 {
		cs = 0
	}
	if total > avail {
		end := total - cs
		start := end - avail
		if start < 0 {
			start = 0
		}
		lines = lines[start:end]
	}
	for len(lines) < avail {
		lines = append([]string{""}, lines...)
	}
	return strings.Join(lines, "\n")
}

// ocAgentChecklist — daftar langkah agent: spinner (jalan), ✓ (ok), ✗ (gagal), · (antri).
func (m Model) ocAgentChecklist(w int) []string {
	okS := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))
	failS := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
	runS := lipgloss.NewStyle().Foreground(ocLerp("#bb9af7", "#7aa2f7", ocBreath(m.frame, 28)))
	done, failed := 0, 0
	for _, s := range m.agentSteps {
		switch s.Status {
		case stepOK:
			done++
		case stepFail:
			failed++
		}
	}
	head := fmt.Sprintf("langkah %d/%d", done+failed, len(m.agentSteps))
	if failed > 0 {
		head += fmt.Sprintf(" · %d gagal", failed)
	}
	lines := []string{"   " + styleDim.Render(head)}
	lw := w - 6
	if lw < 10 {
		lw = 10
	}
	for _, s := range m.agentSteps {
		var icon string
		switch s.Status {
		case stepOK:
			icon = okS.Render("✓")
		case stepFail:
			icon = failS.Render("✗")
		case stepRunning:
			icon = runS.Render(ocSpin(m.frame))
		default:
			icon = styleDim.Render("·")
		}
		verb := agentVerb(s.Tool)
		room := lw - len([]rune(verb)) - 1
		detail := ""
		if s.Status == stepOK && s.Detail != "" {
			detail = "(" + s.Detail + ")"
			room -= len([]rune(detail)) + 1
		}
		label := ocTrunc(s.Label, room)
		ls := styleText
		if s.Status == stepFail {
			ls = failS
		}
		line := "   " + icon + " " + styleMuted.Render(verb) + " " + ls.Render(label)
		if detail != "" {
			line += " " + styleDim.Render(detail)
		}
		lines = append(lines, line)
		if s.Status == stepFail && s.Detail != "" {
			lines = append(lines, "     "+failS.Render(ocTrunc(s.Detail, lw)))
		}
	}
	if m.agentPhase != "" {
		lines = append(lines, "   "+runS.Render(ocSpin(m.frame))+" "+ocShimmer(m.agentPhase, m.frame, "")+styleDim.Render(ocDots(m.frame)))
	}
	return lines
}
