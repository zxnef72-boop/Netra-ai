package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var agentRetryRe = regexp.MustCompile(`try again in (?:(\d+)m)?([0-9.]+)(ms|s)`)

// agentRetryWait — durasi tunggu dari header Retry-After atau teks error rate limit.
func agentRetryWait(header, body string) (time.Duration, bool) {
	if header != "" {
		if sec, err := strconv.ParseFloat(strings.TrimSpace(header), 64); err == nil {
			return time.Duration(sec*float64(time.Second)) + time.Second, true
		}
	}
	m := agentRetryRe.FindStringSubmatch(body)
	if m == nil {
		return 0, false
	}
	val, err := strconv.ParseFloat(m[2], 64)
	if err != nil {
		return 0, false
	}
	unit := time.Second
	if m[3] == "ms" {
		unit = time.Millisecond
	}
	d := time.Duration(val * float64(unit))
	if m[1] != "" {
		mins, _ := strconv.Atoi(m[1])
		d += time.Duration(mins) * time.Minute
	}
	return d + time.Second, true
}
