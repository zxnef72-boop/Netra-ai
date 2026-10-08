// personality_wrap.go — wrapper yang apply personality ke reply Eliza.
package main

// elizaReply — panggil core, lalu apply personality transform kalau ada.
func elizaReply(input string) string {
	reply := elizaReplyCore(input)
	if activePersonality != nil {
		if t := activePersonality.Transform(input, reply); t != "" {
			return t
		}
	}
	return reply
}
