// eliza.go — Eliza "natural" edition (v2)
// 5 trik: topic memory, mood detection, reflective multi-layer, context, variasi
package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type elizaRule struct {
	pattern *regexp.Regexp
	replies []string
	topic   string // topik yang di-set kalo rule ini match
}

// ===== MEMORY =====
var elizaMemory = struct {
	Nama        string
	Mood        string // "sedih", "senang", "takut", "marah", ""
	LastTopic   string // topik terakhir
	LastMention string // hal spesifik yang disebut terakhir
	KaliChat    int
}{
	Nama:      "",
	Mood:      "",
	LastTopic: "",
	KaliChat:  0,
}

// ===== MOOD DETECTION =====
var moodPatterns = map[string]*regexp.Regexp{
	"sedih":  regexp.MustCompile(`(?i)(sedih|galau|down|stress|pusing|lelah|capek|nyerah|putus asa)`),
	"senang": regexp.MustCompile(`(?i)(senang|bahagia|happy|semangat|seneng|gembira|seru|asik)`),
	"takut":  regexp.MustCompile(`(?i)(takut|cemas|khawatir|panik|deg-degan|was-was)`),
	"marah":  regexp.MustCompile(`(?i)(marah|kesal|jengkel|sebel|emosi|benci)`),
}

func detectMood(text string) string {
	for mood, pat := range moodPatterns {
		if pat.MatchString(text) {
			return mood
		}
	}
	return ""
}

// ===== REFLECTIVE PREFIX =====
// Kadang-kadang tambah acknowledgment sebelum pertanyaan, biar lebih natural
var lastMoodSet time.Time

var moodAcks = map[string][]string{
	"sedih":  {"Aku dengerin kok.", "Hmm, berat ya.", "Gapapa, ceritain aja.", "Aku ngerti itu gak enak.", "Kedengeran berat."},
	"senang": {"Asik!", "Wah, bagus dong.", "Seneng denger itu.", "Mantap!", "Nice!"},
	"takut":  {"Aku paham itu bikin gak nyaman.", "Cemas itu wajar.", "Tenang dulu, ceritain.", "Aku di sini kok."},
	"marah":  {"Kesel ya?", "Wajar sih kesel.", "Aku ngerti.", "Ceritain aja, aku gak judge."},
}

// ===== RULES =====
var elizaRules = []elizaRule{
	// GREETING (PALING ATAS)
	{regexp.MustCompile(`^(halo|hai|hey|hi|assalam|pagi|siang|sore|malam|apa kabar)[\s!?.,]*$`), []string{
		"Halo! Ada yang mau kamu ceritain?",
		"Hai. Gimana harimu?",
		"Halo. Gimana perasaanmu hari ini?",
		"Pagi! Apa kabar?",
	}, ""},

	// HELP
	{regexp.MustCompile(`^(help|bantuan|tolong|command|cmd|apa yang bisa|bisa apa)`), []string{"__HELP__"}, ""},

	// NAMA
	{regexp.MustCompile(`(?:nama saya|namaku|panggil aku) ([a-zA-Z][a-zA-Z0-9_]{1,20})$`), []string{
		"Halo $1! Senang kenalan.",
		"Salam kenal, $1. Gimana harimu?",
	}, "perkenalan"},
	{regexp.MustCompile(`^(?:siapa )?nama(?:ku| saya)\??$|^siapa aku\??$|^siapa namaku`), []string{"__ASK_NAME__"}, ""},

	// JAM / TANGGAL
	{regexp.MustCompile(`^(?:jam|waktu|pukul)(?:\s+berapa|\s+sekarang)?$|jam berapa|pukul berapa|sekarang jam|jam sekarang|waktu sekarang`), []string{"_TIME_"}, ""},
	// FUN
	{regexp.MustCompile(`flip ?coin|lempar koin|toss`), []string{"__COIN__"}, ""},
	{regexp.MustCompile(`(?:lempar |roll )?dadu|dice|roll`), []string{"__DICE__"}, ""},
	{regexp.MustCompile(`quote|kata mutiara|motivasi`), []string{"__QUOTE__"}, ""},
	{regexp.MustCompile(`joke|lawak|lelucon|bercanda`), []string{"__JOKE__"}, ""},
	{regexp.MustCompile(`random|angka acak|tebak angka`), []string{"__RANDOM__"}, ""},

	// MOOD — spesifik dulu
	{regexp.MustCompile(`saya (?:lagi )?(sedih|galau|down|stress|lelah|capek|pusing)`), []string{
		"Kenapa kamu $1?",
		"Sejak kapan merasa $1?",
		"Ada yang mau kamu ceritain?",
		"Apa yang bikin $1?",
	}, "sedih"},
	{regexp.MustCompile(`saya (?:lagi )?(senang|bahagia|happy|semangat|seneng)`), []string{
		"Apa yang bikin kamu $1?",
		"Ceritain dong, apa yang bikin seneng?",
		"Selalu seneng kaya gini ya?",
	}, "senang"},
	{regexp.MustCompile(`saya (?:lagi )?(takut|cemas|khawatir|panik)`), []string{
		"Apa yang bikin $1?",
		"Coba jelasin perasaannya...",
	}, "takut"},
	{regexp.MustCompile(`saya (?:lagi )?(marah|kesal|jengkel|sebel|emosi)`), []string{
		"Kenapa marah?",
		"Ada apa? Ceritain aja.",
		"Siapa yang bikin kesel?",
	}, "marah"},
	{regexp.MustCompile(`^tanggal|^hari (?:apa|ini)$|^date`), []string{"__DATE__"}, ""},

	// CURHAT UMUM
	{regexp.MustCompile(`saya (?:butuh|ingin|pengen|mau) (.+)`), []string{
		"Kenapa butuh $1?",
		"Apa yang bikin pengen $1?",
		"Kalo gak dapet $1, gimana?",
	}, "keinginan"},
	{regexp.MustCompile(`(?:aku|saya) (?:suka|seneng sama|dem suka) (.+)`), []string{
		"Menarik, kenapa suka $1?",
		"Sejak kapan suka $1?",
	}, "suka"},
	{regexp.MustCompile(`ibu|ayah|orang ?tua|keluarga|mama|papa`), []string{
		"Ceritain lebih soal keluargamu.",
		"Hubungan kalian gimana?",
		"Sering ngobrol sama mereka?",
	}, "keluarga"},
	{regexp.MustCompile(`teman|sahabat|friend|kawan|bro|bub`), []string{
		"Temanmu penting ya.",
		"Gimana hubungan kalian?",
		"Lama udah temenan?",
	}, "teman"},
	{regexp.MustCompile(`kerja|kantor|boss|atasan|kuli|las|restoran`), []string{
		"Gimana perasaanmu soal kerjaan?",
		"Kerja di mana sekarang?",
		"Capek ya?",
		"Udah lama kerja di situ?",
	}, "kerja"},
	{regexp.MustCompile(`cinta|pacar|gebetan|doi|mantan|sayang`), []string{
		"Ceritain soal perasaanmu.",
		"Gimana hubungan kalian?",
		"Masih ada perasaan?",
	}, "cinta"},
	{regexp.MustCompile(`coding|program|kode|golang|lua|python|netra`), []string{
		"Kamu suka coding?",
		"Bahasa apa favoritmu?",
		"Project apa yang lagi dibikin?",
		"Lama belajar coding?",
	}, "coding"},

	// RESPON KONFIRMASI — kasih variasi lebih banyak
	{regexp.MustCompile(`^(iya|ya|yap|yoi|yes|iya deh|ok|oke|sip)$`), []string{
		"Oh gitu. Terus?",
		"Menarik. Lanjut...",
		"Terus gimana?",
		"Lanjutin...",
		"Iya, terus?",
	}, ""},
	{regexp.MustCompile(`^(tidak|gak|nggak|engga|no|enggak)$`), []string{
		"Kenapa gak?",
		"Coba jelasin alasannya.",
		"Emang kenapa?",
	}, ""},

	// PERTANYAAN UMUM
	{regexp.MustCompile(`kamu (?:siapa|apa|bisa apa)`), []string{
		"Aku ELIZA, program 1966. Ketik 'help' buat liat yang bisa aku lakuin.",
	}, ""},
	{regexp.MustCompile(`kamu (?:bisa|dapat|bisah|bsa) (.+)`), []string{
		"Aku cuma program sederhana. Tapi kamu, bisa $1?",
	}, ""},
	{regexp.MustCompile(`kenapa`), []string{
		"Kenapa menurutmu?",
		"Apa yang kamu pikir jadi penyebabnya?",
		"Menurut kamu sendiri, kenapa?",
		"Ada dugaan?",
	}, ""},

	// THANKS / BYE
	{regexp.MustCompile(`(?:makasih|thanks|thank you|terima kasih|thx)`), []string{
		"Sama-sama!",
		"Gak masalah. Ada lagi?",
		"Senang bisa bantu.",
	}, ""},
	{regexp.MustCompile(`(?:bye|dadah|sampai jumpa|selamat tinggal)`), []string{
		"Dadah! Semoga harimu oke.",
		"Sampai jumpa lagi.",
	}, ""},
}

