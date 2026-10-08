// rules_loader.go — load custom rules dari ~/.netra-ai/rules.txt
// Support regex + capture group $1, $2, dst (ala ELIZA asli)
package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// CustomRule — rule yang di-load dari file
type CustomRule struct {
	Trigger  string
	Compiled *regexp.Regexp // compiled regex (?i) dari trigger
	Replies  []string
}

// customRules — cache rules yang ke-load
var customRules []CustomRule

// rulesPath — lokasi rules.txt
func rulesPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".netra-ai", "rules.txt")
}

// loadCustomRules — baca rules.txt, parse, return
func loadCustomRules() ([]CustomRule, error) {
	path := rulesPath()
	if path == "" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rules []CustomRule
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip komentar & kosong
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Format: trigger|reply1|reply2|...
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}

		trigger := strings.ToLower(strings.TrimSpace(parts[0]))
		if trigger == "" {
			continue
		}

		var replies []string
		for i := 1; i < len(parts); i++ {
			r := strings.TrimSpace(parts[i])
			if r != "" {
				replies = append(replies, r)
			}
		}
		if len(replies) == 0 {
			continue
		}

		// Compile trigger sebagai regex (case-insensitive)
		// Kalo gagal (syntax error), fallback ke literal contains
		var compiled *regexp.Regexp
		if re, err := regexp.Compile("(?i)" + trigger); err == nil {
			compiled = re
		}

		rules = append(rules, CustomRule{
			Trigger:  trigger,
			Compiled: compiled,
			Replies:  replies,
		})
	}

	return rules, scanner.Err()
}

// reloadCustomRules — re-read dari file, update cache
func reloadCustomRules() int {
	rules, err := loadCustomRules()
	if err != nil {
		return -1
	}
	customRules = rules
	return len(rules)
}

// matchCustomRule — cari rule yang match input.
// Support $1, $2, dst untuk capture group dari regex.
// Return (reply, true) kalo match.
func matchCustomRule(input string) (string, bool) {
	original := strings.TrimSpace(input)
	low := strings.ToLower(original)

	for _, r := range customRules {
		var match []string

		// Coba regex match dulu
		if r.Compiled != nil {
			match = r.Compiled.FindStringSubmatch(original)
		}

		// Fallback: literal contains (kalo regex gagal / gak match)
		if match == nil && strings.Contains(low, r.Trigger) {
			match = []string{r.Trigger}
		}

		if match == nil {
			continue
		}

		// Pilih reply random
		idx := 0
		if len(r.Replies) > 1 {
			idx = fastRand(len(r.Replies))
		}
		reply := r.Replies[idx]

		// Substitute $1, $2, $3, ... dengan capture group
		for i := 1; i < len(match); i++ {
			placeholder := "$" + strconv.Itoa(i)
			reply = strings.ReplaceAll(reply, placeholder, match[i])
		}

		return reply, true
	}
	return "", false
}

// fastRand — simple random
func fastRand(n int) int {
	if n <= 1 {
		return 0
	}
	return int(nowUnixNano() % int64(n))
}
