// webfetch.go — fetch halaman, ekstrak teks, buang tag HTML.
package main

import (
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// fetchPageText — GET URL, buang tag, return plain text.
var reSpaceLocal = regexp.MustCompile(`\s+`)

func fetchPageText(rawURL string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Mobile Safari/537.36")
	req.Header.Set("Accept-Language", "id,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", nil // dianggap gagal, tapi bukan error fatal
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 800*1024))
	htmlStr := string(body)

	htmlStr = reScript.ReplaceAllString(htmlStr, " ")
	htmlStr = reStyle.ReplaceAllString(htmlStr, " ")
	htmlStr = reNav.ReplaceAllString(htmlStr, " ")
	htmlStr = reTag.ReplaceAllString(htmlStr, " ")
	htmlStr = html.UnescapeString(htmlStr)
	htmlStr = reSpaceLocal.ReplaceAllString(htmlStr, " ")

	return strings.TrimSpace(htmlStr), nil
}

// summarizePage — ambil 2-3 kalimat terbaik dari teks halaman.
func summarizePage(pageText, query string, maxSentences int) string {
	sentences := extractSentences(pageText)
	if len(sentences) == 0 {
		return ""
	}

	type cand struct {
		text  string
		score int
	}
	var ranked []cand
	for i, s := range sentences {
		ranked = append(ranked, cand{s, scoreSentence(s, query, i)})
	}
	// Sort by score desc
	for i := 1; i < len(ranked); i++ {
		for j := i; j > 0 && ranked[j].score > ranked[j-1].score; j-- {
			ranked[j], ranked[j-1] = ranked[j-1], ranked[j]
		}
	}

	var picked []string
	seen := make(map[string]bool)
	for _, c := range ranked {
		if len(picked) >= maxSentences {
			break
		}
		key := strings.ToLower(c.text)
		if len(key) > 30 {
			key = key[:30]
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		picked = append(picked, c.text)
	}

	var sb strings.Builder
	for i, s := range picked {
		if i > 0 {
			sb.WriteString(" ")
		}
		s = strings.TrimSpace(s)
		if !strings.HasSuffix(s, ".") && !strings.HasSuffix(s, "!") && !strings.HasSuffix(s, "?") {
			s += "."
		}
		sb.WriteString(s)
	}
	return sb.String()
}

// reJSONTemplate — buang artefak JSON/template Wikipedia yang lolos dari parser.
var reJSONTemplate = regexp.MustCompile(`\{[^}]{0,500}\}`)
var reBracket = regexp.MustCompile(`\[[^\]]{0,500}\]`)
var reQuote = regexp.MustCompile(`"[^"]{0,300}"`)

// cleanWikiArtifacts — buang sisa JSON/syntax Wikipedia dari teks.
func cleanWikiArtifacts(s string) string {
	s = reJSONTemplate.ReplaceAllString(s, "")
	s = reBracket.ReplaceAllString(s, "")
	s = reQuote.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "</span>", "")
	s = strings.ReplaceAll(s, "<span>", "")
	s = strings.ReplaceAll(s, "}}", "")
	s = strings.ReplaceAll(s, "{{", "")
	return strings.Join(strings.Fields(s), " ")
}
