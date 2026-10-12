// fetchhtml.go — fetch HTML dari URL, bersihin, tampilkan di chat.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	reHTMLScript  = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reHTMLStyle   = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reHTMLLink    = regexp.MustCompile(`(?i)<link[^>]*>`)
	reHTMLSvg     = regexp.MustCompile(`(?is)<svg.*?</svg>`)
	reHTMLMeta    = regexp.MustCompile(`(?i)<meta[^>]*>`)
	reHTMLSpace   = regexp.MustCompile(`\s+`)
)

func tryFetchHTML(input string) (string, bool) {
	low := strings.ToLower(strings.TrimSpace(input))

	triggers := []string{
		"html tema ", "html kayak ", "html dari ",
		"html asli ", "html full ", "html mentah ",
		"html raw ", "html utuh ",
		"html link ", "html info ",
		"ambil html ", "source html ", "fetch html ",
		"kode html ", "html sumber ",
	}
	matched := ""
	for _, t := range triggers {
		if strings.Contains(low, t) {
			matched = t
			break
		}
	}
	if matched == "" {
		return "", false
	}

	// Extract target
	idx := strings.Index(low, matched)
	rest := strings.TrimSpace(input[idx+len(matched):])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "Pakai: `html tema <url|domain>` — contoh: `html tema google.com`", true
	}
	target := fields[0]

	// Deteksi mode
	fullMode := false
	for _, kw := range []string{"asli", "full", "mentah", "raw", "utuh"} {
		if strings.Contains(low, kw) {
			fullMode = true
			break
		}
	}
	linkMode := strings.Contains(low, "html link ")
	infoMode := strings.Contains(low, "html info ")

	var urlStr string
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		urlStr = target
	} else {
		urlStr = "https://" + target
	}

	// Dispatch mode link/info (sebelum fetch utama)
	if linkMode {
		return tryFetchLink(input, urlStr)
	}
	if infoMode {
		return tryFetchInfo(input, urlStr)
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "URL gak valid: " + target, true
	}
	req.Header.Set("User-Agent", metaUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Fetch `%s` gagal: %v", urlStr, err), true
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Fetch `%s` gagal: HTTP %d (situs mungkin block atau butuh JS)", urlStr, resp.StatusCode), true
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	html := string(body)
	origSize := len(html)

	// Bersihin (skip kalau mode full)
	if !fullMode {
		html = reHTMLScript.ReplaceAllString(html, "")
		html = reHTMLStyle.ReplaceAllString(html, "")
		html = reHTMLComment.ReplaceAllString(html, "")
		html = reHTMLSvg.ReplaceAllString(html, "")
		html = reHTMLLink.ReplaceAllString(html, "")
		html = reHTMLMeta.ReplaceAllString(html, "")
		html = reHTMLSpace.ReplaceAllString(html, " ")
		html = strings.TrimSpace(html)
	} else {
		// Mode full: inject <base href> biar relative URL resolve ke server asli
		baseTag := `<base href="` + urlStr + `">`
		lower := strings.ToLower(html)
		if idx := strings.Index(lower, "<head>"); idx != -1 {
			insertAt := idx + len("<head>")
			html = html[:insertAt] + baseTag + html[insertAt:]
		} else if idx := strings.Index(lower, "<head "); idx != -1 {
			// <head attr="...">
			closeIdx := strings.Index(html[idx:], ">")
			if closeIdx != -1 {
				insertAt := idx + closeIdx + 1
				html = html[:insertAt] + baseTag + html[insertAt:]
			}
		} else {
			// Gak ada <head>, prepend di awal
			html = baseTag + html
		}
	}

	cleanSize := len(html)

	// Simpen full HTML ke file
	host := strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(urlStr, "https://"), "http://"), "/", "_")
	host = strings.Split(host, "?")[0]
	if len(host) > 60 {
		host = host[:60]
	}
	// Prioritas: /sdcard/AiNef/ (keliatan di file manager)
	// Fallback: ~/netra-ai/downloads/ (internal Termux)
	var outDir string
	candidates := []string{
		"/sdcard/AiNef",
		"/storage/emulated/0/AiNef",
		"/sdcard/Download/AiNef",
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Dir(c)); err == nil {
			outDir = c
			break
		}
	}
	if outDir == "" {
		home, _ := os.UserHomeDir()
		outDir = filepath.Join(home, "netra-ai", "downloads")
	}
	os.MkdirAll(outDir, 0755)
	suffix := ".html"
	if fullMode {
		suffix = "-full.html"
	}
	outFile := filepath.Join(outDir, host+suffix)
	os.WriteFile(outFile, []byte(html), 0644)

	// Preview 5KB
	label := "bersih"
	if fullMode {
		label = "mentah (mode full)"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("**HTML `%s`** [%s]\n\n", urlStr, label))
	b.WriteString(fmt.Sprintf("_Asli %d byte -> %s %d byte_\n\n", origSize, label, cleanSize))

	if cleanSize == 0 {
		b.WriteString("(kosong — mungkin halaman JS-rendered)\n")
		return b.String(), true
	}

	preview := html
	truncated := false
	maxPreview := 5000
	if fullMode {
		maxPreview = 10000
	}
	if len(preview) > maxPreview {
		preview = preview[:maxPreview]
		truncated = true
	}

	b.WriteString("```html\n")
	b.WriteString(preview)
	if truncated {
		b.WriteString("\n\n... (dipotong, " + fmt.Sprintf("%d", cleanSize-maxPreview) + " byte lagi)")
	}
	b.WriteString("\n```\n\n")
	b.WriteString(fmt.Sprintf("_Full HTML: `%s`_", outFile))

	return b.String(), true
}

