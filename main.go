// netra-ai — TUI v15: file manager + /save + /saveall + /blocks
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	clrDim      = lipgloss.Color("#565f89")
	clrMuted    = lipgloss.Color("#9aa5ce")
	clrText     = lipgloss.Color("#c0caf5")
	clrAccent   = lipgloss.Color("#bb9af7")
	clrAccent2  = lipgloss.Color("#7aa2f7")
	clrUser     = lipgloss.Color("#9ece6a")
	clrCode     = lipgloss.Color("#e0af68")
	clrCodeNum  = lipgloss.Color("#414868")
	clrErr      = lipgloss.Color("#f7768e")
	clrBorderHi = lipgloss.Color("#7aa2f7")
	clrDir      = lipgloss.Color("#7dcfff")
	clrBgSide   = lipgloss.Color("#0d0d14")
)

var (
	styleDim     = lipgloss.NewStyle().Foreground(clrDim)
	styleMuted   = lipgloss.NewStyle().Foreground(clrMuted)
	styleText    = lipgloss.NewStyle().Foreground(clrText)
	styleAccent  = lipgloss.NewStyle().Foreground(clrAccent).Bold(true)
	styleAccent2 = lipgloss.NewStyle().Foreground(clrAccent2).Bold(true)
	styleUser    = lipgloss.NewStyle().Foreground(clrUser).Bold(true)
	styleCode    = lipgloss.NewStyle().Foreground(clrCode)
	styleCodeNum = lipgloss.NewStyle().Foreground(clrCodeNum)
	styleErr     = lipgloss.NewStyle().Foreground(clrErr)
	styleCursor  = lipgloss.NewStyle().Foreground(clrAccent).Bold(true)
	styleDir     = lipgloss.NewStyle().Foreground(clrDir).Bold(true)
)

type ProjectFile struct {
	Path    string
	Content string
}

type AttachedFile struct {
	Name    string
	Content string
}

var (
	clrBorder     = lipgloss.Color("#1f1f2e")
	clrNeon1      = lipgloss.Color("#a78bfa")
	clrNeon2      = lipgloss.Color("#7aa2f7")
	clrNeon3      = lipgloss.Color("#5eead4")
	clrNeon4      = lipgloss.Color("#f472b6")
	clrNeon5      = lipgloss.Color("#fbbf24")
	clrNeonShadow = lipgloss.Color("#c4b5fd")

	clrPanelBg    = lipgloss.Color("#0d0d14")
	clrShadow     = lipgloss.Color("#0f0f18")
	clrHighlight  = lipgloss.Color("#5a5a78")
	clrChatBg     = lipgloss.Color("#0d0d14")
	clrUserBg     = lipgloss.Color("#0d1f1a")
	clrBotBg      = lipgloss.Color("#151028")
	clrErrBg      = lipgloss.Color("#1a0a0f")
	clrUserBar    = lipgloss.Color("#4ade80")
	clrBotBar     = lipgloss.Color("#a78bfa")
	clrErrBar     = lipgloss.Color("#f87171")
	clrPanelBg2   = lipgloss.Color("#16161e")
	clrCardBorder = lipgloss.Color("#2a2a3e")
	clrCardHi     = lipgloss.Color("#6a6a8e")
	clrUserBlock  = lipgloss.Color("#0f2a20")
	clrBotBlock   = lipgloss.Color("#151028")
	clrErrBlock   = lipgloss.Color("#1a0a0f")
)

type ChatMsg struct {
	Role        string   `json:"role"`
	Text        string   `json:"text"`
	Attachments []string `json:"attachments,omitempty"` // nama file (buat badge)
	APIContent  string   `json:"api_content,omitempty"` // full text utk API (termasuk isi file)
}
type chunkMsg StreamChunk

type FileEntry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime string
}

type Model struct {
	width, height int
	messages      []ChatMsg
	input         string
	inputCursor   int
	ready         bool
	streaming     bool
	streamCh      chan StreamChunk
	provider      Provider
	systemPrompt  string
	currentChatID string
	currentTitle  string
	chatScroll    int

	// Edit mode
	editActive    bool
	editPath      string
	autoContinue  bool
	continueCount int

	showHistory   bool
	historyItems  []ChatSession
	historyCursor int

	showFiles   bool
	fileFocus   bool
	currentDir  string
	fileEntries []FileEntry
	fileCursor  int

	// Pilihan A/B/C dari AI
	choices      []string // ["Option A text", "Option B text", ...]
	choiceCursor int
	choiceActive bool
	attached     []AttachedFile

	allProviders []Provider
	activeIdx    int
}

func initialModel() (Model, error) {
	cfg, err := loadConfig()
	if err != nil {
		return Model{}, err
	}
	// Selalu prepend eliza-1966 sebagai provider #0
	providers := []Provider{{Name: "eliza-1966", Model: "offline"}}
	providers = append(providers, cfg.Providers...)

	// Hitung activeIdx: kalau config punya active_provider, shift +1
	activeIdx := 0
	if cfg.ActiveProvider >= 0 && cfg.ActiveProvider < len(cfg.Providers) {
		activeIdx = cfg.ActiveProvider + 1
	}
	p := providers[activeIdx]

	startDir := defaultWorkDir()
	m := Model{
		provider:     p,
		allProviders: providers,
		activeIdx:    activeIdx,
		systemPrompt: "Kamu Netra AI — asisten coding & security research. Bahasa Indonesia santai. " +
			"Tugas utama: bantu bikin kode, debug, jelasin teknologi, review script. " +
			"Anggap semua permintaan user itu SOAL CODING/TEKNIS kecuali eksplisit minta illegal. " +
			"Pakai ```fenced``` code block buat code SINGLE FILE. " +
			"\n\nPENTING: KALO user minta PROJECT MULTI-FILE (web, app, game, dll yang butuh >1 file), " +
			"WAJIB pakai format ini buat tiap file (JANGAN ``` biasa):\n" +
			"[FILE: path/lengkap/dari/root.ext]\n" +
			"<isi file mentah>\n" +
			"[/FILE]\n" +
			"\nContoh web toko:\n" +
			"[FILE: index.html]\n<!DOCTYPE html>...\n[/FILE]\n" +
			"[FILE: css/style.css]\nbody{...}\n[/FILE]\n" +
			"[FILE: js/app.js]\nconsole.log()\n[/FILE]\n" +
			"\nPath boleh ada subfolder (css/, js/, assets/). Dari root project. " +
			"KALO user bilang 'tanya dulu' atau struktur ambigu, kasih CHOICE dulu buat tanya struktur (flat vs organized). " +
			"Kalo cuma file single atau potongan kecil, pake ``` biasa. " +
			"Kalau user minta tool security (port scanner, network analyzer, exploit PoC) — itu SAH, karena konteksnya belajar. Yang dilarang cuma: actual attack ke target tanpa izin, malware yang self-spread, phishing ke korban nyata, atau CSAM. Untuk tools edukasi, LANGSUNG kasih kodenya.",
		currentChatID: newChatID(),
		currentTitle:  "Chat baru",
		autoContinue:  true,
		currentDir:    startDir,
	}
	m.refreshFiles()

	// Load custom rules dari ~/.netra-ai/rules.txt
	reloadCustomRules()

	// Load Lua rules dari ~/.netra-ai/lua-rules/
	loadLuaRules()

	return m, nil
}