var elizaFallbacks = []string{
	"Ceritain lebih lanjut.",
	"Hmm, menarik. Terus?",
	"Apa yang kamu rasain soal itu?",
	"Bisa jelasin lebih detail?",
	"Gimana perasaanmu sekarang?",
	"Lanjutin ceritamu...",
	"Oh gitu. Terus gimana?",
	"Ada lagi yang mau diceritain?",
}

var elizaQuotes = []string{
	"\"Yang penting bukan seberapa keras kamu jatuh, tapi seberapa cepat kamu bangkit.\"",
	"\"Jangan takut gagal. Takutlah kalo gak pernah coba.\"",
	"\"Hidup itu 10% yang terjadi, 90% cara kamu respon.\"",
	"\"Belajar gak harus di kampus. Lapangan juga guru terbaik.\"",
	"\"Konsisten lebih penting dari motivasi.\"",
}

var elizaJokes = []string{
	"Programmer: orang yang menyelesaikan masalah yang gak kamu tau kamu punya.",
	"Kenapa programmer suka mode gelap? Karena lampu bikin bug keliatan.",
	"Ada 10 tipe orang: yang ngerti binary, dan yang gak.",
	"Lua itu kayak sambal: sedikit tapi nampol.",
	"Kalau error, jangan panik. Baca pesan error-nya. Beneran.",
}

func elizaHelp() string {
	return `**Yang bisa aku lakuin:**

**Template (cepat, instan):**
- "toko" / "game" / "portfolio" / "blog"
- "port scanner" / "scraper" / "fileorg"

**Inget nama:**
- "nama saya Anzar"
- "siapa namaku?"

**Info:**
- "jam berapa?" / "tanggal berapa?"

**Fun:**
- "flip coin" / "roll dadu" / "quote" / "joke" / "random"

**Curhat:**
- Ceritain apa aja. Aku dengerin.

**Tips:** aku cuma if-else, tapi aku berusaha ngerti. Kalo bingung, ketik "help".`
}

// ===== TOPIC MEMORY =====
// Inget topik terakhir buat follow-up natural
var topicFollowup = map[string][]string{
	"kerja":    {"Btw, kerja di bidang apa?", "Sering lembur?", "Ada rencana ganti kerjaan?", "Gimana rekan kerjamu?"},
	"keluarga": {"Kamu anak ke berapa?", "Sering kumpul sama keluarga?", "Hubungan sama ortu gimana?", "Ada adik/kakak?"},
	"cinta":    {"Masih sering kepikiran?", "Udah lama kenal?", "Gimana perasaanmu sekarang?", "Ceritain lebih..."},
	"coding":   {"Project apa yang lagi dikerjain?", "Suka bahasa apa?", "Udah lama belajar?", "Ada yang susah?"},
	"teman":    {"Sering hangout?", "Udah lama temenan?", "Kalian sering ngobrol?", "Ada cerita menarik?"},
	"sedih":    {"Masih kerasa sampe sekarang?", "Ada yang bisa bantu?", "Mau cerita lebih?", "Capek ya?"},
	"senang":   {"Apa lagi yang bikin seneng?", "Sering ngerasain ini?", "Ada rencana lain?", "Seru ya!"},
	"takut":    {"Masih kepikiran?", "Ada yang bisa bantu?", "Cemas soal apa?", "Coba ambil napas..."},
	"marah":    {"Masih kesel?", "Ada yang mau dilakuin?", "Capek marah ya?", "Coba cerita..."},
}

