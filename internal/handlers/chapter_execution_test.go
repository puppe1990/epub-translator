package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/puppe1990/epub-translator/internal/models"
)

func TestChapterListHasExecutionColumnsWithoutInventingLegacyValues(t *testing.T) {
	_, list, err := RenderBookFragments(models.Book{ID: 1}, []models.Chapter{{ID: 1, Position: 1, Title: "Capítulo antigo", Status: "done"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Tempo", "Provedor / modelo", "Não registrado"} {
		if !strings.Contains(list, want) {
			t.Errorf("listagem sem %q", want)
		}
	}
}

func TestChapterExecutionLabelsUseRecordedProviderAndElapsedTime(t *testing.T) {
	started := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	chapters := []models.Chapter{
		{Status: "running", Provider: "groq", Model: "openai/gpt-oss-120b", StartedAt: started.Format(time.RFC3339Nano)},
		{Status: "done", Provider: "openrouter", Model: "google/gemini-2.5-flash", StartedAt: started.Format(time.RFC3339Nano), FinishedAt: started.Add(1500 * time.Millisecond).Format(time.RFC3339Nano), DurationMS: 1500},
	}
	rows := chapterRowsAt(chapters, started.Add(65*time.Second))
	if rows[0].DurationLabel != "1 min 05 s" || rows[0].ProviderLabel != "Groq" || rows[0].Model != chapters[0].Model {
		t.Errorf("execução em andamento incorreta: %+v", rows[0])
	}
	if rows[1].DurationLabel != "1,5 s" || rows[1].ProviderLabel != "OpenRouter" {
		t.Errorf("execução concluída não pode continuar somando tempo: %+v", rows[1])
	}
}

func TestChapterDurationFormattingAndInterruptedUnknownTime(t *testing.T) {
	for duration, want := range map[int64]string{0: "0 ms", 380: "380 ms", 5000: "5 s", 1500: "1,5 s", 65000: "1 min 05 s", 3660000: "1 h 01 min"} {
		if got := formatChapterDuration(duration); got != want {
			t.Errorf("duração %d: %q, esperado %q", duration, got, want)
		}
	}
	rows := chapterRowsAt([]models.Chapter{{Status: "pending", StartedAt: "2026-11-01T12:00:00Z"}}, time.Now())
	if rows[0].DurationLabel != "—" {
		t.Error("execução interrompida por reinício não tem duração final conhecida")
	}
}
