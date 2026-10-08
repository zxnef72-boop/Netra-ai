// search_meta.go — meta-search engine dari 0.
// Gabungin 4 sumber gratis (Marginalia, Mojeek, Wikipedia, DDG).
// Gak butuh API key. Fallback otomatis.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var metaClient = &http.Client{Timeout: 10 * time.Second}

var metaUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"

// ============================================================
// Marginalia — API publik, gak butuh key
// ============================================================
func searchMarginalia(query string, max int) ([]searchResult, error) {
	u := fmt.Sprintf("https://api.marginalia.nu/public/search/%s?count=%d",
		url.QueryEscape(query), max)

	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", metaUA)

	resp, err := metaClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("marginalia HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))

	var data struct {
		Results []struct {
			URL         string `json:"url"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("marginalia parse: %w", err)
	}

	var results []searchResult
	for _, r := range data.Results {
		if len(results) >= max {
			break
		}
		results = append(results, searchResult{
			Title:   strings.TrimSpace(r.Title),
			URL:     r.URL,
			Snippet: strings.TrimSpace(r.Description),
		})
	}
	return results, nil
}

// ============================================================
// Wikipedia (id) — API resmi, gak butuh key
// ============================================================
func searchWikipedia(query string, max int) ([]searchResult, error) {
	// Pake list=search (full-text), bukan opensearch (cuma judul)
	u := fmt.Sprintf("https://id.wikipedia.org/w/api.php?action=query&list=search&srsearch=%s&srlimit=%d&format=json",
		url.QueryEscape(query), max)

	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", metaUA)

	resp, err := metaClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("wikipedia HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))

	// Format list=search: {query:{search:[{title,snippet,...}]}}
	var data struct {
		Query struct {
			Search []struct {
				Title   string `json:"title"`
				Snippet string `json:"snippet"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("wikipedia parse: %w", err)
	}

	var results []searchResult
	for i, s := range data.Query.Search {
		if i >= max {
			break
		}
		// Buang HTML dari snippet
		snippet := strings.ReplaceAll(s.Snippet, "<span class=\"searchmatch\">", "")
		snippet = strings.ReplaceAll(snippet, "</span>", "")
		urlWiki := "https://id.wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(s.Title, " ", "_"))
		results = append(results, searchResult{
			Title:   s.Title,
			URL:     urlWiki,
			Snippet: snippet,
		})
	}
	return results, nil
}

// ============================================================
// Mojeek — scrape HTML (mirip DDG lite)
// ============================================================
var reMojeekResult = regexp.MustCompile(`(?s)<a[^>]+class="ob"[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
var reMojeekTitle = regexp.MustCompile(`<[^>]+>`)

func searchMojeek(query string, max int) ([]searchResult, error) {
	u := fmt.Sprintf("https://www.mojeek.com/search?q=%s", url.QueryEscape(query))

	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", metaUA)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := metaClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("mojeek HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	html := string(body)

	matches := reMojeekResult.FindAllStringSubmatch(html, max+5)
	var results []searchResult
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(results) >= max {
			break
		}
		link := m[1]
		title := reMojeekTitle.ReplaceAllString(m[2], "")
		title = strings.TrimSpace(title)
		if link == "" || title == "" || seen[link] {
			continue
		}
		seen[link] = true
		results = append(results, searchResult{
			Title:   title,
			URL:     link,
			Snippet: "",
		})
	}
	return results, nil
}

// ============================================================
// Meta-search — chain: Marginalia → Wikipedia → Mojeek → DDG
// ============================================================
func searchMeta(query string, max int) ([]searchResult, error) {
	type engine struct {
		name string
		fn   func(string, int) ([]searchResult, error)
	}

	engines := []engine{
		{"DuckDuckGo", ddgSearch},
		{"Wikipedia", searchWikipedia},
		{"Mojeek", searchMojeek},
		{"Marginalia", searchMarginalia},
	}

	var lastErr error
	for _, e := range engines {
		results, err := e.fn(query, max)
		if err != nil {
			lastErr = err
			continue
		}
		if len(results) > 0 {
			_ = e.name // buat debug kalau perlu
			return results, nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("semua engine kosong")
}
