package interpolation_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateHelpers_Split(t *testing.T) {
	tests := []struct {
		name     string
		template string
		inputs   map[string]any
		want     string
		wantErr  bool
	}{
		{
			name:     "split comma-separated values",
			template: `{{range split .inputs.list ","}}{{.}} {{end}}`,
			inputs:   map[string]any{"list": "one,two,three"},
			want:     "one two three ",
		},
		{
			name:     "split with custom delimiter",
			template: `{{range split .inputs.path "/"}}[{{.}}]{{end}}`,
			inputs:   map[string]any{"path": "usr/local/bin"},
			want:     "[usr][local][bin]",
		},
		{
			name:     "split empty string returns single empty element",
			template: `{{split .inputs.empty ","}}`,
			inputs:   map[string]any{"empty": ""},
			want:     "[]",
		},
		{
			name:     "split with no delimiter match returns original",
			template: `{{split .inputs.text "|"}}`,
			inputs:   map[string]any{"text": "no delimiter here"},
			want:     "[no delimiter here]",
		},
		{
			name:     "split single element",
			template: `{{split .inputs.single ","}}`,
			inputs:   map[string]any{"single": "alone"},
			want:     "[alone]",
		},
		{
			name:     "split with spaces",
			template: `{{range split .inputs.words " "}}{{.}},{{end}}`,
			inputs:   map[string]any{"words": "alpha beta gamma"},
			want:     "alpha,beta,gamma,",
		},
		{
			name:     "split consecutive delimiters creates empty elements",
			template: `{{split .inputs.text ","}}`,
			inputs:   map[string]any{"text": "a,,b"},
			want:     "[a  b]",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			for k, v := range tt.inputs {
				ctx.Inputs[k] = v
			}

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

func TestTemplateHelpers_Join(t *testing.T) {
	tests := []struct {
		name     string
		template string
		setup    func(*interpolation.Context)
		want     string
		wantErr  bool
	}{
		{
			name:     "join slice with comma separator",
			template: `{{join (split .inputs.list ",") ", "}}`,
			setup: func(ctx *interpolation.Context) {
				ctx.Inputs["list"] = "one,two,three"
			},
			want: "one, two, three",
		},
		{
			name:     "join with pipe separator",
			template: `{{join (split .inputs.items " ") "|"}}`,
			setup: func(ctx *interpolation.Context) {
				ctx.Inputs["items"] = "a b c"
			},
			want: "a|b|c",
		},
		{
			name:     "join empty slice",
			template: `{{join (split .inputs.empty ",") ","}}`,
			setup: func(ctx *interpolation.Context) {
				ctx.Inputs["empty"] = ""
			},
			want: "",
		},
		{
			name:     "join single element",
			template: `{{join (split .inputs.single ",") ","}}`,
			setup: func(ctx *interpolation.Context) {
				ctx.Inputs["single"] = "alone"
			},
			want: "alone",
		},
		{
			name:     "join with newline separator",
			template: `{{join (split .inputs.text ",") "\n"}}`,
			setup: func(ctx *interpolation.Context) {
				ctx.Inputs["text"] = "line1,line2,line3"
			},
			want: "line1\nline2\nline3",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			tt.setup(ctx)

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

func TestTemplateHelpers_TrimSpace(t *testing.T) {
	tests := []struct {
		name     string
		template string
		inputs   map[string]any
		want     string
		wantErr  bool
	}{
		{
			name:     "trim leading and trailing spaces",
			template: `{{trimSpace .inputs.text}}`,
			inputs:   map[string]any{"text": "  hello  "},
			want:     "hello",
		},
		{
			name:     "trim tabs and newlines",
			template: `{{trimSpace .inputs.text}}`,
			inputs:   map[string]any{"text": "\t\nhello\n\t"},
			want:     "hello",
		},
		{
			name:     "trim mixed whitespace",
			template: `{{trimSpace .inputs.text}}`,
			inputs:   map[string]any{"text": " \t\n world \n\t "},
			want:     "world",
		},
		{
			name:     "trim empty string",
			template: `[{{trimSpace .inputs.empty}}]`,
			inputs:   map[string]any{"empty": ""},
			want:     "[]",
		},
		{
			name:     "trim whitespace-only string",
			template: `[{{trimSpace .inputs.spaces}}]`,
			inputs:   map[string]any{"spaces": "   \t\n   "},
			want:     "[]",
		},
		{
			name:     "trim preserves internal whitespace",
			template: `{{trimSpace .inputs.text}}`,
			inputs:   map[string]any{"text": "  hello  world  "},
			want:     "hello  world",
		},
		{
			name:     "trim no whitespace",
			template: `{{trimSpace .inputs.text}}`,
			inputs:   map[string]any{"text": "clean"},
			want:     "clean",
		},
		{
			name:     "trim from state output",
			template: `{{trimSpace .states.step.Output}}`,
			inputs:   map[string]any{},
			want:     "trimmed output",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			for k, v := range tt.inputs {
				ctx.Inputs[k] = v
			}

			if tt.name == "trim from state output" {
				ctx.States["step"] = interpolation.StepStateData{
					Output: "  trimmed output  \n",
				}
			}

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

func TestTemplateHelpers_ReadFile(t *testing.T) {
	tmpDir := t.TempDir()

	smallFile := filepath.Join(tmpDir, "small.txt")
	require.NoError(t, os.WriteFile(smallFile, []byte("content from file"), 0o600))

	multilineFile := filepath.Join(tmpDir, "multiline.txt")
	require.NoError(t, os.WriteFile(multilineFile, []byte("line1\nline2\nline3"), 0o600))

	largeContent := strings.Repeat("x", 1024*1024+1)
	largeFile := filepath.Join(tmpDir, "large.txt")
	require.NoError(t, os.WriteFile(largeFile, []byte(largeContent), 0o600))

	emptyFile := filepath.Join(tmpDir, "empty.txt")
	require.NoError(t, os.WriteFile(emptyFile, []byte(""), 0o600))

	tests := []struct {
		name     string
		template string
		inputs   map[string]any
		want     string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "read small file",
			template: `{{readFile .inputs.path}}`,
			inputs:   map[string]any{"path": smallFile},
			want:     "content from file",
		},
		{
			name:     "read multiline file",
			template: `{{readFile .inputs.path}}`,
			inputs:   map[string]any{"path": multilineFile},
			want:     "line1\nline2\nline3",
		},
		{
			name:     "read empty file",
			template: `[{{readFile .inputs.path}}]`,
			inputs:   map[string]any{"path": emptyFile},
			want:     "[]",
		},
		{
			name:     "read file exceeding 1MB limit",
			template: `{{readFile .inputs.path}}`,
			inputs:   map[string]any{"path": largeFile},
			wantErr:  true,
			errMsg:   "1MB",
		},
		{
			name:     "read nonexistent file",
			template: `{{readFile .inputs.path}}`,
			inputs:   map[string]any{"path": filepath.Join(tmpDir, "nonexistent.txt")},
			wantErr:  true,
			errMsg:   "no such file",
		},
		{
			name:     "read directory fails",
			template: `{{readFile .inputs.path}}`,
			inputs:   map[string]any{"path": tmpDir},
			wantErr:  true,
			errMsg:   "directory",
		},
		{
			name:     "read file path from state",
			template: `{{readFile .states.save_path.Output}}`,
			inputs:   map[string]any{},
			want:     "content from file",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			for k, v := range tt.inputs {
				ctx.Inputs[k] = v
			}

			if tt.name == "read file path from state" {
				ctx.States["save_path"] = interpolation.StepStateData{
					Output: smallFile,
				}
			}

			got, err := resolver.Resolve(tt.template, ctx)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateHelpers_ReadFile_ExactlyOneMB(t *testing.T) {
	tmpDir := t.TempDir()

	exactlyOneMB := strings.Repeat("a", 1024*1024)
	file := filepath.Join(tmpDir, "1mb.txt")
	require.NoError(t, os.WriteFile(file, []byte(exactlyOneMB), 0o600))

	resolver := interpolation.NewTemplateResolver()
	ctx := interpolation.NewContext()
	ctx.Inputs["path"] = file

	got, err := resolver.Resolve(`{{readFile .inputs.path}}`, ctx)

	require.NoError(t, err)
	assert.Equal(t, exactlyOneMB, got)
}

func TestTemplateHelpers_CombinedUsage(t *testing.T) {
	tmpDir := t.TempDir()

	specFile := filepath.Join(tmpDir, "spec.txt")
	require.NoError(t, os.WriteFile(specFile, []byte("  Feature A, Feature B, Feature C  "), 0o600))

	tests := []struct {
		name     string
		template string
		inputs   map[string]any
		want     string
	}{
		{
			name:     "split then join with different separator",
			template: `{{join (split .inputs.csv ",") " | "}}`,
			inputs:   map[string]any{"csv": "a,b,c"},
			want:     "a | b | c",
		},
		{
			name:     "readFile then trimSpace",
			template: `{{trimSpace (readFile .inputs.path)}}`,
			inputs:   map[string]any{"path": specFile},
			want:     "Feature A, Feature B, Feature C",
		},
		{
			name:     "readFile, trimSpace, split, join",
			template: `{{join (split (trimSpace (readFile .inputs.path)) ",") "\n- "}}`,
			inputs:   map[string]any{"path": specFile},
			want:     "Feature A\n- Feature B\n- Feature C",
		},
		{
			name:     "trim each element after split",
			template: `{{range split .inputs.list ","}}{{trimSpace .}},{{end}}`,
			inputs:   map[string]any{"list": " a , b , c "},
			want:     "a,b,c,",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			for k, v := range tt.inputs {
				ctx.Inputs[k] = v
			}

			got, err := resolver.Resolve(tt.template, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
