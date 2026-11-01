package handlers

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestLibraryUploadHasRequiredFileAndNamedLanguages(t *testing.T) {
	handler, _, _, _ := newBooksHandler(t)
	rr := httptest.NewRecorder()
	handler.Index(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	page := rr.Body.String()
	for _, want := range []string{`id="epub-file"`, `required`, `for="epub-file"`, "Português do Brasil", "Sua biblioteca", `href="#amarra-main"`} {
		if !strings.Contains(page, want) {
			t.Errorf("biblioteca sem %q", want)
		}
	}
	if strings.Contains(page, "fonts.googleapis.com") || strings.Contains(page, `href="/dashboard"`) {
		t.Error("layout depende de fonte bloqueada ou rota inexistente")
	}
}

func TestProviderFieldsHaveUniqueIDsAndAssociatedLabels(t *testing.T) {
	handler, _ := newSettingsHandler(t)
	rr := httptest.NewRecorder()
	handler.Show(rr, httptest.NewRequest(http.MethodGet, "/settings", nil))
	page := rr.Body.String()
	ids := map[string]bool{}
	for _, match := range regexp.MustCompile(`\bid="([^"]+)"`).FindAllStringSubmatch(page, -1) {
		if ids[match[1]] {
			t.Errorf("id duplicado: %s", match[1])
		}
		ids[match[1]] = true
	}
	for _, slug := range []string{"openai", "gemini", "groq", "nvidia", "openrouter", "custom"} {
		if !ids["model-"+slug] || !strings.Contains(page, `for="model-`+slug+`"`) {
			t.Errorf("campo de modelo sem associação exclusiva: %s", slug)
		}
	}
}

func TestBookProgressIsAccessibleAndErrorsCanBeRead(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, chapters := seedStoredBook(t, s, "")
	_ = s.SetChapterDone(chapters[0].ID, "<p>Olá.</p>")
	_ = s.SetChapterFailed(chapters[1].ID, "provedor respondeu 429")
	rr := httptest.NewRecorder()
	handler.Show(rr, httptest.NewRequest(http.MethodGet, "/books/1", nil), bookID)
	page := rr.Body.String()
	for _, want := range []string{`<progress`, `aria-label="Progresso da tradução"`, `<details`, "Ver erro", "provedor respondeu 429", `data-amarra-stream="/books/1/events"`} {
		if !strings.Contains(page, want) {
			t.Errorf("página do livro sem %q", want)
		}
	}
}
