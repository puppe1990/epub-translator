// Package pipeline executa a tradução de um livro capítulo a capítulo,
// gravando o estado no banco a cada passo — é o que torna o retry por
// capítulo possível depois de qualquer falha (inclusive restart do servidor).
package pipeline

import (
	"context"
	"sync"
	"time"

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
	now        func() time.Time

	mu      sync.Mutex
	running map[int64]*bookRun
}

type bookRun struct {
	done     chan struct{}
	cancel   context.CancelFunc
	pauseErr error
}

func NewRunner(s store.Store, translator Translator) *Runner {
	return &Runner{store: s, translator: translator, now: time.Now, running: map[int64]*bookRun{}}
}

// ResumeInterrupted devolve capítulos travados em 'running' à fila e retoma
// só os livros que estavam traduzindo. Pausados e "na fila" ficam como estão
// — o boot de um deploy não pode virar pausa nem arrancar o que o usuário
// não pediu.
func (r *Runner) ResumeInterrupted() error {
	if err := r.store.ResetRunningChapters(); err != nil {
		return err
	}
	books, err := r.store.ListBooks()
	if err != nil {
		return err
	}
	for _, book := range books {
		if book.Status == "translating" {
			r.Start(book.ID)
		}
	}
	return nil
}

// Start dispara a tradução dos capítulos pendentes/falhos. Devolve false se o
// livro já está rodando (o botão de retry pode ser clicado duas vezes).
func (r *Runner) Start(bookID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running[bookID] != nil {
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &bookRun{done: make(chan struct{}), cancel: cancel}
	r.running[bookID] = job
	go r.run(ctx, bookID, job)
	return true
}

// Wait bloqueia até o livro terminar (testes e shutdown gracioso).
func (r *Runner) Wait(bookID int64) {
	r.mu.Lock()
	job := r.running[bookID]
	r.mu.Unlock()
	if job != nil {
		<-job.done
	}
}

// Pause waits for cancellation to be persisted before a subsequent Start can
// resume the book. No new run may overlap the chapter being cancelled.
func (r *Runner) Pause(bookID int64) (bool, error) {
	r.mu.Lock()
	job := r.running[bookID]
	r.mu.Unlock()
	if job == nil {
		return false, nil
	}
	job.cancel()
	<-job.done
	return true, job.pauseErr
}

func (r *Runner) run(ctx context.Context, bookID int64, job *bookRun) {
	defer func() {
		job.cancel()
		r.mu.Lock()
		delete(r.running, bookID)
		close(job.done)
		r.mu.Unlock()
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
		if r.pauseIfCancelled(ctx, bookID, job) {
			return
		}
		r.translateChapter(ctx, book.TargetLanguage, chapter)
		if r.pauseIfCancelled(ctx, bookID, job) {
			return
		}
	}
	r.finish(bookID)
}

func (r *Runner) pauseIfCancelled(ctx context.Context, bookID int64, job *bookRun) bool {
	if ctx.Err() == nil {
		return false
	}
	job.pauseErr = r.store.PauseBook(bookID)
	return true
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
