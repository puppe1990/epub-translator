package handlers

import (
	"context"
	"sync"

	"github.com/puppe1990/epub-translator/internal/models"
)

type providerModelLister interface {
	List(context.Context, string, string) ([]string, error)
}

func (h *SettingsHandler) loadProviderModels(ctx context.Context, rows []providerRow, saved map[string]models.Provider) {
	if h.modelCatalog == nil {
		return
	}
	var pending sync.WaitGroup
	for index := range rows {
		row := &rows[index]
		if !row.HasKey || (row.Slug != "groq" && row.Slug != "openrouter") {
			continue
		}
		pending.Go(func() {
			options, err := h.modelCatalog.List(ctx, row.Slug, saved[row.Slug].APIKey)
			if len(options) > 0 {
				row.ModelOptions = includeSelectedModel(options, row.Model)
			}
			if err != nil {
				row.ModelCatalogWarning = "Não foi possível atualizar o catálogo. A lista anterior continua disponível; recarregue para tentar novamente."
			}
		})
	}
	pending.Wait()
}

// The hosted catalog also lists retired models that return 404. Keep the
// selection focused on current text models, without claiming availability.
func providerModelOptions(slug, selected string) []string {
	var options []string
	switch slug {
	case "groq":
		options = []string{"openai/gpt-oss-120b", "openai/gpt-oss-20b", "qwen/qwen3.8-27b", "allam-2-7b"}
	case "openrouter":
		options = []string{"google/gemini-2.5-flash", "google/gemini-2.5-flash-lite", "anthropic/claude-haiku-4.5", "openai/gpt-oss-120b", "openai/gpt-5.4-mini", "deepseek/deepseek-v3.2", "google/gemma-4-31b-it:free"}
	case "nvidia":
		options = []string{
			"nvidia/riva-translate-4b-instruct-v2",
			"mistralai/mistral-large-4-1t-a52b",
			"deepseek-ai/deepseek-v4.1-flash",
			"google/gemma-4-31b-it",
			"openai/gpt-oss-20b",
			"nvidia/nemotron-3.5-lightning-30b-a3b",
			"z-ai/glm-5.3-flash",
		}
	default:
		return nil
	}
	return includeSelectedModel(options, selected)
}

func includeSelectedModel(options []string, selected string) []string {
	for _, model := range options {
		if model == selected {
			return options
		}
	}
	// Never silently replace a previously saved model outside this list.
	if selected != "" {
		options = append([]string{selected}, options...)
	}
	return options
}
