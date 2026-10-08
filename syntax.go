// syntax.go — Chroma-based syntax highlighting (ala VS Code)
package main

import (
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
)

// Palet warna ala tokyonight/vscode
var tokenColors = map[string]lipgloss.Color{
	"keyword":   lipgloss.Color("#c678dd"),
	"string":    lipgloss.Color("#98c379"),
	"comment":   lipgloss.Color("#5c6370"),
	"number":    lipgloss.Color("#d19a66"),
	"function":  lipgloss.Color("#61afef"),
	"class":     lipgloss.Color("#e5c07b"),
	"operator":  lipgloss.Color("#56b6c2"),
	"tag":       lipgloss.Color("#e06c75"),
	"attr":      lipgloss.Color("#d19a66"),
	"builtin":   lipgloss.Color("#e5c07b"),
	"punct":     lipgloss.Color("#abb2bf"),
	"decorator": lipgloss.Color("#e5c07b"),
	"variable":  lipgloss.Color("#e06c75"),
	"constant":  lipgloss.Color("#d19a66"),
	"text":      lipgloss.Color("#c0caf5"),
}

// chromaToPalette — map chroma token type ke key palet kita.
func chromaToPalette(t chroma.TokenType) string {
	switch {
	case t == chroma.Comment:
		return "comment"
	case t.InCategory(chroma.Comment):
		return "comment"

	case t == chroma.Keyword:
		return "keyword"
	case t.InCategory(chroma.Keyword):
		return "keyword"

	case t == chroma.String:
		return "string"
	case t.InCategory(chroma.String):
		return "string"
	case t.InCategory(chroma.LiteralString):
		return "string"
	case t == chroma.LiteralStringEscape:
		return "operator"

	case t == chroma.Number:
		return "number"
	case t.InCategory(chroma.Number):
		return "number"
	case t.InCategory(chroma.LiteralNumber):
		return "number"

	case t == chroma.NameFunction:
		return "function"
	case t == chroma.NameBuiltin:
		return "builtin"
	case t == chroma.NameClass:
		return "class"
	case t == chroma.NameTag:
		return "tag"
	case t == chroma.NameAttribute:
		return "attr"
	case t == chroma.NameDecorator:
		return "decorator"
	case t == chroma.NameConstant:
		return "constant"
	case t == chroma.NameVariable:
		return "variable"
	case t.InCategory(chroma.Name):
		return "text"

	case t == chroma.Operator:
		return "operator"
	case t.InCategory(chroma.Operator):
		return "operator"
	case t == chroma.OperatorWord:
		return "keyword"

	case t.InCategory(chroma.Punctuation):
		return "punct"

	case t.InCategory(chroma.Literal):
		return "text"
	}
	return "text"
}

// highlightLine — highlight code pakai chroma lexer.
func highlightLine(line, lang string) string {
	if strings.TrimSpace(line) == "" {
		return line
	}

	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		lang = "text"
	}

	// HTML/XML pakai manual highlighter — Chroma HTML lexer gak reliable
	if lang == "html" || lang == "htm" || lang == "xml" {
		return highlightHTMLManual(line)
	}

	// Ambil lexer
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, line)
	if err != nil {
		return lipgloss.NewStyle().Foreground(tokenColors["text"]).Render(line)
	}

	var out strings.Builder
	for _, token := range iterator.Tokens() {
		key := chromaToPalette(token.Type)
		style := lipgloss.NewStyle().Foreground(tokenColors[key])
		out.WriteString(style.Render(token.Value))
	}
	return out.String()
}

