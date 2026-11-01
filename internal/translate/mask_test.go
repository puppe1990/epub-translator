package translate

import (
	"strings"
	"testing"
)

func TestMaskHTMLTags_hidesTagsFromTheModel(t *testing.T) {
	source := `<h2 id="x"><a id="page_3"/>CHAPTER 1</h2>`
	masked, tags := maskHTMLTags(source)
	if strings.Contains(masked, "<h2") || strings.Contains(masked, "<a ") {
		t.Fatalf("Riva não deve ver tags cruas: %q", masked)
	}
	if !strings.Contains(masked, "CHAPTER 1") {
		t.Fatalf("o texto visível precisa continuar no payload: %q", masked)
	}
	if len(tags) != 3 {
		t.Fatalf("tags = %d, want 3 (h2, a, /h2), got %#v", len(tags), tags)
	}
}

func TestUnmaskHTMLTags_roundTripKeepsMarkup(t *testing.T) {
	source := `<p>Hello <i>world</i>.</p>`
	masked, tags := maskHTMLTags(source)
	got := unmaskHTMLTags(masked, tags)
	if got != source {
		t.Errorf("got %q want %q", got, source)
	}
}

func TestUnmaskHTMLTags_reinsertsDroppedSelfClosingAnchors(t *testing.T) {
	source := `<h2 id="x"><a id="page_3"/><a id="ch01"/>CHAPTER 1</h2><p>Hello world, good morning everyone.</p>`
	masked, tags := maskHTMLTags(source)
	translated := strings.ReplaceAll(masked, "CHAPTER 1", "CAPÍTULO 1")
	translated = strings.ReplaceAll(translated, "Hello world, good morning everyone.", "Olá mundo, bom dia a todas as pessoas.")
	for _, tag := range tags {
		if !strings.HasSuffix(strings.TrimSpace(tag), "/>") {
			continue
		}
		translated = strings.Replace(translated, tagPlaceholder(indexOfTag(tags, tag)), "", 1)
	}
	got := unmaskHTMLTags(translated, tags)
	if err := ValidateBody(source, got); err != nil {
		t.Fatalf("âncoras vazias deveriam ter voltado: %v\ngot=%s", err, got)
	}
	if !strings.Contains(got, `id="page_3"`) || !strings.Contains(got, `id="ch01"`) {
		t.Errorf("perdeu âncora: %s", got)
	}
	if !strings.Contains(got, "CAPÍTULO 1") || !strings.Contains(got, "Olá mundo") {
		t.Errorf("não traduziu o texto: %s", got)
	}
}

func TestUnmaskHTMLTags_reinsertsDroppedHrefOpenings(t *testing.T) {
	// INDEX do Total Recall: o Gemini copia o </a> e dropa o <a href>.
	// ValidateBody então vê fonte N, resposta N-1, faltando [a], sobrando [].
	source := `<p class="indexmain">accent, Arnold’s, <a class="nounder1" href="part0057.html#page_193">193</a></p>`
	masked, tags := maskHTMLTags(source)
	translated := strings.ReplaceAll(masked, "accent, Arnold’s,", "sotaque, de Arnold,")
	for i, tag := range tags {
		if strings.Contains(strings.ToLower(tag), "href=") {
			translated = strings.Replace(translated, tagPlaceholder(i), "", 1)
		}
	}
	got := unmaskHTMLTags(translated, tags)
	if err := ValidateBody(source, got); err != nil {
		t.Fatalf("abertura <a href> dropada deveria voltar: %v\ngot=%s", err, got)
	}
	if !strings.Contains(got, `href="part0057.html#page_193"`) {
		t.Errorf("perdeu o href: %s", got)
	}
}

func TestUnmaskHTMLTags_reinsertsEmptyAnchorPairs(t *testing.T) {
	source := `<h2><a id="page_3"></a>CHAPTER 1</h2><p>Hello world, good morning everyone.</p>`
	masked, tags := maskHTMLTags(source)
	translated := strings.ReplaceAll(masked, "CHAPTER 1", "CAPÍTULO 1")
	translated = strings.ReplaceAll(translated, "Hello world, good morning everyone.", "Olá mundo, bom dia a todas as pessoas.")
	for i, tag := range tags {
		name := strings.ToLower(tag)
		if strings.Contains(name, "<a") || strings.HasPrefix(strings.TrimSpace(tag), "</a") {
			translated = strings.Replace(translated, tagPlaceholder(i), "", 1)
		}
	}
	got := unmaskHTMLTags(translated, tags)
	if err := ValidateBody(source, got); err != nil {
		t.Fatalf("par <a></a> vazio deveria voltar: %v\ngot=%s", err, got)
	}
	if !strings.Contains(got, `id="page_3"`) {
		t.Errorf("perdeu âncora: %s", got)
	}
}

func indexOfTag(tags []string, tag string) int {
	for i, candidate := range tags {
		if candidate == tag {
			return i
		}
	}
	return -1
}
