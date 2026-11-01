package handlers

import (
	"fmt"
	"strings"

	"github.com/puppe1990/epub-translator/internal/models"
)

func chapterStatusFilter(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pending", "running", "paused", "done", "failed":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func filterChaptersByStatus(chapters []models.Chapter, status string) []models.Chapter {
	if status == "" {
		return chapters
	}
	filtered := make([]models.Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		if chapter.Status == status {
			filtered = append(filtered, chapter)
		}
	}
	return filtered
}

type chapterFilterChip struct {
	Label  string
	Count  int
	Href   string
	Active bool
}

func chapterFilterChips(bookID int64, chapters []models.Chapter, active string) []chapterFilterChip {
	counts := map[string]int{}
	for _, chapter := range chapters {
		counts[chapter.Status]++
	}
	path := fmt.Sprintf("/books/%d", bookID)
	return []chapterFilterChip{
		{Label: "Todos", Count: len(chapters), Href: path, Active: active == ""},
		statusFilterChip(path, "pending", "Na fila", counts["pending"], active),
		statusFilterChip(path, "running", "Traduzindo", counts["running"], active),
		statusFilterChip(path, "paused", "Pausado", counts["paused"], active),
		statusFilterChip(path, "done", "Traduzido", counts["done"], active),
		statusFilterChip(path, "failed", "Falhou", counts["failed"], active),
	}
}

func statusFilterChip(path, status, label string, count int, active string) chapterFilterChip {
	return chapterFilterChip{
		Label:  label,
		Count:  count,
		Href:   path + "?status=" + status,
		Active: active == status,
	}
}
