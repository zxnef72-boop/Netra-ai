// reader.go — /baca <url> — fetch halaman web, extract teks, tampilin
package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Regex buat strip HTML → text
var (
	reScript     = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle      = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reNav        = regexp.MustCompile(`(?is)<nav[^>]*>.*?</nav>`)
	reHeader     = regexp.MustCompile(`(?is)<header[^>]*>.*?</header>`)
	reFooter     = regexp.MustCompile(`(?is)<footer[^>]*>.*?</footer>`)
	reAside      = regexp.MustCompile(`(?is)<aside[^>]*>.*?</aside>`)
	reComment    = regexp.MustCompile(`(?s)<!--.*?-->`)
	reTag        = regexp.MustCompile(`<[^>]+>`)
	reWhitespace = regexp.MustCompile(`\s+`)
	reTitle      = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reEntitiesR  = strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">",
		"&quot;", "\"", "&#39;", "'", "&#x27;", "'",
		"&nbsp;", " ", "&mdash;", "—", "&ndash;", "–",
		"&hellip;", "…", "&copy;", "©",
	)
)

// extractText — konversi HTML → plain text
func extractText(html string) string {
	html = reScript.ReplaceAllString(html, " ")
	html = reStyle.ReplaceAllString(html, " ")
	html = reNav.ReplaceAllString(html, " ")
	html = reHeader.ReplaceAllString(html, " ")
	html = reFooter.ReplaceAllString(html, " ")
	html = reAside.ReplaceAllString(html, " ")
	html = reComment.ReplaceAllString(html, " ")
	html = reTag.ReplaceAllString(html, " ")
	html = reEntitiesR.Replace(html)
	html = reWhitespace.ReplaceAllString(html, " ")
	return strings.TrimSpace(html)
}

// fetchURL — ambil isi URL, return (title, textContent, error)
func fetchURL(url string) (string, string, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Timeout: 20 * time.Second, Transport: tr}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "id,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", "", err
	}

	html := string(body)

	title := ""
	if m := reTitle.FindStringSubmatch(html); m != nil {
		title = strings.TrimSpace(reEntitiesR.Replace(m[1]))
	}

	text := extractText(html)
	return title, text, nil
}

// handleBaca — /baca <url>
func (m *Model) handleBaca(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant",
			Text: "**Cara pakai:** `/baca <url>`\n\n**Contoh:**\n- `/baca https://example.com`\n- `/baca wikipedia.org/wiki/Go`\n\n" +
				"Aku bakal fetch halaman, extract teks, tampilin di sini."})
		return
	}

	url := args[1]
	m.messages = append(m.messages, ChatMsg{Role: "user", Text: "📖 " + url})

	title, content, err := fetchURL(url)
	if err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error",
			Text: fmt.Sprintf("Gagal fetch: %v", err)})
		return
	}

	maxLen := 3000
	truncated := false
	if len(content) > maxLen {
		content = content[:maxLen]
		truncated = true
	}

	var sb strings.Builder
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true)
	urlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))

	if title != "" {
		sb.WriteString(titleStyle.Render("📄 "+title) + "\n")
	}
	sb.WriteString(urlStyle.Render(url) + "\n\n")
	sb.WriteString("```\n")
	sb.WriteString(content)
	if truncated {
		sb.WriteString(fmt.Sprintf("\n\n... (dipotong, total %d char)", len(content)))
	}
	sb.WriteString("\n```")

	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: sb.String()})
}
