package interpolation_test

import (
	"testing"

	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateResolver_AWF(t *testing.T) {
	tests := []struct {
		name     string
		template string
		awf      map[string]string
		want     string
		wantErr  bool
	}{
		{
			name:     "single AWF variable",
			template: "prompts: {{.awf.prompts_dir}}",
			awf:      map[string]string{"prompts_dir": "/home/user/.config/awf/prompts"},
			want:     "prompts: /home/user/.config/awf/prompts",
		},
		{
			name:     "multiple AWF variables",
			template: "config={{.awf.config_dir}} data={{.awf.data_dir}}",
			awf: map[string]string{
				"config_dir": "/home/user/.config/awf",
				"data_dir":   "/home/user/.local/share/awf",
			},
			want: "config=/home/user/.config/awf data=/home/user/.local/share/awf",
		},
		{
			name:     "AWF variable in path",
			template: "{{.awf.prompts_dir}}/plan/research.md",
			awf:      map[string]string{"prompts_dir": "/etc/awf/prompts"},
			want:     "/etc/awf/prompts/plan/research.md",
		},
		{
			name:     "all standard AWF directories",
			template: "{{.awf.prompts_dir}},{{.awf.config_dir}},{{.awf.workflows_dir}},{{.awf.data_dir}},{{.awf.plugins_dir}},{{.awf.scripts_dir}}",
			awf: map[string]string{
				"prompts_dir":   "/home/user/.config/awf/prompts",
				"config_dir":    "/home/user/.config/awf",
				"workflows_dir": "/home/user/.config/awf/workflows",
				"data_dir":      "/home/user/.local/share/awf",
				"plugins_dir":   "/home/user/.local/share/awf/plugins",
				"scripts_dir":   "/home/user/.config/awf/scripts",
			},
			want: "/home/user/.config/awf/prompts,/home/user/.config/awf,/home/user/.config/awf/workflows,/home/user/.local/share/awf,/home/user/.local/share/awf/plugins,/home/user/.config/awf/scripts",
		},
		{
			name:     "empty AWF map",
			template: "{{.awf.prompts_dir}}",
			awf:      map[string]string{},
			wantErr:  true,
		},
		{
			name:     "undefined AWF key",
			template: "{{.awf.unknown_dir}}",
			awf:      map[string]string{"prompts_dir": "/home/user/.config/awf/prompts"},
			wantErr:  true,
		},
		{
			name:     "AWF with special characters in path",
			template: "path={{.awf.config_dir}}",
			awf:      map[string]string{"config_dir": "/home/user name/My Documents/.config/awf"},
			want:     "path=/home/user name/My Documents/.config/awf",
		},
		{
			name:     "AWF with tilde expansion result",
			template: "{{.awf.prompts_dir}}/custom.md",
			awf:      map[string]string{"prompts_dir": "/home/user/.config/awf/prompts"},
			want:     "/home/user/.config/awf/prompts/custom.md",
		},
		{
			name:     "AWF empty string value",
			template: "dir=[{{.awf.empty_dir}}]",
			awf:      map[string]string{"empty_dir": ""},
			want:     "dir=[]",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.AWF = tt.awf

			got, err := resolver.Resolve(tt.template, ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewContext_AWFInitialization(t *testing.T) {
	ctx := interpolation.NewContext()

	require.NotNil(t, ctx.AWF, "AWF map should be initialized")
	assert.Empty(t, ctx.AWF, "AWF map should be empty initially")
	assert.IsType(t, map[string]string{}, ctx.AWF, "AWF should be map[string]string type")
}

func TestContext_AWFWithOtherVariables(t *testing.T) {
	tests := []struct {
		name     string
		template string
		setup    func(*interpolation.Context)
		want     string
	}{
		{
			name:     "AWF with inputs",
			template: "{{.awf.prompts_dir}}/{{.inputs.filename}}",
			setup: func(ctx *interpolation.Context) {
				ctx.AWF["prompts_dir"] = "/home/user/.config/awf/prompts"
				ctx.Inputs["filename"] = "analyze.md"
			},
			want: "/home/user/.config/awf/prompts/analyze.md",
		},
		{
			name:     "AWF with env variables",
			template: "{{.awf.config_dir}}/{{.env.ENV_NAME}}.yaml",
			setup: func(ctx *interpolation.Context) {
				ctx.AWF["config_dir"] = "/etc/awf"
				ctx.Env["ENV_NAME"] = "production"
			},
			want: "/etc/awf/production.yaml",
		},
		{
			name:     "AWF with state output",
			template: "{{.awf.data_dir}}/{{.states.save.Output}}",
			setup: func(ctx *interpolation.Context) {
				ctx.AWF["data_dir"] = "/var/lib/awf"
				ctx.States["save"] = interpolation.StepStateData{Output: "result.json"}
			},
			want: "/var/lib/awf/result.json",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			tt.setup(ctx)

			got, err := resolver.Resolve(tt.template, ctx)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
