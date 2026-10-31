// Package pipeline executa a tradução de um livro capítulo a capítulo,
// gravando o estado no banco a cada passo — é o que torna o retry por
// capítulo possível depois de qualquer falha (inclusive restart do servidor).
package pipeline

import (
	"context"
	"sync"

	"github.com/puppe1990/epub-translator/internal/store"
)

// Translator é a fronteira com o provedor (translate.Service em produção).
type Translator interface {
	TranslateBody(ctx context.Context, targetLanguage, body string) (string, error)
}

// Runner serializa a tradução por livro: um loop por livro por vez, e o
// capítulo que falha vira 'failed' sem derrubar os restantes.
type Runner struct {
	store      store.Store
	translator Translator

	mu      sync.Mutex
	running map[int64]chan struct{}
}

func NewRunner(s store.Store, translator Translator) *Runner {
	return &Runner{store: s, translator: translator, running: map[int64]chan struct{}{}}
}

// Start dispara a tradução dos capítulos pendentes/falhos. Devolve false se o
// livro já está rodando (o botão de retry pode ser clicado duas vezes).
func (r *Runner) Start(bookID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running[bookID] != nil {
		return false
	}
	done := make(chan struct{})
	r.running[bookID] = done
	go r.run(bookID, done)
	return true
}

// Wait bloqueia até o livro terminar (testes e shutdown gracioso).
func (r *Runner) Wait(bookID int64) {
	r.mu.Lock()
	done := r.running[bookID]
	r.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (r *Runner) run(bookID int64, done chan struct{}) {
	defer func() {
		r.mu.Lock()
		delete(r.running, bookID)
		r.mu.Unlock()
		close(done)
	}()

	book, err := r.store.FindBook(bookID)
	if err != nil {
		return
	}
	chapters, err := r.store.PendingChapters(bookID)
	if err != nil {
		return
	}
	if len(chapters) == 0 {
		r.finish(bookID)
		return
	}
	_ = r.store.SetBookStatus(bookID, "translating")

	for _, chapter := range chapters {
		_ = r.store.SetChapterRunning(chapter.ID)
		out, err := r.translator.TranslateBody(context.Background(), book.TargetLanguage, chapter.SourceText)
		if err != nil {
			_ = r.store.SetChapterFailed(chapter.ID, err.Error())
			continue
		}
		_ = r.store.SetChapterDone(chapter.ID, out)
	}
	r.finish(bookID)
}

func (r *Runner) finish(bookID int64) {
	book, err := r.store.FindBook(bookID)
	if err != nil {
		return
	}
	status := "pending"
	switch {
	case book.FailedChapters > 0:
		status = "failed"
	case book.DoneChapters >= book.TotalChapters && book.TotalChapters > 0:
		status = "done"
	}
	_ = r.store.SetBookStatus(bookID, status)
}
