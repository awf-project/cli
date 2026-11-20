//go:build integration

package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPServerDispatchesNotifierThroughDiscoveredAdapter(t *testing.T) {
	const (
		defaultChannel = "serve-default-channel-output-canary"
		payload        = "serve-payload-output-canary"
		recordLimit    = int64(1 << 20)
	)
	fixture := newNotifierRunFixture(t, []string{"send"})
	fixture.writeConfig(t, fixture.pluginID, defaultChannel)
	fixture.writeSingleOperationWorkflow(t, "notifier.send", "      text: "+payload+"\n")

	localPluginDir := filepath.Join(fixture.root, ".awf", "plugins")
	require.NoError(t, os.MkdirAll(filepath.Dir(localPluginDir), 0o700))
	require.NoError(t, os.Rename(filepath.Join(fixture.root, "plugins"), localPluginDir))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	ctx, cancel := context.WithCancel(t.Context())
	cmd := cli.NewServeCommand()
	cmd.SetContext(ctx)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--host", "127.0.0.1", "--port", fmt.Sprintf("%d", port)})
	done := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		done <- cmd.Execute()
	}()

	shutdown := func() {
		cancel()
		select {
		case serveErr := <-done:
			require.NoError(t, serveErr)
		case <-time.After(5 * time.Second):
			t.Fatal("serve command did not complete bounded shutdown")
		}
	}
	t.Cleanup(func() {
		cancel()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Error("serve command leaked after test cleanup")
		}
	})

	client := &http.Client{Timeout: 2 * time.Second}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		request, requestErr := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/docs", nil)
		if requestErr != nil {
			return false
		}
		response, requestErr := client.Do(request)
		if requestErr != nil {
			return false
		}
		defer response.Body.Close()
		return response.StatusCode == http.StatusOK
	}, 5*time.Second, 25*time.Millisecond, "serve command did not become ready")

	runRequest, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		baseURL+"/api/workflows/local/"+fixture.workflowName+"/run", strings.NewReader(`{"inputs":{}}`))
	require.NoError(t, err)
	runRequest.Header.Set("Content-Type", "application/json")
	runResponse, err := client.Do(runRequest)
	require.NoError(t, err)
	runBody, err := io.ReadAll(io.LimitReader(runResponse.Body, 1<<20))
	require.NoError(t, err)
	require.NoError(t, runResponse.Body.Close())
	require.Equal(t, http.StatusAccepted, runResponse.StatusCode, string(runBody))
	var accepted struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
		} `json:"body"`
	}
	require.NoError(t, json.Unmarshal(runBody, &accepted))
	require.NotEmpty(t, accepted.Body.ExecutionID)

	var statusBody []byte
	require.Eventually(t, func() bool {
		statusRequest, requestErr := http.NewRequestWithContext(t.Context(), http.MethodGet,
			baseURL+"/api/executions/"+accepted.Body.ExecutionID, nil)
		if requestErr != nil {
			return false
		}
		statusResponse, requestErr := client.Do(statusRequest)
		if requestErr != nil {
			return false
		}
		defer statusResponse.Body.Close()
		statusBody, requestErr = io.ReadAll(io.LimitReader(statusResponse.Body, 1<<20))
		return requestErr == nil && statusResponse.StatusCode == http.StatusOK &&
			strings.Contains(strings.ToLower(string(statusBody)), `"status":"completed"`)
	}, 10*time.Second, 25*time.Millisecond, "submitted workflow did not complete")

	shutdown()

	recordInfo, err := os.Stat(fixture.recordPath)
	require.NoError(t, err)
	require.LessOrEqual(t, recordInfo.Size(), recordLimit)
	records := fixture.readRecords(t)
	require.Len(t, records, 1)
	assert.Equal(t, fixture.pluginID, records[0].Adapter)
	assert.Equal(t, "send", records[0].Operation)
	assert.Equal(t, defaultChannel, records[0].Inputs["channel"])
	assert.Equal(t, payload, records[0].Inputs["text"])

	responses := string(runBody) + string(statusBody)
	for _, canary := range []string{defaultChannel, payload, recordingCredential} {
		assert.NotContains(t, responses, canary)
		assert.NotContains(t, stderr.String(), canary)
	}
	fixture.assertConfidential(t, stdout.String()+stderr.String(), []string{defaultChannel, payload, recordingCredential})
}
