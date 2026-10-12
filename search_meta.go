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
// searchWikipediaLang — cari di Wikipedia bahasa tertentu
func searchWikipediaLang(query string, max int, lang string) ([]searchResult, error) {
	u := fmt.Sprintf("https://%s.wikipedia.org/w/api.php?action=query&list=search&srsearch=%s&srlimit=%d&format=json",
		lang, url.QueryEscape(query), max)

	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", metaUA)

	resp, err := metaClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("wikipedia %s HTTP %d", lang, resp.StatusCode)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))

	var data struct {
		Query struct {
			Search []struct {
				Title   string `json:"title"`
				Snippet string `json:"snippet"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("wikipedia %s parse: %w", lang, err)
	}

	var results []searchResult
	for i, s := range data.Query.Search {
		if i >= max {
			break
		}
		snippet := strings.ReplaceAll(s.Snippet, "<span class=\"searchmatch\">", "")
		snippet = strings.ReplaceAll(snippet, "</span>", "")
		snippet = reHTMLTag.ReplaceAllString(snippet, "")
		snippet = strings.TrimSpace(snippet)
		urlWiki := "https://" + lang + ".wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(s.Title, " ", "_"))
		results = append(results, searchResult{
			Title:   s.Title,
			URL:     urlWiki,
			Snippet: snippet,
		})
	}
	return results, nil
}

// searchWikipedia — coba ID dulu, kalau kosong coba EN
func searchWikipedia(query string, max int) ([]searchResult, error) {
	results, errID := searchWikipediaLang(query, max, "id")
	if errID == nil && len(results) > 0 {
		return results, nil
	}
	results, errEN := searchWikipediaLang(query, max, "en")
	if errEN == nil && len(results) > 0 {
		return results, nil
	}
	if errID != nil {
		return nil, errID
	}
	if errEN != nil {
		return nil, errEN
	}
	return nil, fmt.Errorf("wikipedia ID + EN kosong")
}

func searchMeta(query string, max int) ([]searchResult, error) {
	type engine struct {
		name string
		fn   func(string, int) ([]searchResult, error)
	}

	engines := []engine{
		{"Wikipedia", searchWikipedia},
		{"DuckDuckGo", ddgSearch},
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

// ============================================================
// Direct handlers — nembak API langsung, gak lewat search engine
// ============================================================

var cryptoAliases = map[string]string{
	"bitcoin": "bitcoin", "btc": "bitcoin",
	"ethereum": "ethereum", "eth": "ethereum", "ether": "ethereum",
	"dogecoin": "dogecoin", "doge": "dogecoin",
	"solana": "solana", "sol": "solana",
	"bnb": "binancecoin", "binance": "binancecoin",
	"cardano": "cardano", "ada": "cardano",
	"ripple": "ripple", "xrp": "ripple",
	"litecoin": "litecoin", "ltc": "litecoin",
	"tron": "tron", "trx": "tron",
	"polkadot": "polkadot", "dot": "polkadot",
}

var cityAliases = map[string]string{
	"jakarta": "Asia/Jakarta", "tokyo": "Asia/Tokyo",
	"new york": "America/New_York", "london": "Europe/London",
	"singapore": "Asia/Singapore", "singapura": "Asia/Singapore",
	"kuala lumpur": "Asia/Kuala_Lumpur", "bangkok": "Asia/Bangkok",
	"seoul": "Asia/Seoul", "hongkong": "Asia/Hong_Kong", "hong kong": "Asia/Hong_Kong",
	"dubai": "Asia/Dubai", "sydney": "Australia/Sydney",
	"paris": "Europe/Paris", "berlin": "Europe/Berlin",
	"moscow": "Europe/Moscow", "cairo": "Africa/Cairo",
	"los angeles": "America/Los_Angeles", "chicago": "America/Chicago",
	"toronto": "America/Toronto", "mexico": "America/Mexico_City",
	"shanghai": "Asia/Shanghai", "beijing": "Asia/Shanghai",
	"mumbai": "Asia/Kolkata", "delhi": "Asia/Kolkata",
	"makassar": "Asia/Makassar", "surabaya": "Asia/Jakarta",
	"medan": "Asia/Jakarta", "bandung": "Asia/Jakarta",
	"jayapura": "Asia/Jayapura", "bali": "Asia/Makassar",
	"makkah": "Asia/Riyadh", "mekah": "Asia/Riyadh", "mecca": "Asia/Riyadh",
	"madinah": "Asia/Riyadh", "medina": "Asia/Riyadh", "jeddah": "Asia/Riyadh",
	"riyadh": "Asia/Riyadh", "riyad": "Asia/Riyadh", "arab": "Asia/Riyadh",
	"arab saudi": "Asia/Riyadh", "saudi": "Asia/Riyadh",
	"tehran": "Asia/Tehran", "iran": "Asia/Tehran",
	"istanbul": "Europe/Istanbul", "turki": "Europe/Istanbul", "turkey": "Europe/Istanbul",
	"kairo": "Africa/Cairo", "mesir": "Africa/Cairo", "egypt": "Africa/Cairo",
	"amsterdam": "Europe/Amsterdam", "belanda": "Europe/Amsterdam",
	"roma": "Europe/Rome", "italia": "Europe/Rome", "italy": "Europe/Rome",
	"madrid": "Europe/Madrid", "spanyol": "Europe/Madrid", "spain": "Europe/Madrid",
}

var kursAliases = map[string]string{
	"usd": "USD", "dolar": "USD", "dollar": "USD",
	"idr": "IDR", "rupiah": "IDR",
	"sgd": "SGD", "eur": "EUR", "euro": "EUR",
	"gbp": "GBP", "pound": "GBP",
	"jpy": "JPY", "yen": "JPY",
	"myr": "MYR", "ringgit": "MYR",
	"cny": "CNY", "yuan": "CNY", "rmb": "CNY",
	"krw": "KRW", "won": "KRW",
	"aud": "AUD", "cad": "CAD",
	"inr": "INR", "rupee": "INR",
}

// tryDirectHandlers — coba semua handler langsung. Return (jawaban, true) kalo dapet.
func tryDirectHandlers(input string) (string, bool) {
	if ans, ok := tryCrypto(input); ok {
		return ans, true
	}
	if ans, ok := tryTime(input); ok {
		return ans, true
	}
	if ans, ok := tryKurs(input); ok {
		return ans, true
	}
	if ans, ok := tryKamus(input); ok {
		return ans, true
	}
	if ans, ok := trySinonim(input); ok {
		return ans, true
	}
	if ans, ok := tryQuote(input); ok {
		return ans, true
	}
	return "", false
}

// ---- Crypto ----
func tryCrypto(input string) (string, bool) {
	low := strings.ToLower(input)
	if !strings.Contains(low, "harga") && !strings.Contains(low, "price") &&
		!strings.Contains(low, "crypto") && !strings.Contains(low, "koin") {
		return "", false
	}
	var coinID, coinName string
	for alias, id := range cryptoAliases {
		if strings.Contains(low, alias) {
			coinID = id
			coinName = alias
			break
		}
	}
	if coinID == "" {
		return "", false
	}
	u := "https://api.coingecko.com/api/v3/simple/price?ids=" + coinID + "&vs_currencies=idr,usd&include_24hr_change=true"
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", metaUA)
	resp, err := metaClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	var data map[string]struct {
		IDR          float64 `json:"idr"`
		USD          float64 `json:"usd"`
		IDR24hChange float64 `json:"idr_24h_change"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", false
	}
	info, ok := data[coinID]
	if !ok {
		return "", false
	}
	arrow := "→"
	if info.IDR24hChange > 0 {
		arrow = "↑"
	} else if info.IDR24hChange < 0 {
		arrow = "↓"
	}
	return fmt.Sprintf("**Harga %s**\n\n- Rp %s\n- $%s\n- 24 jam: %s %.2f%%\n\n_Sumber: CoinGecko_",
		strings.Title(coinName), formatRibuan(info.IDR), formatRibuan(info.USD), arrow, info.IDR24hChange), true
}

// ---- Waktu kota (Go native timezone, no API) ----
func tryTime(input string) (string, bool) {
	low := strings.ToLower(input)
	if !strings.Contains(low, "jam") && !strings.Contains(low, "waktu") &&
		!strings.Contains(low, "pukul") {
		return "", false
	}
	var tz, cityName string
	for alias, zone := range cityAliases {
		if strings.Contains(low, alias) {
			tz = zone
			cityName = alias
			break
		}
	}
	if tz == "" {
		// Cek apakah ini pertanyaan waktu ("jam di X", "waktu di X")
		if strings.Contains(low, " di ") || strings.Contains(low, "jam di") ||
			strings.Contains(low, "waktu di") {
			return "Kota itu belum ada di daftar bub. Coba: tokyo, london, new york, dubai, makkah, jakarta, singapore.", true
		}
		return "", false
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", false
	}
	now := time.Now().In(loc)
	days := []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	hari := days[int(now.Weekday())]
	offset := now.Format("-07:00")
	return fmt.Sprintf("**Waktu di %s**\n\n%s, %s — %s (UTC%s)",
		strings.Title(cityName), hari, now.Format("02 Jan 2006"), now.Format("15:04:05"), offset), true
}

// ---- Kurs (frankfurter.dev) ----
func tryKurs(input string) (string, bool) {
	low := strings.ToLower(input)
	if !strings.Contains(low, "kurs") && !strings.Contains(low, "tukar") &&
		!strings.Contains(low, "rate") && !strings.Contains(low, "exchange") {
		return "", false
	}
	var from, to string
	for alias, code := range kursAliases {
		if strings.Contains(low, alias) {
			if from == "" {
				from = code
			} else if to == "" && code != from {
				to = code
			}
		}
	}
	if from == "" {
		from = "USD"
	}
	if to == "" {
		to = "IDR"
	}
	if from == to {
		return "", false
	}

	// Coba base=from dulu
	if rate, ok := fetchFrankfurter(from, to); ok {
		return fmt.Sprintf("**Kurs %s → %s**\n\n1 %s = %s %s\n\n_Sumber: frankfurter.dev_",
			from, to, from, formatRibuan(rate), to), true
	}
	// Kalau base gak didukung (misal IDR), coba inverse
	if rate, ok := fetchFrankfurter(to, from); ok && rate > 0 {
		inv := 1.0 / rate
		return fmt.Sprintf("**Kurs %s → %s**\n\n1 %s = %s %s\n\n_Sumber: frankfurter.dev_",
			from, to, from, formatRibuan(inv), to), true
	}
	return "", false
}

func fetchFrankfurter(from, to string) (float64, bool) {
	u := "https://api.frankfurter.dev/v1/latest?base=" + from + "&symbols=" + to
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", metaUA)
	resp, err := metaClient.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	var data struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, false
	}
	rate, ok := data.Rates[to]
	return rate, ok
}

