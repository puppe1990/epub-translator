package translator

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

type cachedModels struct {
	keyHash   [32]byte
	expiresAt time.Time
	models    []string
}

// ModelCatalog keeps provider credentials server-side and bounds settings latency.
type ModelCatalog struct {
	client  *http.Client
	mu      sync.Mutex
	entries map[string]cachedModels
}

func NewModelCatalog(client *http.Client) *ModelCatalog {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	return &ModelCatalog{client: client, entries: make(map[string]cachedModels)}
}

func (c *ModelCatalog) List(ctx context.Context, slug, apiKey string) ([]string, error) {
	if (slug != "groq" && slug != "openrouter") || apiKey == "" {
		return nil, fmt.Errorf("catálogo de %q indisponível: esperado Groq ou OpenRouter com chave salva", slug)
	}
	keyHash := sha256.Sum256([]byte(apiKey))
	c.mu.Lock()
	entry := c.entries[slug]
	c.mu.Unlock()
	if entry.keyHash == keyHash && time.Now().Before(entry.expiresAt) {
		return slices.Clone(entry.models), nil
	}
	models, err := c.fetch(ctx, slug, apiKey)
	if err != nil {
		if entry.keyHash == keyHash {
			return slices.Clone(entry.models), err
		}
		return nil, err
	}
	c.mu.Lock()
	c.entries[slug] = cachedModels{keyHash: keyHash, expiresAt: time.Now().Add(15 * time.Minute), models: slices.Clone(models)}
	c.mu.Unlock()
	return models, nil
}

func (c *ModelCatalog) fetch(ctx context.Context, slug, apiKey string) ([]string, error) {
	preset, _ := PresetBySlug(slug)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, preset.BaseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("montar catálogo de %s: %w", preset.Name, err)
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("catálogo de %s não respondeu", preset.Name)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catálogo de %s respondeu HTTP %d", preset.Name, response.StatusCode)
	}
	return decodeModelCatalog(io.LimitReader(response.Body, 8<<20))
}

type catalogModel struct {
	ID           string `json:"id"`
	Active       *bool  `json:"active"`
	Architecture struct {
		InputModalities  []string `json:"input_modalities"`
		OutputModalities []string `json:"output_modalities"`
	} `json:"architecture"`
}

func decodeModelCatalog(body io.Reader) ([]string, error) {
	var payload struct {
		Models []catalogModel `json:"data"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("catálogo de modelos ilegível")
	}
	var models []string
	for _, model := range payload.Models {
		if model.supportsTextTranslation() {
			models = append(models, model.ID)
		}
	}
	slices.Sort(models)
	models = slices.Compact(models)
	if len(models) == 0 {
		return nil, fmt.Errorf("catálogo sem modelos de tradução de texto")
	}
	return models, nil
}

func (m catalogModel) supportsTextTranslation() bool {
	if strings.TrimSpace(m.ID) == "" || (m.Active != nil && !*m.Active) {
		return false
	}
	for _, excluded := range []string{"whisper", "orpheus", "tts", "guard", "embed", "moderation", ":batch"} {
		if strings.Contains(strings.ToLower(m.ID), excluded) {
			return false
		}
	}
	inputs, outputs := m.Architecture.InputModalities, m.Architecture.OutputModalities
	if len(inputs) > 0 && !slices.Contains(inputs, "text") {
		return false
	}
	return len(outputs) == 0 || (len(outputs) == 1 && outputs[0] == "text")
}
