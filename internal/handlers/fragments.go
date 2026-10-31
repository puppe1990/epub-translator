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
		return "border-copper/60 text-copper"
	case "running", "translating":
		return "border-amber-400/60 text-amber-400"
	case "failed":
		return "border-red-500/60 text-red-400"
	default:
		return "border-foam/25 text-foam/50"
	}
}

// fragmentsTemplate é a fonte única do HTML que o SSE morfa (#book-progress e
// #chapter-list); a página do livro renderiza os mesmos blocos via
// RenderBookFragments para o primeiro paint.
var fragmentsTemplate = template.Must(template.New("fragments").Parse(`
{{ define "book-progress" }}
<div class="flex flex-wrap items-center gap-4">
  <span class="border px-2 py-0.5 font-mono text-[10px] uppercase tracking-[0.16em] {{ .ChipClass }}">{{ .StatusLabel }}</span>
  <div class="h-1 max-w-md flex-1 bg-foam/10"><div class="h-1 bg-copper transition-all" style="width: {{ .Percent }}%"></div></div>
  <span class="font-mono text-xs text-foam/60">{{ .DoneChapters }}/{{ .TotalChapters }} capítulos</span>
  {{ if .FailedChapters }}<span class="font-mono text-xs text-red-400">{{ .FailedChapters }} falha(s)</span>{{ end }}
</div>
{{ end }}

{{ define "chapter-list" }}
<table class="w-full text-sm">
  <thead>
    <tr class="border-b border-foam/10 text-left">
      <th scope="col" class="px-3 py-2 font-mono text-[10px] uppercase tracking-[0.22em] text-foam/50">#</th>
      <th scope="col" class="px-3 py-2 font-mono text-[10px] uppercase tracking-[0.22em] text-foam/50">Capítulo</th>
      <th scope="col" class="px-3 py-2 font-mono text-[10px] uppercase tracking-[0.22em] text-foam/50">Status</th>
      <th scope="col" class="px-3 py-2 font-mono text-[10px] uppercase tracking-[0.22em] text-foam/50">Tentativas</th>
      <th scope="col" class="px-3 py-2 font-mono text-[10px] uppercase tracking-[0.22em] text-foam/50">Erro</th>
      <th scope="col" class="px-3 py-2"></th>
    </tr>
  </thead>
  <tbody class="divide-y divide-foam/5">
    {{ range .Chapters }}
    <tr class="hover:bg-tide/40 transition-colors">
      <td class="px-3 py-2 font-mono text-xs text-foam/40">{{ .Position }}</td>
      <td class="px-3 py-2 text-foam">{{ .Title }}</td>
      <td class="px-3 py-2"><span class="border px-2 py-0.5 font-mono text-[10px] uppercase tracking-[0.14em] {{ .ChipClass }}">{{ .StatusLabel }}</span></td>
      <td class="px-3 py-2 font-mono text-xs text-foam/50">{{ .Attempts }}</td>
      <td class="px-3 py-2 max-w-md truncate text-xs text-red-400" title="{{ .Error }}">{{ .Error }}</td>
      <td class="px-3 py-2 text-right">
        {{ if eq .Status "failed" }}
        <form action="/books/{{ $.BookID }}/chapters/{{ .ID }}/retry" method="post" class="inline">
          <button type="submit" class="font-mono text-[10px] uppercase tracking-[0.18em] text-copper hover:text-foam transition-colors" title="Marca para retry e roda a fila">Retentar</button>
        </form>
        {{ end }}
      </td>
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
