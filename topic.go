// topic.go — Topic Memory buat Eliza
// Inget topik terakhir yang dibahas user, biar follow-up nyambung
package main

import (
	"strings"
)

// TopicKeywords — mapping keyword → topik
var topicKeywords = map[string]string{
	// Cinta / Pacar
	"pacar": "pacar", "gebetan": "pacar", "doi": "pacar", "mantan": "pacar",
	"cinta": "pacar", "sayang": "pacar", "putus": "pacar", "jadian": "pacar",

	// Kerja
	"kerja": "kerja", "kantor": "kerja", "boss": "kerja", "atasan": "kerja",
	"gaji": "kerja", "karier": "kerja", "proyek": "kerja", "kuli": "kerja",
	"usaha": "kerja", "bisnis": "kerja", "jualan": "kerja",

	// Keluarga
	"keluarga": "keluarga", "ibu": "keluarga", "ayah": "keluarga", "mama": "keluarga",
	"papa": "keluarga", "bapak": "keluarga", "kakak": "keluarga", "adik": "keluarga",
	"orang tua": "keluarga", "ortu": "keluarga",

	// Teman
	"teman": "teman", "temen": "teman", "sahabat": "teman", "kawan": "teman",
	"bro": "teman", "bestie": "teman",

	// Coding / Belajar
	"coding": "coding", "ngoding": "coding", "koding": "coding", "program": "coding",
	"golang": "coding", "python": "coding", "lua": "coding", "javascript": "coding",
	"belajar": "coding", "sekolah": "sekolah", "kuliah": "sekolah",

	// Game
	"game": "game", "main game": "game", "gaming": "game",

	// Uang
	"uang": "uang", "duit": "uang", "harga": "uang", "bayar": "uang",
	"hutang": "uang", "belanja": "uang",

	// Kesehatan
	"sakit": "kesehatan", "capek": "kesehatan", "lelah": "kesehatan",
	"stres": "kesehatan", "tidur": "kesehatan", "istirahat": "kesehatan",
}

// TopicReplies — reply per topik kalo gak ada entity baru
var topicReplies = map[string][]string{
	"pacar": {
		"Gimana hubungan kalian sekarang?",
		"Sejak kapan?",
		"Masih sering ketemu?",
		"Ada masalah?",
	},
	"kerja": {
		"Kerja di mana?",
		"Capek ya?",
		"Udah berapa lama?",
		"Ada yang bikin susah?",
	},
	"keluarga": {
		"Gimana hubungan kalian?",
		"Sering ngobrol?",
		"Ada cerita menarik?",
	},
	"teman": {
		"Lama udah temenan?",
		"Sering hangout?",
		"Ada cerita seru?",
	},
	"coding": {
		"Bahasa apa yang lagi dipelajari?",
		"Ada project apa?",
		"Udah berapa lama belajar?",
	},
	"sekolah": {
		"Kelas berapa?",
		"Ada yang susah?",
		"Suka pelajaran apa?",
	},
	"game": {
		"Game apa favoritmu?",
		"Main di HP atau PC?",
		"Sering main?",
	},
	"uang": {
		"Lagi susah ya?",
		"Ada masalah keuangan?",
	},
	"kesehatan": {
		"Istirahat dulu gapapa.",
		"Udah ke dokter?",
		"Kurangi beban dulu ya.",
	},
}

// ElizaTopic — state topik
type ElizaTopic struct {
	LastTopic    string
	TopicHistory []string // 5 topik terakhir
}

var elizaTopic = ElizaTopic{
	TopicHistory: []string{},
}

// detectTopic — cari topik dari input
func detectTopic(input string) string {
	low := strings.ToLower(input)
	for kw, topic := range topicKeywords {
		if strings.Contains(low, kw) {
			return topic
		}
	}
	return ""
}

// updateTopic — kalo ada topik baru, update
// Return: topik yang dipakai (baru atau lama)
func updateTopic(input string) string {
	newTopic := detectTopic(input)
	if newTopic != "" {
		elizaTopic.LastTopic = newTopic
		elizaTopic.TopicHistory = append(elizaTopic.TopicHistory, newTopic)
		if len(elizaTopic.TopicHistory) > 5 {
			elizaTopic.TopicHistory = elizaTopic.TopicHistory[1:]
		}
		return newTopic
	}
	return elizaTopic.LastTopic
}

// getTopicReply — reply sesuai topik terakhir
func getTopicReply() (string, bool) {
	if elizaTopic.LastTopic == "" {
		return "", false
	}
	replies, ok := topicReplies[elizaTopic.LastTopic]
	if !ok || len(replies) == 0 {
		return "", false
	}
	return replies[fastRand(len(replies))], true
}

// getLastTopic — helper
func getLastTopic() string {
	return elizaTopic.LastTopic
}

// shouldUseTopic — cek apakah input layak pake topic reply
// Skip kalo pertanyaan search, greeting, atau input terlalu panjang
func shouldUseTopic(input string) bool {
	low := strings.ToLower(strings.TrimSpace(input))

	if len(low) < 5 || len(low) > 60 {
		return false
	}

	// Skip kalo ada trigger search
	searchTriggers := []string{"cara ", "gimana ", "bagaimana ", "apa itu", "tutorial", "resep "}
	for _, t := range searchTriggers {
		if strings.Contains(low, t) {
			return false
		}
	}

	// Skip greeting / konfirmasi
	skip := []string{"halo", "hai", "hey", "iya", "ya", "tidak", "gak", "ok", "makasih", "thanks"}
	for _, s := range skip {
		if low == s || strings.HasPrefix(low, s+" ") {
			return false
		}
	}

	return true
}

// buildTopicReply — bikin reply topik dengan acknowledgment
func buildTopicReply(input, topic string) string {
	low := strings.ToLower(input)
	name := getLastEntity()

	// Ambil reply topik
	replies, ok := topicReplies[topic]
	if !ok || len(replies) == 0 {
		return ""
	}
	baseReply := replies[fastRand(len(replies))]

	// Cek kalo ada kata emosi → acknowledgement
	var ack string
	switch {
	case strings.Contains(low, "capek") || strings.Contains(low, "cape") || strings.Contains(low, "lelah"):
		ack = "Capek ya. "
	case strings.Contains(low, "sedih") || strings.Contains(low, "galau"):
		ack = "Sedih ya. "
	case strings.Contains(low, "susah") || strings.Contains(low, "sulit") || strings.Contains(low, "berat"):
		ack = "Berat ya. "
	case strings.Contains(low, "senang") || strings.Contains(low, "bahagia") || strings.Contains(low, "happy"):
		ack = "Seneng ya. "
	default:
		ack = ""
	}

	// Kalo ada nama, kadang pake
	if name != "" && fastRand(3) == 0 {
		return ack + name + ", " + strings.ToLower(baseReply[:1]) + baseReply[1:]
	}

	return ack + baseReply
}
