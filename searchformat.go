// searchformat.go — helper buat ngerapiin output search di Eliza.
// Ganti dump markdown mentah jadi format "ngobrol".
package main

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"time"
)

// Variasi intro biar gak selalu "Aku gak punya jawaban pasti"
var searchIntroVariants = []string{
	"Aku cari nih. Dari beberapa sumber:",
	"Oke, aku udah cari. Ini yang ketemu:",
	"Dari web, ada beberapa hasil nih:",
	"Ada hasilnya. Cek ini:",
	"Aku nemu beberapa sumber:",
	"Hmm, aku cari di web. Ini hasilnya:",
}

// Variasi follow-up biar gak buntu
var searchFollowUpVariants = []string{
	"Mau aku baca-in salah satu lebih lengkap?",
	"Ada yang mau ditanyain lagi?",
	"Kalau butuh lebih detail, bilang aja.",
	"Mana yang paling relevan menurut kamu?",
	"Ada yang menarik dari hasil ini?",
}

// extractDomain — tarik domain dari URL. Fallback ke URL asli kalau gagal.
func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	host := strings.TrimPrefix(u.Host, "www.")
	return host
}

// smartTruncate — potong snippet di akhir kata (bukan di tengah kata).
// Buang newline + spasi berlebih dulu.
func smartTruncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.Join(strings.Fields(s), " ") // collapse spasi
	if len(s) <= maxLen {
		return s
	}
	// Potong di akhir kata terakhir sebelum maxLen
	cut := s[:maxLen]
	if idx := strings.LastIndex(cut, " "); idx > maxLen*3/4 {
		cut = cut[:idx]
	}
	return strings.TrimSpace(cut) + "..."
}

// formatSearchResults — hasil search jadi output rapi.
func formatSearchResults(results []searchResult, query string) string {
	if len(results) == 0 {
		return "Aku udah cari, tapi gak nemu hasil yang relevan. Coba tanya ulang dengan kata kunci lain?"
	}

	var sb strings.Builder

	// Intro random
	sb.WriteString(searchIntroVariants[rand.Intn(len(searchIntroVariants))])
	sb.WriteString("\n\n")

	// === COBA BACA HALAMAN ASLI (top result) ===
	summary := ""
	for i := 0; i < len(results) && i < 2; i++ {
		pageText, err := fetchPageText(results[i].URL, 8*time.Second)
		if err != nil || len(pageText) < 300 {
			continue
		}
		s := summarizePage(pageText, query, 3)
		if len(s) > 50 {
			summary = s
			break
		}
	}

	// Fallback: pakai snippet kalau fetch gagal
	if summary == "" {
		summary = buildSummary(results, query)
	}

	if summary != "" {
		summary = cleanWikiArtifacts(summary)
		sb.WriteString(summary)
		sb.WriteString("\n\n")
		sb.WriteString("─── Sumber lengkap ───\n\n")
	}

	// Hasil — max 3, ringkas (judul + domain)
	maxShow := len(results)
	if maxShow > 3 {
		maxShow = 3
	}
	for i := 0; i < maxShow; i++ {
		r := results[i]
		title := strings.TrimSpace(r.Title)
		if title == "" {
			title = "(tanpa judul)"
		}
		if len(title) > 80 {
			title = title[:77] + "..."
		}

		sb.WriteString(fmt.Sprintf("📌 %s\n", title))
		sb.WriteString(fmt.Sprintf("   └─ %s\n", extractDomain(r.URL)))

		if i < maxShow-1 {
			sb.WriteString("\n")
		}
	}

	// Follow-up random
	sb.WriteString("\n")
	sb.WriteString(searchFollowUpVariants[rand.Intn(len(searchFollowUpVariants))])

	return sb.String()
}
