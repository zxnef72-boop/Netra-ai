// search.go — /search <query> via DuckDuckGo HTML (no API key)
package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

var (
	// Lite endpoint pakai <a class="result-link"> bukan <a class="result__a">
	reDDGResult  = regexp.MustCompile(`(?s)<a[^>]+href="([^"]+)"[^>]+class=['"]result-link['"][^>]*>(.*?)</a>`)
	reDDGSnippet = regexp.MustCompile(`(?s)<td[^>]+class=['"]result-snippet['"][^>]*>(.*?)</td>`)
	reSearchTag  = regexp.MustCompile(`<[^>]+>`)
	reEntities   = strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">",
		"&quot;", "\"", "&#x27;", "'", "&#39;", "'",
		"&nbsp;", " ",
	)
)

func stripHTML(s string) string {
	s = reSearchTag.ReplaceAllString(s, "")
	s = reEntities.Replace(s)
	return strings.TrimSpace(s)
}

func ddgCleanURL(raw string) string {
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if strings.Contains(u.Host, "duckduckgo.com") && u.Path == "/l/" {
		if real := u.Query().Get("uddg"); real != "" {
			return real
		}
	}
	return raw
}

func ddgSearch(query string, maxResults int) ([]searchResult, error) {
	form := url.Values{}
	form.Set("q", query)

	// Endpoint lite — lebih simpel, jarang kena bot detect
	req, err := http.NewRequest("GET",
		"https://lite.duckduckgo.com/lite/?"+form.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Referer", "https://lite.duckduckgo.com/")
	req.Header.Set("DNT", "1")

	// Skip TLS verify — ISP Indonesia (Indosat/Telkomsel) suka intercept HTTPS
	// Ini cuma search query publik, aman.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Timeout: 15 * time.Second, Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// DDG lite kadang balikin 202 (Accepted) — tetep valid
	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		return nil, fmt.Errorf("DDG balikin HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	html := string(body)

	titles := reDDGResult.FindAllStringSubmatch(html, -1)
	snippets := reDDGSnippet.FindAllStringSubmatch(html, -1)

	var results []searchResult
	for i, m := range titles {
		if i >= maxResults {
			break
		}
		r := searchResult{
			URL:   ddgCleanURL(m[1]),
			Title: stripHTML(m[2]),
		}
		if i < len(snippets) {
			r.Snippet = stripHTML(snippets[i][1])
		}
		results = append(results, r)
	}
	return results, nil
}

func (m *Model) handleSearch(args []string) {
	if len(args) < 2 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant",
			Text: "**Cara pakai:** `/search <query>`\n\n**Contoh:**\n- `/search cara bikin website`\n- `/search berita teknologi hari ini`"})
		return
	}

	query := strings.Join(args[1:], " ")
	m.messages = append(m.messages, ChatMsg{Role: "user", Text: "🔍 " + query})

	cfg, _ := loadConfig()
	var results []searchResult
	var err error
	var engine string

	// Prioritas: Google CSE → SearXNG → DDG
	if cfg != nil && cfg.Search.GoogleAPIKey != "" && cfg.Search.GoogleCX != "" {
		engine = "Google"
		results, err = googleCSESearch(query, cfg.Search.GoogleAPIKey, cfg.Search.GoogleCX, 5)
		if err != nil {
			// Fallback ke SearXNG
			engine = "SearXNG"
			results, err = searchMeta(query, 5)
		}
	} else {
		engine = "SearXNG"
		results, err = searchMeta(query, 5)
	}

	// Fallback terakhir: DDG
	if err != nil {
		engine = "DuckDuckGo"
		results, err = ddgSearch(query, 5)
	}

	if err != nil {
		m.messages = append(m.messages, ChatMsg{Role: "error",
			Text: fmt.Sprintf("Search gagal (%s): %v", engine, err)})
		return
	}
	if len(results) == 0 {
		m.messages = append(m.messages, ChatMsg{Role: "assistant",
			Text: fmt.Sprintf("Gak ada hasil buat: %s", query)})
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Hasil untuk:** %s\n\n", query))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("**%d. %s**\n", i+1, r.Title))
		sb.WriteString(r.URL + "\n")
		if r.Snippet != "" {
			sb.WriteString(r.Snippet + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("---\n_Sumber: %s_", engine))

	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: sb.String()})
}

// ===== FILTER & SCORING untuk hasil search =====

// blockedDomains — domain yang gak informatif / skip
var blockedDomains = []string{
	"youtube.com", "youtu.be", "m.youtube.com",
	"tiktok.com", "instagram.com", "facebook.com",
	"pinterest.com", "twitter.com", "x.com",
	"snapchat.com", "threads.net",
}

// priorityDomains — domain yang diutamakan (informatif)
var priorityDomains = []string{
	"wikipedia.org", "id.wikipedia.org",
	"medium.com", "dev.to", "hashnode.com",
	"stackoverflow.com", "stackexchange.com",
	"github.com", "gitlab.com",
	"kominfo.go.id", "go.id",
	"kompas.com", "detik.com", "tempo.co", "cnnindonesia.com",
	"kumparan.com", "tirto.id", "katadata.co.id",
	"bbc.com", "reuters.com",
	"britannica.com",
}

// isBlocked — cek domain ada di blocklist
func isBlocked(url string) bool {
	low := strings.ToLower(url)
	for _, d := range blockedDomains {
		if strings.Contains(low, d) {
			return true
		}
	}
	return false
}

// scoreResult — kasih skor, makin tinggi makin diprioritas
func scoreResult(r searchResult) int {
	low := strings.ToLower(r.URL)
	score := 0

	// Prioritas domain
	for _, d := range priorityDomains {
		if strings.Contains(low, d) {
			score += 100
			break
		}
	}

	// Snippet bagus — panjang & gak cuma potongan
	snipLen := len(r.Snippet)
	if snipLen > 80 {
		score += 20
	} else if snipLen > 40 {
		score += 10
	}

	// Judul bagus
	titleLen := len(r.Title)
	if titleLen > 15 {
		score += 5
	}

	// Penalti kalo judul ada "youtube", "video", "shorts"
	lowTitle := strings.ToLower(r.Title)
	if strings.Contains(lowTitle, "youtube") || strings.Contains(lowTitle, "video") {
		score -= 50
	}

	return score
}

// filterAndScore — buang blocked, urut by score
func filterAndScore(results []searchResult) []searchResult {
	var clean []searchResult
	for _, r := range results {
		if !isBlocked(r.URL) {
			clean = append(clean, r)
		}
	}

	// Sort by score (descending)
	sort.Slice(clean, func(i, j int) bool {
		return scoreResult(clean[i]) > scoreResult(clean[j])
	})
	return clean
}