// formatRibuan — 1234567.89 -> "1.234.567,89"
func formatRibuan(f float64) string {
	s := fmt.Sprintf("%.2f", f)
	parts := strings.Split(s, ".")
	intPart := parts[0]
	decPart := parts[1]
	var out []string
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			out = append(out, ".")
		}
		out = append(out, string(c))
	}
	return strings.Join(out, "") + "," + decPart
}

// ---- Kamus (dictionaryapi.dev) ----
func tryKamus(input string) (string, bool) {
	low := strings.ToLower(input)
	if !strings.Contains(low, "arti kata") && !strings.Contains(low, "arti ") &&
		!strings.Contains(low, "kamus") && !strings.Contains(low, "definisi ") {
		return "", false
	}
	// Extract kata: buang prefix trigger
	words := strings.Fields(low)
	skip := map[string]bool{
		"kata": true, "arti": true, "kamus": true, "definisi": true,
		"dari": true, "bahasa": true, "inggris": true, "indonesia": true,
		"apa": true, "itu": true, "yang": true, "adalah": true,
	}
	var target string
	for _, w := range words {
		if !skip[w] && len(w) >= 2 {
			target = w
			break
		}
	}
	if target == "" && len(words) > 0 {
		target = words[len(words)-1]
	}
	if target == "" || len(target) < 2 {
		return "", false
	}
	// Cek ASCII-only (kamus English)
	for _, c := range target {
		if c > 127 {
			return "Kamus ini cuma English bub. Buat Indonesia belum ada API gratis.", true
		}
	}
	u := "https://api.dictionaryapi.dev/api/v2/entries/en/" + target
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", metaUA)
	resp, err := metaClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	var data []struct {
		Word     string `json:"word"`
		Phonetic string `json:"phonetic"`
		Meanings []struct {
			PartOfSpeech string `json:"partOfSpeech"`
			Definitions  []struct {
				Definition string `json:"definition"`
				Example    string `json:"example"`
			} `json:"definitions"`
		} `json:"meanings"`
	}
	if err := json.Unmarshal(body, &data); err != nil || len(data) == 0 {
		return "", false
	}
	d := data[0]
	var b strings.Builder
	b.WriteString(fmt.Sprintf("**%s**", d.Word))
	if d.Phonetic != "" {
		b.WriteString(fmt.Sprintf("  `%s`", d.Phonetic))
	}
	b.WriteString("\n\n")
	count := 0
	for _, m := range d.Meanings {
		if count >= 3 {
			break
		}
		b.WriteString(fmt.Sprintf("**%s**\n", m.PartOfSpeech))
		for i, def := range m.Definitions {
			if i >= 2 {
				break
			}
			b.WriteString(fmt.Sprintf("- %s\n", def.Definition))
			if def.Example != "" {
				b.WriteString(fmt.Sprintf("  _contoh: %s_\n", def.Example))
			}
		}
		b.WriteString("\n")
		count++
	}
	b.WriteString("_Sumber: dictionaryapi.dev_")
	return b.String(), true
}