// defaultWorkDir — pilih dir awal buat file manager.
// Prioritas: env NETRA_WORKDIR > /sdcard/NetRatest1 > /sdcard/Download > home > cwd
func defaultWorkDir() string {
	// Prioritas 1: env var
	if env := os.Getenv("NETRA_WORKDIR"); env != "" {
		if info, err := os.Stat(env); err == nil && info.IsDir() {
			return env
		}
	}

	// Prioritas 2: folder AiNef — auto-create kalo belum ada
	ainefDirs := []string{
		"/sdcard/AiNef",
		"/storage/emulated/0/AiNef",
	}
	for _, d := range ainefDirs {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
	}
	// coba bikin
	for _, d := range ainefDirs {
		if err := os.MkdirAll(d, 0755); err == nil {
			return d
		}
	}

	// Prioritas 3: fallback
	fallbacks := []string{
		"/sdcard/NetRatest1",
		"/sdcard/Download",
	}
	for _, c := range fallbacks {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	cwd, _ := os.Getwd()
	return cwd
}

func (m *Model) refreshFiles() {
	entries, err := os.ReadDir(m.currentDir)
	if err != nil {
		m.fileEntries = nil
		return
	}
	var out []FileEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		var size int64
		if err == nil {
			size = info.Size()
		}
		modTime := ""
		if info != nil {
			modTime = info.ModTime().Format("02/01 15:04")
		}
		out = append(out, FileEntry{Name: name, IsDir: e.IsDir(), Size: size, ModTime: modTime})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	m.fileEntries = out
	if m.fileCursor >= len(out) {
		m.fileCursor = 0
	}
	if m.fileCursor < 0 {
		m.fileCursor = 0
	}
}

func (m *Model) isWelcome() bool {
	return len(m.messages) == 0
}

func (m Model) Init() tea.Cmd { return nil }

func waitForChunk(ch chan StreamChunk) tea.Cmd {
	return func() tea.Msg { return chunkMsg(<-ch) }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		return m, nil

	case chunkMsg:
		if msg.Err != nil {
			if msg.RateLimit && len(m.allProviders) > 1 {
				next := (m.activeIdx + 1) % len(m.allProviders)
				if next != m.activeIdx {
					oldName := m.allProviders[m.activeIdx].Name
					m.activeIdx = next
					m.provider = m.allProviders[m.activeIdx]
					if cfg, err := loadConfig(); err == nil {
						cfg.ActiveProvider = next
						cfg.Providers = m.allProviders
						cfg.Save()
					}
					m.messages = append(m.messages, ChatMsg{Role: "assistant",
						Text: fmt.Sprintf("Provider %s kena limit. Auto-switch ke %s, coba lagi...",
							oldName, m.provider.Name)})
					history := []Message{{Role: "system", Content: m.systemPrompt}}
					for _, cm := range m.messages {
						if cm.Role == "error" || cm.Text == "" {
							continue
						}
						if strings.HasPrefix(cm.Text, "Provider ") && strings.Contains(cm.Text, "kena limit") {
							continue
						}
						history = append(history, Message{Role: cm.Role, Content: cm.Text})
					}
					if len(history) > 0 && history[len(history)-1].Role == "assistant" && history[len(history)-1].Content == "" {
						history = history[:len(history)-1]
					}
					// === Eliza offline — default. AI cuma kalau /ask ===
					lastUserText := ""
					for i := len(m.messages) - 1; i >= 0; i-- {
						if m.messages[i].Role == "user" {
							lastUserText = m.messages[i].Text
							break
						}
					}

					useAI := strings.HasPrefix(strings.TrimSpace(lastUserText), "/ask ")
					if !useAI {
						reply := elizaReplyCore(lastUserText)
						if reply == "" {
							reply = "Hmm. Ceritain lebih dong."
						}
						m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: reply})
						m.streaming = false
						m.chatScroll = 0
						return m, nil
					}

					// User pakai /ask — baru route ke AI
					ch := make(chan StreamChunk, 64)
					m.streamCh = ch
					m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ""})
					go streamChat(m.provider, history, ch)
					return m, waitForChunk(ch)
				}
			}
			// Fallback ke Eliza offline
			lastUser := ""
			for i := len(m.messages) - 1; i >= 0; i-- {
				if m.messages[i].Role == "user" {
					lastUser = m.messages[i].Text
					break
				}
			}
			reply := elizaReplyCore(lastUser)
			if reply == "" {
				reply = "Eliza offline mode. API gak tersambung."
			}
			m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: reply})
			m.streaming = false
			m.chatScroll = 0
			return m, nil
		}
		if msg.Done {
			m.streaming = false
			m.currentTitle = titleFromMessages(m.messages)

			// Kalau editMode, auto-save hasil edit
			if m.editActive {
				m.completeEdit()
			}
			// parse CHOICE block dari message assistant terakhir
			if len(m.messages) > 0 {
				last := m.messages[len(m.messages)-1]
				if last.Role == "assistant" {
					cleaned, choices := parseChoiceBlock(last.Text)
					if len(choices) > 0 {
						m.messages[len(m.messages)-1].Text = cleaned
						m.choices = choices
						m.choiceCursor = 0
						m.choiceActive = true
					}
				}
			}
			go saveChat(ChatSession{ID: m.currentChatID, Title: m.currentTitle, Messages: m.messages})
			return m, nil
		}
		if msg.Text != "" && len(m.messages) > 0 {
			m.messages[len(m.messages)-1].Text += msg.Text
			m.chatScroll = 0
		}
		return m, waitForChunk(m.streamCh)

	case tea.KeyMsg:
		k := msg.String()

		if m.showHistory {
			switch k {
			case "esc", "ctrl+s":
				m.showHistory = false
			case "up":
				if m.historyCursor > 0 {
					m.historyCursor--
				}
			case "down":
				if m.historyCursor < len(m.historyItems)-1 {
					m.historyCursor++
				}
			case "enter":
				if m.historyCursor < len(m.historyItems) {
					c := m.historyItems[m.historyCursor]
					m.currentChatID = c.ID
					m.currentTitle = c.Title
					m.messages = c.Messages
					m.chatScroll = 0
					m.showHistory = false
				}
			case "n":
				m.newChat()
				m.showHistory = false
			}
			return m, nil
		}

		// Global
		switch k {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+s":
			m.showHistory = !m.showHistory
			m.historyItems = listChats()
			m.historyCursor = 0
			return m, nil
		case "ctrl+b":
			m.showFiles = !m.showFiles
			m.fileFocus = m.showFiles
			if m.showFiles {
				m.refreshFiles()
			}
			return m, nil
		case "ctrl+n":
			m.newChat()
			return m, nil
		case "ctrl+y":
			m.chatScroll += 5
			return m, nil
		case "ctrl+e":
			m.chatScroll -= 5
			if m.chatScroll < 0 {
				m.chatScroll = 0
			}
			return m, nil
		case "ctrl+g":
			m.chatScroll = 0
			return m, nil
		case "ctrl+r":
			if len(m.attached) > 0 {
				n := len(m.attached)
				m.attached = nil
				m.messages = append(m.messages, ChatMsg{Role: "assistant",
					Text: fmt.Sprintf("(hapus %d file attachment)", n)})
			}
			return m, nil
		}

		// File browser focus
		if m.showFiles && m.fileFocus {
			switch k {
			case "esc":
				// cuma blur dari file focus, sidebar tetep kebuka
				m.fileFocus = false
				return m, nil
			case "tab":
				m.fileFocus = false
				return m, nil
			case "up":
				if m.fileCursor > 0 {
					m.fileCursor--
				}
				return m, nil
			case "down":
				if m.fileCursor < len(m.fileEntries)-1 {
					m.fileCursor++
				}
				return m, nil
			case "backspace":
				m.currentDir = filepath.Dir(m.currentDir)
				m.fileCursor = 0
				m.refreshFiles()
				return m, nil
			case "enter":
				if m.fileCursor < len(m.fileEntries) {
					fe := m.fileEntries[m.fileCursor]
					full := filepath.Join(m.currentDir, fe.Name)
					if fe.IsDir {
						m.currentDir = full
						m.fileCursor = 0
						m.refreshFiles()
					} else {
						// HTML AUTO-DETECT: file .html → LANGSUNG BUILD (bypass prompt)
						low := strings.ToLower(fe.Name)
						if strings.HasSuffix(low, ".html") || strings.HasSuffix(low, ".htm") {
							appName := strings.TrimSuffix(fe.Name, filepath.Ext(fe.Name))
							appName = sanitizeName(appName)
							if appName == "" {
								appName = "MyApp"
							}
							m.fileFocus = false
							m.showFiles = false
							m.messages = append(m.messages, ChatMsg{Role: "assistant",
								Text: fmt.Sprintf("🚀 Auto-build: **%s** — tunggu...", fe.Name)})
							// Sync build — block TUI sebentar
							log, err := cmdBuildAPKFromHTML(full, appName, true)
							if err != nil {
								m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Build gagal: " + err.Error()})
							} else {
								m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: log})
							}
							m.chatScroll = 0
							return m, nil
						}

						// ATTACH file ke chat (bukan kirim langsung)
						data, err := os.ReadFile(full)
						if err != nil {
							m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Gagal baca: " + err.Error()})
						} else {
							content := string(data)
							if len(content) > 16000 {
								content = content[:16000] + "\n\n...[dipotong, terlalu panjang]"
							}
							m.attached = append(m.attached, AttachedFile{
								Name:    fe.Name,
								Content: content,
							})
						}
						m.fileFocus = false
						m.showFiles = false
					}
				}
				return m, nil
			}
			return m, nil
		}

		// Esc: layer-aware — tutup layer teratas dulu, baru quit
		if k == "esc" {
			// PRIORITAS 1: CHOICE widget aktif → cuma cancel
			if m.choiceActive {
				m.choiceActive = false
				m.choices = nil
				return m, nil
			}
			// PRIORITAS 2: history modal
			if m.showHistory {
				m.showHistory = false
				return m, nil
			}
			// PRIORITAS 3: file browser
			if m.showFiles {
				m.showFiles = false
				m.fileFocus = false
				return m, nil
			}
			// PRIORITAS 4: ada teks di input → clear
			if len(m.input) > 0 {
				m.input = ""
				m.inputCursor = 0
				return m, nil
			}
			// PRIORITAS 5: baru quit
			return m, tea.Quit
		}
		if k == "tab" && m.showFiles {
			m.fileFocus = true
			return m, nil
		}

		// Choice widget mode (kalo aktif)
		if m.choiceActive {
			switch k {
			case "up":
				if m.choiceCursor > 0 {
					m.choiceCursor--
				}
				return m, nil
			case "down":
				if m.choiceCursor < len(m.choices)-1 {
					m.choiceCursor++
				}
				return m, nil
			case "enter":
				// pilih opsi
				chosen := m.choices[m.choiceCursor]
				letter := string(rune('A' + m.choiceCursor))
				m.choiceActive = false
				m.choices = nil
				// kirim sebagai user message
				text := letter + ". " + chosen
				m.messages = append(m.messages, ChatMsg{Role: "user", Text: text})
				m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ""})
				m.streaming = true
				m.chatScroll = 0
				history := []Message{{Role: "system", Content: m.systemPrompt}}
				for _, cm := range m.messages {
					if cm.Role == "error" || cm.Text == "" {
						continue
					}
					history = append(history, Message{Role: cm.Role, Content: cm.Text})
				}
				if len(history) > 0 && history[len(history)-1].Role == "assistant" && history[len(history)-1].Content == "" {
					history = history[:len(history)-1]
				}
				ch := make(chan StreamChunk, 64)
				m.streamCh = ch
				go streamChat(m.provider, history, ch)
				return m, waitForChunk(ch)
			case "esc":
				m.choiceActive = false
				m.choices = nil
				return m, nil
			case "1", "2", "3", "4", "5":
				n := int(k[0] - '1')
				if n >= 0 && n < len(m.choices) {
					m.choiceCursor = n
					return m.Update(tea.KeyMsg{Type: tea.KeyEnter})
				}
			}
			return m, nil
		}

		// Input mode
		switch k {
		case "enter":
			if m.streaming {
				return m, nil
			}
			text := strings.TrimSpace(m.input)

			// === AUTO-SCAN APK ===
			for _, af := range m.attached {
				if strings.HasSuffix(strings.ToLower(af.Name), ".apk") {
					fullPath := af.Name
					if !filepath.IsAbs(fullPath) {
						fullPath = filepath.Join(m.currentDir, af.Name)
					}
					result := scanAPKFile(fullPath)
					m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: result})
					m.attached = nil
					m.input = ""
					m.inputCursor = 0
					m.chatScroll = 0
					return m, nil
				}
			}

			if text == "" {
				return m, nil
			}
			// Auto-convert \ jadi / (mis. \saveall → /saveall)
			if strings.HasPrefix(text, "\\") {
				text = "/" + text[1:]
			}
			// Auto-convert \n jadi /n (kalo user salah paste)
			if strings.HasPrefix(text, "\\") {
				text = "/" + text[1:]
			}
			if strings.HasPrefix(text, "/") {
				m.handleSlash(text)
				m.input = ""
				m.inputCursor = 0
				return m, nil
			}
			// Prepend attachment kalo ada
			var apiContent string
			var attachNames []string
			if len(m.attached) > 0 {
				var sb strings.Builder
				sb.WriteString(text)
				sb.WriteString("\n\n")
				for _, af := range m.attached {
					lang := langFromPath(af.Name)
					sb.WriteString(fmt.Sprintf("📎 %s\n```%s\n%s\n```\n\n",
						af.Name, lang, strings.TrimRight(af.Content, "\n")))
					attachNames = append(attachNames, af.Name)
				}
				apiContent = strings.TrimRight(sb.String(), "\n")
				m.attached = nil
			}

			m.messages = append(m.messages, ChatMsg{
				Role:        "user",
				Text:        text,
				Attachments: attachNames,
				APIContent:  apiContent,
			})
			m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ""})
			m.input = ""
			m.inputCursor = 0
			m.streaming = true
			m.chatScroll = 0
			history := []Message{{Role: "system", Content: m.systemPrompt}}
			for _, cm := range m.messages {
				if cm.Role == "error" || cm.Text == "" {
					continue
				}
				content := cm.Text
				if cm.APIContent != "" {
					content = cm.APIContent
				}
				history = append(history, Message{Role: cm.Role, Content: content})
			}
			if len(history) > 0 && history[len(history)-1].Role == "assistant" && history[len(history)-1].Content == "" {
				history = history[:len(history)-1]
			}
			ch := make(chan StreamChunk, 64)
			m.streamCh = ch

			// === VENDOR ELIZA (lokal if-else) ===
			if strings.HasPrefix(strings.ToLower(m.provider.Name), "eliza") {
				lastUser := ""
				lastAPI := ""
				for i := len(m.messages) - 1; i >= 0; i-- {
					if m.messages[i].Role == "user" {
						lastUser = m.messages[i].Text
						lastAPI = m.messages[i].APIContent
						break
					}
				}
				elizaInput := lastUser
				if lastAPI != "" {
					elizaInput = lastAPI
				}
				reply := elizaReply(elizaInput)
				go elizaFakeStream(reply, ch)
				return m, waitForChunk(ch)
			}

			go streamChat(m.provider, history, ch)
			return m, waitForChunk(ch)
		case "backspace":
			if m.inputCursor > 0 {
				m.input = m.input[:m.inputCursor-1] + m.input[m.inputCursor:]
				m.inputCursor--
			}
		case "left":
			if m.inputCursor > 0 {
				m.inputCursor--
			}
		case "right":
			if m.inputCursor < len(m.input) {
				m.inputCursor++
			}
		default:
			if len(k) == 1 {
				m.input = m.input[:m.inputCursor] + k + m.input[m.inputCursor:]
				m.inputCursor++
			}
		}
	}
	return m, nil
}

