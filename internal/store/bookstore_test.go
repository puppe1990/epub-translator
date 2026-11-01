package store

import (
	"errors"
	"testing"
	"unicode/utf8"

	"github.com/puppe1990/epub-translator/internal/models"
)

func seedBook(t *testing.T, s *SQLiteStore) int64 {
	t.Helper()
	bookID, err := s.CreateBook(models.Book{
		Title: "Livro Teste", Author: "Autora", SourceLanguage: "en",
		TargetLanguage: "pt-BR", Filename: "teste.epub", SourcePath: "data/books/1.epub",
	})
	if err != nil {
		t.Fatal(err)
	}
	chapters := []models.Chapter{
		{Position: 1, DocPath: "OEBPS/ch01.xhtml", Title: "Um", SourceText: "<p>One</p>"},
		{Position: 2, DocPath: "OEBPS/ch02.xhtml", Title: "Dois", SourceText: "<p>Two</p>"},
		{Position: 3, DocPath: "OEBPS/ch03.xhtml", Title: "Três", SourceText: "<p>Three</p>"},
	}
	if err := s.CreateChapters(bookID, chapters); err != nil {
		t.Fatal(err)
	}
	return bookID
}

func TestStore_CreateBookAndChaptersThenList(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)

	chapters, err := s.ListChapters(bookID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 3 || chapters[0].DocPath != "OEBPS/ch01.xhtml" || chapters[2].Position != 3 {
		t.Errorf("chapters = %+v", chapters)
	}
	if chapters[0].SourceText != "" {
		t.Error("a lista não deve carregar os corpos (source_text)")
	}
	if chapters[0].SourceChars != utf8.RuneCountInString("<p>One</p>") {
		t.Errorf("SourceChars = %d, want length of original body", chapters[0].SourceChars)
	}

	book, err := s.FindBook(bookID)
	if err != nil {
		t.Fatal(err)
	}
	if book.Title != "Livro Teste" || book.TotalChapters != 3 || book.DoneChapters != 0 || book.Status != "pending" {
		t.Errorf("book = %+v", book)
	}

	if _, err := s.FindBook(9999); !errors.Is(err, ErrBookNotFound) {
		t.Errorf("err = %v, want ErrBookNotFound", err)
	}
}

func TestStore_ListChaptersCountsUnicodeCharsWithoutLoadingBodies(t *testing.T) {
	s := newTestStore(t)
	bookID, err := s.CreateBook(models.Book{
		Title: "Livro Unicode", Author: "Autora", SourceLanguage: "pt",
		TargetLanguage: "pt-BR", Filename: "unicode.epub", SourcePath: "data/books/u.epub",
	})
	if err != nil {
		t.Fatal(err)
	}
	body := "<p>ação</p>"
	if err := s.CreateChapters(bookID, []models.Chapter{
		{Position: 1, DocPath: "OEBPS/ch01.xhtml", Title: "Um", SourceText: body},
	}); err != nil {
		t.Fatal(err)
	}

	chapters, err := s.ListChapters(bookID)
	if err != nil {
		t.Fatal(err)
	}
	if chapters[0].SourceText != "" {
		t.Error("a lista não deve carregar os corpos (source_text)")
	}
	want := utf8.RuneCountInString(body)
	if chapters[0].SourceChars != want {
		t.Errorf("SourceChars = %d, want %d (runes, not bytes)", chapters[0].SourceChars, want)
	}
}

func TestStore_chapterLifecycleAndCounts(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)
	chapters, _ := s.ListChapters(bookID)

	if err := s.SetChapterRunning(chapters[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetChapterDone(chapters[0].ID, "<p>Um traduzido</p>"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetChapterFailed(chapters[1].ID, "provedor respondeu 429"); err != nil {
		t.Fatal(err)
	}

	book, _ := s.FindBook(bookID)
	if book.DoneChapters != 1 || book.FailedChapters != 1 {
		t.Errorf("counts = %+v", book)
	}

	full, err := s.FindChapter(chapters[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if full.TranslatedText != "<p>Um traduzido</p>" || full.Status != "done" {
		t.Errorf("chapter = %+v", full)
	}

	failed, _ := s.FindChapter(chapters[1].ID)
	if failed.Attempts != 1 || failed.Error == "" || failed.Status != "failed" {
		t.Errorf("failed chapter = %+v", failed)
	}

	pending, err := s.PendingChapters(bookID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 || pending[0].Position != 2 || pending[1].Position != 3 {
		t.Errorf("pending = %+v (esperava falho + pendente em ordem)", pending)
	}

	translations, err := s.ChapterTranslations(bookID)
	if err != nil {
		t.Fatal(err)
	}
	if len(translations) != 1 || translations["OEBPS/ch01.xhtml"] != "<p>Um traduzido</p>" {
		t.Errorf("translations = %+v", translations)
	}
}

func TestStore_resetRunningChapters(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)
	chapters, _ := s.ListChapters(bookID)

	if err := s.SetChapterRunning(chapters[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetRunningChapters(); err != nil {
		t.Fatal(err)
	}
	stuck, _ := s.FindChapter(chapters[0].ID)
	if stuck.Status != "pending" {
		t.Errorf("status = %q, want pending", stuck.Status)
	}
}

func TestStore_settingsRoundtrip(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetSetting("translate_api_key", "abc123"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting("translate_model", "gemini-3.5-flash-lite"); err != nil {
		t.Fatal(err)
	}
	value, err := s.GetSetting("translate_api_key")
	if err != nil || value != "abc123" {
		t.Errorf("GetSetting = %q, %v", value, err)
	}
	if err := s.SetSetting("translate_api_key", "novo"); err != nil {
		t.Fatal(err)
	}
	value, _ = s.GetSetting("translate_api_key")
	if value != "novo" {
		t.Errorf("upsert falhou: %q", value)
	}

	all, err := s.Settings()
	if err != nil || all["translate_model"] != "gemini-3.5-flash-lite" {
		t.Errorf("Settings = %v, %v", all, err)
	}
}

func TestStore_SetBookStatus(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)

	if err := s.SetBookStatus(bookID, "translating"); err != nil {
		t.Fatal(err)
	}
	book, _ := s.FindBook(bookID)
	if book.Status != "translating" {
		t.Errorf("status = %q", book.Status)
	}

	books, err := s.ListBooks()
	if err != nil || len(books) != 1 || books[0].ID != bookID {
		t.Errorf("ListBooks = %+v, %v", books, err)
	}
}
