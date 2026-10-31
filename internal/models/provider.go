package models

// Provider é um perfil de provedor de tradução (OpenAI-compatível) com a
// própria chave salva. Slugs conhecidos: openai, gemini, groq, nvidia,
// openrouter, custom.
type Provider struct {
	Slug    string
	BaseURL string
	Model   string
	APIKey  string
}
