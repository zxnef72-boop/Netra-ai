package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Provider struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	APIKey    string `json:"api_key"`
	Model     string `json:"model"`
	BaseURL   string `json:"base_url"`
	MaxTokens int    `json:"max_tokens,omitempty"`
}

type Config struct {
	Providers      []Provider   `json:"providers"`
	ActiveProvider int          `json:"active_provider"`
	Search         SearchConfig `json:"search,omitempty"`
}

type SearchConfig struct {
	GoogleAPIKey string `json:"google_api_key,omitempty"`
	GoogleCX     string `json:"google_cx,omitempty"`
	Engine       string `json:"engine,omitempty"` // "google" | "ddg" | "" (auto)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	Stream    bool      `json:"stream"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("config.json: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("providers kosong")
	}
	if cfg.ActiveProvider < 0 || cfg.ActiveProvider >= len(cfg.Providers) {
		cfg.ActiveProvider = 0
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	data, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile("config.json", data, 0600)
}

type StreamChunk struct {
	Text      string
	Done      bool
	Err       error
	RateLimit bool
	Provider  string
}

func streamChat(p Provider, msgs []Message, ch chan<- StreamChunk) {
	defer close(ch)

	maxTok := p.MaxTokens
	if maxTok == 0 {
		maxTok = 2000
	}

	body, _ := json.Marshal(ChatRequest{
		Model:     p.Model,
		Messages:  msgs,
		Stream:    true,
		MaxTokens: maxTok,
	})

	req, err := http.NewRequest("POST", p.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		ch <- StreamChunk{Err: err, Provider: p.Name}
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("HTTP-Referer", "https://netra.local")
	req.Header.Set("X-Title", "Netra AI")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		ch <- StreamChunk{Err: err, Provider: p.Name}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		isRL := resp.StatusCode == 429 ||
			strings.Contains(string(b), "rate_limit") ||
			strings.Contains(string(b), "tokens per minute") ||
			strings.Contains(string(b), "quota")
		ch <- StreamChunk{
			Err:       fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b))),
			RateLimit: isRL,
			Provider:  p.Name,
		}
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		chunk := strings.TrimPrefix(line, "data: ")
		if chunk == "[DONE]" {
			ch <- StreamChunk{Done: true, Provider: p.Name}
			return
		}
		var parsed struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(chunk), &parsed); err != nil {
			continue
		}
		if len(parsed.Choices) > 0 {
			if parsed.Choices[0].Delta.Content != "" {
				ch <- StreamChunk{Text: parsed.Choices[0].Delta.Content, Provider: p.Name}
			}
			if parsed.Choices[0].FinishReason == "length" {
				ch <- StreamChunk{Text: "\n\n_[stream kepotong. Ketik /continue buat lanjut.]_", Provider: p.Name}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Err: err, Provider: p.Name}
		return
	}
	ch <- StreamChunk{Done: true, Provider: p.Name}
}
