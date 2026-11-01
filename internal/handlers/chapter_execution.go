package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/translator"
)

func chapterProviderLabel(slug string) string {
	if preset, ok := translator.PresetBySlug(slug); ok {
		return preset.Name
	}
	if slug == "" {
		return "—"
	}
	return slug
}

func chapterDurationLabel(chapter models.Chapter, now time.Time) string {
	if chapter.Status == "running" && chapter.FinishedAt == "" {
		started, err := time.Parse(time.RFC3339Nano, chapter.StartedAt)
		if err != nil {
			return "—"
		}
		return formatChapterDuration(max(0, now.Sub(started).Milliseconds()))
	}
	if chapter.FinishedAt == "" {
		return "—"
	}
	return formatChapterDuration(chapter.DurationMS)
}

func formatChapterDuration(milliseconds int64) string {
	if milliseconds < 1000 {
		return fmt.Sprintf("%d ms", milliseconds)
	}
	seconds := milliseconds / 1000
	if seconds >= 3600 {
		return fmt.Sprintf("%d h %02d min", seconds/3600, seconds%3600/60)
	}
	if seconds >= 60 {
		return fmt.Sprintf("%d min %02d s", seconds/60, seconds%60)
	}
	if milliseconds%1000 == 0 {
		return fmt.Sprintf("%d s", seconds)
	}
	return strings.ReplaceAll(fmt.Sprintf("%.1f s", float64(milliseconds)/1000), ".", ",")
}
