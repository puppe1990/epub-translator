package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/puppe1990/epub-translator/internal/models"
)

func seedChaptersWithProviders(t *testing.T, s interface {
	CreateBook(models.Book) (int64, error)
	CreateChapters(int64, []models.Chapter) error
	ListChapters(int64) ([]models.Chapter, error)
	StartChapterExecution(int64, models.TranslationDetails, time.Time) error
	SetChapterDone(int64, string) error
	SetChapterFailed(int64, string) error
}) int64 {
	t.Helper()
	bookID, err := s.CreateBook(models.Book{
		Title: "Provedores", TargetLanguage: "pt-BR", Filename: "provedores.epub",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateChapters(bookID, []models.Chapter{
		{Position: 1, DocPath: "ch01.xhtml", Title: "Capítulo Gemini", SourceText: "<p>Hello gemini.</p>"},
		{Position: 2, DocPath: "ch02.xhtml", Title: "Capítulo NVIDIA", SourceText: "<p>Hello nvidia.</p>"},
		{Position: 3, DocPath: "ch03.xhtml", Title: "Capítulo sem registro", SourceText: "<p>Hello none.</p>"},
	}); err != nil {
		t.Fatal(err)
	}
	chapters, err := s.ListChapters(bookID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	if err := s.StartChapterExecution(chapters[0].ID, models.TranslationDetails{
		Provider: "gemini", Model: "gemini-3.5-flash-lite",
	}, now); err != nil {
		t.Fatal(err)
	}
	if err := s.SetChapterDone(chapters[0].ID, "<p>Olá gemini.</p>"); err != nil {
		t.Fatal(err)
	}
	if err := s.StartChapterExecution(chapters[1].ID, models.TranslationDetails{
		Provider: "nvidia", Model: "nvidia/riva-translate-4b-instruct-v2",
	}, now); err != nil {
		t.Fatal(err)
	}
	if err := s.SetChapterFailed(chapters[1].ID, "tags alteradas"); err != nil {
		t.Fatal(err)
	}
	return bookID
}

func TestBooksShow_filtersChaptersByProvider(t *testing.T) {
	handler, store, _, _ := newBooksHandler(t)
	bookID := seedChaptersWithProviders(t, store)

	req := httptest.NewRequest(http.MethodGet, "/books/1?provider=gemini", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req, bookID)
	page := rr.Body.String()

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(page, "Capítulo Gemini") {
		t.Error("filtro gemini deveria manter o capítulo do Gemini")
	}
	for _, hidden := range []string{"Capítulo NVIDIA", "Capítulo sem registro"} {
		if strings.Contains(page, hidden) {
			t.Errorf("filtro gemini ainda listou %q", hidden)
		}
	}
	if !strings.Contains(page, `href="/books/1?provider=gemini"`) {
		t.Error("chip Gemini deveria apontar para ?provider=gemini")
	}
	if !strings.Contains(page, `data-amarra-stream="/books/1/events?provider=gemini"`) {
		t.Error("SSE tem que repetir o provedor senão o morph traz a lista inteira")
	}
	if !strings.Contains(page, `aria-label="Filtrar capítulos por provedor"`) {
		t.Error("chips de provedor precisam de rótulo próprio")
	}
}

func TestBooksShow_filtersChaptersByModel(t *testing.T) {
	handler, store, _, _ := newBooksHandler(t)
	bookID := seedChaptersWithProviders(t, store)

	req := httptest.NewRequest(http.MethodGet, "/books/1?model=gemini-3.5-flash-lite", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req, bookID)
	page := rr.Body.String()

	if !strings.Contains(page, "Capítulo Gemini") {
		t.Error("filtro do modelo deveria manter o capítulo correspondente")
	}
	if strings.Contains(page, "Capítulo NVIDIA") {
		t.Error("filtro do modelo vazou o capítulo de outro modelo")
	}
	if !strings.Contains(page, `href="/books/1?model=gemini-3.5-flash-lite"`) {
		t.Error("chip do modelo deveria apontar para ?model=")
	}
	if !strings.Contains(page, `data-amarra-stream="/books/1/events?model=gemini-3.5-flash-lite"`) {
		t.Error("SSE tem que repetir o modelo")
	}
}

func TestBooksShow_providerFilterKeepsStatusQuery(t *testing.T) {
	handler, store, _, _ := newBooksHandler(t)
	bookID := seedChaptersWithProviders(t, store)

	req := httptest.NewRequest(http.MethodGet, "/books/1?provider=nvidia&status=failed", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req, bookID)
	page := rr.Body.String()

	if !strings.Contains(page, "Capítulo NVIDIA") {
		t.Error("nvidia+failed deveria listar o capítulo NVIDIA")
	}
	if strings.Contains(page, "Capítulo Gemini") {
		t.Error("nvidia+failed vazou o capítulo Gemini")
	}
	if !strings.Contains(page, `href="/books/1?provider=nvidia&amp;status=failed"`) &&
		!strings.Contains(page, `href="/books/1?status=failed&amp;provider=nvidia"`) {
		t.Error("chips precisam preservar provedor e status juntos")
	}
	if !strings.Contains(page, `data-amarra-stream="/books/1/events?provider=nvidia&amp;status=failed"`) &&
		!strings.Contains(page, `data-amarra-stream="/books/1/events?status=failed&amp;provider=nvidia"`) {
		t.Error("SSE precisa dos dois filtros")
	}
}

func TestBooksShow_unknownProviderShowsEveryChapter(t *testing.T) {
	handler, store, _, _ := newBooksHandler(t)
	bookID := seedChaptersWithProviders(t, store)

	req := httptest.NewRequest(http.MethodGet, "/books/1?provider=banana!", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req, bookID)
	page := rr.Body.String()

	for _, title := range []string{"Capítulo Gemini", "Capítulo NVIDIA", "Capítulo sem registro"} {
		if !strings.Contains(page, title) {
			t.Errorf("provedor inválido deveria mostrar %q", title)
		}
	}
}

func TestBooksShow_unrecordedProviderChip(t *testing.T) {
	handler, store, _, _ := newBooksHandler(t)
	bookID := seedChaptersWithProviders(t, store)

	req := httptest.NewRequest(http.MethodGet, "/books/1?provider=none", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req, bookID)
	page := rr.Body.String()

	if !strings.Contains(page, "Capítulo sem registro") {
		t.Error("filtro Sem registro deveria listar capítulo sem provedor")
	}
	if strings.Contains(page, "Capítulo Gemini") || strings.Contains(page, "Capítulo NVIDIA") {
		t.Error("Sem registro vazou capítulo com provedor")
	}
}
