package pipeline

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type cancellableTranslator struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (c *cancellableTranslator) TranslateBody(ctx context.Context, _, body string) (string, error) {
	c.calls.Add(1)
	c.started <- struct{}{}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-c.release:
		return strings.ReplaceAll(body, "Hello", "Olá"), nil
	}
}

func TestRunnerPauseCancelsCurrentChapterAndResumesOnlyRemaining(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)
	chapters, _ := s.ListChapters(bookID)
	_ = s.SetChapterDone(chapters[0].ID, "<p>Tradução preservada.</p>")
	translator := &cancellableTranslator{started: make(chan struct{}, 10), release: make(chan struct{})}
	runner := NewRunner(s, translator)
	runner.Start(bookID)
	select {
	case <-translator.started:
	case <-time.After(2 * time.Second):
		t.Fatal("tradução não iniciou")
	}
	stopped, err := runner.Pause(bookID)
	if !stopped || err != nil {
		t.Fatalf("Pause=%v, err=%v", stopped, err)
	}
	runner.Wait(bookID)
	book, _ := s.FindBook(bookID)
	if book.Status != "paused" || book.DoneChapters != 1 || book.FailedChapters != 0 {
		t.Fatalf("pausa não preservou progresso: %+v", book)
	}
	chapters, _ = s.ListChapters(bookID)
	if chapters[1].StartedAt == "" || chapters[1].FinishedAt == "" {
		t.Error("pausa precisa preservar início e término da execução interrompida")
	}
	for _, chapter := range chapters[1:] {
		if chapter.Status != "paused" || chapter.Attempts != 0 || chapter.Error != "" {
			t.Errorf("capítulo interrompido não pode ser falha nem fila: %+v", chapter)
		}
	}
	_ = s.ResetRunningChapters()
	book, _ = s.FindBook(bookID)
	if book.Status != "paused" {
		t.Error("reiniciar servidor não pode remover a pausa")
	}
	close(translator.release)
	runner.Start(bookID)
	runner.Wait(bookID)
	book, _ = s.FindBook(bookID)
	if book.Status != "done" || book.DoneChapters != 3 || translator.calls.Load() != 3 {
		t.Errorf("retomada refez capítulos ou deixou pendências: %+v, chamadas=%d", book, translator.calls.Load())
	}
	first, _ := s.FindChapter(chapters[0].ID)
	if first.TranslatedText != "<p>Tradução preservada.</p>" {
		t.Error("capítulo concluído foi sobrescrito")
	}
	if stopped, _ := runner.Pause(bookID); stopped {
		t.Error("livro concluído não tem execução para cancelar")
	}
}

func TestRunner_drainLeavesBookTranslatingSoBootResumes(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)
	chapters, _ := s.ListChapters(bookID)
	_ = s.SetChapterDone(chapters[0].ID, "<p>Olá one</p>")
	translator := &cancellableTranslator{started: make(chan struct{}, 10), release: make(chan struct{})}
	runner := NewRunner(s, translator)
	if !runner.Start(bookID) {
		t.Fatal("Start")
	}
	select {
	case <-translator.started:
	case <-time.After(2 * time.Second):
		t.Fatal("tradução não iniciou")
	}
	if err := runner.Drain(2 * time.Second); err != nil {
		t.Fatal(err)
	}

	book, _ := s.FindBook(bookID)
	if book.Status != "translating" {
		t.Fatalf("deploy não pode pausar o livro: %+v", book)
	}
	chapters, _ = s.ListChapters(bookID)
	if chapters[0].Status != "done" {
		t.Fatal("capítulo pronto não pode ser tocado")
	}
	if chapters[1].Status != "pending" || chapters[1].Error != "" || chapters[1].Attempts != 0 {
		t.Fatalf("capítulo no ar deve voltar pra fila, sem falha nem pausa: %+v", chapters[1])
	}
	if chapters[2].Status != "pending" {
		t.Fatalf("restante deve continuar na fila: %+v", chapters[2])
	}

	close(translator.release)
	if err := runner.ResumeInterrupted(); err != nil {
		t.Fatal(err)
	}
	runner.Wait(bookID)
	book, _ = s.FindBook(bookID)
	if book.Status != "done" || book.DoneChapters != 3 {
		t.Fatalf("boot deveria retomar: %+v", book)
	}
	if translator.calls.Load() != 3 {
		t.Fatalf("chamadas=%d, want 3 (capítulo interrompido refeito + último)", translator.calls.Load())
	}
}
