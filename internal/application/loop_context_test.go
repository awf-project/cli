package application

import (
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildLoopDataChain_Nil(t *testing.T) {
	// Nil input should return nil
	result := buildLoopDataChain(nil)
	assert.Nil(t, result)
}

func TestBuildLoopDataChain_SingleLevel(t *testing.T) {
	single := &workflow.LoopContext{
		Item:   "A",
		Index:  0,
		First:  true,
		Last:   true,
		Length: 1,
		Parent: nil,
	}

	result := buildLoopDataChain(single)

	require.NotNil(t, result)
	assert.Equal(t, "A", result.Item)
	assert.Equal(t, 0, result.Index)
	assert.True(t, result.First)
	assert.True(t, result.Last)
	assert.Equal(t, 1, result.Length)
	assert.Nil(t, result.Parent)
}

func TestBuildLoopDataChain_TwoLevels(t *testing.T) {
	outer := &workflow.LoopContext{
		Item:   "outer-item",
		Index:  1,
		First:  false,
		Last:   true,
		Length: 2,
		Parent: nil,
	}
	inner := &workflow.LoopContext{
		Item:   "inner-item",
		Index:  0,
		First:  true,
		Last:   false,
		Length: 3,
		Parent: outer,
	}

	result := buildLoopDataChain(inner)

	require.NotNil(t, result)
	assert.Equal(t, "inner-item", result.Item)
	assert.Equal(t, 0, result.Index)
	assert.True(t, result.First)
	assert.False(t, result.Last)
	assert.Equal(t, 3, result.Length)

	// Verify parent chain
	require.NotNil(t, result.Parent)
	assert.Equal(t, "outer-item", result.Parent.Item)
	assert.Equal(t, 1, result.Parent.Index)
	assert.False(t, result.Parent.First)
	assert.True(t, result.Parent.Last)
	assert.Equal(t, 2, result.Parent.Length)
	assert.Nil(t, result.Parent.Parent)
}

func TestBuildLoopDataChain_ThreeLevels(t *testing.T) {
	l1 := &workflow.LoopContext{
		Item:   "L1",
		Index:  0,
		First:  true,
		Last:   true,
		Length: 1,
		Parent: nil,
	}
	l2 := &workflow.LoopContext{
		Item:   "L2",
		Index:  1,
		First:  false,
		Last:   false,
		Length: 5,
		Parent: l1,
	}
	l3 := &workflow.LoopContext{
		Item:   "L3",
		Index:  2,
		First:  false,
		Last:   true,
		Length: 3,
		Parent: l2,
	}

	result := buildLoopDataChain(l3)

	// Verify L3
	require.NotNil(t, result)
	assert.Equal(t, "L3", result.Item)
	assert.Equal(t, 2, result.Index)

	// Verify L2
	require.NotNil(t, result.Parent)
	assert.Equal(t, "L2", result.Parent.Item)
	assert.Equal(t, 1, result.Parent.Index)
	assert.Equal(t, 5, result.Parent.Length)

	// Verify L1
	require.NotNil(t, result.Parent.Parent)
	assert.Equal(t, "L1", result.Parent.Parent.Item)
	assert.Equal(t, 0, result.Parent.Parent.Index)

	// No L0
	assert.Nil(t, result.Parent.Parent.Parent)
}

func TestBuildLoopDataChain_WhileLoop(t *testing.T) {
	// While loops have Length = -1
	whileCtx := &workflow.LoopContext{
		Item:   nil, // while loops don't have Item
		Index:  5,
		First:  false,
		Last:   false, // unknown for while
		Length: -1,    // sentinel for while loops
		Parent: nil,
	}

	result := buildLoopDataChain(whileCtx)

	require.NotNil(t, result)
	assert.Nil(t, result.Item)
	assert.Equal(t, 5, result.Index)
	assert.False(t, result.First)
	assert.False(t, result.Last)
	assert.Equal(t, -1, result.Length)
	assert.Nil(t, result.Parent)
}

func TestBuildLoopDataChain_DeepNesting(t *testing.T) {
	// Build a 5-level deep chain
	l1 := &workflow.LoopContext{Item: "L1", Index: 0, First: true, Last: false, Length: 2, Parent: nil}
	l2 := &workflow.LoopContext{Item: "L2", Index: 1, First: false, Last: true, Length: 2, Parent: l1}
	l3 := &workflow.LoopContext{Item: "L3", Index: 2, First: false, Last: false, Length: 5, Parent: l2}
	l4 := &workflow.LoopContext{Item: "L4", Index: 0, First: true, Last: false, Length: 3, Parent: l3}
	l5 := &workflow.LoopContext{Item: "L5", Index: 9, First: false, Last: true, Length: 10, Parent: l4}

	result := buildLoopDataChain(l5)

	// Verify the entire chain is converted correctly
	require.NotNil(t, result)
	assert.Equal(t, "L5", result.Item)
	assert.Equal(t, 9, result.Index)
	assert.Equal(t, 10, result.Length)

	require.NotNil(t, result.Parent)
	assert.Equal(t, "L4", result.Parent.Item)

	require.NotNil(t, result.Parent.Parent)
	assert.Equal(t, "L3", result.Parent.Parent.Item)

	require.NotNil(t, result.Parent.Parent.Parent)
	assert.Equal(t, "L2", result.Parent.Parent.Parent.Item)

	require.NotNil(t, result.Parent.Parent.Parent.Parent)
	assert.Equal(t, "L1", result.Parent.Parent.Parent.Parent.Item)

	assert.Nil(t, result.Parent.Parent.Parent.Parent.Parent)
}

func TestBuildLoopDataChain_MixedLoopTypes(t *testing.T) {
	// Outer: for_each loop (has item, known length)
	forEachCtx := &workflow.LoopContext{
		Item:   "outer-item",
		Index:  1,
		First:  false,
		Last:   true,
		Length: 2,
		Parent: nil,
	}

	// Inner: while loop (no item, unknown length)
	whileCtx := &workflow.LoopContext{
		Item:   nil,
		Index:  3,
		First:  false,
		Last:   false,
		Length: -1,
		Parent: forEachCtx,
	}

	result := buildLoopDataChain(whileCtx)

	// Verify inner (while)
	require.NotNil(t, result)
	assert.Nil(t, result.Item)
	assert.Equal(t, 3, result.Index)
	assert.Equal(t, -1, result.Length)

	// Verify outer (for_each)
	require.NotNil(t, result.Parent)
	assert.Equal(t, "outer-item", result.Parent.Item)
	assert.Equal(t, 1, result.Parent.Index)
	assert.Equal(t, 2, result.Parent.Length)
	assert.True(t, result.Parent.Last)

	assert.Nil(t, result.Parent.Parent)
}

func TestBuildLoopDataChain_ComplexItems(t *testing.T) {
	tests := []struct {
		name string
		item any
	}{
		{name: "string item", item: "test"},
		{name: "int item", item: 42},
		{name: "float item", item: 3.14},
		{name: "bool item", item: true},
		{name: "nil item", item: nil},
		{name: "slice item", item: []string{"a", "b", "c"}},
		{name: "map item", item: map[string]any{"key": "value"}},
		{name: "nested map", item: map[string]any{"outer": map[string]int{"inner": 123}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &workflow.LoopContext{
				Item:   tt.item,
				Index:  0,
				First:  true,
				Last:   true,
				Length: 1,
				Parent: nil,
			}

			result := buildLoopDataChain(ctx)

			require.NotNil(t, result)
			assert.Equal(t, tt.item, result.Item)
		})
	}
}

func TestBuildLoopDataChain_PreservesAllFields(t *testing.T) {
	// Create a chain with all field combinations
	outer := &workflow.LoopContext{
		Item:   "outer",
		Index:  0,
		First:  true,
		Last:   false,
		Length: 5,
		Parent: nil,
	}
	inner := &workflow.LoopContext{
		Item:   "inner",
		Index:  4,
		First:  false,
		Last:   true,
		Length: 5,
		Parent: outer,
	}

	result := buildLoopDataChain(inner)

	// Verify inner loop data
	require.NotNil(t, result)
	assert.Equal(t, "inner", result.Item)
	assert.Equal(t, 4, result.Index)
	assert.False(t, result.First)
	assert.True(t, result.Last)
	assert.Equal(t, 5, result.Length)
	assert.Equal(t, 5, result.Index1()) // Test Index1() helper

	// Verify outer loop data
	require.NotNil(t, result.Parent)
	assert.Equal(t, "outer", result.Parent.Item)
	assert.Equal(t, 0, result.Parent.Index)
	assert.True(t, result.Parent.First)
	assert.False(t, result.Parent.Last)
	assert.Equal(t, 5, result.Parent.Length)
	assert.Equal(t, 1, result.Parent.Index1())
}

func TestBuildLoopDataChain_IndependentInstances(t *testing.T) {
	// Verify that buildLoopDataChain creates new instances, not shared references
	ctx := &workflow.LoopContext{
		Item:   "item",
		Index:  0,
		First:  true,
		Last:   true,
		Length: 1,
		Parent: nil,
	}

	result1 := buildLoopDataChain(ctx)
	result2 := buildLoopDataChain(ctx)

	// Should be equal in value
	assert.Equal(t, result1.Item, result2.Item)
	assert.Equal(t, result1.Index, result2.Index)

	// But different instances (not same pointer)
	assert.NotSame(t, result1, result2)
}
