package store

import (
	"testing"
	"time"

	"github.com/puppe1990/epub-translator/internal/models"
)

func TestChapterExecutionRoundtripAndRetryKeepsFailureHistory(t *testing.T) {
	s := newTestStore(t)
	bookID := seedBook(t, s)
	chapters, _ := s.ListChapters(bookID)
	id := chapters[0].ID
	started := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	details := models.TranslationDetails{Provider: "groq", Model: "openai/gpt-oss-120b"}
	if err := s.StartChapterExecution(id, details, started); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishChapterExecution(id, started.Add(1500*time.Millisecond), 1500); err != nil {
		t.Fatal(err)
	}
	_ = s.SetChapterFailed(id, "limite do provedor")
	chapter, _ := s.FindChapter(id)
	listed, _ := s.ListChapters(bookID)
	pending, _ := s.PendingChapters(bookID)
	for _, result := range []models.Chapter{chapter, listed[0], pending[0]} {
		if result.Provider != details.Provider || result.Model != details.Model || result.DurationMS != 1500 || result.StartedAt == "" || result.FinishedAt == "" {
			t.Errorf("metadados perdidos: %+v", result)
		}
	}
	details = models.TranslationDetails{Provider: "openrouter", Model: "google/gemini-2.5-flash"}
	if err := s.StartChapterExecution(id, details, started.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	chapter, _ = s.FindChapter(id)
	if chapter.Provider != "openrouter" || chapter.Model != details.Model || chapter.DurationMS != 0 || chapter.FinishedAt != "" || chapter.Attempts != 1 {
		t.Errorf("retry precisa renovar a execução e preservar falhas: %+v", chapter)
	}
}
