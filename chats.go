package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ChatSession struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []ChatMsg `json:"messages"`
}

func chatsDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".netra-ai", "chats")
	os.MkdirAll(dir, 0700)
	return dir
}

func listChats() []ChatSession {
	dir := chatsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []ChatSession
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s ChatSession
		if json.Unmarshal(data, &s) == nil && s.ID != "" {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

func saveChat(s ChatSession) error {
	s.UpdatedAt = time.Now()
	if s.Title == "" {
		s.Title = "Chat " + s.ID
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(filepath.Join(chatsDir(), s.ID+".json"), data, 0600)
}

func newChatID() string {
	return time.Now().Format("20060102-150405")
}

func titleFromMessages(msgs []ChatMsg) string {
	for _, m := range msgs {
		if m.Role == "user" && strings.TrimSpace(m.Text) != "" {
			t := strings.TrimSpace(m.Text)
			if len(t) > 40 {
				t = t[:37] + "..."
			}
			return t
		}
	}
	return "Chat baru"
}
