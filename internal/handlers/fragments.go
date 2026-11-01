package handlers

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/puppe1990/epub-translator/internal/models"
)

// bookView é o livro já com rótulos e percentual para os templates.
type bookView struct {
	models.Book
	Percent     int
	StatusLabel string
	ChipClass   string
	HasPending  bool
}

func newBookView(book models.Book) bookView {
	percent := 0
	if book.TotalChapters > 0 {
		percent = book.DoneChapters * 100 / book.TotalChapters
	}
	return bookView{
		Book:        book,
		Percent:     percent,
		StatusLabel: bookStatusLabel(book.Status),
		ChipClass:   statusChipClass(book.Status),
		HasPending:  book.DoneChapters < book.TotalChapters,
	}
}

func bookViews(books []models.Book) []bookView {
	views := make([]bookView, 0, len(books))
	for _, book := range books {
		views = append(views, newBookView(book))
	}
	return views
}

func bookStatusLabel(status string) string {
	switch status {
	case "translating":
		return "Traduzindo"
	case "done":
		return "Concluído"
	case "failed":
		return "Com falhas"
	default:
		return "Na fila"
	}
}

func chapterStatusLabel(status string) string {
	switch status {
	case "running":
		return "Traduzindo"
	case "done":
		return "Traduzido"
	case "failed":
		return "Falhou"
	default:
		return "Na fila"
	}
}

func statusChipClass(status string) string {
	switch status {
	case "done":
		return "status-done"
	case "running", "translating":
		return "status-running"
	case "failed":
		return "status-failed"
	default:
		return "status-pending"
	}
}

// fragmentsTemplate é a fonte única do HTML que o SSE morfa (#book-progress e
// #chapter-list); a página do livro renderiza os mesmos blocos via
// RenderBookFragments para o primeiro paint.
var fragmentsTemplate = template.Must(template.New("fragments").Parse(`
{{ define "book-progress" }}
<div class="progress-heading"><div><p class="eyebrow">PROGRESSO DA TRADUÇÃO</p><h2>{{ .DoneChapters }} <span>de {{ .TotalChapters }} capítulos</span></h2></div><span class="progress-percent">{{ .Percent }}<small>%</small></span></div>
<progress value="{{ .Percent }}" max="100" aria-label="Progresso da tradução">{{ .Percent }}%</progress>
<div class="progress-footer"><span class="status-chip {{ .ChipClass }}">{{ .StatusLabel }}</span>{{ if .FailedChapters }}<span class="failure-count">{{ .FailedChapters }} falha(s) · retente os capítulos abaixo</span>{{ else }}<span class="muted">{{ if eq .Status "done" }}Tudo pronto para sua próxima leitura.{{ else }}A tradução continua mesmo com esta aba fechada.{{ end }}</span>{{ end }}</div>
{{ end }}

{{ define "chapter-list" }}
<table class="chapter-table">
  <caption class="sr-only">Capítulos e estado da tradução</caption>
  <thead><tr><th scope="col" class="chapter-number">Nº</th><th scope="col">Capítulo</th><th scope="col">Status</th><th scope="col" class="chapter-attempts">Falhas</th><th scope="col"><span class="sr-only">Ações</span></th></tr></thead>
  <tbody>
    {{ range .Chapters }}
    <tr class="chapter-row chapter-{{ .Status }}">
      <td class="chapter-number">{{ printf "%02d" .Position }}</td>
      <td class="chapter-title"><span>{{ .Title }}</span>{{ if .Error }}<details class="chapter-error"><summary>Ver erro</summary><p>{{ .Error }}</p></details>{{ end }}</td>
      <td class="chapter-status"><span class="status-chip {{ .ChipClass }}">{{ .StatusLabel }}</span></td>
      <td class="chapter-attempts">{{ .Attempts }}</td>
      <td class="chapter-action">{{ if eq .Status "failed" }}<form action="/books/{{ $.BookID }}/chapters/{{ .ID }}/retry" method="post"><button type="submit" class="text-button" aria-label="Retentar capítulo {{ .Position }}">Retentar <span>↻</span></button></form>{{ else if eq .Status "done" }}<span class="chapter-check" aria-label="Traduzido">✓</span>{{ end }}</td>
    </tr>
    {{ end }}
  </tbody>
</table>
{{ end }}
`))

// chapterRow é a linha da tabela de capítulos na página do livro.
type chapterRow struct {
	ID          int64
	BookID      int64
	Position    int
	Title       string
	Status      string
	StatusLabel string
	ChipClass   string
	Attempts    int
	Error       string
}

func chapterRows(chapters []models.Chapter) []chapterRow {
	rows := make([]chapterRow, 0, len(chapters))
	for _, chapter := range chapters {
		rows = append(rows, chapterRow{
			ID: chapter.ID, BookID: chapter.BookID, Position: chapter.Position, Title: chapter.Title,
			Status: chapter.Status, StatusLabel: chapterStatusLabel(chapter.Status),
			ChipClass: statusChipClass(chapter.Status),
			Attempts:  chapter.Attempts, Error: chapter.Error,
		})
	}
	return rows
}

// RenderBookFragments devolve o progresso e a lista de capítulos no formato
// que o SSE morfa (innerHTML de #book-progress e #chapter-list).
func RenderBookFragments(book models.Book, chapters []models.Chapter) (progress, list string, err error) {
	progress, err = renderFragment("book-progress", newBookView(book))
	if err != nil {
		return "", "", err
	}
	list, err = renderFragment("chapter-list", map[string]any{
		"BookID":   book.ID,
		"Chapters": chapterRows(chapters),
	})
	if err != nil {
		return "", "", err
	}
	return progress, list, nil
}

func renderFragment(name string, data any) (string, error) {
	var builder strings.Builder
	if err := fragmentsTemplate.ExecuteTemplate(&builder, name, data); err != nil {
		return "", fmt.Errorf("renderizar fragmento %s: %w", name, err)
	}
	return builder.String(), nil
}
