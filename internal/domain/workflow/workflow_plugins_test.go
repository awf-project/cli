package workflow_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
)

func TestResolveStepType(t *testing.T) {
	tests := []struct {
		name     string
		plugins  map[string]string
		typeName string
		want     string
	}{
		{
			name:     "no alias no dot passes through",
			plugins:  map[string]string{"pg": "database"},
			typeName: "command",
			want:     "command",
		},
		{
			name:     "alias resolves prefix",
			plugins:  map[string]string{"pg": "database"},
			typeName: "pg.query",
			want:     "database.query",
		},
		{
			name:     "unaliased prefix passes through",
			plugins:  map[string]string{"pg": "database"},
			typeName: "k8s.deploy",
			want:     "k8s.deploy",
		},
		{
			name:     "nil plugins map passes through",
			plugins:  nil,
			typeName: "database.query",
			want:     "database.query",
		},
		{
			name:     "empty plugins map passes through",
			plugins:  map[string]string{},
			typeName: "database.query",
			want:     "database.query",
		},
		{
			name:     "multiple dots only splits on first",
			plugins:  map[string]string{"pg": "database"},
			typeName: "pg.schema.migrate",
			want:     "database.schema.migrate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &workflow.Workflow{Plugins: tt.plugins}
			got := wf.ResolveStepType(tt.typeName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidate_PluginAliases(t *testing.T) {
	base := func() *workflow.Workflow {
		return &workflow.Workflow{
			Name:    "test",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name:    "start",
					Type:    workflow.StepTypeTerminal,
					Message: "done",
				},
			},
		}
	}

	t.Run("valid aliases pass validation", func(t *testing.T) {
		wf := base()
		wf.Plugins = map[string]string{"pg": "database", "k8s": "kubernetes"}
		err := wf.Validate(nil, nil)
		assert.NoError(t, err)
	})

	t.Run("empty target fails validation", func(t *testing.T) {
		wf := base()
		wf.Plugins = map[string]string{"pg": ""}
		err := wf.Validate(nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty target")
	})

	t.Run("nil plugins passes validation", func(t *testing.T) {
		wf := base()
		wf.Plugins = nil
		err := wf.Validate(nil, nil)
		assert.NoError(t, err)
	})
}