// ===== MAIN LOGIC =====
func elizaReplyCore(input string) string {
	// Persistent memory — update counter + save
	updateMemory(func(m *Memory) { m.KaliChat++ })

	// Auto-detect nama + mood dari INPUT user (langsung, gak nunggu rule)
	low := strings.ToLower(strings.TrimSpace(input))

	// Deteksi nama: "nama aku X", "namaku X", "aku X" (khusus nama)
	if m := regexp.MustCompile(`(?i)(?:nama\s+aku|namaku|nama\s+saya|panggil\s+aku)\s+([a-zA-Z]{2,20})`).FindStringSubmatch(input); m != nil {
		nama := strings.Title(strings.ToLower(m[1]))
		updateMemory(func(mem *Memory) { mem.NamaUser = nama })
	}

	// Deteksi mood dari input
	moodWords := map[string]string{
		"sedih": "sedih", "galau": "galau", "nangis": "sedih",
		"capek": "capek", "lelah": "capek", "penat": "capek", "cape": "capek",
		"seneng": "senang", "senang": "senang", "bahagia": "senang",
		"happy": "senang", "gembira": "senang",
		"marah": "marah", "kesel": "marah", "jengkel": "marah",
		"takut": "takut", "cemas": "cemas", "khawatir": "cemas",
		"stress": "stress", "pusing": "pusing", "bingung": "bingung",
	}
	for kw, mood := range moodWords {
		if strings.Contains(low, kw) {
			updateMemory(func(mem *Memory) { mem.MoodTerakhir = mood })
			break
		}
	}
	defer saveMemory()

	text := strings.ToLower(strings.TrimSpace(input))
	if text == "" {
		return "Hmm?"
	}

	// === CONTEXT MEMORY — PALING ATAS, sebelum apapun ===
	// 1. Resolve pronoun dulu (kalo ada "dia" → ganti nama terakhir)
	input = resolvePronoun(input)
	text = strings.ToLower(strings.TrimSpace(input))

	// 2. Extract entity dari input user (nama orang)
	// Cek apakah entity BARU ke-set
	entityBefore := getLastEntity()
	extractEntity(input)
	entityAfter := getLastEntity()
	entityChanged := entityAfter != "" && entityAfter != entityBefore

	// 3. Update topic (kalo ada keyword topik)
	updateTopic(input)

	// 4. Simpen ke context
	addToContext("user", input)

	// === LUA RULES (prioritas paling atas, sebelum entity ack & curhat) ===
	if reply, ok := matchLuaRules(input); ok && reply != "" {
		addToContext("eliza", reply)
		return reply
	}

	// Cek tutorial DULUAN
	if tpl, ok := elizaTryTutorial(input); ok {
		addToContext("eliza", "[template dikirim]")
		return tpl
	}

	// Cek template
	if tpl, ok := elizaTryTemplate(input); ok {
		addToContext("eliza", "[template dikirim]")
		return tpl
	}

	elizaMemory.KaliChat++

	// === PRIORITY: detect nama orang dulu ===
	// "pacar aku namanya Rika" / "temanku Budi" / "ibuku Siti"
	// Cek pake extractEntity lagi biar toleran "nama nya" / "namanya"
	if getLastEntity() != "" {
		lowCheckName := strings.ToLower(strings.TrimSpace(input))
		hasName := strings.Contains(lowCheckName, "nama") || strings.Contains(lowCheckName, "bernama") || strings.Contains(lowCheckName, "panggil")
		hasRelation := strings.Contains(lowCheckName, "pacar") || strings.Contains(lowCheckName, "teman") ||
			strings.Contains(lowCheckName, "temen") || strings.Contains(lowCheckName, "ibu") ||
			strings.Contains(lowCheckName, "ayah") || strings.Contains(lowCheckName, "mama") ||
			strings.Contains(lowCheckName, "papa") || strings.Contains(lowCheckName, "kakak") ||
			strings.Contains(lowCheckName, "adik") || strings.Contains(lowCheckName, "sahabat")

		if hasName && hasRelation {
			name := getLastEntity()
			// Variasi reply biar gak monoton
			replies := []string{
				name + " ya. Ceritain lebih lanjut soal " + name + ".",
				"Oke, " + name + ". Gimana hubungan kalian?",
				name + ". Menarik. Terus?",
				"Wah, " + name + ". Ceritain dong.",
			}
			return replies[fastRand(len(replies))]
		}
	}

	// === OSINT TEMPLATE AUTO-SUGGEST ===
	lowOSINT := strings.ToLower(input)
	osintKeywords := map[string]string{
		"cek ip":       "ip-info.py",
		"info ip":      "ip-info.py",
		"cek username": "username-check.py",
		"cek email":    "email-check.py",
		"recon domain": "domain-recon.py",
		"dork":         "dork-search.py",
	}
	for kw, file := range osintKeywords {
		if strings.Contains(lowOSINT, kw) {
			path := "osint-templates/" + file
			if data, err := os.ReadFile(path); err == nil {
				return fmt.Sprintf("**📄 Template: `%s`**\n\nCopy ke file dan jalanin:\n```\npython3 %s <argumen>\n```\n\n```python\n%s\n```",
					file, file, strings.TrimRight(string(data), "\n"))
			}
		}
	}

	// === PYTHON SCRIPT REQUEST (natural language) ===
	lowNL := strings.ToLower(strings.TrimSpace(input))

	// Trigger: user minta "python script"
	pyTriggers := []string{
		"python", "script", "coding", "program", "tool",
		"buatkan", "bikin", "buat ", "bikin ",
	}
	hasPyTrigger := false
	for _, t := range pyTriggers {
		if strings.Contains(lowNL, t) {
			hasPyTrigger = true
			break
		}
	}

	if hasPyTrigger {
		// Map topic → file template
		type pyTemplate struct {
			file     string
			keywords []string
			desc     string
		}
		templates := []pyTemplate{
			{
				file:     "sherlocklite.py",
				keywords: []string{"username", "user name", "user", "sherlock", "social media", "akun"},
				desc:     "Cek username di banyak platform (Sherlock-style)",
			},
			{
				file:     "email-check.py",
				keywords: []string{"email", "gmail", "mail", "surel"},
				desc:     "Cek email: Gravatar, breach database, dork",
			},
			{
				file:     "ip-info.py",
				keywords: []string{"ip", "ip address", "alamat ip"},
				desc:     "Info IP: negara, ISP, ASN, reverse DNS",
			},
			{
				file:     "domain-recon.py",
				keywords: []string{"domain", "website", "web", "recon", "subdomain"},
				desc:     "Recon domain: HTTP header, robots, DNS",
			},
			{
				file:     "dork-search.py",
				keywords: []string{"dork", "google dork", "search engine"},
				desc:     "Generate Google dork link",
			},
		}

		// Cari template yang paling cocok
		for _, tpl := range templates {
			for _, kw := range tpl.keywords {
				if strings.Contains(lowNL, kw) {
					// Coba baca file dari 2 lokasi
					paths := []string{
						"osint-templates/" + tpl.file,
						"templates/" + tpl.file,
						filepath.Join(os.Getenv("HOME"), "netra-ai", "templates", tpl.file),
					}
					for _, path := range paths {
						if data, err := os.ReadFile(path); err == nil {
							return fmt.Sprintf("**📄 Template Python: `%s`**\n\n_%s_\n\n**Cara pakai:**\n```\npython3 %s <argumen>\n```\n\n```python\n%s\n```",
								tpl.file, tpl.desc, tpl.file,
								strings.TrimRight(string(data), "\n"))
						}
					}
				}
			}
		}
	}

	// Cek trigger search — pakai fungsi shouldSearch yang lengkap
	hasSearchTrigger := false // DISABLED: Eliza prioritas, search cuma /search
	// Kalau input diawali "baca"/"read", jangan ke-trigger search
	inputLowCheck := strings.ToLower(strings.TrimSpace(input))
	if strings.HasPrefix(inputLowCheck, "baca ") || strings.HasPrefix(inputLowCheck, "read ") {
		hasSearchTrigger = false
	}
	// Kalau input match plugin (crypto, cuaca, gempa, dll), matiin search
	if hasPluginMatch(inputLowCheck) {
		hasSearchTrigger = false
	}

	// === SEARCH PRIORITY — sebelum pattern Eliza 1966 ===
	if hasSearchTrigger {
		cfg, _ := loadConfig()
		var results []searchResult
		var err error
		if cfg != nil && cfg.Search.GoogleAPIKey != "" && cfg.Search.GoogleCX != "" {
			results, err = googleCSESearch(input, cfg.Search.GoogleAPIKey, cfg.Search.GoogleCX, 3)
			if err != nil {
				results, err = searchMeta(input, 3)
			}
		} else {
			results, err = searchMeta(input, 3)
		}
		if err == nil {
			results = filterAndScore(results)
			if len(results) > 0 {
				return formatSearchResults(results, input)
			}
		}
	}

	// === PRIORITY: entity BARU ke-set → acknowledge langsung ===
	// TAPI skip kalo ada trigger search
	if entityChanged && !hasSearchTrigger {
		name := entityAfter
		typ := getLastEntityType()
		var replies []string
		switch typ {
		case "pacar":
			replies = []string{
				name + " ya. Ceritain lebih soal " + name + ".",
				"Oke, " + name + ". Gimana hubungan kalian?",
				"Wah, " + name + ". Ceritain dong.",
			}
		case "teman":
			replies = []string{
				name + " ya. Lama udah temenan?",
				"Oke, " + name + ". Ceritain soal dia.",
				name + ". Menarik. Terus?",
			}
		case "keluarga":
			replies = []string{
				name + " ya. Gimana hubungan kalian?",
				"Oke, " + name + ". Sering ketemu?",
			}
		default:
			replies = []string{
				name + " ya. Ceritain lebih lanjut.",
				"Oke, " + name + ".",
			}
		}
		return replies[fastRand(len(replies))]
	}

	// === PRIORITY: kalo input mengandung nama entity → acknowledgment ===
	if getLastEntity() != "" && strings.Contains(input, getLastEntity()) {
		// Extract bagian SETELAH nama
		idx := strings.Index(input, getLastEntity())
		if idx != -1 {
			after := strings.TrimSpace(input[idx+len(getLastEntity()):])
			// Buang tanda baca
			after = strings.Trim(after, ".,!?")
			if after != "" && len(after) < 80 {
				// Kalo cuma 1-5 kata, balikin acknowledgment PAKE NAMA
				words := strings.Fields(after)
				if len(words) <= 5 {
					name := getLastEntity()
					afterLow := strings.ToLower(after)
					var replies []string
					// Kalau after cuma 1-2 kata, jangan echo — tanya balik
					if len(words) <= 2 {
						replies = []string{
							"Oh, gitu. Kenapa tuh?",
							"Hmm. Ceritain lebih dong.",
							"Emang kenapa?",
							"Lanjut, aku dengerin.",
							"Terus gimana?",
						}
					} else {
						// 3-5 kata: kadang reflect, kadang tanya
						replies = []string{
							"Wah, " + afterLow + "? Ceritain lebih.",
							"Oh, " + name + " " + afterLow + ". Terus?",
							"Menarik. Kenapa tuh?",
							"Oke, lanjut. " + name + " gimana?",
							"Hmm. Emang kenapa " + afterLow + "?",
						}
					}
					return replies[fastRand(len(replies))]
				}
			}
		}
	}

	// === CUSTOM RULES dari rules.txt (prioritas tinggi) ===
	if reply, ok := matchCustomRule(input); ok {
		// Substitute nama kalo ada
		if getLastEntity() != "" && rand.Intn(4) == 0 && !strings.Contains(reply, getLastEntity()) {
			reply = getLastEntity() + ", " + strings.ToLower(reply[:1]) + reply[1:]
		}

		addToContext("eliza", reply)
		return reply
	}

	// === PRIORITY HANDLER — cek beberapa case sebelum rules ===
	lowText := strings.ToLower(strings.TrimSpace(text))

	// === PLUGIN ROUTING M-bM-^@M-^T cek plugin Lua sebelum handler lain ===
	if reply, matched := tryPlugin(lowText); matched {
		return reply
	}

	// Detect URL → tawarin baca
	// Detect "baca" / "read" — support file lokal + URL
	if strings.HasPrefix(lowText, "baca ") || strings.HasPrefix(lowText, "read ") {
		arg := strings.TrimSpace(input[5:])
		if arg == "" {
			return "Pakai: baca <path-file> atau baca <url>"
		}
		// Normalisasi: "Judul domain.tld" -> URL valid
		arg = normalizeReadArg(arg)

		// Cek dulu: file lokal ada?
		if info, err := os.Stat(arg); err == nil && !info.IsDir() {
			data, rerr := os.ReadFile(arg)
			if rerr != nil {
				return fmt.Sprintf("Gagal baca file: %v", rerr)
			}
			content := string(data)
			if len(content) > 3000 {
				content = content[:3000] + "\n...\n(terpotong)"
			}
			return fmt.Sprintf("📄 **%s** (%d bytes)\n\n```\n%s\n```",
				arg, info.Size(), strings.TrimRight(content, "\n"))
		}

		// Bukan file, coba sebagai URL
		if strings.HasPrefix(arg, "http") || strings.Contains(arg, ".") {
			title, content, ferr := fetchURL(arg)
			if ferr != nil {
				return fmt.Sprintf("Bukan file lokal, dan gagal fetch sebagai URL: %v", ferr)
			}
			maxLen := 1500
			if len(content) > maxLen {
				content = content[:maxLen] + "..."
			}
			out := ""
			if title != "" {
				out = "📖 **" + title + "**\n\n"
			}
			out += "```\n" + content + "\n```"
			return out
		}

		return "Bukan file lokal, dan bukan URL valid: " + arg
	}

	// Simpan nama: "nama aku X" / "namaku X" / "nama saya X"
	if m := regexp.MustCompile(`(?:nama aku|namaku|nama saya|panggil aku|nama ku)\s+([a-zA-Z]{2,15})`).FindStringSubmatch(lowText); m != nil {
		name := strings.ToUpper(m[1][:1]) + strings.ToLower(m[1][1:])
		elizaMemory.Nama = name
		updateMemory(func(m *Memory) { m.NamaUser = name })
		return fmt.Sprintf("Halo %s! Senang kenalan. Gimana harimu?", name)
	}

	// Tanya nama: "siapa nama aku" / "siapa namaku" / "namaku siapa"
	if strings.Contains(lowText, "siapa nama") || strings.Contains(lowText, "namaku siapa") {
		if elizaMemory.Nama != "" {
			return fmt.Sprintf("Namamu %s, kan? Aku inget.", elizaMemory.Nama)
		}
		return "Aku belum tau namamu. Kasih tau dong: \"nama aku ...\""
	}

	// Extract entities dari input
	extractEntity(text)

	// Detect mood
	newMood := detectMood(text)
	if newMood != "" {
		elizaMemory.Mood = newMood
		lastMoodSet = time.Now()
		updateMemory(func(m *Memory) { m.MoodTerakhir = newMood })
		trackMood(newMood)
	}

	// Cek rules
	for _, r := range elizaRules {
		if r.pattern.MatchString(text) {
			// Set topic
			if r.topic != "" {
				elizaMemory.LastTopic = r.topic
				updateMemory(func(m *Memory) { m.TopikTerakhir = r.topic })
			}

			reply := r.replies[rand.Intn(len(r.replies))]

			// Handle special commands
			switch reply {
			case "__HELP__":
				return elizaHelp()
			case "__ASK_NAME__":
				if elizaMemory.Nama != "" {
					return fmt.Sprintf("Namamu %s, kan? Aku inget.", elizaMemory.Nama)
				}
				return "Aku belum tau namamu. Kasih tau dong: \"nama saya ...\""
			case "__TIME__":
				return fmt.Sprintf("Jam %s WIB, %s.", time.Now().Format("15:04"), time.Now().Format("Senin, 02 Jan"))
			case "__DATE__":
				return fmt.Sprintf("Hari ini %s, %s.", time.Now().Format("Senin"), time.Now().Format("02 Januari 2006"))
			case "__COIN__":
				if rand.Intn(2) == 0 {
					return "🪙 Heads (gambar)!"
				}
				return "🪙 Tails (angka)!"
			case "__DICE__":
				return fmt.Sprintf("🎲 Dadu: **%d**", rand.Intn(6)+1)
			case "__QUOTE__":
				return elizaQuotes[rand.Intn(len(elizaQuotes))]
			case "__JOKE__":
				return elizaJokes[rand.Intn(len(elizaJokes))]
			case "__RANDOM__":
				return fmt.Sprintf("🎯 Angka acak: **%d**", rand.Intn(100)+1)
			}

			// Simpan nama
			if strings.Contains(r.pattern.String(), "nama saya") ||
				strings.Contains(r.pattern.String(), "namaku") {
				parts := strings.Fields(text)
				if len(parts) >= 2 {
					elizaMemory.Nama = strings.Title(parts[len(parts)-1])
				}
			}

			// Replace $1, $2
			// Replace $1, $2 secara MANUAL
			if strings.Contains(reply, "$1") || strings.Contains(reply, "$2") {
				m := r.pattern.FindStringSubmatch(text)
				if m != nil {
					for i := 1; i < len(m); i++ {
						placeholder := "$" + string(rune('0'+i))
						val := strings.TrimSpace(m[i])
						reply = strings.ReplaceAll(reply, placeholder, val)
					}
				}
				reply = strings.ReplaceAll(reply, "$1", "")
				reply = strings.ReplaceAll(reply, "$2", "")
			}

			// TRIK NATURAL 1: Acknowledge mood + question
			if elizaMemory.Mood != "" && rand.Intn(2) == 0 && moodFresh(5*time.Minute) && newMood != "" {
				if acks, ok := moodAcks[elizaMemory.Mood]; ok {
					ack := acks[rand.Intn(len(acks))]
					reply = ack + " " + reply
				}
			}

			// TRIK NATURAL 2: Kadang panggil nama
			if elizaMemory.Nama != "" && rand.Intn(4) == 0 && !strings.HasPrefix(reply, elizaMemory.Nama) {
				reply = elizaMemory.Nama + ", " + strings.ToLower(reply[:1]) + reply[1:]
			}

			return strings.TrimSpace(reply)
		}
	}

	// TRIK NATURAL 3: Kalo mood terdeteksi tapi gak match rule, pake fallback mood-aware
	if elizaMemory.Mood != "" && rand.Intn(2) == 0 && moodFresh(5*time.Minute) && newMood != "" {
		if acks, ok := moodAcks[elizaMemory.Mood]; ok {
			ack := acks[rand.Intn(len(acks))]
			fb := elizaFallbacks[rand.Intn(len(elizaFallbacks))]
			return ack + " " + fb
		}
	}

	// PRIORITAS: kalo user CURHAT, jangan search — respon hangat dulu
	// TAPI skip kalau ada search trigger (biar "cara download X" gak dianggap curhat)
	if isCurhat(input) && !hasSearchTrigger {
		// Kalo mood detected, kasih acknowledgment
		if elizaMemory.Mood != "" {
			if acks, ok := moodAcks[elizaMemory.Mood]; ok {
				ack := acks[rand.Intn(len(acks))]
				follow := []string{
					"Ceritain aja, aku dengerin.",
					"Ada yang mau kamu bagi?",
					"Aku di sini kok.",
					"Lanjutin ceritanya...",
					"Gapapa, santai aja.",
				}
				return ack + " " + follow[rand.Intn(len(follow))]
			}
		}
		// Kalo belum ada mood, respon pembuka
		openers := []string{
			"Aku dengerin. Ceritain aja.",
			"Gapapa, ceritain apa yang lagi kamu rasain.",
			"Aku di sini. Ada apa?",
			"Ceritain pelan-pelan, aku gak kemana-mana.",
			"Kedengeran berat. Mau cerita?",
			"Aku dengerin kok. Lanjut.",
		}
		return openers[rand.Intn(len(openers))]
	}

	// TOPIC REPLY — kalo ada topik, kasih follow-up topik
	if shouldUseTopic(input) {
		topic := getLastTopic()
		if topic != "" {
			if reply := buildTopicReply(input, topic); reply != "" {
				return reply
			}
		}
	}

	// FALLBACK TERAKHIR: cari di DuckDuckGo
	// Trigger: cuma kalo input keliatan kayak pertanyaan/perintah
	// (min 2 kata, max 100 char, bukan cuma "halo")
	if shouldSearch(input) {
		cfg, _ := loadConfig()
		var results []searchResult
		var err error
		if cfg != nil && cfg.Search.GoogleAPIKey != "" && cfg.Search.GoogleCX != "" {
			results, err = googleCSESearch(input, cfg.Search.GoogleAPIKey, cfg.Search.GoogleCX, 3)
			if err != nil {
				results, err = searchMeta(input, 3)
			}
		} else {
			results, err = searchMeta(input, 3)
		}
		// Fallback terakhir: DDG
		if err != nil {
			results, err = ddgSearch(input, 3)
		}

		if err != nil {
			return fmt.Sprintf("Search error: %v", err)
		}
		// Filter hasil — buang YouTube, TikTok, sosmed
		results = filterAndScore(results)
		if len(results) == 0 {
			return "Gak nemu hasil informatif di web."
		}
		return formatSearchResults(results, input)
	}

	// PRIORITAS TINGGI: kalo input ada "kamu" + "bisa/apa" → langsung jawab
	lowCheck := strings.ToLower(input)
	if strings.Contains(lowCheck, "kamu") &&
		(strings.Contains(lowCheck, "bisa") || strings.Contains(lowCheck, "apa") || strings.Contains(lowCheck, "bisah")) {
		return "Aku ELIZA, program if-else dari 1966. Aku bisa: balikin template, tutorial, cari di web, atau dengerin curhat lu. Ketik 'help' buat liat semua."
	}

	// TRIK NATURAL 4: Follow-up dari topik terakhir
	if elizaMemory.LastTopic != "" && rand.Intn(3) == 0 {
		if fus, ok := topicFollowup[elizaMemory.LastTopic]; ok {
			return fus[rand.Intn(len(fus))]
		}
	}

	return elizaFallbacks[rand.Intn(len(elizaFallbacks))]
}