func (m *Model) newChat() {
	m.currentChatID = newChatID()
	m.currentTitle = "Chat baru"
	m.messages = nil
	m.input = ""
	m.inputCursor = 0
	m.chatScroll = 0
}

func (m *Model) handleSlash(cmd string) {
	cmd = strings.TrimSpace(cmd)

	if cmd == "/copy" || strings.HasPrefix(cmd, "/copy ") {
		m.handleCopy(strings.Fields(cmd))
		return
	}
	if cmd == "/save" || strings.HasPrefix(cmd, "/save ") {
		m.handleSave(strings.Fields(cmd))
		return
	}
	if cmd == "/saveall" || strings.HasPrefix(cmd, "/saveall ") {
		m.handleSaveAll(strings.Fields(cmd))
		return
	}
	if cmd == "/blocks" {
		m.handleBlocks()
		return
	}
	if cmd == "/cd" || strings.HasPrefix(cmd, "/cd ") {
		m.handleCd(strings.Fields(cmd))
		return
	}
	if cmd == "/continue" || cmd == "/lanjut" {
		m.handleContinue()
		return
	}

	if cmd == "/auto" {
		m.autoContinue = !m.autoContinue
		status := "OFF"
		if m.autoContinue {
			status = "ON"
		}
		m.messages = append(m.messages, ChatMsg{Role: "assistant",
			Text: fmt.Sprintf("Auto-continue: **%s**\nKalo stream kepotong, otomatis lanjut sampe selesai (max 10x).", status)})
		return
	}
	if cmd == "/baca" || strings.HasPrefix(cmd, "/baca ") {
		m.handleBaca(strings.Fields(cmd))
		return
	}
	if cmd == "/read" || strings.HasPrefix(cmd, "/read ") {
		args := strings.Fields(cmd)
		args[0] = "/baca"
		m.handleBaca(args)
		return
	}
	if cmd == "/memory" || strings.HasPrefix(cmd, "/memory ") {
		arg := strings.TrimSpace(strings.TrimPrefix(cmd, "/memory"))
		if arg == "reset" {
			updateMemory(func(m *Memory) {
				m.NamaUser = ""
				m.MoodTerakhir = ""
				m.TopikTerakhir = ""
				m.KaliChat = 0
				m.Catatan = make(map[string]string)
			})
			m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "🧹 Memory di-reset."})
			return
		}
		// Default: tampilin memory
		mem := loadMemory()
		txt := "🧠 **Memory Eliza:**\n\n"
		txt += fmt.Sprintf("- Nama user: `%s`\n", mem.NamaUser)
		txt += fmt.Sprintf("- Mood terakhir: `%s`\n", mem.MoodTerakhir)
		txt += fmt.Sprintf("- Topik terakhir: `%s`\n", mem.TopikTerakhir)
		txt += fmt.Sprintf("- Total chat: `%d`\n", mem.KaliChat)
		txt += fmt.Sprintf("- First seen: `%s`\n", mem.FirstSeen)
		if len(mem.Catatan) > 0 {
			txt += "\n**Catatan:**\n"
			for k, v := range mem.Catatan {
				txt += fmt.Sprintf("- `%s`: %s\n", k, v)
			}
		}
		txt += "\nReset: `/memory reset`"
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: txt})
		return
	}
	if cmd == "/persona" || strings.HasPrefix(cmd, "/persona ") {
		arg := strings.TrimSpace(strings.TrimPrefix(cmd, "/persona"))
		if arg == "" || arg == "list" {
			list := listPersonalities()
			active := "default"
			if activePersonality != nil {
				active = activePersonality.Name
			}
			txt := "**Personality aktif:** " + active + "\n\n**Tersedia:**\n"
			for _, p := range list {
				txt += "- `" + p + "`\n"
			}
			txt += "\nGanti: `/persona <nama>` atau `/persona default` buat matiin"
			m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: txt})
			return
		}
		if err := setPersonality(arg); err != nil {
			m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "❌ " + err.Error()})
			return
		}
		if activePersonality == nil {
			m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "✅ Personality dimatiin (default)"})

			m.streaming = false
		} else {
			g := activePersonality.Greeting()
			if g == "" {
				g = "Halo!"
			}
			m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "✅ Personality: **" + activePersonality.Name + "**\n\n" + g})
		}
		return
	}
	if cmd == "/apkscan" || strings.HasPrefix(cmd, "/apkscan ") {
		arg := strings.TrimSpace(strings.TrimPrefix(cmd, "/apkscan"))
		if arg == "" {
			m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "Pakai: /apkscan <path.apk>"})
			return
		}
		result := scanAPKFile(arg)
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: result})
		return
	}
	if cmd == "/build" || strings.HasPrefix(cmd, "/build ") {
		parts := strings.Fields(strings.TrimPrefix(cmd, "/build"))
		if len(parts) < 3 {
			m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "Pakai: /build <apk|html|native|exe|exe-win> <url|@file> <nama-app>"})
			return
		}
		kind, targetURL, appName := parts[0], parts[1], strings.Join(parts[2:], " ")
		var logStr string
		var err error
		if kind == "apk" {
			logStr, err = cmdBuildAPK(targetURL, appName)
		} else if kind == "html" {
			// Cek flag --fullscreen di appName
			fullscreen := strings.Contains(strings.ToLower(appName), "--fullscreen")
			if fullscreen {
				appName = strings.TrimSpace(strings.ReplaceAll(appName, "--fullscreen", ""))
				appName = strings.TrimSpace(strings.ReplaceAll(appName, "--FULLSCREEN", ""))
			}
			logStr, err = cmdBuildAPKFromHTML(targetURL, appName, fullscreen)
		} else if kind == "native" {
			logStr, err = cmdBuildNativeAPK(targetURL, appName)
		} else if kind == "exe" {
			logStr, err = cmdBuildInstallerEXE(targetURL, appName)
		} else if kind == "exe-win" || kind == "win" {
			logStr, err = cmdBuildWinInstaller(targetURL, appName)
		}
		if err != nil && logStr == "" {
			logStr = "Error: " + err.Error()
		}
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: logStr})
		return
	}
	if cmd == "/sandbox" || strings.HasPrefix(cmd, "/sandbox ") {
		script := strings.TrimSpace(strings.TrimPrefix(cmd, "/sandbox"))
		logStr, serr := cmdSandboxRun(script)
		if serr != nil && logStr == "" {
			logStr = "Error: " + serr.Error()
		}
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: logStr})
		return
	}
	if cmd == "/search" || strings.HasPrefix(cmd, "/search ") {
		m.handleSearch(strings.Fields(cmd))
		return
	}
	if cmd == "/tutorial" || strings.HasPrefix(cmd, "/tutorial ") {
		m.handleTutorial(strings.Fields(cmd))
		return
	}
	if cmd == "/lua-rules-reload" {
		n := loadLuaRules()
		m.messages = append(m.messages, ChatMsg{Role: "assistant",
			Text: fmt.Sprintf("OK: %d Lua rules di-load dari %s", n, luaRulesDir())})
		return
	}
	if cmd == "/lua-rules-list" {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**%d Lua rules:**\n\n", len(luaRules)))
		for _, r := range luaRules {
			sb.WriteString(fmt.Sprintf("- `%s`\n", r.Name))
		}
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: sb.String()})
		return
	}
	if cmd == "/rules-reload" {
		n := reloadCustomRules()
		if n < 0 {
			m.messages = append(m.messages, ChatMsg{Role: "error",
				Text: "Gagal reload rules. Cek file " + rulesPath()})
		} else {
			m.messages = append(m.messages, ChatMsg{Role: "assistant",
				Text: fmt.Sprintf("OK: %d rules di-load dari %s", n, rulesPath())})
		}
		return
	}
	if cmd == "/rules-list" {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**%d custom rules** (dari %s):\n\n", len(customRules), rulesPath()))
		for i, r := range customRules {
			if i >= 30 {
				sb.WriteString("\n... (sisanya kepotong)")
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s` → %s\n", r.Trigger, r.Replies[0]))
		}
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: sb.String()})
		return
	}
	if cmd == "/about-eliza" || cmd == "/about_eliza" || cmd == "/eliza-about" {
		m.handleAboutEliza()
		return
	}
	if cmd == "/edit" || strings.HasPrefix(cmd, "/edit ") {
		m.handleEdit(strings.Fields(cmd))
		return
	}
	if cmd == "/lua" || strings.HasPrefix(cmd, "/lua ") {
		m.handleLua(strings.Fields(cmd))
		return
	}
	if cmd == "/template" || strings.HasPrefix(cmd, "/template ") {
		m.handleTemplate(strings.Fields(cmd))
		return
	}
	if cmd == "/project" || strings.HasPrefix(cmd, "/project ") {
		m.handleProject(strings.Fields(cmd))
		return
	}
	if cmd == "/unzip" || strings.HasPrefix(cmd, "/unzip ") {
		m.handleUnzip(strings.Fields(cmd))
		return
	}
	if cmd == "/zip" || strings.HasPrefix(cmd, "/zip ") {
		m.handleZip(strings.Fields(cmd))
		return
	}
	if cmd == "/vendors" {
		m.handleVendors()
		return
	}
	if cmd == "/vendor" || strings.HasPrefix(cmd, "/vendor ") {
		m.handleVendor(strings.Fields(cmd))
		return
	}

	switch cmd {
	case "/help":
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "**Commands:**\n" +
			"- `/help` — bantuan\n" +
			"- `/new` — chat baru (Ctrl+N)\n" +
			"- `/sessions` — history chat (Ctrl+S)\n" +
			"- `/clear` — bersihin tampilan\n" +
			"- `/model` — info model\n" +
			"- `/blocks` — list code block di percakapan\n" +
			"- `/copy [N]` — copy block (default: terpanjang)\n" +
			"- `/save <file> [N]` — simpan block ke file\n" +
			"- `/saveall <prefix>` — save semua block\n- `/cd <path>` — pindah folder kerja\n- `/continue` — lanjut output yang kepotong\n" +
			"\n**File Browser:** `Ctrl+B` toggle sidebar · Tab pindah fokus · Enter buka · Backspace naik folder"})
	case "/new":
		m.newChat()
	case "/sessions":
		m.showHistory = true
		m.historyItems = listChats()
		m.historyCursor = 0
	case "/clear":
		m.messages = nil
	case "/model":
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: fmt.Sprintf("Provider: **%s** · Model: **%s**", m.provider.Name, m.provider.Model)})
	default:
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "Command gak dikenal: " + cmd})
	}
}

func (m *Model) handleCd(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant",
			Text: "**Cara pakai:** `/cd <path>`\nContoh: `/cd /sdcard/NetRatest1`\nPath saat ini: " + m.currentDir})
		return
	}
	path := args[1]
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.currentDir, path)
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Folder gak ketemu: " + path})
		return
	}
	m.currentDir = path
	m.fileCursor = 0
	m.refreshFiles()
	if !m.showFiles {
		m.showFiles = true
	}
	m.fileFocus = true
	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "✓ Pindah ke: " + path})
}

// handleContinue — kirim prompt buat AI lanjutin output yang kepotong.
func (m *Model) handleContinue() {
	if m.streaming {
		return
	}
	prompt := "Output kamu tadi kepotong di tengah. Lanjutkan PERSIS dari titik terakhir tanpa mengulang dari awal. Jangan basa-basi, langsung sambung kodenya."
	m.messages = append(m.messages, ChatMsg{Role: "user", Text: prompt})
	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: ""})
	m.streaming = true
	m.chatScroll = 0

	history := []Message{{Role: "system", Content: m.systemPrompt}}
	for _, cm := range m.messages {
		if cm.Role == "error" || cm.Text == "" {
			continue
		}
		history = append(history, Message{Role: cm.Role, Content: cm.Text})
	}
	if len(history) > 0 && history[len(history)-1].Role == "assistant" && history[len(history)-1].Content == "" {
		history = history[:len(history)-1]
	}
	ch := make(chan StreamChunk, 64)
	m.streamCh = ch
	go streamChat(m.provider, history, ch)
}

// handleSaveAll — (udah ada di v15, ini referensi)

func (m *Model) handleVendors() {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d provider tersedia:\n", len(m.allProviders)))
	for i, p := range m.allProviders {
		mark := "  "
		if i == m.activeIdx {
			mark = "> "
		}
		maxTok := p.MaxTokens
		if maxTok == 0 {
			maxTok = 2000
		}
		sb.WriteString(fmt.Sprintf("\n%s%d. %s  -  %s  (max %d tok)",
			mark, i, p.Name, p.Model, maxTok))
	}
	sb.WriteString("\n\nGanti pake: /vendor <nomor>  atau  /vendor <nama>")
	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: sb.String()})
}

func (m *Model) handleVendor(args []string) {
	if len(args) < 2 {
		m.handleVendors()
		return
	}
	target := args[1]
	idx := -1
	if n, err := strconv.Atoi(target); err == nil && n >= 0 && n < len(m.allProviders) {
		idx = n
	} else {
		for i, p := range m.allProviders {
			if strings.EqualFold(p.Name, target) {
				idx = i
				break
			}
		}
	}
	if idx == -1 {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Provider gak ketemu: " + target})
		return
	}
	m.activeIdx = idx
	m.provider = m.allProviders[idx]
	if cfg, err := loadConfig(); err == nil {
		// allProviders[0] = eliza-1966 (bukan bagian config)
		// config.ActiveProvider = idx - 1 (kalau idx > 0), else -1
		if idx == 0 {
			cfg.ActiveProvider = -1
		} else {
			cfg.ActiveProvider = idx - 1
		}
		cfg.Save()
	}

	// Kalo pindah ke ELIZA — kasih peringatan Weizenbaum
	var note string
	if strings.HasPrefix(strings.ToLower(m.provider.Name), "eliza") {
		note = fmt.Sprintf("Pindah ke **%s** (%s)\n\n%s", m.provider.Name, m.provider.Model, getWeizenbaumWarning())
	} else {
		note = fmt.Sprintf("Pindah ke **%s** (%s, max %d token)",
			m.provider.Name, m.provider.Model, m.provider.MaxTokens)
	}
	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: note})
}

func (m *Model) handleUnzip(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant",
			Text: "**Cara pakai:** `/unzip <file.zip> [target_dir]`\nOtomatis extract ke folder senama (file.zip -> file/)"})
		return
	}
	zipPath := args[1]
	if !filepath.IsAbs(zipPath) {
		zipPath = filepath.Join(m.currentDir, zipPath)
	}
	if _, err := os.Stat(zipPath); err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "File gak ketemu: " + zipPath})
		return
	}
	targetDir := strings.TrimSuffix(zipPath, filepath.Ext(zipPath))
	if len(args) >= 3 {
		targetDir = args[2]
		if !filepath.IsAbs(targetDir) {
			targetDir = filepath.Join(m.currentDir, targetDir)
		}
	}
	n, err := extractZip(zipPath, targetDir)
	if err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Extract gagal: " + err.Error()})
		return
	}
	m.currentDir = targetDir
	m.fileCursor = 0
	m.refreshFiles()
	if !m.showFiles {
		m.showFiles = true
	}
	m.fileFocus = true
	m.messages = append(m.messages, ChatMsg{Role: "assistant",
		Text: fmt.Sprintf("✓ Extract %d file ke %s", n, targetDir)})
}

func (m *Model) handleZip(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant",
			Text: "**Cara pakai:** `/zip <output.zip> [folder]`\nMax total 10MB. Folder default = currentDir."})
		return
	}
	outName := args[1]
	if !strings.HasSuffix(strings.ToLower(outName), ".zip") {
		outName += ".zip"
	}
	if !filepath.IsAbs(outName) {
		outName = filepath.Join(m.currentDir, outName)
	}
	srcDir := m.currentDir
	if len(args) >= 3 {
		srcDir = args[2]
		if !filepath.IsAbs(srcDir) {
			srcDir = filepath.Join(m.currentDir, srcDir)
		}
	}
	if info, err := os.Stat(srcDir); err != nil || !info.IsDir() {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Folder gak ketemu: " + srcDir})
		return
	}
	n, total, err := createZip(outName, srcDir, 10*1024*1024)
	if err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Zip gagal: " + err.Error()})
		return
	}
	m.refreshFiles()
	m.messages = append(m.messages, ChatMsg{Role: "assistant",
		Text: fmt.Sprintf("✓ %s\n- %d file, original %.2f MB (max 10MB)",
			filepath.Base(outName), n, float64(total)/(1024*1024))})
}

// extractZip — extract zip file dengan proteksi zip-slip.
func extractZip(zipPath, targetDir string) (int, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return 0, err
	}
	targetClean := filepath.Clean(targetDir) + string(os.PathSeparator)
	count := 0
	for _, f := range r.File {
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			continue
		}
		target := filepath.Join(targetDir, cleanName)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), targetClean) && filepath.Clean(target) != filepath.Clean(targetDir) {
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(out, rc)
		out.Close()
		rc.Close()
		count++
	}
	return count, nil
}

// createZip — bikin zip dari folder, max total original bytes.
func createZip(outPath, srcDir string, maxBytes int64) (int, int64, error) {
	out, err := os.Create(outPath)
	if err != nil {
		return 0, 0, err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()
	srcClean := filepath.Clean(srcDir)
	outClean := filepath.Clean(outPath)
	var count int
	var totalBytes int64
	filepath.Walk(srcClean, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Clean(path) == outClean {
			return nil
		}
		if info.Size() > 5*1024*1024 {
			return nil
		}
		if totalBytes+info.Size() > maxBytes {
			return filepath.SkipAll
		}
		relPath, err := filepath.Rel(srcClean, path)
		if err != nil {
			return nil
		}
		w, err := zw.Create(relPath)
		if err != nil {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		n, _ := io.Copy(w, f)
		f.Close()
		totalBytes += n
		count++
		return nil
	})
	return count, totalBytes, nil
}

func (m *Model) handleProject(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant",
			Text: "**Cara pakai:** `/project <nama>` atau `/project <nama>.zip`\n" +
				"Bikin folder <nama>/ + tulis semua file sesuai struktur [FILE: path] dari AI.\n" +
				"Kalo gak ada [FILE: path] di pesan, fallback ke .txt"})
		return
	}
	name := args[1]
	if !filepath.IsAbs(name) {
		name = filepath.Join(m.currentDir, name)
	}

	// Detect zip mode
	zipMode := strings.HasSuffix(strings.ToLower(name), ".zip")
	rootDir := name
	if zipMode {
		rootDir = strings.TrimSuffix(name, filepath.Ext(name))
	}

	files := extractProjectFiles(m.messages)
	if len(files) == 0 {
		// Fallback: pake extractCodeBlocks + ekstensi
		blocks := extractCodeBlocks(m.messages)
		langs := detectBlockLangs(m.messages)
		for i, b := range blocks {
			ext := ".txt"
			if i < len(langs) && langs[i] != "" {
				ext = extFromLang(langs[i])
			}
			files = append(files, ProjectFile{
				Path:    fmt.Sprintf("file_%d%s", i+1, ext),
				Content: b,
			})
		}
		if len(files) == 0 {
			m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Gak ada file code di percakapan ini."})
			return
		}
	}

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Mkdir gagal: " + err.Error()})
		return
	}

	var results []string
	for _, pf := range files {
		clean := filepath.Clean(pf.Path)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			results = append(results, "skip (path aneh): "+pf.Path)
			continue
		}
		full := filepath.Join(rootDir, clean)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			results = append(results, "x "+clean+": "+err.Error())
			continue
		}
		if err := os.WriteFile(full, []byte(pf.Content), 0644); err != nil {
			results = append(results, "x "+clean+": "+err.Error())
			continue
		}
		results = append(results, fmt.Sprintf("ok %-30s (%d byte)", clean, len(pf.Content)))
	}

	var note string
	if zipMode {
		n, total, err := createZip(name, rootDir, 10*1024*1024)
		if err != nil {
			results = append(results, "zip gagal: "+err.Error())
		} else {
			note = fmt.Sprintf("\n\n✓ Auto-zip: %s (%d file, %.2f KB)",
				filepath.Base(name), n, float64(total)/1024)
		}
	}

	m.currentDir = rootDir
	m.fileCursor = 0
	m.refreshFiles()
	if !m.showFiles {
		m.showFiles = true
	}
	m.fileFocus = true

	m.messages = append(m.messages, ChatMsg{Role: "assistant",
		Text: fmt.Sprintf("**Project: %s**\n%s%s", filepath.Base(rootDir), strings.Join(results, "\n"), note)})
}

// extractProjectFiles — scan [FILE: path]...[\/FILE] dari assistant messages.
func extractProjectFiles(msgs []ChatMsg) []ProjectFile {
	var files []ProjectFile
	const open = "[FILE:"
	const close = "[/FILE]"
	for _, m := range msgs {
		if m.Role == "user" || m.Role == "error" {
			continue
		}
		text := m.Text
		for {
			i := strings.Index(text, open)
			if i == -1 {
				break
			}
			j := strings.Index(text[i:], close)
			if j == -1 {
				break
			}
			// extract path
			headerEnd := strings.Index(text[i:], "]")
			if headerEnd == -1 {
				break
			}
			path := strings.TrimSpace(text[i+len(open) : i+headerEnd])
			path = strings.TrimSuffix(path, "]")
			bodyStart := i + headerEnd + 1
			bodyEnd := i + j
			body := text[bodyStart:bodyEnd]
			body = strings.TrimPrefix(body, "\n")
			body = strings.TrimLeft(body, "\r\n")
			body = strings.TrimRight(body, "\n\r")
			if path != "" {
				files = append(files, ProjectFile{Path: path, Content: body})
			}
			text = text[i+j+len(close):]
		}
	}
	return files
}

// handleTemplate — copy template siap-pakai ke currentDir.
// Usage: /template            → list
//
//	/template <nama>     → copy ke <currentDir>/<nama>.html
//	/template <nama> <file.html>  → copy dengan nama custom
func (m *Model) handleTemplate(args []string) {
	// Cari folder templates (relatif ke binary/exe location atau cwd)
	tmplDirs := []string{
		filepath.Join(m.currentDir, "templates"),
		"templates",
		filepath.Join(filepath.Dir(os.Args[0]), "templates"),
	}
	var tmplDir string
	for _, d := range tmplDirs {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			tmplDir = d
			break
		}
	}
	if tmplDir == "" {
		m.messages = append(m.messages, ChatMsg{Role: "error",
			Text: "Folder 'templates' gak ketemu. Bikin dulu di ~/netra-ai/templates/"})
		return
	}

	// List semua template
	entries, _ := os.ReadDir(tmplDir)
	var tmpls []string
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".html") || strings.HasSuffix(e.Name(), ".py")) {
			name := e.Name()
			for _, ext := range []string{".html", ".py"} {
				if strings.HasSuffix(name, ext) {
					tmpls = append(tmpls, strings.TrimSuffix(name, ext))
					break
				}
			}
		}
	}
	sort.Strings(tmpls)

	// Kalo cuma "/template" → list
	if len(args) < 2 {
		if len(tmpls) == 0 {
			m.messages = append(m.messages, ChatMsg{Role: "assistant",
				Text: "Belum ada template. Taruh file .html di " + tmplDir})
			return
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**%d template tersedia:**\n", len(tmpls)))
		for _, t := range tmpls {
			sb.WriteString(fmt.Sprintf("\n- `%s`", t))
		}
		sb.WriteString("\n\nPakai: `/template <nama>` → copy ke " + m.currentDir + "/<nama>.html")
		sb.WriteString("\nAtau: `/template <nama> <file.html>` → nama custom")
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: sb.String()})
		return
	}

	// Validate template name
	name := args[1]
	valid := false
	for _, t := range tmpls {
		if t == name {
			valid = true
			break
		}
	}
	if !valid {
		m.messages = append(m.messages, ChatMsg{Role: "error",
			Text: "Template gak ada: " + name + ". Pakai `/template` buat liat list."})
		return
	}

	srcPath := filepath.Join(tmplDir, name+".html")
	data, err := os.ReadFile(srcPath)
	if err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Baca template gagal: " + err.Error()})
		return
	}

	// Tentukan nama output
	outName := name + ".html"
	if len(args) >= 3 {
		outName = args[2]
	}
	outPath := outName
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(m.currentDir, outName)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Mkdir gagal: " + err.Error()})
		return
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Tulis gagal: " + err.Error()})
		return
	}

	m.refreshFiles()
	m.messages = append(m.messages, ChatMsg{Role: "assistant",
		Text: fmt.Sprintf("✓ Template '%s' disalin ke **%s** (%d byte)\n\nBuka di browser atau edit langsung.", name, outPath, len(data))})
}

func (m *Model) handleBlocks() {
	blocks := extractCodeBlocks(m.messages)
	if len(blocks) == 0 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "Gak ada code block."})
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%d code block di percakapan:**\n", len(blocks)))
	for i, b := range blocks {
		first := b
		if idx := strings.Index(first, "\n"); idx != -1 {
			first = first[:idx]
		}
		if len(first) > 50 {
			first = first[:47] + "..."
		}
		sb.WriteString(fmt.Sprintf("\n`#%d` — %d byte — `%s`", i+1, len(b), first))
	}
	sb.WriteString("\n\nPakai `/copy N` atau `/save <file> N`")
	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: sb.String()})
}

