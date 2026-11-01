package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/store"
)

type executionTranslator struct {
	clock  *time.Time
	count  int
	s      store.Store
	bookID int64
	t      *testing.T
}

func (e *executionTranslator) TranslateBody(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("runner precisa preparar a configuração antes da chamada")
}

func (e *executionTranslator) Prepare() (Translator, models.TranslationDetails, error) {
	e.count++
	provider := "groq"
	if e.count > 1 {
		provider = "openrouter"
	}
	return &preparedExecution{e: e}, models.TranslationDetails{Provider: provider, Model: fmt.Sprintf("model-%d", e.count)}, nil
}

type preparedExecution struct{ e *executionTranslator }

func (p *preparedExecution) TranslateBody(_ context.Context, _, body string) (string, error) {
	chapters, _ := p.e.s.ListChapters(p.e.bookID)
	current := chapters[p.e.count-1]
	if current.Model != fmt.Sprintf("model-%d", p.e.count) || current.StartedAt == "" || current.Status != "running" {
		p.e.t.Error("modelo e início precisam estar registrados durante a chamada")
	}
	*p.e.clock = p.e.clock.Add(time.Duration(p.e.count) * time.Second)
	if strings.Contains(body, "two") {
		return "", fmt.Errorf("provedor respondeu 429")
	}
	return strings.ReplaceAll(body, "Hello", "Olá"), nil
}

func TestRunnerRecordsEachChapterExecutionIncludingFailures(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)
	clock := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	translator := &executionTranslator{clock: &clock, s: s, bookID: bookID, t: t}
	runner := NewRunner(s, translator)
	runner.now = func() time.Time { return clock }
	runner.Start(bookID)
	runner.Wait(bookID)
	chapters, _ := s.ListChapters(bookID)
	for index, chapter := range chapters {
		provider := "openrouter"
		if index == 0 {
			provider = "groq"
		}
		if chapter.Provider != provider || chapter.Model != fmt.Sprintf("model-%d", index+1) || chapter.DurationMS != int64(index+1)*1000 || chapter.FinishedAt == "" {
			t.Errorf("execução não registrada: %+v", chapter)
		}
	}
	if chapters[1].Status != "failed" {
		t.Error("a execução falha também precisa registrar duração e modelo")
	}
}