// shouldSearch — cek apakah input layak di-search.
// Prinsip: cari web CUMA kalo user explicitly minta info/fakta.
func shouldSearch(input string) bool {
	low := strings.ToLower(strings.TrimSpace(input))

	// Terlalu pendek / panjang
	if len(low) < 4 || len(low) > 120 {
		return false
	}

	// Cuma 1 kata
	if len(strings.Fields(low)) < 2 {
		return false
	}

	// === STOP 1: CURHAT / EMOSI ===
	// Kalo ada kata ini, JANGAN search — biar Eliza yang jawab
	curhatWords := []string{
		"curhat", "pengen cerita", "mau cerita", "pengen ngobrol", "pengen ngomong",
		"sedih", "galau", "kesepian", "sendirian", "capek", "lelah", "stress",
		"takut", "cemas", "khawatir", "panik", "marah", "kesel", "jengkel",
		"pusing", "bingung", "susah", "berat", "nyerah", "putus asa",
		"aku ", "saya ", "gue ", "gw ", "kamu ", "elu ",
	}
	for _, w := range curhatWords {
		if strings.Contains(low, w) {
			return false
		}
	}

	// === STOP 2: GREETING ===
	greetings := []string{"halo", "hai", "hey", "apa kabar", "selamat pagi",
		"selamat siang", "selamat malam", "pagi", "siang", "malam",
		"makasih", "thanks", "bye", "dadah", "sampai jumpa"}
	for _, g := range greetings {
		if strings.HasPrefix(low, g) || low == g {
			return false
		}
	}

	// === STOP 3: CHAT PENDEK / KONFIRMASI ===
	if low == "iya" || low == "ya" || low == "tidak" || low == "gak" ||
		low == "ok" || low == "oke" || low == "hmm" || low == "oh" {
		return false
	}

	// === WAJIB: harus ada trigger PENCARIAN ===
	// Kalo gak ada trigger, gak search — biar Eliza fallback aja
	searchTriggers := []string{
		"apa itu", "apakah", "yang mana",
		"kenapa", "mengapa", "kapan", "dimana", "di mana",
		"berapa", "apa sih", "apa yg", "apa yang",
		"cari di", "search", "googling", "google",
		"kasih tau info", "informasi tentang",
		"cara ", "gimana ", "bagaimana ", "tutorial ",
		"berita", "harga",
		"belajar",
	}
	for _, t := range searchTriggers {
		if strings.Contains(low, t) {
			return true
		}
	}

	// Tambahan: kalau ada tanda tanya, anggap pertanyaan
	if strings.Contains(low, "?") {
		return true
	}

	// Gak ada trigger pencarian → JANGAN search, biar Eliza yang handle
	return false
}

