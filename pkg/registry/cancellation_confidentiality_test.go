package registry_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/registry"
)

func TestRegistryCancellationBeforeHeaders(t *testing.T) {
	const secret = "PRIVATE_DOWNLOAD_TOKEN_CANARY"
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	go func() { _, err := registry.Download(ctx, server.Client(), server.URL+"?token="+secret); result <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download never started")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("lost cancellation sentinel: %v", err)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("T041: download error exposes confidential URL query: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("download did not cancel")
	}
}
