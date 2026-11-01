package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puppe1990/epub-translator/internal/models"
)

func seedChaptersWithStatuses(t *testing.T, s interface {
	CreateBook(models.Book) (int64, error)
	CreateChapters(int64, []models.Chapter) error
	ListChapters(int64) ([]models.Chapter, error)
	SetChapterDone(int64, string) error
	SetChapterFailed(int64, string) error
	SetChapterRunning(int64) error
}) int64 {
	t.Helper()
	bookID, err := s.CreateBook(models.Book{
		Title: "Filtro", TargetLanguage: "pt-BR", Filename: "filtro.epub",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateChapters(bookID, []models.Chapter{
		{Position: 1, DocPath: "ch01.xhtml", Title: "Capítulo traduzido", SourceText: "<p>Hello one.</p>"},
		{Position: 2, DocPath: "ch02.xhtml", Title: "Capítulo falhou", SourceText: "<p>Hello two.</p>"},
		{Position: 3, DocPath: "ch03.xhtml", Title: "Capítulo na fila", SourceText: "<p>Hello three.</p>"},
		{Position: 4, DocPath: "ch04.xhtml", Title: "Capítulo traduzindo", SourceText: "<p>Hello four.</p>"},
	}); err != nil {
		t.Fatal(err)
	}
	chapters, err := s.ListChapters(bookID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetChapterDone(chapters[0].ID, "<p>Olá um.</p>"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetChapterFailed(chapters[1].ID, "tags alteradas"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetChapterRunning(chapters[3].ID); err != nil {
		t.Fatal(err)
	}
	return bookID
}

func TestBooksShow_filtersChaptersByStatus(t *testing.T) {
	handler, store, _, _ := newBooksHandler(t)
	bookID := seedChaptersWithStatuses(t, store)

	req := httptest.NewRequest(http.MethodGet, "/books/1?status=failed", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req, bookID)
	page := rr.Body.String()

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(page, "Capítulo falhou") {
		t.Error("filtro failed deveria manter o capítulo que falhou")
	}
	for _, hidden := range []string{"Capítulo traduzido", "Capítulo na fila", "Capítulo traduzindo"} {
		if strings.Contains(page, hidden) {
			t.Errorf("filtro failed ainda listou %q", hidden)
		}
	}
	if !strings.Contains(page, `aria-current="page"`) {
		t.Error("o filtro ativo precisa de aria-current")
	}
	if !strings.Contains(page, `href="/books/1?status=failed"`) {
		t.Error("chip Falhou deveria apontar para ?status=failed")
	}
	if !strings.Contains(page, `data-amarra-stream="/books/1/events?status=failed"`) {
		t.Error("SSE tem que repetir o filtro senão o morph traz a lista inteira")
	}
}

func TestBooksShow_unknownStatusShowsEveryChapter(t *testing.T) {
	handler, store, _, _ := newBooksHandler(t)
	bookID := seedChaptersWithStatuses(t, store)

	req := httptest.NewRequest(http.MethodGet, "/books/1?status=banana", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req, bookID)
	page := rr.Body.String()

	for _, title := range []string{"Capítulo traduzido", "Capítulo falhou", "Capítulo na fila", "Capítulo traduzindo"} {
		if !strings.Contains(page, title) {
			t.Errorf("status inválido deveria mostrar %q", title)
		}
	}
	if !strings.Contains(page, `data-amarra-stream="/books/1/events"`) {
		t.Error("sem filtro válido o stream não leva query")
	}
}

func TestBooksShow_emptyStatusFilterKeepsChips(t *testing.T) {
	handler, store, _, _ := newBooksHandler(t)
	bookID, chapters := seedStoredBook(t, store, "")
	_ = store.SetChapterDone(chapters[0].ID, "<p>Olá.</p>")
	_ = store.SetChapterDone(chapters[1].ID, "<p>Olá dois.</p>")

	req := httptest.NewRequest(http.MethodGet, "/books/1?status=failed", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req, bookID)
	page := rr.Body.String()

	if !strings.Contains(page, "Nenhum capítulo com este status.") {
		t.Error("filtro sem linhas deveria explicar o vazio")
	}
	if !strings.Contains(page, "Todos") || !strings.Contains(page, "Falhou") {
		t.Error("vazio ainda precisa dos chips para sair do filtro")
	}
}

func TestBooksShow_pausedChapterIsNotListedAsQueued(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, chapters := seedStoredBook(t, s, "")
	_ = s.SetChapterRunning(chapters[0].ID)
	if err := s.PauseBook(bookID); err != nil {
		t.Fatal(err)
	}

	queued := httptest.NewRecorder()
	handler.Show(queued, httptest.NewRequest(http.MethodGet, "/books/1?status=pending", nil), bookID)
	queuePage := queued.Body.String()
	if !strings.Contains(queuePage, "Nenhum capítulo com este status.") {
		t.Error("capítulo interrompido não pode aparecer em Na fila")
	}
	if !strings.Contains(queuePage, `href="/books/1?status=paused"`) {
		t.Error("chips precisam de Pausado")
	}

	parked := httptest.NewRecorder()
	handler.Show(parked, httptest.NewRequest(http.MethodGet, "/books/1?status=paused", nil), bookID)
	parkedPage := parked.Body.String()
	if !strings.Contains(parkedPage, ">Um<") && !strings.Contains(parkedPage, ">Um</span>") {
		t.Error("filtro paused deveria listar o capítulo estacionado")
	}
	if !strings.Contains(parkedPage, `aria-label="Pausado"`) && !strings.Contains(parkedPage, ">Pausado<") {
		t.Error("capítulo estacionado precisa do rótulo Pausado")
	}
}

func TestChapterList_statusFilterChipsCountEveryStatus(t *testing.T) {
	_, list, err := RenderBookFragments(models.Book{ID: 7}, []models.Chapter{
		{ID: 1, Position: 1, Title: "Feito", Status: "done"},
		{ID: 2, Position: 2, Title: "Erro", Status: "failed"},
		{ID: 3, Position: 3, Title: "Fila", Status: "pending"},
		{ID: 4, Position: 4, Title: "Agora", Status: "running"},
	}, "pending")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(list, "Feito") || strings.Contains(list, "Erro") || strings.Contains(list, "Agora") {
		t.Error("lista filtrada em pending vazou outro status")
	}
	if !strings.Contains(list, "Fila") {
		t.Error("lista pending perdeu o capítulo na fila")
	}
	for _, chip := range []string{
		`href="/books/7"`,
		`href="/books/7?status=pending"`,
		`href="/books/7?status=running"`,
		`href="/books/7?status=done"`,
		`href="/books/7?status=paused"`,
		`href="/books/7?status=failed"`,
		"Todos", "Na fila", "Traduzindo", "Pausado", "Traduzido", "Falhou",
	} {
		if !strings.Contains(list, chip) {
			t.Errorf("chips sem %q", chip)
		}
	}
}
