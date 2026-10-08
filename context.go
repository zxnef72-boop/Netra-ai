// context.go — Entity Memory + Pronoun Resolution buat Eliza
// Fokus: inget nama orang + ganti "dia" jadi nama
package main

import (
	"regexp"
	"strings"
)

type ContextMsg struct {
	Role string // "user" | "eliza"
	Text string
}

type ElizaContext struct {
	RecentMessages []ContextMsg
	LastEntity     string   // nama terakhir disebut
	LastEntityType string   // "pacar" | "teman" | "keluarga" | "self"
	MoodHistory    []string // 5 mood terakhir
}

var elizaCtx = ElizaContext{
	RecentMessages: []ContextMsg{},
	MoodHistory:    []string{},
}

// ===== Regex entity — fokus, gak over-complex =====
var (
	// "pacar aku namanya Rika" / "pacar ku Rika" / "pacar saya Rika"
	rePacar = regexp.MustCompile(`(?i)pacar(?:\s*(?:aku|ku|saya))?\s*(?:nama\s*nya|bernama|itu)?\s*([a-zA-Z]{3,15})`)
	// "temanku Budi" / "teman saya Budi" / "teman namanya Budi"
	reTeman = regexp.MustCompile(`(?i)(?:teman|sahabat|kawan|temen|bro)(?:\s*(?:aku|ku|saya))?\s*(?:nama\s*nya|bernama|itu)?\s*([a-zA-Z]{3,15})`)
	// "ibuku Siti" / "ibu saya Siti"
	reKeluarga = regexp.MustCompile(`(?i)(?:ibu|ayah|mama|papa|bapak|kakak|adik)(?:\s*(?:aku|ku|saya))?\s*(?:nama\s*nya|bernama|itu)?\s*([a-zA-Z]{3,15})`)
	// "nama aku Nef" / "namaku Nef" / "nama saya Nef"
	reSelf = regexp.MustCompile(`(?i)(?:nama\s+(?:aku|ku|saya)|namaku|panggil aku)\s+([a-zA-Z]{2,15})`)
)

// Stop words — jangan jadi entity
var entityStopwords = map[string]bool{
	"yang": true, "itu": true, "ini": true, "ada": true, "sama": true,
	"lagi": true, "juga": true, "sih": true, "dong": true, "kok": true,
	"aku": true, "saya": true, "kamu": true, "dia": true, "mereka": true,
	"cantik": true, "ganteng": true, "baik": true, "pintar": true,
	"keren": true, "jahat": true, "bodoh": true, "suka": true,
	"main": true, "punya": true, "dari": true, "untuk": true,
	"dengan": true, "tapi": true, "kalo": true, "kalau": true,
	"aja": true, "saja": true, "banget": true, "sangat": true,
	"terus": true, "udah": true, "sudah": true, "belum": true,
}

// addToContext — simpen chat terakhir (max 10)
func addToContext(role, text string) {
	elizaCtx.RecentMessages = append(elizaCtx.RecentMessages, ContextMsg{role, text})
	if len(elizaCtx.RecentMessages) > 10 {
		elizaCtx.RecentMessages = elizaCtx.RecentMessages[1:]
	}
}

// extractEntity — cari 1 entity dari input (prioritas: pacar > teman > keluarga > self)
func extractEntity(input string) {
	// Tokenizer manual — lebih predictable dari regex greedy
	low := strings.ToLower(input)
	words := strings.Fields(low)

	// Kata relasi → tipe
	relations := map[string]string{
		"pacar": "pacar", "gebetan": "pacar", "doi": "pacar", "mantan": "pacar",
		"teman": "teman", "sahabat": "teman", "kawan": "teman", "temen": "teman", "bro": "teman",
		"ibu": "keluarga", "ayah": "keluarga", "mama": "keluarga",
		"papa": "keluarga", "bapak": "keluarga", "kakak": "keluarga", "adik": "keluarga",
	}

	// Kata yang di-skip (jangan jadi nama)
	skip := map[string]bool{
		"ku": true, "saya": true, "aku": true, "gue": true, "gw": true,
		"adalah": true, "namanya": true, "bernama": true, "nama": true, "nya": true,
		"itu": true, "yang": true, "dan": true, "atau": true, "juga": true,
		"sama": true, "sih": true, "dong": true, "kok": true, "lagi": true,
		"baik": true, "cantik": true, "ganteng": true, "pintar": true,
		"keren": true, "jahat": true, "bodoh": true, "suka": true, "main": true,
		"punya": true, "dari": true, "untuk": true, "dengan": true, "tapi": true,
		"kalo": true, "kalau": true, "aja": true, "saja": true, "banget": true,
		"sangat": true, "terus": true, "udah": true, "sudah": true, "belum": true,
	}

	for i, w := range words {
		w = strings.Trim(w, ".,!?:;")
		typ, ok := relations[w]
		if !ok {
			continue
		}
		// Cari kata berikutnya yang valid (max 4 kata ke depan)
		for j := i + 1; j < len(words) && j <= i+4; j++ {
			cand := strings.Trim(words[j], ".,!?:;")
			if cand == "" || skip[cand] {
				continue
			}
			if len(cand) < 3 || len(cand) > 15 {
				continue
			}
			// Semua huruf?
			allAlpha := true
			for _, c := range cand {
				if c < 'a' || c > 'z' {
					allAlpha = false
					break
				}
			}
			if !allAlpha {
				continue
			}
			name := strings.ToUpper(cand[:1]) + cand[1:]
			elizaCtx.LastEntity = name
			elizaCtx.LastEntityType = typ
			return
		}
	}
}

// Pronoun resolution — ganti "dia"/"dirinya" jadi nama terakhir
var (
	reDia     = regexp.MustCompile(`(?i)\bdia\b`)
	reDirinya = regexp.MustCompile(`(?i)\bdirinya\b`)
)

func resolvePronoun(input string) string {
	low := strings.ToLower(input)
	if !strings.Contains(low, "dia") && !strings.Contains(low, "dirinya") {
		return input
	}
	if elizaCtx.LastEntity == "" {
		return input
	}
	out := reDia.ReplaceAllString(input, elizaCtx.LastEntity)
	out = reDirinya.ReplaceAllString(out, elizaCtx.LastEntity)
	return out
}

// getLastEntity — helper
func getLastEntity() string {
	return elizaCtx.LastEntity
}

// getLastEntityType — helper
func getLastEntityType() string {
	return elizaCtx.LastEntityType
}

// trackMood — simpen mood terakhir (max 5)
func trackMood(mood string) {
	if mood == "" {
		return
	}
	elizaCtx.MoodHistory = append(elizaCtx.MoodHistory, mood)
	if len(elizaCtx.MoodHistory) > 5 {
		elizaCtx.MoodHistory = elizaCtx.MoodHistory[1:]
	}
}

// getDominantMood — mood yang paling sering muncul di 5 chat terakhir
func getDominantMood() string {
	if len(elizaCtx.MoodHistory) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, m := range elizaCtx.MoodHistory {
		counts[m]++
	}
	best := ""
	bestCount := 0
	for m, c := range counts {
		if c > bestCount {
			best = m
			bestCount = c
		}
	}
	if bestCount >= 2 { // minimal 2x baru dianggap dominant
		return best
	}
	return ""
}
