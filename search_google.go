// search_google.go — Google Custom Search API (lebih reliable dari DDG)
package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// GoogleCSEItem — 1 item hasil Google CSE
type GoogleCSEItem struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

// GoogleCSEResponse — response dari Google API
type GoogleCSEResponse struct {
	Items []GoogleCSEItem `json:"items"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// googleCSESearch — search via Google Custom Search Engine
func googleCSESearch(query, apiKey, cx string, maxResults int) ([]searchResult, error) {
	if apiKey == "" || cx == "" {
		return nil, fmt.Errorf("Google API key atau CX kosong")
	}

	// Max 10 per request
	num := maxResults
	if num > 10 {
		num = 10
	}

	params := url.Values{}
	params.Set("key", apiKey)
	params.Set("cx", cx)
	params.Set("q", query)
	params.Set("num", fmt.Sprintf("%d", num))
	params.Set("safe", "off")

	reqURL := "https://www.googleapis.com/customsearch/v1?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "netra-ai/1.0")

	// Skip TLS verify — kalo ISP intercept
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Timeout: 20 * time.Second, Transport: tr}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Google CSE HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var parsed GoogleCSEResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse JSON gagal: %w", err)
	}

	if parsed.Error != nil {
		return nil, fmt.Errorf("Google API error %d: %s", parsed.Error.Code, parsed.Error.Message)
	}

	var results []searchResult
	for _, item := range parsed.Items {
		results = append(results, searchResult{
			Title:   item.Title,
			URL:     item.Link,
			Snippet: item.Snippet,
		})
	}
	return results, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
