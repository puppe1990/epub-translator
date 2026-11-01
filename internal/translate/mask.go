package translate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func tagPlaceholder(i int) string {
	return fmt.Sprintf("⟦t%d⟧", i)
}

var placeholderRE = regexp.MustCompile(`⟦\s*t\s*(\d+)\s*⟧`)

func maskHTMLTags(html string) (string, []string) {
	var tags []string
	masked := tagRE.ReplaceAllStringFunc(html, func(tag string) string {
		token := tagPlaceholder(len(tags))
		tags = append(tags, tag)
		return token
	})
	return masked, tags
}

func selfClosingTag(tag string) bool {
	t := strings.TrimSpace(tag)
	return !strings.HasPrefix(t, "</") && strings.HasSuffix(t, "/>")
}

// keepIfDropped recoloca âncoras do EPUB que o modelo dropa: page-breaks
// self-closing (`<a id="page_3"/>`) e o par do INDEX (`<a href>…</a>`),
// em que o Gemini costuma copiar o </a> e omitir a abertura.
func keepIfDropped(tag string) bool {
	if selfClosingTag(tag) {
		return true
	}
	match := tagRE.FindStringSubmatch(tag)
	return len(match) >= 3 && strings.EqualFold(match[2], "a")
}

func unmaskHTMLTags(text string, tags []string) string {
	type span struct{ start, end int }
	seen := map[int]span{}
	for _, match := range placeholderRE.FindAllStringSubmatchIndex(text, -1) {
		n, err := strconv.Atoi(text[match[2]:match[3]])
		if err != nil {
			continue
		}
		if _, exists := seen[n]; !exists {
			seen[n] = span{match[0], match[1]}
		}
	}
	var builder strings.Builder
	cursor := 0
	for i, tag := range tags {
		if pos, ok := seen[i]; ok && pos.start >= cursor {
			builder.WriteString(text[cursor:pos.start])
			builder.WriteString(tag)
			cursor = pos.end
			continue
		}
		if keepIfDropped(tag) {
			builder.WriteString(tag)
		}
	}
	builder.WriteString(text[cursor:])
	return builder.String()
}
