package translator

// Preset descreve um provedor conhecido: base URL e modelo padrão. A chave é
// sempre do usuário (tabela providers) — nada de chave no código.
type Preset struct {
	Slug         string
	Name         string
	BaseURL      string
	DefaultModel string
}

// Presets são os provedores oferecidos em /settings (e o "custom" livre).
var Presets = []Preset{
	{Slug: "openai", Name: "OpenAI", BaseURL: "https://api.openai.com/v1", DefaultModel: "gpt-4.1-mini"},
	{Slug: "gemini", Name: "Gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", DefaultModel: "gemini-3.5-flash-lite"},
	{Slug: "groq", Name: "Groq", BaseURL: "https://api.groq.com/openai/v1", DefaultModel: "openai/gpt-oss-120b"},
	{Slug: "nvidia", Name: "NVIDIA NIM", BaseURL: "https://integrate.api.nvidia.com/v1", DefaultModel: "minimaxai/minimax-m3.1-flash"},
	{Slug: "openrouter", Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", DefaultModel: "google/gemini-2.5-flash"},
	{Slug: "custom", Name: "Personalizado", BaseURL: "", DefaultModel: ""},
}

func PresetBySlug(slug string) (Preset, bool) {
	for _, preset := range Presets {
		if preset.Slug == slug {
			return preset, true
		}
	}
	return Preset{}, false
}
