package notify

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhookCancellationBeforeHeaders(t *testing.T) {
	t.Setenv("AWF_TEST_MODE", "")
	const secret = "PRIVATE_WEBHOOK_TOKEN_CANARY"
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	backend := newWebhookBackend()
	go func() {
		_, err := backend.Send(ctx, NotificationPayload{Message: "notification", Metadata: map[string]string{"webhook_url": server.URL + "?token=" + secret}})
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("webhook never started")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("lost cancellation sentinel: %v", err)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("T039: webhook error exposes confidential URL query: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("webhook did not cancel")
	}
}