func (m *Model) handleCopy(args []string) {
	blocks := extractCodeBlocks(m.messages)
	if len(blocks) == 0 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "Gak ada code block."})
		return
	}
	idx := pickBlockIndex(args, blocks)
	chosen := blocks[idx]
	if err := copyToClipboard(chosen); err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Copy gagal: " + err.Error()})
		return
	}
	var note string
	if len(blocks) == 1 {
		note = fmt.Sprintf("✓ Code block (%d karakter) udah di clipboard.", len(chosen))
	} else {
		note = fmt.Sprintf("✓ Code block #%d dari %d (%d karakter) udah di clipboard.", idx+1, len(blocks), len(chosen))
	}
	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: note})
}

func (m *Model) handleSave(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "**Cara pakai:**\n- `/save namafile.py` — block terpanjang\n- `/save namafile.py 2` — block ke-2\n- Relatif ke: " + m.currentDir})
		return
	}
	path := args[1]
	// relatif ke currentDir
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.currentDir, path)
	}

	var pickArgs []string
	if len(args) >= 3 {
		pickArgs = []string{"/copy", args[2]}
	} else {
		pickArgs = []string{"/copy"}
	}

	blocks := extractCodeBlocks(m.messages)
	if len(blocks) == 0 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "Gak ada code block."})
		return
	}
	idx := pickBlockIndex(pickArgs, blocks)
	chosen := blocks[idx]

	// auto mkdir
	if dir := filepath.Dir(path); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	backupNote := ""
	if _, err := os.Stat(path); err == nil {
		bakPath := path + ".bak"
		if err := os.Rename(path, bakPath); err == nil {
			backupNote = " (backup: " + filepath.Base(bakPath) + ")"
		}
	}

	if err := os.WriteFile(path, []byte(chosen), 0644); err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error", Text: "Save gagal: " + err.Error()})
		return
	}
	m.messages = append(m.messages, ChatMsg{Role: "assistant",
		Text: fmt.Sprintf("✓ Disimpan: **%s** (%d byte)%s", path, len(chosen), backupNote)})
	m.refreshFiles()
}

