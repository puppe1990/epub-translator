package pipeline

import (
	"context"

	"github.com/puppe1990/epub-translator/internal/models"
)

type preparedTranslator interface {
	Prepare() (Translator, models.TranslationDetails, error)
}

func (r *Runner) prepareTranslator() (Translator, models.TranslationDetails, error) {
	if configured, ok := r.translator.(preparedTranslator); ok {
		return configured.Prepare()
	}
	return r.translator, models.TranslationDetails{}, nil
}

func (r *Runner) translateChapter(ctx context.Context, language string, chapter models.Chapter) {
	prepared, details, err := r.prepareTranslator()
	started := r.now()
	if startErr := r.store.StartChapterExecution(chapter.ID, details, started); startErr != nil {
		_ = r.store.SetChapterFailed(chapter.ID, startErr.Error())
		return
	}
	out := ""
	if err == nil {
		out, err = prepared.TranslateBody(ctx, language, chapter.SourceText)
	}
	finished := r.now()
	if finishErr := r.store.FinishChapterExecution(chapter.ID, finished, finished.Sub(started).Milliseconds()); finishErr != nil {
		err = finishErr
	}
	r.completeChapter(ctx, chapter.ID, out, err)
}

func (r *Runner) completeChapter(ctx context.Context, id int64, out string, err error) {
	if err != nil {
		if ctx.Err() == nil {
			_ = r.store.SetChapterFailed(id, err.Error())
		}
		return
	}
	_ = r.store.SetChapterDone(id, out)
}
