package pipeline

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/store"
)

func newTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:", "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seedBook(t *testing.T, s store.Store) int64 {
	t.Helper()
	bookID, err := s.CreateBook(models.Book{Title: "Livro", TargetLanguage: "pt-BR"})
	if err != nil {
		t.Fatal(err)
	}
	chapters := []models.Chapter{
		{Position: 1, DocPath: "ch01.xhtml", Title: "Um", SourceText: "<p>Hello one</p>"},
		{Position: 2, DocPath: "ch02.xhtml", Title: "Dois", SourceText: "<p>Hello two</p>"},
		{Position: 3, DocPath: "ch03.xhtml", Title: "Três", SourceText: "<p>Hello three</p>"},
	}
	if err := s.CreateChapters(bookID, chapters); err != nil {
		t.Fatal(err)
	}
	return bookID
}

type fakeTranslator struct {
	mu      sync.Mutex
	failing map[string]bool
	calls   int
}

func (f *fakeTranslator) TranslateBody(_ context.Context, _, body string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.failing[body] {
		return "", errors.New("provedor respondeu 429")
	}
	return strings.NewReplacer("Hello", "Olá").Replace(body), nil
}

// blockingTranslator segura a primeira chamada até o release — deixa o teste
// do guard de Start determinístico.
type blockingTranslator struct {
	release chan struct{}
	once    sync.Once
}

func (b *blockingTranslator) TranslateBody(_ context.Context, _, body string) (string, error) {
	b.once.Do(func() { <-b.release })
	return strings.NewReplacer("Hello", "Olá").Replace(body), nil
}

func TestRunner_translatesEveryChapterAndClosesBook(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)
	runner := NewRunner(s, &fakeTranslator{})

	if !runner.Start(bookID) {
		t.Fatal("Start devolveu false na primeira vez")
	}
	runner.Wait(bookID)

	book, _ := s.FindBook(bookID)
	if book.Status != "done" || book.DoneChapters != 3 || book.FailedChapters != 0 {
		t.Errorf("book = %+v", book)
	}
	chapters, _ := s.ListChapters(bookID)
	for _, chapter := range chapters {
		if chapter.Status != "done" {
			t.Errorf("capítulo %d não concluído: %+v", chapter.Position, chapter)
		}
		full, _ := s.FindChapter(chapter.ID)
		if !strings.Contains(full.TranslatedText, "Olá") {
			t.Errorf("capítulo %d sem tradução: %q", chapter.Position, full.TranslatedText)
		}
	}
}

func TestRunner_refusesSecondRunWhileBusy(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)
	release := make(chan struct{})
	runner := NewRunner(s, &blockingTranslator{release: release})

	if !runner.Start(bookID) {
		t.Fatal("Start devolveu false na primeira vez")
	}
	if runner.Start(bookID) {
		t.Fatal("Start deveria recusar com o livro rodando")
	}
	close(release)
	runner.Wait(bookID)

	book, _ := s.FindBook(bookID)
	if book.Status != "done" {
		t.Errorf("book = %+v", book)
	}
}

type holdingOrderTranslator struct {
	started chan struct{}
	release chan struct{}
	mu      sync.Mutex
	order   []string
}

func (h *holdingOrderTranslator) TranslateBody(ctx context.Context, _, body string) (string, error) {
	h.mu.Lock()
	h.order = append(h.order, body)
	h.mu.Unlock()
	h.started <- struct{}{}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-h.release:
		return strings.ReplaceAll(body, "Hello", "Olá"), nil
	}
}

func TestRunner_preferredChapterRunsNextDuringActiveRun(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)
	chapters, _ := s.ListChapters(bookID)
	translator := &holdingOrderTranslator{started: make(chan struct{}, 10), release: make(chan struct{})}
	runner := NewRunner(s, translator)
	if !runner.Start(bookID) {
		t.Fatal("Start")
	}
	select {
	case <-translator.started:
	case <-time.After(2 * time.Second):
		t.Fatal("capítulo atual não começou")
	}
	if runner.StartPreferred(bookID, chapters[2].ID) {
		t.Fatal("StartPreferred deveria só furar a fila com o livro já rodando")
	}
	close(translator.release)
	runner.Wait(bookID)

	translator.mu.Lock()
	order := append([]string{}, translator.order...)
	translator.mu.Unlock()
	if len(order) < 2 || order[1] != "<p>Hello three</p>" {
		t.Fatalf("depois do capítulo atual a fila foi %v, queria o capítulo 3 em seguida", order)
	}
}

func TestRunner_resumeInterruptedRestartsTranslatingBooks(t *testing.T) {
	s := newTestStore(t)
	activeID := seedBook(t, s)
	pausedID := seedBook(t, s)
	queuedID := seedBook(t, s)
	activeChapters, _ := s.ListChapters(activeID)
	if err := s.SetChapterDone(activeChapters[0].ID, "<p>Olá one</p>"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetChapterRunning(activeChapters[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBookStatus(activeID, "translating"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBookStatus(pausedID, "paused"); err != nil {
		t.Fatal(err)
	}

	translator := &fakeTranslator{}
	runner := NewRunner(s, translator)
	if err := runner.ResumeInterrupted(); err != nil {
		t.Fatal(err)
	}
	runner.Wait(activeID)

	active, _ := s.FindBook(activeID)
	if active.Status != "done" || active.DoneChapters != 3 {
		t.Errorf("livro em tradução não retomou: %+v", active)
	}
	paused, _ := s.FindBook(pausedID)
	if paused.Status != "paused" {
		t.Errorf("pausa não sobreviveu ao boot: %+v", paused)
	}
	queued, _ := s.FindBook(queuedID)
	if queued.Status != "pending" || queued.DoneChapters != 0 {
		t.Errorf("fila não pedida não pode arrancar no boot: %+v", queued)
	}
	translator.mu.Lock()
	calls := translator.calls
	translator.mu.Unlock()
	if calls != 2 {
		t.Errorf("chamadas = %d, want 2 (capítulo travado + restante do livro em tradução)", calls)
	}
}

func TestRunner_marksFailureAndRetryCompletes(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)
	translator := &fakeTranslator{failing: map[string]bool{"<p>Hello two</p>": true}}
	runner := NewRunner(s, translator)

	runner.Start(bookID)
	runner.Wait(bookID)

	book, _ := s.FindBook(bookID)
	if book.Status != "failed" || book.DoneChapters != 2 || book.FailedChapters != 1 {
		t.Errorf("book = %+v", book)
	}
	chapters, _ := s.ListChapters(bookID)
	failed := chapters[1]
	if failed.Status != "failed" || failed.Attempts != 1 || !strings.Contains(failed.Error, "429") {
		t.Errorf("capítulo falho = %+v", failed)
	}

	translator.mu.Lock()
	translator.failing = map[string]bool{}
	translator.mu.Unlock()

	runner.Start(bookID)
	runner.Wait(bookID)

	book, _ = s.FindBook(bookID)
	if book.Status != "done" || book.DoneChapters != 3 {
		t.Errorf("após retry book = %+v", book)
	}
}
