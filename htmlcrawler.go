// htmlcrawler.go — multi-page HTML snapshot (mini web mirror).
// Crawl halaman utama + link internal, simpen semua ke folder.
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

var reCrawlHref = regexp.MustCompile(`(?i)href="([^"]+)"`)

const crawlMaxPages = 10

func tryHtmlCrawl(input string) (string, bool) {
	low := strings.ToLower(strings.TrimSpace(input))

	var target string
	if strings.HasPrefix(low, "html crawl ") {
		target = strings.TrimSpace(input[len("html crawl "):])
	} else if strings.HasPrefix(low, "html mirror ") {
		target = strings.TrimSpace(input[len("html mirror "):])
	} else {
		return "", false
	}

	fields := strings.Fields(target)
	if len(fields) == 0 {
		return "Pakai: html crawl <url>", true
	}

	urlStr := fields[0]
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + urlStr
	}

	base, err := url.Parse(urlStr)
	if err != nil || base.Host == "" {
		return "URL gak valid: " + urlStr, true
	}

	// Output folder
	host := strings.ReplaceAll(base.Host, ":", "_")
	outDir := "/sdcard/AiNef/crawl/" + host
	if _, err := os.Stat("/sdcard/AiNef"); err != nil {
		home, _ := os.UserHomeDir()
		outDir = filepath.Join(home, "netra-ai", "downloads", "crawl", host)
	}
	os.MkdirAll(outDir, 0755)

	client := &http.Client{Timeout: 15 * time.Second}
	visited := make(map[string]bool)
	pages := make(map[string]string)
	queue := []string{urlStr}

	for len(queue) > 0 && len(pages) < crawlMaxPages {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		// Cek robots.txt sebelum fetch
		rb := robotsFor(client, current)
		if !rb.IsAllowed(current) {
			continue
		}
		if d := rb.Delay(); d > 0 {
			time.Sleep(d)
		}
		html, err := fetchCrawl(client, current)
		if err != nil {
			continue
		}

		fname := urlToFilename(current)
		pages[current] = fname

		for _, m := range reCrawlHref.FindAllStringSubmatch(html, -1) {
			if len(m) < 2 {
				continue
			}
			link := resolveURL(base, m[1])
			if link == "" {
				continue
			}
			u, err := url.Parse(link)
			if err != nil || u.Host != base.Host {
				continue
			}
			if i := strings.Index(link, "#"); i != -1 {
				link = link[:i]
			}
			if !isCrawlablePage(link) {
				continue
			}
			if !visited[link] {
				queue = append(queue, link)
			}
		}
	}

	// Save files with rewritten links
	saved := 0
	for pageURL, fname := range pages {
		html, err := fetchCrawl(client, pageURL)
		if err != nil {
			continue
		}
		html = rewriteCrawlLinks(html, base, pages)
		outPath := filepath.Join(outDir, fname)
		os.MkdirAll(filepath.Dir(outPath), 0755)
		os.WriteFile(outPath, []byte(html), 0644)
		saved++
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("**Crawl %s**\n\n", urlStr))
	b.WriteString(fmt.Sprintf("- Halaman: %d\n", saved))
	b.WriteString(fmt.Sprintf("- Folder: %s\n\n", outDir))
	b.WriteString("Buka folder di file manager, klik file HTML")

	return b.String(), true
}

func fetchCrawl(client *http.Client, pageURL string) (string, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", metaUA)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 3*1024*1024))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func urlToFilename(pageURL string) string {
	u, err := url.Parse(pageURL)
	if err != nil {
		return "index.html"
	}
	p := strings.TrimPrefix(u.Path, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return "index.html"
	}
	p = strings.ReplaceAll(p, "/", "_")
	if !strings.Contains(p, ".") {
		p += ".html"
	}
	return p
}

func rewriteCrawlLinks(html string, base *url.URL, pages map[string]string) string {
	return reCrawlHref.ReplaceAllStringFunc(html, func(match string) string {
		m := reCrawlHref.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		raw := m[1]
		if strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "javascript:") ||
			strings.HasPrefix(raw, "mailto:") {
			return match
		}
		abs := resolveURL(base, raw)
		if abs == "" {
			return match
		}
		hash := ""
		if i := strings.Index(abs, "#"); i != -1 {
			hash = abs[i:]
			abs = abs[:i]
		}
		if fname, ok := pages[abs]; ok {
			return strings.Replace(match, raw, fname+hash, 1)
		}
		return match
	})
}

// isCrawlablePage — cek link ini worth di-crawl apa nggak
func isCrawlablePage(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	p := strings.ToLower(u.Path)

	// Skip asset extension
	assetExts := []string{
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp",
		".css", ".js", ".woff", ".woff2", ".ttf", ".otf",
		".pdf", ".zip", ".tar", ".gz", ".mp4", ".mp3", ".webm",
		".xml", ".json", ".txt", ".csv",
	}
	for _, ext := range assetExts {
		if strings.HasSuffix(p, ext) {
			return false
		}
	}

	// Skip non-HTML
	if strings.HasSuffix(p, ".php") || strings.HasSuffix(p, ".asp") ||
		strings.HasSuffix(p, ".jsp") || strings.HasSuffix(p, ".cgi") {
		return false
	}

	// Skip API endpoints
	if strings.Contains(p, "/api/") ||
		strings.Contains(p, "/rest.") ||
		strings.Contains(p, "/w/") ||
		strings.Contains(p, "/static/") ||
		strings.Contains(p, "/assets/") ||
		strings.Contains(p, "action=") {
		return false
	}

	// Skip query params (kecuali title=)
	if u.RawQuery != "" && !strings.Contains(u.RawQuery, "title=") {
		return false
	}

	return true
}
