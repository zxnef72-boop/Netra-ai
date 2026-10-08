// memory.go — persistent memory buat Eliza.
// Simpen nama user, mood, topik, jumlah chat — biar Eliza "inget"
// antar sesi (gak reset tiap restart).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Memory struct {
	NamaUser      string            `json:"nama_user,omitempty"`
	MoodTerakhir  string            `json:"mood_terakhir,omitempty"`
	TopikTerakhir string            `json:"topik_terakhir,omitempty"`
	KaliChat      int               `json:"kali_chat"`
	FirstSeen     string            `json:"first_seen,omitempty"`
	LastSeen      string            `json:"last_seen,omitempty"`
	Catatan       map[string]string `json:"catatan,omitempty"`
}

var (
	memoryMu    sync.Mutex
	memoryCache *Memory
)

func memoryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".netra-ai", "memory.json")
}

// loadMemory — baca memory dari file, atau return default.
func loadMemory() *Memory {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	if memoryCache != nil {
		return memoryCache
	}

	data, err := os.ReadFile(memoryPath())
	if err != nil {
		memoryCache = &Memory{
			Catatan:   make(map[string]string),
			FirstSeen: time.Now().Format(time.RFC3339),
		}
		return memoryCache
	}

	m := &Memory{}
	if err := json.Unmarshal(data, m); err != nil {
		memoryCache = &Memory{Catatan: make(map[string]string)}
		return memoryCache
	}
	if m.Catatan == nil {
		m.Catatan = make(map[string]string)
	}
	memoryCache = m
	return memoryCache
}

// saveMemory — tulis memory ke file.
func saveMemory() error {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	if memoryCache == nil {
		return nil
	}
	memoryCache.LastSeen = time.Now().Format(time.RFC3339)

	dir := filepath.Dir(memoryPath())
	os.MkdirAll(dir, 0755)

	data, err := json.MarshalIndent(memoryCache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(memoryPath(), data, 0644)
}

// updateMemory — modify + auto save.
func updateMemory(fn func(*Memory)) {
	m := loadMemory()
	memoryMu.Lock()
	fn(m)
	memoryMu.Unlock()
	saveMemory()
}

// memoryGreeting — greeting kalau udah kenal user + udah lama gak chat.
func memoryGreeting() string {
	m := loadMemory()
	if m.NamaUser == "" {
		return ""
	}

	// Kalau last_seen > 6 jam, kasih greeting special
	if m.LastSeen != "" {
		t, err := time.Parse(time.RFC3339, m.LastSeen)
		if err == nil {
			gap := time.Since(t)
			if gap > 6*time.Hour {
				parts := []string{fmt.Sprintf("Halo %s!", m.NamaUser)}
				if m.TopikTerakhir != "" {
					parts = append(parts, fmt.Sprintf("Terakhir kita ngobrol soal %s.", m.TopikTerakhir))
				}
				if m.MoodTerakhir != "" && m.MoodTerakhir != "netral" {
					parts = append(parts, fmt.Sprintf("Kamu lagi %s ya waktu itu.", m.MoodTerakhir))
				}
				return joinSpaces(parts)
			}
		}
	}
	return ""
}

func joinSpaces(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += p
	}
	return out
}
