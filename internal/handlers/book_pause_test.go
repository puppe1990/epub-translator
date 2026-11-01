package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBookShowsPauseWhileTranslatingAndResumeWhenPaused(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, _ := seedStoredBook(t, s, "")
	for _, status := range []string{"translating", "paused"} {
		if err := s.SetBookStatus(bookID, status); err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		handler.Show(rr, httptest.NewRequest(http.MethodGet, "/books/1", nil), bookID)
		page := rr.Body.String()
		if status == "translating" && !strings.Contains(page, `action="/books/1/pause"`) {
			t.Error("livro em tradução precisa oferecer Pausar")
		}
		if status == "paused" {
			if !strings.Contains(page, "Pausado") || !strings.Contains(page, "Retomar tradução") {
				t.Error("livro pausado precisa mostrar seu estado e oferecer retomada")
			}
			if strings.Contains(page, "A tradução continua mesmo com esta aba fechada") {
				t.Error("livro pausado não pode afirmar que continua traduzindo")
			}
		}
	}
}

func TestBookPauseIsIdempotentAndResumePreservesCompletedChapters(t *testing.T) {
	handler, s, runner, _ := newBooksHandler(t)
	bookID, chapters := seedStoredBook(t, s, "")
	_ = s.SetChapterDone(chapters[0].ID, "<p>Já traduzido.</p>")
	for range 2 {
		rr := httptest.NewRecorder()
		handler.Pause(rr, httptest.NewRequest(http.MethodPost, "/books/1/pause", nil), bookID)
		book, _ := s.FindBook(bookID)
		if rr.Code != http.StatusSeeOther || book.Status != "paused" || book.DoneChapters != 1 {
			t.Fatalf("pausa não preservou estado: status=%d, livro=%+v", rr.Code, book)
		}
	}
	handler.Retry(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/books/1/retry", nil), bookID)
	runner.Wait(bookID)
	handler.Pause(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/books/1/pause", nil), bookID)
	book, _ := s.FindBook(bookID)
	first, _ := s.FindChapter(chapters[0].ID)
	if book.Status != "done" || book.DoneChapters != 2 || first.TranslatedText != "<p>Já traduzido.</p>" {
		t.Errorf("pausa/retomada alterou tradução concluída: %+v", book)
	}
	missing := httptest.NewRecorder()
	handler.Pause(missing, httptest.NewRequest(http.MethodPost, "/books/99/pause", nil), 99)
	if missing.Code != http.StatusNotFound {
		t.Errorf("livro inexistente: status=%d", missing.Code)
	}
}