func (m *Model) handleSaveAll(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant",
			Text: "**Cara pakai:**\n- `/saveall <prefix>` - save jadi prefix_1.ext, prefix_2.ext, ...\n- `/saveall <name>.zip` - save + auto-zip jadi <name>.zip"})
		return
	}
	prefix := args[1]
	if !filepath.IsAbs(prefix) {
		prefix = filepath.Join(m.currentDir, prefix)
	}

	// Detect zip mode
	zipMode := strings.HasSuffix(strings.ToLower(prefix), ".zip")
	var zipOutPath, tempDir string
	if zipMode {
		zipOutPath = prefix
		tempDir = strings.TrimSuffix(prefix, filepath.Ext(prefix))
		os.MkdirAll(tempDir, 0755)
		prefix = filepath.Join(tempDir, filepath.Base(tempDir))
	}

	blocks := extractCodeBlocks(m.messages)
	if len(blocks) == 0 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: "Gak ada code block."})
		return
	}

	langs := detectBlockLangs(m.messages)
	var saved []string
	for i, b := range blocks {
		ext := ".txt"
		if i < len(langs) && langs[i] != "" {
			ext = extFromLang(langs[i])
		}
		path := fmt.Sprintf("%s_%d%s", prefix, i+1, ext)
		if err := os.WriteFile(path, []byte(b), 0644); err != nil {
			saved = append(saved, fmt.Sprintf("x %s: %v", filepath.Base(path), err))
			continue
		}
		saved = append(saved, fmt.Sprintf("ok %s (%d byte)", filepath.Base(path), len(b)))
	}

	var note string
	if zipMode {
		n, total, err := createZip(zipOutPath, tempDir, 10*1024*1024)
		if err != nil {
			saved = append(saved, fmt.Sprintf("zip gagal: %v", err))
		} else {
			note = fmt.Sprintf("\n\n✓ Auto-zip: %s (%d file, %.2f MB)",
				filepath.Base(zipOutPath), n, float64(total)/(1024*1024))
		}
	}
	m.messages = append(m.messages, ChatMsg{Role: "assistant",
		Text: "**Saved:**\n" + strings.Join(saved, "\n") + note})
	m.refreshFiles()
}

func detectBlockLangs(msgs []ChatMsg) []string {
	var langs []string
	for _, m := range msgs {
		if m.Role == "user" || m.Role == "error" {
			continue
		}
		parts := strings.Split(m.Text, "```")
		for i, p := range parts {
			if i%2 == 1 {
				firstLine := p
				if idx := strings.Index(firstLine, "\n"); idx != -1 {
					firstLine = firstLine[:idx]
				}
				langs = append(langs, strings.TrimSpace(firstLine))
			}
		}
	}
	return langs
}

