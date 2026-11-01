package tokens

import (
	"sync"

	"github.com/tiktoken-go/tokenizer"
)

var (
	once      sync.Once
	cl100k    tokenizer.Codec
	cl100kErr error
)

func codec() tokenizer.Codec {
	once.Do(func() {
		cl100k, cl100kErr = tokenizer.Get(tokenizer.Cl100kBase)
	})
	if cl100kErr != nil {
		panic("tokens: cl100k_base: " + cl100kErr.Error())
	}
	return cl100k
}

// Count devolve os tokens cl100k do texto. É o mesmo recorte da família
// GPT-4; a listagem usa o inteiro persistido para não recarregar o XHTML.
func Count(text string) int {
	n, err := codec().Count(text)
	if err != nil {
		return 0
	}
	return n
}
