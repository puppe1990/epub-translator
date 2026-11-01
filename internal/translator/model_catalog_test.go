package translator

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type catalogTransport func(*http.Request) (*http.Response, error)

func (f catalogTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestModelCatalogFiltersAndCachesTextModels(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: catalogTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://openrouter.ai/api/v1/models" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("catálogo precisa usar o endpoint fixo e a chave do perfil")
		}
		body := `{"data":[
		{"id":"z/text","architecture":{"input_modalities":["text"],"output_modalities":["text"]}},
		{"id":"a/text","architecture":{"output_modalities":["text"]}},
		{"id":"a/text","architecture":{"output_modalities":["text"]}},
		{"id":"image/model","architecture":{"output_modalities":["text","image"]}},
		{"id":"audio/model","architecture":{"output_modalities":["audio"]}},
		{"id":"meta/llama-guard-4","architecture":{"output_modalities":["text"]}},
		{"id":"a/text:batch","architecture":{"output_modalities":["text"]}},
		{"id":"disabled/model","active":false}, {"id":""}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	catalog := NewModelCatalog(client)
	for range 2 {
		models, err := catalog.List(context.Background(), "openrouter", "test-key")
		if err != nil || strings.Join(models, ",") != "a/text,z/text" {
			t.Fatalf("modelos = %v, err = %v", models, err)
		}
		models[0] = "não altere o cache"
	}
	if calls != 1 {
		t.Errorf("chamadas=%d; esperava cache", calls)
	}
}

func TestModelCatalogGroqFiltersAudioAndRefreshesAfterKeyChange(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: catalogTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "api.groq.com" {
			t.Error("chave do Groq enviada a outro host")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"openai/gpt-oss-120b"},{"id":"canopylabs/orpheus-v1-english"},{"id":"whisper-large-v3"},{"id":"meta/llama-guard"}]}`))}, nil
	})}
	catalog := NewModelCatalog(client)
	for _, key := range []string{"first-key", "second-key"} {
		models, err := catalog.List(context.Background(), "groq", key)
		if err != nil || strings.Join(models, ",") != "openai/gpt-oss-120b" {
			t.Fatalf("modelos=%v, err=%v", models, err)
		}
	}
	if calls != 2 {
		t.Errorf("trocar a chave precisa atualizar o catálogo, chamadas=%d", calls)
	}
}

func TestModelCatalogUsesStaleCacheOnFailureWithoutExposingKeys(t *testing.T) {
	status := http.StatusOK
	client := &http.Client{Transport: catalogTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"openai/gpt-oss-120b"}]}`))}, nil
	})}
	catalog := NewModelCatalog(client)
	_, _ = catalog.List(context.Background(), "groq", "secret-key")
	entry := catalog.entries["groq"]
	entry.expiresAt = time.Now().Add(-time.Minute)
	catalog.entries["groq"] = entry
	status = http.StatusUnauthorized
	models, err := catalog.List(context.Background(), "groq", "secret-key")
	if err == nil || len(models) != 1 || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("catálogo anterior deve sobreviver sem expor chave: modelos=%v, err=%v", models, err)
	}
	models, err = catalog.List(context.Background(), "groq", "different-key")
	if err == nil || len(models) != 0 {
		t.Error("cache da chave anterior não pode ser usado para outra chave")
	}
}

func TestModelCatalogRejectsUnexpectedProviderAndEmptyResponse(t *testing.T) {
	client := &http.Client{Transport: catalogTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}, nil
	})}
	catalog := NewModelCatalog(client)
	for _, slug := range []string{"custom", "groq"} {
		if _, err := catalog.List(context.Background(), slug, "key"); err == nil {
			t.Errorf("catálogo %s deveria retornar erro", slug)
		}
	}
}
