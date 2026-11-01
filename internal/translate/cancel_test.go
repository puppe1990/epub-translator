package translate

import (
	"context"
	"errors"
	"testing"
)

type cancellingClient struct {
	cancel context.CancelFunc
	calls  int
}

func (c *cancellingClient) Translate(ctx context.Context, _, _ string) (string, error) {
	c.calls++
	c.cancel()
	return "", ctx.Err()
}

func TestServiceCancellationStopsRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &cancellingClient{cancel: cancel}
	_, err := (Service{Client: client, Attempts: 3}).TranslateBody(ctx, "pt-BR", "<p>Hello.</p>")
	if !errors.Is(err, context.Canceled) || client.calls != 1 {
		t.Errorf("cancelamento precisa parar imediatamente: chamadas=%d, err=%v", client.calls, err)
	}
}

func TestServiceAlreadyCancelledDoesNotCallProvider(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeClient{responses: []string{"<p>Olá.</p>"}}
	_, err := (Service{Client: client}).TranslateBody(ctx, "pt-BR", "<p>Hello.</p>")
	if !errors.Is(err, context.Canceled) || client.calls != 0 {
		t.Errorf("contexto cancelado não pode chamar provedor: chamadas=%d, err=%v", client.calls, err)
	}
}