// extractLinksFromHTML — tarik semua link dari HTML
func extractLinksFromHTML(html, baseURL string) []string {
	var links []string
	seen := make(map[string]bool)

	// Regex href dan src
	hrefRe := regexp.MustCompile(`(?i)(?:href|src)\s*=\s*["']([^"']+)["']`)
	matches := hrefRe.FindAllStringSubmatch(html, -1)

	// Parse base URL
	baseParsed, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw := strings.TrimSpace(m[1])

		// Skip yang gak relevan
		if strings.HasPrefix(raw, "#") ||
			strings.HasPrefix(raw, "javascript:") ||
			strings.HasPrefix(raw, "mailto:") ||
			strings.HasPrefix(raw, "tel:") ||
			strings.HasPrefix(raw, "data:") {
			continue
		}

		// Resolve relative URL
		var abs string
		if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
			abs = raw
		} else if strings.HasPrefix(raw, "//") {
			abs = baseParsed.Scheme + ":" + raw
		} else {
			ref, err := url.Parse(raw)
			if err != nil {
				continue
			}
			abs = baseParsed.ResolveReference(ref).String()
		}

		// Buang fragment
		if idx := strings.Index(abs, "#"); idx != -1 {
			abs = abs[:idx]
		}
		if abs == "" || len(abs) > 500 {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		links = append(links, abs)
	}
	return links
}

// extractMetaFromHTML — tarik metadata (title, description, OG, dll)
func extractMetaFromHTML(html string) map[string]string {
	meta := make(map[string]string)

	// Title
	titleRe := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	if m := titleRe.FindStringSubmatch(html); len(m) > 1 {
		meta["title"] = strings.TrimSpace(reHTMLTag.ReplaceAllString(m[1], ""))
	}

	// Meta tags
	metaRe := regexp.MustCompile(`(?is)<meta\s+([^>]+)>`)
	for _, m := range metaRe.FindAllStringSubmatch(html, -1) {
		if len(m) < 2 {
			continue
		}
		attrs := m[1]
		nameRe := regexp.MustCompile(`(?i)(?:name|property)\s*=\s*["']([^"']+)["']`)
		contentRe := regexp.MustCompile(`(?i)content\s*=\s*["']([^"']*)["']`)
		nm := nameRe.FindStringSubmatch(attrs)
		ct := contentRe.FindStringSubmatch(attrs)
		if len(nm) > 1 && len(ct) > 1 {
			key := strings.ToLower(strings.TrimSpace(nm[1]))
			val := strings.TrimSpace(ct[1])
			if key != "" && val != "" {
				meta[key] = val
			}
		}
	}
	return meta
}

// tryFetchLink — mode "html link X": extract semua link
func tryFetchLink(input, urlStr string) (string, bool) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "URL gak valid", true
	}
	req.Header.Set("User-Agent", metaUA)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Fetch `%s` gagal: %v", urlStr, err), true
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Sprintf("Fetch gagal: HTTP %d", resp.StatusCode), true
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	links := extractLinksFromHTML(string(body), urlStr)

	if len(links) == 0 {
		return fmt.Sprintf("Gak ada link di `%s` (mungkin JS-rendered)", urlStr), true
	}

	// Group by domain
	baseHost := ""
	if u, err := url.Parse(urlStr); err == nil {
		baseHost = u.Host
	}
	var internal, external []string
	for _, l := range links {
		if u, err := url.Parse(l); err == nil {
			if u.Host == baseHost {
				internal = append(internal, l)
			} else {
				external = append(external, l)
			}
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("**Link di `%s`**\n\n", urlStr))
	b.WriteString(fmt.Sprintf("Total: **%d** link (%d internal, %d external)\n\n",
		len(links), len(internal), len(external)))

	if len(internal) > 0 {
		b.WriteString(fmt.Sprintf("**Internal (%d)**\n", len(internal)))
		max := 30
		for i, l := range internal {
			if i >= max {
				b.WriteString(fmt.Sprintf("- _... %d lagi_\n", len(internal)-max))
				break
			}
			b.WriteString("- " + l + "\n")
		}
		b.WriteString("\n")
	}

	if len(external) > 0 {
		b.WriteString(fmt.Sprintf("**External (%d)**\n", len(external)))
		max := 30
		for i, l := range external {
			if i >= max {
				b.WriteString(fmt.Sprintf("- _... %d lagi_\n", len(external)-max))
				break
			}
			b.WriteString("- " + l + "\n")
		}
	}
	return b.String(), true
}

// tryFetchInfo — mode "html info X": extract metadata
func tryFetchInfo(input, urlStr string) (string, bool) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "URL gak valid", true
	}
	req.Header.Set("User-Agent", metaUA)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Fetch `%s` gagal: %v", urlStr, err), true
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Sprintf("Fetch gagal: HTTP %d", resp.StatusCode), true
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	meta := extractMetaFromHTML(string(body))

	if len(meta) == 0 {
		return fmt.Sprintf("Gak ada metadata di `%s`", urlStr), true
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("**Info `%s`**\n\n", urlStr))

	// Priority keys
	priority := []string{
		"title", "description", "og:title", "og:description",
		"og:image", "og:url", "og:site_name", "og:type",
		"twitter:card", "twitter:title", "twitter:description",
		"author", "keywords", "viewport",
	}
	shown := make(map[string]bool)

	for _, k := range priority {
		if v, ok := meta[k]; ok {
			b.WriteString(fmt.Sprintf("- **%s**: %s\n", k, v))
			shown[k] = true
		}
	}
	// Sisa meta
	for k, v := range meta {
		if shown[k] {
			continue
		}
		b.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
	}
	return b.String(), true
}