// ===== TEMPLATE HANDLER =====
var elizaTemplateKeywords = map[string]string{
	"toko": "toko.html", "toko online": "toko.html", "shop": "toko.html",
	"landing": "landing.html", "landing page": "landing.html",
	"portfolio": "portfolio.html", "portofolio": "portfolio.html",
	"blog": "blog.html",
	"todo": "todo.html", "todo list": "todo.html", "to-do": "todo.html",
	"notes": "notes.html", "catatan": "notes.html",
	"calc": "calc.html", "kalkulator": "calc.html", "calculator": "calc.html",
	"simple": "simple.html",
	"eliza":  "eliza-chat.html", "chat": "eliza-chat.html",
	"game": "game.html", "snake": "game.html", "permainan": "game.html",
	"jam": "clock.html", "clock": "clock.html", "waktu": "clock.html",
	"portscan": "portscan.py", "port scan": "portscan.py", "port scanner": "portscan.py",
	"scanner": "portscan.py",
	"fileorg": "fileorg.py", "file org": "fileorg.py", "organize": "fileorg.py",
	"organizer": "fileorg.py", "rapiin file": "fileorg.py",
	"scraper": "scraper.py", "scrape": "scraper.py",
	"jsontool": "jsontool.py", "json tool": "jsontool.py",

	"sherlock": "sherlocklite.py", "sherlock lite": "sherlocklite.py", "username check": "sherlocklite.py",
	"cek username": "sherlocklite.py", "cek user": "sherlocklite.py", "user check": "sherlocklite.py",
	"osint username": "sherlocklite.py", "user enum": "sherlocklite.py",

	"toko pro": "toko-pro.html", "toko profesional": "toko-pro.html", "shop pro": "toko-pro.html",
	"portfolio pro": "portfolio-pro.html", "portofolio pro": "portfolio-pro.html",
	"admin": "admin.html", "admin panel": "admin.html", "dashboard": "admin.html",
	"admin login": "admin.html", "login admin": "admin.html",

	"detect http": "detect-http.lua", "deteksi http": "detect-http.lua", "http detect": "detect-http.lua",
	"cek server": "detect-http.lua", "detect server": "detect-http.lua",
}