// highlightBlock — highlight seluruh code block (buat future use).
func highlightBlock(code, lang string) []string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		lang = "text"
	}

	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return strings.Split(code, "\n")
	}

	var lines []string
	var current strings.Builder

	for _, token := range iterator.Tokens() {
		key := chromaToPalette(token.Type)
		style := lipgloss.NewStyle().Foreground(tokenColors[key])

		parts := strings.Split(token.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				lines = append(lines, current.String())
				current.Reset()
			}
			if part != "" {
				current.WriteString(style.Render(part))
			}
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

// indentAndHighlight — ganti leading whitespace jadi │ guide ala VS Code.
// Level 1 indent (4 spasi) → 1 garis │, level 2 → 2 garis, dst.
func indentAndHighlight(line, lang string) string {
	// Normalize tab jadi 4 spasi
	line = strings.ReplaceAll(line, "\t", "    ")

	// Hitung leading spaces
	leading := 0
	for leading < len(line) && line[leading] == ' ' {
		leading++
	}
	level := leading / 4

	// Gak ada indent → langsung highlight
	if level == 0 {
		return highlightLine(line, lang)
	}

	// Style buat garis guide — abu tipis
	guideStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#414868"))

	// Bangun prefix: │   │   │   ...
	var prefix strings.Builder
	for i := 0; i < level; i++ {
		prefix.WriteString(guideStyle.Render("│"))
		prefix.WriteString("   ")
	}

	// Konten = sisa setelah level*4 spasi (kalo ada sisa karena bukan kelipatan 4)
	content := line[level*4:]

	// Highlight konten
	highlighted := highlightLine(content, lang)
	return prefix.String() + highlighted
}

// highlightHTMLManual — regex-based HTML highlighter (fallback buat Chroma).
func highlightHTMLManual(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return line
	}

	// Full-line comment
	if strings.HasPrefix(trimmed, "<!--") {
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		return indent + lipgloss.NewStyle().Foreground(lipgloss.Color("#5c6370")).Italic(true).Render(trimmed)
	}

	// Cek ada tag
	if !strings.Contains(line, "<") {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5")).Render(line)
	}

	var out strings.Builder
	rest := line
	for {
		loc := reHTMLTag.FindStringIndex(rest)
		if loc == nil {
			out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5")).Render(rest))
			break
		}
		// Teks sebelum tag
		out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5")).Render(rest[:loc[0]]))
		// Tag
		tag := rest[loc[0]:loc[1]]
		out.WriteString(highlightTagManual(tag))
		rest = rest[loc[1]:]
	}
	return out.String()
}

// highlightTagManual — highlight isi tag: nama tag merah, attr kuning, string hijau.
func highlightTagManual(tag string) string {
	tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e06c75"))
	attrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#d19a66"))
	strStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#98c379"))

	if len(tag) < 3 {
		return tagStyle.Render(tag)
	}

	var out strings.Builder
	if strings.HasPrefix(tag, "</") {
		out.WriteString(tagStyle.Render("</"))
		tag = tag[2:]
	} else if strings.HasPrefix(tag, "<") {
		out.WriteString(tagStyle.Render("<"))
		tag = tag[1:]
	}

	// Nama tag
	i := 0
	for i < len(tag) && (isAlphaNum(tag[i]) || tag[i] == '-' || tag[i] == '!') {
		i++
	}
	out.WriteString(tagStyle.Render(tag[:i]))
	rest := tag[i:]

	// Atribut + string + >
	for len(rest) > 0 {
		// Close tag
		if strings.HasPrefix(rest, ">") || strings.HasPrefix(rest, "/>") {
			out.WriteString(tagStyle.Render(rest))
			break
		}
		// Spasi
		if rest[0] == ' ' || rest[0] == '\t' {
			out.WriteString(rest[:1])
			rest = rest[1:]
			continue
		}
		// String
		if rest[0] == '"' || rest[0] == '\'' {
			q := rest[0]
			end := strings.IndexByte(rest[1:], q)
			if end == -1 {
				out.WriteString(strStyle.Render(rest))
				break
			}
			out.WriteString(strStyle.Render(rest[:end+2]))
			rest = rest[end+2:]
			continue
		}
		// Equals
		if rest[0] == '=' {
			out.WriteString(attrStyle.Render("="))
			rest = rest[1:]
			continue
		}
		// Nama atribut
		j := 0
		for j < len(rest) && rest[j] != '=' && rest[j] != '>' && rest[j] != ' ' && rest[j] != '"' && rest[j] != '\'' {
			j++
		}
		if j == 0 {
			out.WriteString(rest[:1])
			rest = rest[1:]
			continue
		}
		out.WriteString(attrStyle.Render(rest[:j]))
		rest = rest[j:]
	}
	return out.String()
}

var reHTMLTag = regexp.MustCompile(`</?[a-zA-Z!][^>]*>`)

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
