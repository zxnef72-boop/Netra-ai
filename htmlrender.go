// htmlrender.go — convert tag HTML ringan ke ANSI escape buat TUI Termux.
// Dipakai Eliza biar bisa output bold/italic/link clickable.
package main

import (
	"regexp"
	"strings"
)

var (
	reRenderTag  = regexp.MustCompile(`<[^>]+>`)
	reRenderLink = regexp.MustCompile(`(?is)<a\s+[^>]*href\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a>`)
)

// htmlToANSI convert tag HTML umum ke ANSI.
// Didukung: b/strong, i/em, u, s/del, code, a href, br, p.
// Tag lain di-strip. Kalau gak ada "<", langsung balik apa adanya.
func htmlToANSI(s string) string {
	if s == "" || !strings.Contains(s, "<") {
		return s
	}

	// OSC 8 hyperlink duluan, sebelum tag lain ke-strip.
	s = reRenderLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := reRenderLink.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		url := sub[1]
		text := reRenderTag.ReplaceAllString(sub[2], "")
		// \x1b]8;;URL\x1b\\ TEXT \x1b]8;;\x1b\\
		return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
	})

	repl := []struct {
		pat *regexp.Regexp
		rep string
	}{
		{regexp.MustCompile(`(?i)<(b|strong)\s*>`), "\x1b[1m"},
		{regexp.MustCompile(`(?i)</(b|strong)\s*>`), "\x1b[22m"},
		{regexp.MustCompile(`(?i)<(i|em)\s*>`), "\x1b[3m"},
		{regexp.MustCompile(`(?i)</(i|em)\s*>`), "\x1b[23m"},
		{regexp.MustCompile(`(?i)<u\s*>`), "\x1b[4m"},
		{regexp.MustCompile(`(?i)</u\s*>`), "\x1b[24m"},
		{regexp.MustCompile(`(?i)<(s|strike|del)\s*>`), "\x1b[9m"},
		{regexp.MustCompile(`(?i)</(s|strike|del)\s*>`), "\x1b[29m"},
		{regexp.MustCompile(`(?i)<code\s*>`), "\x1b[7m"},
		{regexp.MustCompile(`(?i)</code\s*>`), "\x1b[27m"},
		{regexp.MustCompile(`(?i)<br\s*/?\s*>`), "\n"},
		{regexp.MustCompile(`(?i)</p\s*>`), "\n"},
		{regexp.MustCompile(`(?i)<p[^>]*>`), ""},
	}
	for _, r := range repl {
		s = r.pat.ReplaceAllString(s, r.rep)
	}

	// Sisa tag yang gak dikenal -> buang
	s = reRenderTag.ReplaceAllString(s, "")

	// Entity dasar
	r := strings.NewReplacer(
		"&nbsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
	)
	return r.Replace(s)
}