func extFromLang(lang string) string {
	switch strings.ToLower(lang) {
	case "python", "py":
		return ".py"
	case "javascript", "js":
		return ".js"
	case "typescript", "ts":
		return ".ts"
	case "go", "golang":
		return ".go"
	case "html":
		return ".html"
	case "css":
		return ".css"
	case "json":
		return ".json"
	case "yaml", "yml":
		return ".yaml"
	case "bash", "sh", "shell":
		return ".sh"
	case "c":
		return ".c"
	case "cpp", "c++":
		return ".cpp"
	case "java":
		return ".java"
	case "rust", "rs":
		return ".rs"
	case "sql":
		return ".sql"
	case "md", "markdown":
		return ".md"
	case "xml":
		return ".xml"
	case "php":
		return ".php"
	case "ruby", "rb":
		return ".rb"
	default:
		return ".txt"
	}
}

func pickBlockIndex(args []string, blocks []string) int {
	idx := 0
	auto := true
	if len(args) >= 2 {
		if n, err := strconv.Atoi(args[1]); err == nil && n >= 1 && n <= len(blocks) {
			idx = n - 1
			auto = false
		}
	}
	if auto {
		maxLen := 0
		for i, b := range blocks {
			if len(b) > maxLen {
				maxLen = len(b)
				idx = i
			}
		}
	}
	return idx
}

func (m Model) View() string {
	if !m.ready {
		return ""
	}
	w := m.width
	h := m.height
	if w < 40 {
		w = 40
	}
	if h < 10 {
		h = 10
	}

	if m.showHistory {
		return m.renderHistory(w, h)
	}

	// Welcome screen
	if m.isWelcome() && !m.showFiles {
		return m.renderWelcomeScreen(w, h)
	}

	// Chat layout:
	//   row 0..h-5  → chat panel (dengan file browser kalo aktif)
	//   h-4..h-2    → input box
	//   h-1         → footer bar
	chatH := h - 4
	if chatH < 4 {
		chatH = 4
	}

	var chatArea string
	if m.showFiles {
		sidebarW := 26
		if w < 80 {
			sidebarW = 22
		}
		chatW := w - sidebarW - 3
		if chatW < 30 {
			chatW = 30
		}
		sidebar := m.renderFileBrowser(sidebarW, chatH)
		chat := m.renderChatPane(chatW, chatH)
		chatArea = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, "  ", chat)
	} else {
		chatArea = m.renderChatPane(w, chatH)
	}

	inputBox := m.renderInputBar(w)

	// Footer bar
	footer := m.renderStatusBar(w)

	return chatArea + "\n" + inputBox + "\n" + footer
}

// renderInputBar — input box terpisah dgn background gelap + shadow.
func (m Model) renderInputBar(w int) string {
	boxW := w - 2
	if boxW < 30 {
		boxW = 30
	}

	attachLine := ""
	if len(m.attached) > 0 {
		var names []string
		for _, af := range m.attached {
			names = append(names, af.Name)
		}
		attachLine = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Render(
			" attach: "+strings.Join(names, ", ")+" · Ctrl+R") + "\n"
	}

	prompt := lipgloss.NewStyle().Foreground(lipgloss.Color("#9aa5ce")).Render("> ")
	display := m.input
	cursor := lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true).Render("█")
	if m.inputCursor <= len(display) {
		display = display[:m.inputCursor] + cursor + display[m.inputCursor:]
	}
	placeholder := ""
	if len(m.input) == 0 {
		placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render("tulis pesan...")
	}

	content := attachLine + prompt + styleText.Render(display) + placeholder

	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#5a7ab8")).
		Background(lipgloss.Color("#1a1a24")).
		Padding(0, 1).
		Width(boxW - 4)

	return box.Render(content)
}

// renderStatusBar — bar info di bawah: kiri mode, kanan model.
func (m Model) renderStatusBar(w int) string {
	var left string
	if m.streaming {
		left = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true).Render("◐ streaming")
	} else if m.fileFocus {
		left = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")).Render("● files  ·  Tab  ·  Esc")
	} else {
		left = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render("enter send")
	}
	right := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render(m.provider.Model + "  ")

	pad := w - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

// renderWelcomeScreen — layout opencode: logo + input box di tengah + footer.
func (m Model) renderWelcomeScreen(w, h int) string {
	logo := []string{
		`███    ██ ███████ ████████ ██████   █████  `,
		`████   ██ ██         ██    ██   ██ ██   ██ `,
		`██ ██  ██ █████      ██    ██████  ███████ `,
		`██  ██ ██ ██         ██    ██   ██ ██   ██ `,
		`██   ████ ███████    ██    ██   ██ ██   ██ `,
	}
	gradient := []lipgloss.Color{
		lipgloss.Color("#f0f0f0"), lipgloss.Color("#d8d8d8"),
		lipgloss.Color("#c0c0c0"), lipgloss.Color("#a8a8a8"),
		lipgloss.Color("#909090"),
	}
	var logoLines []string
	for i, l := range logo {
		logoLines = append(logoLines, lipgloss.NewStyle().Foreground(gradient[i]).Bold(true).Render(l))
	}

	// Shadow: 1 baris abu gelap di bawah logo, geser 1 kolom
	shadowClr := lipgloss.Color("#404040")
	shadowLine := " " + strings.Repeat("▄", 45)
	shadow := lipgloss.NewStyle().Foreground(shadowClr).Render(shadowLine)

	logoBlock := strings.Join(logoLines, "\n") + "\n" + shadow

	boxW := w - 16
	if boxW < 50 {
		boxW = 50
	}
	if boxW > 90 {
		boxW = 90
	}
	prompt := styleText.Bold(true).Render("> ")
	display := m.input
	if m.inputCursor <= len(display) {
		display = display[:m.inputCursor] + styleCursor.Render("█") + display[m.inputCursor:]
	}
	placeholder := ""
	if len(m.input) == 0 {
		placeholder = styleDim.Render("Tanya apa aja...")
	}
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(clrDim).
		Padding(0, 1).
		Width(boxW - 4).
		Render(" " + prompt + styleText.Render(display) + placeholder)

	hintLine := "  " + styleDim.Render("enter") + " kirim   " +
		styleDim.Render("ctrl+b") + " files   " +
		styleDim.Render("ctrl+s") + " sessions   " +
		styleDim.Render("esc") + " quit"

	infoLines := []string{
		"  " + styleDim.Render("provider  ") + styleText.Render(m.provider.Name),
		"  " + styleDim.Render("model     ") + styleText.Render(m.provider.Model),
		"  " + styleDim.Render("cwd       ") + styleMuted.Render(m.currentDir),
	}
	infoPanel := strings.Join(infoLines, "\n")

	middle := lipgloss.JoinVertical(lipgloss.Center,
		"", logoBlock, "", styleMuted.Render("netra-ai · v0.1.0"),
		"", "", inputBox, hintLine, "", "", infoPanel,
	)

	topPad := (h - lipgloss.Height(middle) - 2) / 2
	if topPad < 1 {
		topPad = 1
	}
	body := strings.Repeat("\n", topPad) + lipgloss.PlaceHorizontal(w, lipgloss.Center, middle)

	bodyLines := strings.Count(body, "\n") + 1
	if bodyLines > h-1 {
		body = strings.Join(strings.Split(body, "\n")[:h-1], "\n")
	}
	rem := h - 1 - (strings.Count(body, "\n") + 1)
	if rem > 0 {
		body += strings.Repeat("\n", rem)
	}

	status := "● ready"
	if m.streaming {
		status = "◐ streaming..."
	}
	left := "  " + styleAccent.Bold(true).Render("netra-ai")
	right := styleAccent2.Render(status) + "  "
	pad := w - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	footer := left + strings.Repeat(" ", pad) + right
	footerStyled := lipgloss.NewStyle().Background(clrPanelBg2).Width(w).Render(footer)
	return body + "\n" + footerStyled
}

// shadowPanel — bungkus panel dgn shadow bawah buat efek 3D.
func shadowPanel(panel string, shadowClr lipgloss.Color) string {
	lines := strings.Split(panel, "\n")
	if len(lines) == 0 {
		return panel
	}
	w := lipgloss.Width(lines[0])
	if w < 4 {
		return panel
	}
	// baris shadow: ▌ offset 1 kolom dari kiri, panjang w-2
	shadowLine := " " + strings.Repeat("▀", w-2)
	shadow := lipgloss.NewStyle().Foreground(shadowClr).Render(shadowLine)
	return panel + "\n" + shadow
}

// formatSize — 1234 → "1.2 KB", 1234567 → "1.2 MB"
func formatSize(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%4d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%5.1f KB", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%5.1f MB", float64(b)/(1024*1024))
	}
	return fmt.Sprintf("%5.1f GB", float64(b)/(1024*1024*1024))
}

