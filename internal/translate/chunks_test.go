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
