// searchsummary.go — extractive summarizer sederhana.
// Ambil kalimat-kalimat penting dari snippet search, gabung jadi 1 paragraf.
// Bukan parafrase — ini masih pakai kalimat asli dari sumber.
package main

import (
	"regexp"
	"sort"
	"strings"
)

var reSentenceEnd = regexp.MustCompile(`[.!?]+\s+`)
var reLeadingNum = regexp.MustCompile(`^\d+\.\s*`)
var reOnlyNum = regexp.MustCompile(`^\d+\.?\s*$`)

// extractSentences — pecah teks jadi kalimat, filter yang jelek.
func extractSentences(text string) []string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.Join(strings.Fields(text), " ")

	parts := reSentenceEnd.Split(text, -1)
	var out []string

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if reOnlyNum.MatchString(p) {
			continue
		}
		p = reLeadingNum.ReplaceAllString(p, "")
		p = strings.TrimSpace(p)
		if len(p) < 25 || len(p) > 200 {
			continue
		}
		// Buang kalau >50% angkanya
		digits := 0
		for _, c := range p {
			if c >= '0' && c <= '9' {
				digits++
			}
		}
		if float64(digits)/float64(len(p)) > 0.5 {
			continue
		}
		out = append(out, p)
	}
	return out
}

// scoreSentence — skor berdasarkan keyword query + posisi.
func scoreSentence(sentence, query string, pos int) int {
	score := 0
	low := strings.ToLower(sentence)
	for _, w := range strings.Fields(strings.ToLower(query)) {
		if len(w) < 3 {
			continue
		}
		if strings.Contains(low, w) {
			score += 3
		}
	}
	// Bonus kalau muncul awal (posisi 0 = paling atas)
	if pos < 5 {
		score += 5 - pos
	}
	return score
}

// buildSummary — ambil 2-3 kalimat terbaik, gabung jadi paragraf.
func buildSummary(results []searchResult, query string) string {
	type cand struct {
		text  string
		score int
	}
	var all []cand

	pos := 0
	for _, r := range results {
		for _, s := range extractSentences(r.Snippet) {
			all = append(all, cand{text: s, score: scoreSentence(s, query, pos)})
			pos++
		}
	}
	if len(all) == 0 {
		return ""
	}

	sort.SliceStable(all, func(i, j int) bool {
		return all[i].score > all[j].score
	})

	// Ambil max 2 kalimat, jangan duplikat
	var picked []string
	seen := make(map[string]bool)
	for _, c := range all {
		if len(picked) >= 2 {
			break
		}
		key := strings.ToLower(c.text)
		if len(key) > 25 {
			key = key[:25]
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		picked = append(picked, c.text)
	}
	if len(picked) == 0 {
		return ""
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