func elizaTryTemplate(input string) (string, bool) {
	low := strings.ToLower(input)
	original := input

	// Skip kalo ini CERITA, bukan REQUEST template
	storyPatterns := []string{
		"namanya", "bernama",
		"suka main", "suka makan", "suka nonton", "suka ngoding",
		"tadi aku", "tadi saya", "kemarin aku", "kemarin saya",
		"pacar aku", "pacar ku", "pacar saya",
		"teman aku", "teman ku", "teman saya",
		"temen aku", "temen ku", "temen saya",
		"ibuku", "ayahku", "mama ku", "papa ku",
		"kakak ku", "adik ku", "sahabatku",
		// Tanpa "ku/saya" — "teman budi suka game"
		"teman ", "temen ", "sahabat ", "kawan ",
		"pacar ", "gebetan ", "ibu ", "ayah ",
		"mama ", "papa ", "kakak ", "adik ",
	}
	for _, sp := range storyPatterns {
		if strings.Contains(low, sp) {
			return "", false
		}
	}

	// Skip kata umum
	// Skip kalo cuma 1 kata umum (bukan permintaan jelas)
	skipWords := []string{"python", "coding", "program", "kode", "golang", "lua", "netra", "web", "website", "eliza"}
	for _, sw := range skipWords {
		if strings.TrimSpace(low) == sw {
			return "", false
		}
	}

	// STOP — kalo ada kata curhat/emosi, JANGAN match template
	stopWords := []string{
		"pengen curhat", "mau curhat", "curhat", "sedih", "galau",
		"capek", "lelah", "stress", "takut", "cemas", "marah",
		"kesel", "pusing", "bingung", "susah",
	}
	for _, sw := range stopWords {
		if strings.Contains(low, sw) {
			return "", false
		}
	}

	// Kalo panjang (> 40 char), kemungkinan cerita, bukan request template
	if len([]rune(low)) > 40 {
		return "", false
	}

	// Cari keyword cocok
	// PENTING: sort dari keyword terpanjang → terpendek
	// Biar "toko pro" dicek SEBELUM "toko", hasilnya deterministik.
	type kwPair struct{ kw, file string }
	var pairs []kwPair
	for kw, fname := range elizaTemplateKeywords {
		pairs = append(pairs, kwPair{kw, fname})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return len(pairs[i].kw) > len(pairs[j].kw)
	})

	var foundKeyword, foundFile string
	for _, p := range pairs {
		kw := p.kw
		fname := p.file
		// Word boundary regex
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(kw) + `\b`)
		if !re.MatchString(low) {
			continue
		}

		// Cek apakah ini permintaan template atau cuma nyebut
		requestWords := []string{"bikin", "buat", "kasih", "mau", "template", "pengen", "tolong", "generate", "tampilkan", "coba", "cek", "check", "cari"}
		isRequest := false
		for _, rw := range requestWords {
			if strings.Contains(low, rw) {
				isRequest = true
				break
			}
		}

		// Kalo bukan permintaan DAN input panjang → skip
		// TAPI kalo input pendek (<=30 char), allow tanpa trigger
		if !isRequest && len(original) > 30 {
			continue
		}

		// Kalo input panjang (> 150 char), skip — kemungkinan cerita
		if len(original) > 150 {
			continue
		}

		foundKeyword = kw
		foundFile = fname
		break
	}

	if foundKeyword == "" {
		return "", false
	}

	// Baca file
	tmplDirs := []string{
		filepath.Join(elizaTemplateBaseDir(), "templates"),
		"templates",
		filepath.Join(filepath.Dir(os.Args[0]), "templates"),
	}
	for _, dir := range tmplDirs {
		full := filepath.Join(dir, foundFile)
		if data, err := os.ReadFile(full); err == nil {
			lang := "text"
			ext := strings.ToLower(filepath.Ext(foundFile))
			switch ext {
			case ".html", ".htm":
				lang = "html"
			case ".py":
				lang = "python"
			case ".js":
				lang = "javascript"
			case ".go":
				lang = "go"
			case ".lua":
				lang = "lua"
			case ".css":
				lang = "css"
			}
			nameNoExt := strings.TrimSuffix(foundFile, ext)
			return fmt.Sprintf("📄 **Template: %s** (dari `templates/%s`)\n\n```%s\n%s\n```\n\n"+
				"Ketik `/template %s` buat save ke folder aktif.",
				foundKeyword, foundFile, lang, strings.TrimRight(string(data), "\n"), nameNoExt), true
		}
	}
	return fmt.Sprintf("Template '%s' gak ketemu. Bikin file `%s` dulu.", foundKeyword, foundFile), true
}

