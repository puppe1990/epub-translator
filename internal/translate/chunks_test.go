package translate

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitBody_keepsBodyUnderBudgetWhole(t *testing.T) {
	body := "<p>Hello world.</p>"
	got := splitBody(body, 100)
	if len(got) != 1 || got[0] != body {
		t.Errorf("splitBody = %#v", got)
	}
}

func TestSplitBody_packsParagraphsWithoutExceedingBudget(t *testing.T) {
	first := "<p>" + strings.Repeat("Hello world. ", 8) + "</p>"
	second := "<p>" + strings.Repeat("Good morning. ", 8) + "</p>"
	body := first + second
	budget := utf8.RuneCountInString(first) + 10
	got := splitBody(body, budget)
	if len(got) != 2 {
		t.Fatalf("chunks = %d, want 2 (budget %d, first %d)", len(got), budget, utf8.RuneCountInString(first))
	}
	if strings.Join(got, "") != body {
		t.Fatal("juntar as partes tem que reproduzir o original")
	}
	for i, chunk := range got {
		if utf8.RuneCountInString(chunk) > budget {
			t.Errorf("chunk %d tem %d runes, orçamento %d", i, utf8.RuneCountInString(chunk), budget)
		}
	}
}

func TestSplitBody_hardSplitsGiantParagraphWithoutBreakingTags(t *testing.T) {
	body := "<p>" + strings.Repeat("Hello world. ", 80) + "</p>"
	budget := 120
	got := splitBody(body, budget)
	if len(got) < 2 {
		t.Fatalf("parágrafo gigante deveria fatiar, chunks=%d", len(got))
	}
	if strings.Join(got, "") != body {
		t.Fatal("juntar as partes tem que reproduzir o original")
	}
	for i, chunk := range got {
		if utf8.RuneCountInString(chunk) > budget {
			t.Errorf("chunk %d tem %d runes, orçamento %d", i, utf8.RuneCountInString(chunk), budget)
		}
		if strings.Count(chunk, "<") != strings.Count(chunk, ">") {
			t.Errorf("chunk %d partiu uma tag: %q", i, snippet(chunk, 60))
		}
	}
}

func TestInputCharBudget_rivaLeavesRoomForOutputIn8k(t *testing.T) {
	got := InputCharBudget("nvidia/riva-translate-4b-instruct-v2")
	if got <= 0 || got >= 8192 {
		t.Errorf("Riva 8k precisa de folga para a saída, orçamento=%d", got)
	}
	if InputCharBudget("openai/gpt-oss-20b") != 0 {
		t.Error("modelos de contexto longo não fatiam")
	}
}

func TestInputCharBudget_geminiSplitsLargeIndexUnderOutputCap(t *testing.T) {
	// gemini-3.5-flash-lite: 1M de entrada, 65_536 de saída. O INDEX do
	// Total Recall tem ~80k tokens / 218k chars — um shot só trunca e o
	// ValidateBody perde centenas de <p>/<a>.
	budget := InputCharBudget("gemini-3.5-flash-lite")
	if budget <= 0 {
		t.Fatal("Gemini precisa fatiar índice grande: senão a saída trunca em 64k tokens")
	}
	if budget >= 65536 {
		t.Fatalf("orçamento %d não deixa folga para a saída de 64k", budget)
	}
	if InputCharBudget("google/gemini-2.5-flash") != budget {
		t.Error("OpenRouter google/gemini-* também precisa fatiar")
	}
	body := indexLikeBody(1302)
	parts := splitBody(body, budget)
	if len(parts) < 2 {
		t.Fatalf("índice de %d runes deveria virar várias partes, parts=%d budget=%d",
			utf8.RuneCountInString(body), len(parts), budget)
	}
	if strings.Join(parts, "") != body {
		t.Fatal("juntar as partes tem que reproduzir o original")
	}
	for i, chunk := range parts {
		if utf8.RuneCountInString(chunk) > budget {
			t.Errorf("chunk %d tem %d runes, orçamento %d", i, utf8.RuneCountInString(chunk), budget)
		}
	}
}

func indexLikeBody(entries int) string {
	var b strings.Builder
	b.WriteString(`<h2 class="h2c" id="idx"><a class="calibre2" id="page_625"/><a class="calibre2" id="index"/>INDEX</h2>`)
	for i := 0; i < entries; i++ {
		b.WriteString(`<p class="indexmain">topic `)
		b.WriteString(strings.Repeat("entry ", 4))
		b.WriteString(`<a class="nounder1" href="part.html#page_n">`)
		b.WriteString("12")
		b.WriteString(`</a></p>`)
	}
	return b.String()
}