func (m Model) renderFileBrowser(w, h int) string {
	innerW := w - 4
	innerH := h - 2

	var lines []string

	// Header
	headIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true).Render("❐")
	headText := lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true).Render(" FILES")
	lines = append(lines, " "+headIcon+headText)
	lines = append(lines, "")

	// Path
	crumb := m.currentDir
	if len([]rune(crumb)) > innerW-2 {
		r := []rune(crumb)
		crumb = "…" + string(r[len(r)-(innerW-3):])
	}
	lines = append(lines, " "+lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render(crumb))
	lines = append(lines, "")

	// Kolom adaptif — kalo sidebar sempit, size disembunyiin
	showSize := innerW >= 26
	sizeColW := 6
	nameColW := innerW - 4
	if showSize {
		nameColW = innerW - sizeColW - 3
	}
	if nameColW < 8 {
		nameColW = 8
	}

	hdrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Bold(true)
	var header string
	if showSize {
		header = fmt.Sprintf(" %-*s %*s", nameColW, "NAME", sizeColW, "SIZE")
	} else {
		header = fmt.Sprintf(" %-*s", nameColW, "NAME")
	}
	lines = append(lines, hdrStyle.Render(header))
	lines = append(lines, " "+lipgloss.NewStyle().Foreground(lipgloss.Color("#414868")).Render(strings.Repeat("─", innerW-1)))
	lines = append(lines, "")

	// List
	maxRows := innerH - 10
	if maxRows < 1 {
		maxRows = 1
	}
	for i, fe := range m.fileEntries {
		if i >= maxRows {
			break
		}
		isCursor := i == m.fileCursor && m.fileFocus

		icon := " "
		name := fe.Name
		if fe.IsDir {
			icon = "▸"
			name = fe.Name + "/"
		}

		// Truncate name — pakai rune biar gak motong char aneh
		runes := []rune(name)
		maxNameLen := nameColW - 2
		if maxNameLen < 4 {
			maxNameLen = 4
		}
		if len(runes) > maxNameLen {
			name = string(runes[:maxNameLen-1]) + "…"
		}

		sizeStr := ""
		if fe.IsDir {
			sizeStr = "DIR"
		} else {
			sizeStr = formatSizeShort(fe.Size)
		}

		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5"))
		if fe.IsDir {
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff")).Bold(true)
		}
		sizeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9aa5ce"))

		if isCursor {
			bg := lipgloss.NewStyle().Background(lipgloss.Color("#2a2a3e"))
			marker := lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7")).Bold(true).Render("❯")
			namePart := fmt.Sprintf("%-*s", nameColW, icon+" "+name)
			content := nameStyle.Render(namePart)
			if showSize {
				sizePart := fmt.Sprintf("%*s", sizeColW, sizeStr)
				content += " " + sizeStyle.Render(sizePart)
			}
			pad := innerW - lipgloss.Width(content) - 2
			if pad > 0 {
				content += strings.Repeat(" ", pad)
			}
			lines = append(lines, " "+marker+" "+bg.Render(" "+content+" "))
		} else {
			namePart := fmt.Sprintf("%-*s", nameColW, icon+" "+name)
			line := "   " + nameStyle.Render(namePart)
			if showSize {
				sizePart := fmt.Sprintf("%*s", sizeColW, sizeStr)
				line += " " + sizeStyle.Render(sizePart)
			}
			lines = append(lines, line)
		}
	}
	if len(m.fileEntries) == 0 {
		lines = append(lines, "   "+lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Italic(true).Render("(kosong)"))
	}

	for len(lines) < innerH-3 {
		lines = append(lines, "")
	}
	modeLabel := "chat"
	if m.fileFocus {
		modeLabel = "FILES"
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render(
		" " + modeLabel + "  ·  Tab  ·  Esc")
	lines = append(lines, "")
	lines = append(lines, footer)

	borderClr := lipgloss.Color("#414868")
	if m.fileFocus {
		borderClr = lipgloss.Color("#7aa2f7")
	}
	bgClr := lipgloss.Color("#16161e")
	if m.fileFocus {
		bgClr = lipgloss.Color("#1a1a2a")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderClr).
		Background(bgClr).
		Padding(0, 1).
		Width(w - 2).
		Height(innerH)
	return box.Render(strings.Join(lines, "\n"))
}

// formatSizeShort — versi singkat tanpa padding
func formatSizeShort(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(b)/(1024*1024))
	}
	return fmt.Sprintf("%.1fG", float64(b)/(1024*1024*1024))
}

// glowBox — panel dengan border gradient 4 warna + shadow terang di bawah.
// Atas: ungu, Kanan: biru, Bawah: teal, Kiri: pink.
// Shadow di bawah = garis terang biar keliatan ngambang.
func glowBox(width, height int, content string, focused bool) string {
	borderColorTop := clrNeon1
	borderColorRight := clrNeon2
	borderColorBottom := clrNeon3
	borderColorLeft := clrNeon4
	if !focused {
		borderColorTop = clrBorder
		borderColorRight = clrBorder
		borderColorBottom = clrBorder
		borderColorLeft = clrBorder
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderTopForeground(borderColorTop).
		BorderRightForeground(borderColorRight).
		BorderBottomForeground(borderColorBottom).
		BorderLeftForeground(borderColorLeft).
		Width(width).
		Height(height)

	out := box.Render(content)

	// Glow shadow: garis terang di bawah
	if focused {
		w := lipgloss.Width(out)
		if w > 4 {
			shadow := lipgloss.NewStyle().
				Foreground(clrNeonShadow).
				Render(" " + strings.Repeat("▁", w-2))
			out += "\n" + shadow
		}
	}
	return out
}

func (m Model) renderWelcome(w, h int) string {
	logo := []string{
		"███╗   ██╗███████╗████████╗██████╗  █████╗ ",
		"████╗  ██║██╔════╝╚══██╔══╝██╔══██╗██╔══██╗",
		"██╔██╗ ██║█████╗     ██║   ██████╔╝███████║",
		"██║╚██╗██║███████╗   ██║   ██╔══██╗██║  ██║",
		"╚═╝ ╚═╝╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝",
	}
	gradient := []lipgloss.Color{
		lipgloss.Color("#c4b5fd"),
		lipgloss.Color("#a78bfa"),
		lipgloss.Color("#8b5cf6"),
		lipgloss.Color("#7aa2f7"),
		lipgloss.Color("#5b8def"),
	}
	var logoLines []string
	for i, l := range logo {
		logoLines = append(logoLines, lipgloss.NewStyle().Foreground(gradient[i]).Bold(true).Render(l))
	}
	// neon shadow di bawah logo
	logoWidth := 0
	for _, l := range logo {
		if w := lipgloss.Width(l); w > logoWidth {
			logoWidth = w
		}
	}
	shadow := lipgloss.NewStyle().Foreground(clrNeonShadow).Bold(true).
		Render(strings.Repeat("▁", logoWidth))
	logoLines = append(logoLines, shadow)
	logoBlock := strings.Join(logoLines, "\n")
	version := styleDim.Render("  netra-ai · v0.1.0")

	type cmdRow struct{ cmd, desc, key string }
	cmds := []cmdRow{
		{"/help", "show help", "ctrl+h"},
		{"/new", "new chat", "ctrl+n"},
		{"/sessions", "list sessions", "ctrl+s"},
		{"/blocks", "list code blocks", ""},
		{"/copy", "copy code ke clipboard", ""},
		{"/save", "save code ke file", ""},
		{"/saveall", "save semua block", ""},
		{"/continue", "lanjut output kepotong", ""},
		{"files", "toggle file browser", "ctrl+b"},
	}
	var cmdLines []string
	for _, c := range cmds {
		cmdLines = append(cmdLines,
			styleAccent.Render(fmt.Sprintf("%-12s", c.cmd))+
				styleMuted.Render(fmt.Sprintf("%-24s", c.desc))+
				styleDim.Render(c.key))
	}
	cmdsBlock := strings.Join(cmdLines, "\n")

	inner := lipgloss.JoinVertical(lipgloss.Left,
		"", logoBlock, version, "", "", "  "+strings.ReplaceAll(cmdsBlock, "\n", "\n  "),
	)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, inner)
}