func elizaTemplateBaseDir() string {
	if _, err := os.Stat("templates"); err == nil {
		wd, _ := os.Getwd()
		return wd
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, "netra-ai")
		if _, err := os.Stat(filepath.Join(candidate, "templates")); err == nil {
			return candidate
		}
	}
	return "."
}

// elizaFakeStream — fake streaming
func elizaFakeStream(reply string, ch chan<- StreamChunk) {
	defer close(ch)
	if strings.Contains(reply, "\n") {
		lines := strings.Split(reply, "\n")
		for i, line := range lines {
			words := strings.SplitAfter(line, " ")
			for _, w := range words {
				ch <- StreamChunk{Text: w, Provider: "eliza"}
				time.Sleep(20 * time.Millisecond)
			}
			if i < len(lines)-1 {
				ch <- StreamChunk{Text: "\n", Provider: "eliza"}
			}
		}
		ch <- StreamChunk{Done: true, Provider: "eliza"}
		return
	}
	words := strings.SplitAfter(reply, " ")
	for _, w := range words {
		ch <- StreamChunk{Text: w, Provider: "eliza"}
		time.Sleep(35 * time.Millisecond)
	}
	ch <- StreamChunk{Done: true, Provider: "eliza"}
}

func elizaTryTutorial(input string) (string, bool) {
	low := strings.ToLower(strings.TrimSpace(input))
	triggers := []string{"tutorial ", "cara bikin ", "cara buat ", "cara masak ", "resep ", "tutor "}
	var query string
	for _, t := range triggers {
		if strings.HasPrefix(low, t) {
			query = strings.TrimSpace(low[len(t):])
			break
		}
	}
	if query == "" {
		return "", false
	}
	if tutorialBaseDir() == "" {
		return "", false
	}
	fname, content := tutorialFind(query)
	if content == "" {
		return "", false
	}
	display := strings.TrimSuffix(fname, ".txt")
	return fmt.Sprintf("📖 **Tutorial: %s**\n\n```\n%s\n```",
		strings.ReplaceAll(display, "-", " "),
		strings.TrimRight(content, "\n")), true
}

