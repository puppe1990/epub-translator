package translate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDefaultHTTPClient_waitsForNIMBufferedCompletion(t *testing.T) {
	// DeepSeek-v4.1-flash no NIM só envia headers no fim da geração.
	// Chapter Eight (~8k tokens) estourou 240s × 3 tentativas = 12 min.
	got := defaultHTTPClient().Timeout
	if got < 8*time.Minute {
		t.Fatalf("timeout = %s, NIM precisa de folga para o primeiro header", got)
	}
}

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

func TestClient_Translate_rivaRestoresEmptyAnchorsDroppedByModel(t *testing.T) {
	source := `<h2 id="x"><a id="page_3"/><a id="ch01"/>CHAPTER 1</h2><p>Hello world, good morning everyone.</p>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload chatRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("payload inválido: %v", err)
			return
		}
		user := payload.Messages[1].Content
		if strings.Contains(user, "<a ") || strings.Contains(user, "<h2") {
			t.Errorf("Riva recebeu tag crua: %q", user)
		}
		out := user
		_, tags := maskHTMLTags(source)
		for i, tag := range tags {
			if selfClosingTag(tag) {
				out = strings.Replace(out, tagPlaceholder(i), "", 1)
			}
		}
		out = strings.ReplaceAll(out, "CHAPTER 1", "CAPÍTULO 1")
		out = strings.ReplaceAll(out, "Hello world, good morning everyone.", "Olá mundo, bom dia a todas as pessoas.")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + jsonString(out) + `}}]}`))
	}))
	defer server.Close()

	client := Client{
		BaseURL: server.URL, APIKey: "k", Model: "nvidia/riva-translate-4b-instruct-v2",
		SourceLanguage: "en", HTTP: server.Client(),
	}
	got, err := client.Translate(context.Background(), "pt-BR", "Traduza para português do Brasil:\n\n"+source)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateBody(source, got); err != nil {
		t.Fatalf("tradução com âncoras restauradas deveria validar: %v\ngot=%s", err, got)
	}
	if !strings.Contains(got, `id="page_3"`) || !strings.Contains(got, `id="ch01"`) {
		t.Errorf("perdeu âncora de página: %s", got)
	}
}

func jsonString(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func TestClient_Translate_geminiRestoresEmptyAnchorsDroppedByModel(t *testing.T) {
	source := `<h2 id="x"><a id="page_3"/><a id="ch01"/>CHAPTER 1</h2><p>Hello world, good morning everyone.</p>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload chatRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("payload inválido: %v", err)
			return
		}
		if payload.Messages[0].Content == "en-pt-br" {
			t.Errorf("Gemini deve manter o prompt literário, não o par Riva")
		}
		user := payload.Messages[1].Content
		if strings.Contains(user, "<a ") || strings.Contains(user, "<h2") {
			t.Errorf("Gemini recebeu tag crua: %q", user)
		}
		out := user
		if i := strings.Index(out, "\n\n"); i >= 0 && strings.HasPrefix(out, "Traduza para ") {
			out = out[i+2:]
		}
		_, tags := maskHTMLTags(source)
		for i, tag := range tags {
			if selfClosingTag(tag) {
				out = strings.Replace(out, tagPlaceholder(i), "", 1)
			}
		}
		out = strings.ReplaceAll(out, "CHAPTER 1", "CAPÍTULO 1")
		out = strings.ReplaceAll(out, "Hello world, good morning everyone.", "Olá mundo, bom dia a todas as pessoas.")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + jsonString(out) + `}}]}`))
	}))
	defer server.Close()

	client := Client{
		BaseURL: server.URL, APIKey: "k", Model: "gemini-3.5-flash-lite",
		SourceLanguage: "en", HTTP: server.Client(),
	}
	got, err := client.Translate(context.Background(), "pt-BR", "Traduza para português do Brasil:\n\n"+source)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateBody(source, got); err != nil {
		t.Fatalf("tradução com âncoras restauradas deveria validar: %v\ngot=%s", err, got)
	}
	if !strings.Contains(got, `id="page_3"`) || !strings.Contains(got, `id="ch01"`) {
		t.Errorf("perdeu âncora de página: %s", got)
	}
}

func TestClient_Translate_geminiPromptKeepsTagPlaceholders(t *testing.T) {
	var payload chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("payload inválido: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"⟦t0⟧Olá.⟦t1⟧"}}]}`))
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, APIKey: "k", Model: "gemini-3.5-flash-lite", HTTP: server.Client()}
	if _, err := client.Translate(context.Background(), "pt-BR", "<p>Hello.</p>"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload.Messages[0].Content, "⟦t") {
		t.Errorf("system Gemini = %q; precisa pedir para copiar os marcadores de tag", payload.Messages[0].Content)
	}
}

func TestClient_Translate_rivaUsesLanguagePairAndBareXHTML(t *testing.T) {
	var payload chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("payload inválido: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"<p>Olá mundo.</p>"}}]}`))
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, APIKey: "k", Model: "nvidia/riva-translate-4b-instruct-v2", HTTP: server.Client()}
	wrapped := "Traduza para português do Brasil:\n\n<p>Hello world.</p>\n\nATENÇÃO: a resposta anterior veio torta"
	if _, err := client.Translate(context.Background(), "pt-BR", wrapped); err != nil {
		t.Fatal(err)
	}
	if payload.Messages[0].Content != "en-pt-br" {
		t.Errorf("system Riva = %q, want en-pt-br", payload.Messages[0].Content)
	}
	user := payload.Messages[1].Content
	if strings.Contains(user, "Traduza para") || strings.Contains(user, "ATENÇÃO") {
		t.Errorf("user Riva = %q; o par de idiomas não aceita o prompt literário", user)
	}
	if strings.Contains(user, "<p>") {
		t.Errorf("user Riva = %q; tags cruas são mascaradas para o modelo não dropar âncoras", user)
	}
	if !strings.Contains(user, "Hello world.") {
		t.Errorf("user Riva perdeu o texto: %q", user)
	}
	if payload.MaxTokens < 1 {
		t.Error("Riva precisa de max_tokens > 0; senão o NIM reserva 0 tokens de saída quando o prompt enche a janela")
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