// ---- Sinonim (Datamuse) ----
func trySinonim(input string) (string, bool) {
	low := strings.ToLower(input)
	if !strings.Contains(low, "sinonim") && !strings.Contains(low, "antonim") &&
		!strings.Contains(low, "persamaan kata") && !strings.Contains(low, "lawan kata") {
		return "", false
	}
	isAntonym := strings.Contains(low, "antonim") || strings.Contains(low, "lawan kata")
	words := strings.Fields(low)
	var target string
	for i, w := range words {
		if w == "sinonim" || w == "antonim" || w == "kata" {
			if i+1 < len(words) {
				target = words[i+1]
				break
			}
		}
	}
	if target == "" && len(words) > 0 {
		target = words[len(words)-1]
	}
	if target == "" {
		return "", false
	}
	rel := "rel_syn"
	if isAntonym {
		rel = "rel_ant"
	}
	u := "https://api.datamuse.com/words?" + rel + "=" + target + "&max=8"
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", metaUA)
	resp, err := metaClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	var data []struct {
		Word string `json:"word"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", false
	}
	if len(data) == 0 {
		return fmt.Sprintf("Sinonim `%s` gak ketemu. Datamuse cuma English — coba kata English (happy, sad, fast, dll).", target), true
	}
	var list []string
	for _, d := range data {
		list = append(list, d.Word)
	}
	label := "Sinonim"
	if isAntonym {
		label = "Antonim"
	}
	return fmt.Sprintf("**%s `%s`**\n\n%s\n\n_Sumber: Datamuse_",
		label, target, strings.Join(list, ", ")), true
}

// ---- Quote (ZenQuotes) ----
func tryQuote(input string) (string, bool) {
	low := strings.ToLower(input)
	if !strings.Contains(low, "quote") && !strings.Contains(low, "kutipan") &&
		!strings.Contains(low, "motivasi") && !strings.Contains(low, "kata mutiara") {
		return "", false
	}
	u := "https://zenquotes.io/api/random"
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", metaUA)
	resp, err := metaClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	var data []struct {
		Q string `json:"q"`
		A string `json:"a"`
	}
	if err := json.Unmarshal(body, &data); err != nil || len(data) == 0 {
		return "", false
	}
	return fmt.Sprintf("\"%s\"\n\n— %s", data[0].Q, data[0].A), true
}
