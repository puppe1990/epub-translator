package translate

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Riva v2 tem janela de 8192 tokens. Metade fica para a saída; 2 runes/token
// cobre XHTML com muitas tags. Modelos longos (0) não fatiam.
const rivaInputCharBudget = 6000

// Gemini 3.5 Flash-Lite aceita 1M de entrada e só 65_536 de saída. Um INDEX
// de ~80k tokens (Total Recall cap. 221) estoura esse teto num shot só:
// ValidateBody vê fonte 7645 vs resposta ~6806. 12k runes cabem folgado.
const geminiInputCharBudget = 12000

func InputCharBudget(model string) int {
	lower := strings.ToLower(model)
	if strings.Contains(lower, "riva-translate") {
		return rivaInputCharBudget
	}
	if strings.Contains(lower, "gemini") {
		return geminiInputCharBudget
	}
	return 0
}

var blockCloseRE = regexp.MustCompile(`(?i)</(?:p|div|h[1-6]|li|blockquote|section|article|table|tr|ul|ol|header|footer)>`)

func splitBody(body string, maxChars int) []string {
	if maxChars <= 0 || utf8.RuneCountInString(body) <= maxChars {
		return []string{body}
	}
	var chunks []string
	var buf strings.Builder
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		chunks = append(chunks, buf.String())
		buf.Reset()
	}
	for _, piece := range splitAtBlockCloses(body) {
		pieceRunes := utf8.RuneCountInString(piece)
		if pieceRunes > maxChars {
			flush()
			chunks = append(chunks, hardSplit(piece, maxChars)...)
			continue
		}
		if buf.Len() > 0 && utf8.RuneCountInString(buf.String())+pieceRunes > maxChars {
			flush()
		}
		buf.WriteString(piece)
	}
	flush()
	if len(chunks) == 0 {
		return []string{body}
	}
	return chunks
}

func splitAtBlockCloses(body string) []string {
	matches := blockCloseRE.FindAllStringIndex(body, -1)
	if len(matches) == 0 {
		return []string{body}
	}
	pieces := make([]string, 0, len(matches)+1)
	start := 0
	for _, match := range matches {
		pieces = append(pieces, body[start:match[1]])
		start = match[1]
	}
	if start < len(body) {
		pieces = append(pieces, body[start:])
	}
	return pieces
}

func hardSplit(s string, maxChars int) []string {
	var chunks []string
	for utf8.RuneCountInString(s) > maxChars {
		cut := splitIndex(s, maxChars)
		if cut <= 0 || cut >= len(s) {
			break
		}
		chunks = append(chunks, s[:cut])
		s = s[cut:]
	}
	if s != "" {
		chunks = append(chunks, s)
	}
	return chunks
}

func splitIndex(s string, maxChars int) int {
	inTag := false
	lastSafe := 0
	n := 0
	for i, r := range s {
		if r == '<' {
			inTag = true
		}
		n++
		next := i + utf8.RuneLen(r)
		if r == '>' {
			inTag = false
			lastSafe = next
		} else if !inTag && (r == ' ' || r == '\n' || r == '.' || r == ';' || r == '!' || r == '?') {
			lastSafe = next
		}
		if n < maxChars {
			continue
		}
		if lastSafe > 0 && lastSafe < len(s) {
			return lastSafe
		}
		return next
	}
	return len(s)
}
