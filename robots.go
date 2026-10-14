// robots.go — baca + parse robots.txt buat crawler sopan.
// Fail-open: kalau gagal fetch / gak ada rules, semua URL dianggap boleh.
package main

import (
	"bufio"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type robotsRules struct {
	disallow []string
	allow    []string
	delay    time.Duration
}

var robotsCache sync.Map // host -> *robotsRules

// robotsFor — cache wrapper per host.
func robotsFor(client *http.Client, rawURL string) *robotsRules {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return &robotsRules{}
	}
	host := strings.ToLower(u.Host)
	if v, ok := robotsCache.Load(host); ok {
		return v.(*robotsRules)
	}
	r := fetchRobots(client, u.Scheme, u.Host)
	robotsCache.Store(host, r)
	return r
}

func fetchRobots(client *http.Client, scheme, host string) *robotsRules {
	r := &robotsRules{}
	if scheme == "" {
		scheme = "https"
	}
	req, err := http.NewRequest("GET", scheme+"://"+host+"/robots.txt", nil)
	if err != nil {
		return r
	}
	req.Header.Set("User-Agent", metaUA)
	resp, err := client.Do(req)
	if err != nil {
		return r
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return r
	}

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	agent := ""
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "#"); i != -1 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		switch key {
		case "user-agent":
			agent = strings.ToLower(val)
		case "disallow":
			if agent == "*" && val != "" {
				r.disallow = append(r.disallow, val)
			}
		case "allow":
			if agent == "*" && val != "" {
				r.allow = append(r.allow, val)
			}
		case "crawl-delay":
			if agent == "*" {
				if sec, err := time.ParseDuration(val + "s"); err == nil {
					r.delay = sec
				}
			}
		}
	}
	return r
}

// IsAllowed — longest match menang. Allow menang kalau panjangnya sama.
func (r *robotsRules) IsAllowed(rawURL string) bool {
	if r == nil {
		return true
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	best := -1
	allowed := true
	for _, p := range r.disallow {
		if strings.HasPrefix(path, p) && len(p) > best {
			best = len(p)
			allowed = false
		}
	}
	for _, p := range r.allow {
		if strings.HasPrefix(path, p) && len(p) >= best {
			best = len(p)
			allowed = true
		}
	}
	return allowed
}

// Delay — capped max 10 detik biar gak bikin user nunggu kelamaan.
func (r *robotsRules) Delay() time.Duration {
	if r == nil || r.delay <= 0 {
		return 0
	}
	if r.delay > 10*time.Second {
		return 10 * time.Second
	}
	return r.delay
}
