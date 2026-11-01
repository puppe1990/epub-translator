package handlers

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/puppe1990/epub-translator/internal/models"
)

type chapterListFilter struct {
	Status   string
	Provider string
	Model    string
}

func parseChapterListFilter(query url.Values) chapterListFilter {
	if len(query) == 0 {
		return chapterListFilter{}
	}
	return chapterListFilter{
		Status:   chapterStatusFilter(query.Get("status")),
		Provider: chapterProviderFilter(query.Get("provider")),
		Model:    chapterModelFilter(query.Get("model")),
	}
}

func chapterStatusFilter(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pending", "running", "paused", "done", "failed":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

var providerSlugRE = regexp.MustCompile(`^[a-z0-9_-]+$`)

func chapterProviderFilter(raw string) string {
	slug := strings.ToLower(strings.TrimSpace(raw))
	if slug == "none" {
		return "none"
	}
	if !providerSlugRE.MatchString(slug) {
		return ""
	}
	return slug
}

func chapterModelFilter(raw string) string {
	return strings.TrimSpace(raw)
}

func (f chapterListFilter) query() string {
	values := url.Values{}
	if f.Status != "" {
		values.Set("status", f.Status)
	}
	if f.Provider != "" {
		values.Set("provider", f.Provider)
	}
	if f.Model != "" {
		values.Set("model", f.Model)
	}
	encoded := values.Encode()
	if encoded == "" {
		return ""
	}
	return "?" + encoded
}

func (f chapterListFilter) path(bookID int64) string {
	return fmt.Sprintf("/books/%d%s", bookID, f.query())
}

func (f chapterListFilter) withStatus(status string) chapterListFilter {
	f.Status = status
	return f
}

func (f chapterListFilter) withProvider(provider string) chapterListFilter {
	f.Provider = provider
	f.Model = ""
	return f
}

func (f chapterListFilter) withModel(model string) chapterListFilter {
	f.Model = model
	return f
}

func filterChapters(chapters []models.Chapter, filter chapterListFilter) []models.Chapter {
	return filterChaptersByModel(
		filterChaptersByProvider(
			filterChaptersByStatus(chapters, filter.Status),
			filter.Provider,
		),
		filter.Model,
	)
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

func filterChaptersByProvider(chapters []models.Chapter, provider string) []models.Chapter {
	if provider == "" {
		return chapters
	}
	filtered := make([]models.Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		if provider == "none" {
			if chapter.Provider == "" {
				filtered = append(filtered, chapter)
			}
			continue
		}
		if chapter.Provider == provider {
			filtered = append(filtered, chapter)
		}
	}
	return filtered
}

func filterChaptersByModel(chapters []models.Chapter, model string) []models.Chapter {
	if model == "" {
		return chapters
	}
	filtered := make([]models.Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		if chapter.Model == model {
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

func chapterFilterChips(bookID int64, chapters []models.Chapter, active chapterListFilter) []chapterFilterChip {
	counts := map[string]int{}
	for _, chapter := range chapters {
		counts[chapter.Status]++
	}
	return []chapterFilterChip{
		{Label: "Todos", Count: len(chapters), Href: active.withStatus("").path(bookID), Active: active.Status == ""},
		statusFilterChip(bookID, "pending", "Na fila", counts["pending"], active),
		statusFilterChip(bookID, "running", "Traduzindo", counts["running"], active),
		statusFilterChip(bookID, "paused", "Pausado", counts["paused"], active),
		statusFilterChip(bookID, "done", "Traduzido", counts["done"], active),
		statusFilterChip(bookID, "failed", "Falhou", counts["failed"], active),
	}
}

func statusFilterChip(bookID int64, status, label string, count int, active chapterListFilter) chapterFilterChip {
	return chapterFilterChip{
		Label:  label,
		Count:  count,
		Href:   active.withStatus(status).path(bookID),
		Active: active.Status == status,
	}
}

func chapterProviderFilterChips(bookID int64, chapters []models.Chapter, active chapterListFilter) []chapterFilterChip {
	counts := map[string]int{}
	unrecorded := 0
	for _, chapter := range chapters {
		if chapter.Provider == "" {
			unrecorded++
			continue
		}
		counts[chapter.Provider]++
	}
	chips := []chapterFilterChip{{
		Label: "Todos", Count: len(chapters),
		Href: active.withProvider("").path(bookID), Active: active.Provider == "",
	}}
	slugs := make([]string, 0, len(counts))
	for slug := range counts {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	for _, slug := range slugs {
		chips = append(chips, chapterFilterChip{
			Label:  chapterProviderLabel(slug),
			Count:  counts[slug],
			Href:   active.withProvider(slug).path(bookID),
			Active: active.Provider == slug,
		})
	}
	if unrecorded > 0 {
		chips = append(chips, chapterFilterChip{
			Label: "Sem registro", Count: unrecorded,
			Href: active.withProvider("none").path(bookID), Active: active.Provider == "none",
		})
	}
	return chips
}

func chapterModelFilterChips(bookID int64, chapters []models.Chapter, active chapterListFilter) []chapterFilterChip {
	scoped := filterChaptersByProvider(chapters, active.Provider)
	counts := map[string]int{}
	for _, chapter := range scoped {
		if chapter.Model == "" {
			continue
		}
		counts[chapter.Model]++
	}
	if len(counts) == 0 {
		return nil
	}
	chips := []chapterFilterChip{{
		Label: "Todos", Count: len(scoped),
		Href: active.withModel("").path(bookID), Active: active.Model == "",
	}}
	ids := make([]string, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		chips = append(chips, chapterFilterChip{
			Label:  id,
			Count:  counts[id],
			Href:   active.withModel(id).path(bookID),
			Active: active.Model == id,
		})
	}
	return chips
}