// isCurhat — detect apakah input adalah curhat / cerita pribadi.
func isCurhat(input string) bool {
	low := strings.ToLower(strings.TrimSpace(input))

	// Kata kunci curhat langsung
	directKeywords := []string{
		"curhat", "pengen cerita", "mau cerita", "pengen ngobrol",
		"pengen ngomong", "pengen sharing", "pengen bagi",
	}
	for _, k := range directKeywords {
		if strings.Contains(low, k) {
			return true
		}
	}

	// Kata emosi + subjek orang pertama
	emotionWords := []string{
		"sedih", "galau", "kesepian", "sendirian", "capek", "lelah",
		"stress", "takut", "cemas", "khawatir", "panik", "marah",
		"kesel", "jengkel", "pusing", "bingung", "susah", "berat",
		"nyerah", "putus asa", "down", "gak tau harus",
	}
	firstPerson := []string{"aku ", "saya ", "gue ", "gw ", "i'm", "i am"}

	hasEmotion := false
	for _, e := range emotionWords {
		if strings.Contains(low, e) {
			hasEmotion = true
			break
		}
	}
	hasFirstPerson := false
	for _, fp := range firstPerson {
		if strings.Contains(low, fp) {
			hasFirstPerson = true
			break
		}
	}

	if hasEmotion && hasFirstPerson {
		return true
	}

	// Kalo input pendek + ada emosi
	if hasEmotion && len(low) < 60 {
		return true
	}

	return false
}

// normalizeReadArg — handle "baca <judul> <domain>" jadi URL valid.
// Contoh:
//
//	"Pembelajaran id.wikipedia.org"   -> "https://id.wikipedia.org/wiki/Pembelajaran"
//	"artikel example.com"             -> "https://example.com/artikel"
//	"example.com"                     -> "example.com" (gak diubah)
func normalizeReadArg(arg string) string {
	if !strings.Contains(arg, " ") {
		return arg
	}

	domainRe := regexp.MustCompile(`([a-z0-9][a-z0-9-]*\.)+[a-z]{2,}`)
	lower := strings.ToLower(arg)
	domain := domainRe.FindString(lower)
	if domain == "" {
		return arg
	}

	idx := strings.Index(lower, domain)
	title := strings.TrimSpace(arg[:idx])
	if title == "" {
		return domain
	}

	slug := strings.ReplaceAll(title, " ", "_")

	if strings.Contains(domain, "wikipedia.org") {
		return "https://" + domain + "/wiki/" + slug
	}
	return "https://" + domain + "/" + slug
}

func moodFresh(d time.Duration) bool {
	if lastMoodSet.IsZero() {
		return false
	}
	return time.Since(lastMoodSet) < d
}
