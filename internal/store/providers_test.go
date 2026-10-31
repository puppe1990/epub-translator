package store

import (
	"testing"

	"github.com/puppe1990/epub-translator/internal/models"
)

func TestStore_SaveProvider_upsertsAndKeepsKeyWhenEmpty(t *testing.T) {
	s := newTestStore(t)

	if err := s.SaveProvider(models.Provider{
		Slug: "nvidia", BaseURL: "https://integrate.api.nvidia.com/v1",
		Model: "minimaxai/minimax-m3.1-flash", APIKey: "nvapi-teste",
	}); err != nil {
		t.Fatal(err)
	}
	provider, err := s.GetProvider("nvidia")
	if err != nil {
		t.Fatal(err)
	}
	if provider.APIKey != "nvapi-teste" || provider.Model != "minimaxai/minimax-m3.1-flash" {
		t.Errorf("provider = %+v", provider)
	}

	// Atualizar só o modelo (chave vazia) não pode apagar a chave salva.
	if err := s.SaveProvider(models.Provider{Slug: "nvidia", BaseURL: provider.BaseURL, Model: "outro-modelo"}); err != nil {
		t.Fatal(err)
	}
	provider, _ = s.GetProvider("nvidia")
	if provider.Model != "outro-modelo" || provider.APIKey != "nvapi-teste" {
		t.Errorf("após upsert = %+v", provider)
	}

	providers, err := s.ListProviders()
	if err != nil || len(providers) != 1 || providers[0].Slug != "nvidia" {
		t.Errorf("ListProviders = %+v, %v", providers, err)
	}

	if _, err := s.GetProvider("inexistente"); err == nil {
		t.Error("GetProvider deveria errar para slug inexistente")
	}
}
