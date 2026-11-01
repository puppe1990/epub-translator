package models

// Book é um EPUB enviado para tradução. Status: pending → translating →
// paused (retomada manual), done (tudo traduzido) ou failed (pede retry).
type Book struct {
	ID             int64
	Title          string
	Author         string
	SourceLanguage string
	TargetLanguage string
	Filename       string
	SourcePath     string // arquivo original em data/books
	Status         string
	TotalChapters  int
	DoneChapters   int
	FailedChapters int
	CreatedAt      string
}

// Chapter é um documento XHTML do EPUB (unidade de tradução e de retry).
type Chapter struct {
	ID             int64
	BookID         int64
	Position       int
	DocPath        string // caminho dentro do zip
	Title          string
	SourceText     string // miolo de <body> original
	TranslatedText string // miolo de <body> traduzido
	Status         string // pending | running | done | failed
	Attempts       int
	Error          string
	Provider       string
	Model          string
	StartedAt      string
	FinishedAt     string
	DurationMS     int64
}

// TranslationDetails describes the provider snapshot actually used, never its key.
type TranslationDetails struct {
	Provider string
	Model    string
}
