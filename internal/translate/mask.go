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

// keepIfDropped recoloca qualquer tag cujo placeholder o modelo omitiu.
// O XHTML original é a fonte da verdade; o modelo só traduz o texto. Sem
// isso o INDEX perde um </p> (cap. 222: fonte 401, resposta 400).
func keepIfDropped(_ string) bool {
	return true
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