func (m Model) renderChatPane(w, h int) string {
	innerW := w - 4
	if innerW < 30 {
		innerW = 30
	}
	var lines []string

	for _, msg := range m.messages {
		var dot, roleName string
		var roleStyle, dotStyle lipgloss.Style
		switch msg.Role {
		case "user":
			dotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))
			roleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")).Bold(true)
			dot = "●"
			roleName = "you"
		case "error":
			dotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
			roleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Bold(true)
			dot = "●"
			roleName = "error"
		default:
			dotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))
			roleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7")).Bold(true)
			dot = "●"
			roleName = "netra"
		}

		lines = append(lines, "")
		// Header: "  ● you"
		lines = append(lines, "  "+dotStyle.Render(dot)+" "+roleStyle.Render(roleName))
		lines = append(lines, "")

		// Attachments badge (kalo ada) — display only
		if len(msg.Attachments) > 0 {
			for _, name := range msg.Attachments {
				sizeStr := ""
				if info, err := os.Stat(name); err == nil {
					sizeStr = fmt.Sprintf(" (%.1f KB)", float64(info.Size())/1024)
				}
				lines = append(lines, "    "+lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Render("📎 "+name)+
					lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render(sizeStr))
			}
			lines = append(lines, "")
		}

		// Isi (pake Text — bukan APIContent)
		segments := splitCodeBlocks(msg.Text)
		anyContent := false
		for _, seg := range segments {
			if seg.IsCode {
				anyContent = true
				label := seg.Lang
				if label == "" {
					label = "code"
				}

				// Box code
				boxW := innerW - 6
				if boxW < 20 {
					boxW = 20
				}
				head := "─ " + label + " "
				fill := boxW - len([]rune(head)) - 1
				if fill < 1 {
					fill = 1
				}
				lines = append(lines, "    "+lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render("╭"+head+strings.Repeat("─", fill)+"╮"))

				codeMaxW := boxW - 8
				if codeMaxW < 20 {
					codeMaxW = 20
				}
				inStyle := false
				inScript := false
				baseLang := strings.ToLower(seg.Lang)

				for i, cl := range strings.Split(seg.Text, "\n") {
					// Detect CSS/JS block
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
						var numStr string
						if k == 0 {
							numStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#414868")).Render(fmt.Sprintf("%3d │ ", i+1))
						} else {
							numStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#414868")).Render("    │ ")
						}
						rendered := indentAndHighlight(wl, lineLang)
						lines = append(lines, "    "+lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render("│ ")+numStr+rendered)
					}
				}
				lines = append(lines, "    "+lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render("╰"+strings.Repeat("─", boxW-1)+"╯"))
				lines = append(lines, "")
			} else {
				clean := strings.ReplaceAll(seg.Text, "**", "")
				wrapped := wrapText(clean, innerW-4)
				for _, wl := range wrapped {
					if strings.TrimSpace(wl) == "" {
						lines = append(lines, "")
						continue
					}
					anyContent = true
					lines = append(lines, "    "+styleText.Render(wl))
				}
			}
		}
		if !anyContent && m.streaming {
			lines = append(lines, "    "+lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Italic(true).Render("(thinking...)"))
		}
	}

	total := len(lines)
	avail := h
	maxScroll := 0
	if total > avail {
		maxScroll = total - avail
	}
	if m.chatScroll > maxScroll {
		m.chatScroll = maxScroll
	}
	if m.chatScroll < 0 {
		m.chatScroll = 0
	}
	if total > avail {
		end := total - m.chatScroll
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

func (m Model) renderInput(w int) string {
	if m.choiceActive && len(m.choices) > 0 {
		var box strings.Builder
		for i, c := range m.choices {
			letter := string(rune('A' + i))
			cursor := "  "
			label := styleMuted.Render(c)
			if i == m.choiceCursor {
				cursor = styleCursor.Render("▸ ")
				label = styleText.Render(c)
			}
			box.WriteString(fmt.Sprintf("  %s%s. %s\n", cursor, styleAccent.Render(letter), label))
		}
		box.WriteString("  " + styleDim.Render("↑↓ pilih · Enter kirim · 1-9 pilih · Esc batal"))
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(clrAccent).
			Padding(0, 1).
			Width(w - 4)
		return boxStyle.Render(strings.TrimRight(box.String(), "\n"))
	}
	prompt := styleAccent.Bold(true).Render("> ")
	display := m.input
	if m.inputCursor <= len(display) {
		display = display[:m.inputCursor] + styleCursor.Render("█") + display[m.inputCursor:]
	}
	placeholder := ""
	if len(m.input) == 0 {
		placeholder = styleDim.Render("tulis pesan...")
	}
	attachLine := ""
	if len(m.attached) > 0 {
		var names []string
		for _, af := range m.attached {
			names = append(names, af.Name)
		}
		attachLine = styleAccent2.Render("  [attach] "+strings.Join(names, ", ")+"  ·  Ctrl+R") + "\n"
	}
	content := attachLine + prompt + styleText.Render(display) + placeholder
	boxW := w - 4
	if boxW < 20 {
		boxW = 20
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(clrAccent).
		Padding(0, 1).
		Width(boxW - 4)
	return box.Render(content)
}

func (m Model) renderFooter(w int) string {
	var left string
	if m.streaming {
		left = styleAccent2.Render("◐ streaming...")
	} else {
		left = styleDim.Render("enter send")
	}
	right := styleDim.Render(m.provider.Model)
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	pad := w - leftW - rightW
	if pad < 1 {
		pad = 1
	}
	line := left + strings.Repeat(" ", pad) + right
	return lipgloss.NewStyle().Background(clrPanelBg2).Width(w).Render(line)
}

func (m Model) renderHistory(w, h int) string {
	var lines []string
	lines = append(lines, "")
	lines = append(lines, "  "+styleAccent.Bold(true).Render("Sessions"))
	lines = append(lines, "")
	lines = append(lines, "  "+styleDim.Render("↑↓ pilih · Enter buka · N baru · Esc tutup"))
	lines = append(lines, "")
	for i, c := range m.historyItems {
		if i >= h-8 {
			break
		}
		bullet := styleDim.Render("·")
		label := styleMuted.Render(c.Title)
		if i == m.historyCursor {
			bullet = styleAccent.Render("▸")
			label = styleText.Render(c.Title)
		}
		lines = append(lines, "  "+bullet+" "+label)
	}
	if len(m.historyItems) == 0 {
		lines = append(lines, "  "+styleDim.Render("(belum ada history)"))
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// --- Helpers ---

type segment struct {
	IsCode bool
	Lang   string
	Text   string
}

func splitCodeBlocks(s string) []segment {
	// Pre-process: ubah [FILE: path]...[/FILE] jadi ```path ... ```
	// biar parser existing bisa handle
	s = convertFileMarkers(s)

	var segs []segment
	parts := strings.Split(s, "```")
	for i, p := range parts {
		if p == "" {
			continue
		}
		isCode := i%2 == 1
		text := p
		lang := ""
		if isCode {
			if idx := strings.Index(text, "\n"); idx != -1 {
				lang = strings.TrimSpace(text[:idx])
				text = text[idx+1:]
			}
			text = strings.TrimRight(text, "\n")
		}
		segs = append(segs, segment{IsCode: isCode, Lang: lang, Text: text})
	}
	return segs
}

// convertFileMarkers — ubah [FILE: path]...[/FILE] jadi ```lang ... ```
// biar bisa masuk pipeline highlighting yang sama.
func convertFileMarkers(s string) string {
	if !strings.Contains(s, "[FILE:") {
		return s
	}
	var out strings.Builder
	rest := s
	for {
		i := strings.Index(rest, "[FILE:")
		if i == -1 {
			out.WriteString(rest)
			break
		}
		// tulis bagian sebelum marker
		out.WriteString(rest[:i])
		rest = rest[i:]

		// cari akhir header "]"
		headEnd := strings.Index(rest, "]")
		if headEnd == -1 {
			out.WriteString(rest)
			break
		}
		path := strings.TrimSpace(rest[len("[FILE:"):headEnd])
		rest = rest[headEnd+1:]

		// cari [/FILE]
		end := strings.Index(rest, "[/FILE]")
		if end == -1 {
			// gak ada close, ambil sampe akhir
			out.WriteString("```" + langFromPath(path) + "\n")
			out.WriteString(rest)
			break
		}
		body := rest[:end]
		rest = rest[end+len("[/FILE]"):]

		// Konversi jadi fenced block
		out.WriteString("\n```" + langFromPath(path) + "\n")
		out.WriteString(strings.Trim(body, "\n"))
		out.WriteString("\n```\n")
	}
	return out.String()
}

// langFromPath — tebak bahasa dari ekstensi file.
func langFromPath(p string) string {
	p = strings.ToLower(p)
	switch {
	case strings.HasSuffix(p, ".py"):
		return "python"
	case strings.HasSuffix(p, ".js"):
		return "javascript"
	case strings.HasSuffix(p, ".ts"):
		return "typescript"
	case strings.HasSuffix(p, ".go"):
		return "go"
	case strings.HasSuffix(p, ".html") || strings.HasSuffix(p, ".htm"):
		return "html"
	case strings.HasSuffix(p, ".css"):
		return "css"
	case strings.HasSuffix(p, ".json"):
		return "json"
	case strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml"):
		return "yaml"
	case strings.HasSuffix(p, ".sh"):
		return "bash"
	case strings.HasSuffix(p, ".c"):
		return "c"
	case strings.HasSuffix(p, ".cpp") || strings.HasSuffix(p, ".cc"):
		return "cpp"
	case strings.HasSuffix(p, ".java"):
		return "java"
	case strings.HasSuffix(p, ".rs"):
		return "rust"
	case strings.HasSuffix(p, ".sql"):
		return "sql"
	case strings.HasSuffix(p, ".php"):
		return "php"
	case strings.HasSuffix(p, ".rb"):
		return "ruby"
	case strings.HasSuffix(p, ".md"):
		return "markdown"
	case strings.HasSuffix(p, ".xml"):
		return "xml"
	}
	return "text"
}

func extractCodeBlocks(msgs []ChatMsg) []string {
	var blocks []string
	for _, m := range msgs {
		if m.Role == "user" || m.Role == "error" {
			continue
		}
		parts := strings.Split(m.Text, "```")
		for i, p := range parts {
			if i%2 == 1 {
				text := p
				if idx := strings.Index(text, "\n"); idx != -1 {
					text = text[idx+1:]
				}
				text = strings.TrimRight(text, "\n")
				if strings.TrimSpace(text) != "" {
					blocks = append(blocks, text)
				}
			}
		}
	}
	return blocks
}

// parseChoiceBlock — deteksi [CHOICE]...[/CHOICE] di response AI.
// Return: teks tanpa block CHOICE, + slice pilihan.

// parseBulletFallback — kalo AI gak pake [CHOICE], coba detect bullet list
// di akhir pesan ("- opsi", "* opsi", "1. opsi") minimal 2 item berurutan.
func parseBulletFallback(text string) (string, []string) {
	lines := strings.Split(text, "\n")
	// scan dari bawah, kumpulin bullet sampai ketemu non-bullet (skip baris kosong)
	var bullets []string
	var startIdx = -1
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			if len(bullets) > 0 {
				break
			}
			continue
		}
		isBullet := false
		var content string
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "• ") {
			isBullet = true
			content = strings.TrimSpace(line[2:])
			if strings.HasPrefix(line, "• ") {
				content = strings.TrimSpace(line[len("• "):])
			}
		} else if len(line) >= 3 && (line[1] == '.' || line[1] == ')') && line[0] >= '1' && line[0] <= '9' {
			isBullet = true
			content = strings.TrimSpace(line[2:])
		} else if len(line) >= 3 && (line[1] == '.' || line[1] == ')') && line[0] >= 'A' && line[0] <= 'Z' {
			isBullet = true
			content = strings.TrimSpace(line[2:])
		}
		if isBullet {
			bullets = append([]string{content}, bullets...)
			startIdx = i
		} else {
			break
		}
	}
	if len(bullets) < 2 {
		return text, nil
	}

	// buang bullet dari teks
	newLines := append([]string{}, lines[:startIdx]...)
	cleaned := strings.Join(newLines, "\n")
	cleaned = strings.TrimRight(cleaned, " \n\t")
	return cleaned, bullets
}

func parseChoiceBlock(text string) (string, []string) {
	startMarker := "[CHOICE]"
	endMarker := "[/CHOICE]"
	start := strings.Index(text, startMarker)
	if start == -1 {
		// FALLBACK: detect bullet list di akhir pesan
		return parseBulletFallback(text)
	}
	end := strings.Index(text[start:], endMarker)
	if end == -1 {
		return text, nil
	}
	end += start + len(endMarker)

	// ambil isi antara marker
	inside := text[start+len(startMarker) : end-len(endMarker)]
	inside = strings.TrimSpace(inside)

	var choices []string
	for _, line := range strings.Split(inside, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// format: "A. deskripsi" atau "A) deskripsi" atau "- deskripsi"
		if len(line) >= 3 && (line[1] == '.' || line[1] == ')') {
			choices = append(choices, strings.TrimSpace(line[2:]))
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			choices = append(choices, strings.TrimSpace(line[2:]))
		} else {
			choices = append(choices, line)
		}
	}

	// hapus block CHOICE dari teks
	cleaned := (text[:start] + text[end:])
	cleaned = strings.TrimRight(cleaned, " \n\t")
	return cleaned, choices
}

func wrapCodeLine(line string, maxW int) []string {
	if len(line) == 0 {
		return []string{""}
	}
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	indentLen := len(indent)
	if indentLen >= maxW-10 {
		indentLen = 0
		indent = ""
	}
	contentW := maxW - indentLen
	if contentW < 15 {
		contentW = 15
	}
	words := strings.Fields(trimmed)
	if len(words) == 0 {
		return []string{line}
	}
	var out []string
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
		} else if len(cur)+1+len(w) <= contentW {
			cur += " " + w
		} else {
			out = append(out, indent+cur)
			cur = w
		}
	}
	if cur != "" {
		out = append(out, indent+cur)
	}
	return out
}

func wrapText(s string, maxW int) []string {
	if maxW < 5 {
		maxW = 5
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		cur := ""
		for _, w := range words {
			if cur == "" {
				cur = w
			} else if len([]rune(cur))+1+len([]rune(w)) <= maxW {
				cur += " " + w
			} else {
				out = append(out, cur)
				cur = w
			}
		}
		if cur != "" {
			out = append(out, cur)
		}
	}
	return out
}

var _ = time.Now

func main() {
	// CLI mode: netra-ai build-apk <url> <name>
	if len(os.Args) > 1 && os.Args[1] == "build-apk" {
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Pakai: netra-ai build-apk <url> <nama>")
			os.Exit(1)
		}
		logStr, err := cmdBuildAPK(os.Args[2], os.Args[3])
		fmt.Println(logStr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	model, err := initialModel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		time.Sleep(time.Second)
		os.Exit(1)
	}
}
