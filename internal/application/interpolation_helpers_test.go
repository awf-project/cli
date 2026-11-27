package application

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func TestProcessSnapshot_ReadsTheProcessOnce(t *testing.T) {
	t.Setenv("AWF_SNAPSHOT_TEST", "before")
	var process processSnapshot

	first := process.get()
	t.Setenv("AWF_SNAPSHOT_TEST", "after")
	second := process.get()

	assert.Equal(t, "before", first.env["AWF_SNAPSHOT_TEST"])
	assert.Equal(t, "before", second.env["AWF_SNAPSHOT_TEST"], "the snapshot is taken on first use only")
	wd, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, wd, second.wd)
	hostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, hostname, second.hostname)
	assert.Equal(t, os.Getenv("USER"), second.user)
}

func TestProcessSnapshot_KeepsValuesContainingEquals(t *testing.T) {
	t.Setenv("AWF_SNAPSHOT_EQUALS", "a=b=c")
	var process processSnapshot

	assert.Equal(t, "a=b=c", process.get().env["AWF_SNAPSHOT_EQUALS"])
}

func TestBuildInterpolationContext_ProcessEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		workflowEnv map[string]string
		want        map[string]string
		shared      bool
	}{
		{
			name:   "without workflow env the snapshot is shared",
			want:   map[string]string{"AWF_CTX_BASE": "process", "AWF_CTX_OVERRIDE": "process"},
			shared: true,
		},
		{
			name:        "workflow env overrides a copy of the snapshot",
			workflowEnv: map[string]string{"AWF_CTX_OVERRIDE": "workflow", "AWF_CTX_EXTRA": "workflow"},
			want: map[string]string{
				"AWF_CTX_BASE":     "process",
				"AWF_CTX_OVERRIDE": "workflow",
				"AWF_CTX_EXTRA":    "workflow",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AWF_CTX_BASE", "process")
			t.Setenv("AWF_CTX_OVERRIDE", "process")
			var process processSnapshot
			execCtx := workflow.NewExecutionContext("wf-id", "wf")
			for k, v := range tt.workflowEnv {
				execCtx.Env[k] = v
			}

			intCtx := buildInterpolationContext(execCtx, nil, &process)

			for k, v := range tt.want {
				assert.Equal(t, v, intCtx.Env[k], k)
			}
			snapshot := process.get().env
			assert.Equal(t, "process", snapshot["AWF_CTX_OVERRIDE"], "the snapshot is never modified")
			assert.NotContains(t, snapshot, "AWF_CTX_EXTRA")
			// Same map or not: a write through one shows in the other only when shared.
			intCtx.Env["AWF_CTX_PROBE"] = "x"
			_, seen := snapshot["AWF_CTX_PROBE"]
			delete(intCtx.Env, "AWF_CTX_PROBE")
			assert.Equal(t, tt.shared, seen)
		})
	}
}

func TestBuildInterpolationContext_RuntimeContextFromSnapshot(t *testing.T) {
	process := processSnapshot{}
	snap := process.get()

	intCtx := buildInterpolationContext(workflow.NewExecutionContext("wf-id", "wf"), nil, &process)

	assert.Equal(t, snap.wd, intCtx.Context.WorkingDir)
	assert.Equal(t, snap.hostname, intCtx.Context.Hostname)
	assert.Equal(t, snap.user, intCtx.Context.User)
}

func TestBuildInterpolationContext_ConcurrentUse(t *testing.T) {
	var process processSnapshot
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(withEnv bool) {
			defer wg.Done()
			execCtx := workflow.NewExecutionContext("wf-id", "wf")
			if withEnv {
				execCtx.Env["AWF_CTX_PARALLEL"] = "yes"
			}
			intCtx := buildInterpolationContext(execCtx, nil, &process)
			assert.NotEmpty(t, intCtx.Env)
		}(i%2 == 0)
	}
	wg.Wait()
}

func BenchmarkBuildInterpolationContext(b *testing.B) {
	execCtx := workflow.NewExecutionContext("wf-id", "wf")
	for i := 0; i < 20; i++ {
		execCtx.SetStepState(string(rune('a'+i)), workflow.StepState{Output: "some output", Status: workflow.StatusCompleted})
	}
	b.Run("shared snapshot", func(b *testing.B) {
		var process processSnapshot
		b.ReportAllocs()
		for b.Loop() {
			_ = buildInterpolationContext(execCtx, nil, &process)
		}
	})
	b.Run("snapshot per call (previous behavior)", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var process processSnapshot
			_ = buildInterpolationContext(execCtx, nil, &process)
		}
	})
}
