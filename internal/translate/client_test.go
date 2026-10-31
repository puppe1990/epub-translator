package translate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_Translate_sendsOpenAICompatibleRequest(t *testing.T) {
	var gotAuth, gotPath string
	var payload chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("payload inválido: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"<p>Olá mundo.</p>"}}]}`))
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, APIKey: "chave-teste", Model: "modelo-teste", HTTP: server.Client()}
	out, err := client.Translate(context.Background(), "pt-BR", "Traduza para português do Brasil:\n\n<p>Hello world.</p>")
	if err != nil {
		t.Fatal(err)
	}
	if out != "<p>Olá mundo.</p>" {
		t.Errorf("out = %q", out)
	}
	if gotAuth != "Bearer chave-teste" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q", gotPath)
	}
	if payload.Model != "modelo-teste" || len(payload.Messages) != 2 {
		t.Errorf("payload = %+v", payload)
	}
	if payload.Messages[0].Role != "system" || !strings.Contains(payload.Messages[0].Content, "português do Brasil") {
		t.Errorf("system prompt = %q", payload.Messages[0].Content)
	}
	if !strings.Contains(payload.Messages[1].Content, "<p>Hello world.</p>") {
		t.Errorf("user content = %q", payload.Messages[1].Content)
	}
}

func TestClient_Translate_reportsHTTPErrorWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"quota exceeded"}}`))
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, APIKey: "k", Model: "m", HTTP: server.Client()}
	_, err := client.Translate(context.Background(), "pt-BR", "x")
	if err == nil {
		t.Fatal("esperava erro")
	}
	if !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "quota exceeded") {
		t.Errorf("erro sem contexto suficiente: %v", err)
	}
}
