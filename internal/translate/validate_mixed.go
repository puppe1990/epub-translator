package translate

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Riva às vezes ecoa o primeiro corte em inglês e traduz o resto. O
// ValidateBody antigo só barrava texto 100% idêntico, então o capítulo
// inteiro passava. Uma frase curta de nome próprio não conta.
const (
	mixedMinSentenceWords = 8
	mixedMinCopiedWords   = 40
	mixedMinNewWords      = 40
	mixedCopyJaccard      = 0.78
)

// translatableWordCount: legendas de foto com só o nome (Peter Brenner)
// não têm o que traduzir — ValidateBody deixa o texto idêntico passar.
func translatableWordCount(text string) int {
	return len(strings.Fields(text))
}

var sentenceSplitRE = regexp.MustCompile(`[.!?]+(?:\s+|$)`)

// denseShortParagraphs detecta INDEX/sumário: dezenas de <p> curtos. O
// splitter de frases quebra em "Mr." e trata blocos de nomes como "eco" do
// original, então o join falha como texto misturado mesmo traduzido.
func denseShortParagraphs(html string) bool {
	paragraphs := 0
	for _, match := range tagRE.FindAllStringSubmatch(html, -1) {
		if match[1] == "" && strings.EqualFold(match[2], "p") {
			paragraphs++
		}
	}
	words := len(strings.Fields(plainText(html)))
	if paragraphs < 80 || words == 0 {
		return false
	}
	return words/paragraphs <= 20
}

// referenceMarkerRE reconhece notas e bibliografia: anos de publicação,
// DOI/URL e termos de editoras/periódicos.
var referenceMarkerRE = regexp.MustCompile(`(?i)\b(?:1[89]\d{2}|20\d{2})\b|\bhttps?://|doi:|\bvol\.|\bpp?\.|\bed\.|\bpress\b|\bjournal\b|\bpublishing\b|\bpublications?\b`)

// Notas/bibliografia mantêm títulos, periódicos e editoras no idioma original
// (regra 3 do prompt), então o gate de texto misturado acusaria uma tradução
// correta (Pre-Suasion ref/notes, Influence índice 4). Prosa some ~0–0.3
// marcadores por 100 palavras; notas passam de 3.
const (
	referenceMinMarkers = 10
	referenceMinWords   = 300
	referenceRatePer100 = 1.0
)

// referenceDense informa se o trecho é dominado por citações — nesse caso o
// gate de texto misturado é pulado.
func referenceDense(text string) bool {
	words := len(strings.Fields(text))
	if words < referenceMinWords {
		return false
	}
	markers := len(referenceMarkerRE.FindAllString(text, -1))
	return markers >= referenceMinMarkers && float64(markers)*100/float64(words) >= referenceRatePer100
}

func rejectMixedLanguage(sourceText, outText string) error {
	srcSents := splitSentences(sourceText)
	srcWords := make([][]string, 0, len(srcSents))
	for _, sent := range srcSents {
		srcWords = append(srcWords, normalizeWords(sent))
	}
	srcJoined := strings.Join(normalizeWords(sourceText), " ")

	copied, fresh := 0, 0
	for _, sent := range splitSentences(outText) {
		words := normalizeWords(sent)
		if len(words) < mixedMinSentenceWords {
			continue
		}
		if sentenceCopiedFrom(srcJoined, srcWords, words) {
			copied += len(words)
			continue
		}
		fresh += len(words)
	}
	if copied >= mixedMinCopiedWords && fresh >= mixedMinNewWords {
		return fmt.Errorf("texto misturado: parte ainda no idioma original")
	}
	return nil
}

func sentenceCopiedFrom(srcJoined string, srcSents [][]string, words []string) bool {
	joined := strings.Join(words, " ")
	if strings.Contains(srcJoined, joined) {
		return true
	}
	best := 0.0
	for _, src := range srcSents {
		if len(src) < 5 {
			continue
		}
		if j := wordJaccard(words, src); j > best {
			best = j
		}
	}
	return best >= mixedCopyJaccard
}

func splitSentences(text string) []string {
	parts := sentenceSplitRE.Split(text, -1)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func normalizeWords(text string) []string {
	var builder strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(unicode.ToLower(r))
			continue
		}
		builder.WriteByte(' ')
	}
	return strings.Fields(builder.String())
}

func wordJaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := make(map[string]struct{}, len(a))
	for _, word := range a {
		setA[word] = struct{}{}
	}
	inter := 0
	setB := make(map[string]struct{}, len(b))
	for _, word := range b {
		if _, seen := setB[word]; seen {
			continue
		}
		setB[word] = struct{}{}
		if _, ok := setA[word]; ok {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
