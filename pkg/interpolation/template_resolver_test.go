package interpolation_test

import (
	"testing"

	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateResolver_makeLoopAccessor_Index1(t *testing.T) {
	tests := []struct {
		name      string
		loopIndex int
		want      string
	}{
		{
			name:      "index1 at zero",
			loopIndex: 0,
			want:      "1",
		},
		{
			name:      "index1 basic",
			loopIndex: 2,
			want:      "3",
		},
		{
			name:      "index1 large index",
			loopIndex: 99,
			want:      "100",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = &interpolation.LoopData{
				Index: tt.loopIndex,
				Item:  "test",
			}

			result, err := resolver.Resolve("{{(loop).index1}}", ctx)

			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestTemplateResolver_makeLoopAccessor_Parent_Nil(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()
	ctx := interpolation.NewContext()
	ctx.Loop = &interpolation.LoopData{
		Index:  0,
		Item:   "child",
		Parent: nil,
	}

	result, err := resolver.Resolve("{{if (loop).parent}}has-parent{{else}}no-parent{{end}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, "no-parent", result)
}

func TestTemplateResolver_makeLoopAccessor_Parent_WithValue(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()
	ctx := interpolation.NewContext()
	ctx.Loop = &interpolation.LoopData{
		Index: 2,
		Item:  "child",
		Parent: &interpolation.LoopData{
			Index: 1,
			Item:  "parent-item",
		},
	}

	result, err := resolver.Resolve("{{(loop).parent.index}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, "1", result)
}

func TestTemplateResolver_makeLoopAccessor_Parent_Index1(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()
	ctx := interpolation.NewContext()
	ctx.Loop = &interpolation.LoopData{
		Index: 2,
		Item:  "child",
		Parent: &interpolation.LoopData{
			Index: 0,
			Item:  "parent",
		},
	}

	result, err := resolver.Resolve("{{(loop).parent.index1}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, "1", result)
}

func TestTemplateResolver_makeLoopAccessor_NoLoop(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()
	ctx := interpolation.NewContext()
	ctx.Loop = nil

	_, err := resolver.Resolve("{{(loop).index1}}", ctx)

	require.Error(t, err)
}

func TestTemplateResolver_makeLoopAccessor_NestedParent(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()
	ctx := interpolation.NewContext()
	ctx.Loop = &interpolation.LoopData{
		Index: 3,
		Item:  "grandchild",
		Parent: &interpolation.LoopData{
			Index: 1,
			Item:  "child",
			Parent: &interpolation.LoopData{
				Index: 0,
				Item:  "grandparent",
			},
		},
	}

	result, err := resolver.Resolve("{{(loop).parent.parent.index}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, "0", result)
}

func TestTemplateResolver_makeLoopAccessor_ParentWithComplexItem(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()
	ctx := interpolation.NewContext()
	ctx.Loop = &interpolation.LoopData{
		Index: 0,
		Item:  "child",
		Parent: &interpolation.LoopData{
			Index:  1,
			Item:   map[string]any{"key": "value", "count": 42},
			Length: 5,
		},
	}

	result, err := resolver.Resolve("{{(loop).parent.index}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, "1", result)
}
