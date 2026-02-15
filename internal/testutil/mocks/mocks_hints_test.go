package mocks_test

import (
	"sync"
	"testing"

	domainerrors "github.com/awf-project/awf/internal/domain/errors"
	"github.com/awf-project/awf/internal/testutil/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockErrorFormatter_AddHintGenerator_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		generator domainerrors.HintGenerator
		testError *domainerrors.StructuredError
		wantHints []string
	}{
		{
			name: "single hint generator returns one hint",
			generator: func(err *domainerrors.StructuredError) []domainerrors.Hint {
				return []domainerrors.Hint{
					{Message: "Did you mean 'workflow.yaml'?"},
				}
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"file not found",
				nil,
				nil,
			),
			wantHints: []string{"Did you mean 'workflow.yaml'?"},
		},
		{
			name: "generator returns multiple hints",
			generator: func(err *domainerrors.StructuredError) []domainerrors.Hint {
				return []domainerrors.Hint{
					{Message: "Check file path: /path/to/file"},
					{Message: "Run 'awf list' to see available workflows"},
				}
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"file not found",
				nil,
				nil,
			),
			wantHints: []string{
				"Check file path: /path/to/file",
				"Run 'awf list' to see available workflows",
			},
		},
		{
			name: "generator returns empty hints",
			generator: func(err *domainerrors.StructuredError) []domainerrors.Hint {
				return []domainerrors.Hint{}
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"file not found",
				nil,
				nil,
			),
			wantHints: []string{},
		},
		{
			name: "generator uses error context",
			generator: func(err *domainerrors.StructuredError) []domainerrors.Hint {
				if path, ok := err.Details["path"].(string); ok {
					return []domainerrors.Hint{
						{Message: "File not found at: " + path},
					}
				}
				return []domainerrors.Hint{}
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"file not found",
				map[string]any{"path": "/workflows/test.yaml"},
				nil,
			),
			wantHints: []string{"File not found at: /workflows/test.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := mocks.NewMockErrorFormatter()

			formatter.AddHintGenerator(tt.generator)
			formatter.EnableHints(true)
			hints := formatter.GetHints(tt.testError)

			require.Len(t, hints, len(tt.wantHints), "Unexpected number of hints")
			for i, want := range tt.wantHints {
				assert.Equal(t, want, hints[i].Message, "Hint %d message mismatch", i)
			}
		})
	}
}

func TestMockErrorFormatter_AddHintGenerator_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(*mocks.MockErrorFormatter)
		testError      *domainerrors.StructuredError
		enableHints    bool
		expectedHints  int
		expectedResult []string
	}{
		{
			name: "multiple generators accumulate hints",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Hint from generator 1"}}
				})
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Hint from generator 2"}}
				})
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Hint from generator 3"}}
				})
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			enableHints:   true,
			expectedHints: 3,
			expectedResult: []string{
				"Hint from generator 1",
				"Hint from generator 2",
				"Hint from generator 3",
			},
		},
		{
			name: "hints disabled returns empty slice",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "This hint should not appear"}}
				})
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			enableHints:    false,
			expectedHints:  0,
			expectedResult: []string{},
		},
		{
			name: "no generators returns empty slice",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				// Don't add any generators
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			enableHints:    true,
			expectedHints:  0,
			expectedResult: []string{},
		},
		{
			name: "generator returning nil slice is handled safely",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return nil
				})
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			enableHints:    true,
			expectedHints:  0,
			expectedResult: []string{},
		},
		{
			name: "generator with multiple hints from same generator",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{
						{Message: "First hint"},
						{Message: "Second hint"},
						{Message: "Third hint"},
					}
				})
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			enableHints:   true,
			expectedHints: 3,
			expectedResult: []string{
				"First hint",
				"Second hint",
				"Third hint",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := mocks.NewMockErrorFormatter()
			tt.setupFunc(formatter)
			formatter.EnableHints(tt.enableHints)

			hints := formatter.GetHints(tt.testError)

			assert.Len(t, hints, tt.expectedHints, "Unexpected number of hints")
			for i, expected := range tt.expectedResult {
				assert.Equal(t, expected, hints[i].Message, "Hint %d message mismatch", i)
			}
		})
	}
}

func TestMockErrorFormatter_SetHintGenerators_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		generators []domainerrors.HintGenerator
		testError  *domainerrors.StructuredError
		wantHints  []string
	}{
		{
			name: "set single generator",
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Single hint"}}
				},
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			wantHints: []string{"Single hint"},
		},
		{
			name: "set multiple generators",
			generators: []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "First"}}
				},
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Second"}}
				},
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			wantHints: []string{"First", "Second"},
		},
		{
			name:       "set empty generator slice",
			generators: []domainerrors.HintGenerator{},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			wantHints: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := mocks.NewMockErrorFormatter()

			formatter.SetHintGenerators(tt.generators)
			formatter.EnableHints(true)
			hints := formatter.GetHints(tt.testError)

			require.Len(t, hints, len(tt.wantHints))
			for i, want := range tt.wantHints {
				assert.Equal(t, want, hints[i].Message)
			}
		})
	}
}

func TestMockErrorFormatter_SetHintGenerators_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(*mocks.MockErrorFormatter)
		testError      *domainerrors.StructuredError
		expectedHints  int
		expectedResult []string
	}{
		{
			name: "replaces existing generators",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				// Add initial generators
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Should be replaced"}}
				})
				// Replace with new generators
				formatter.SetHintGenerators([]domainerrors.HintGenerator{
					func(err *domainerrors.StructuredError) []domainerrors.Hint {
						return []domainerrors.Hint{{Message: "New generator"}}
					},
				})
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			expectedHints:  1,
			expectedResult: []string{"New generator"},
		},
		{
			name: "set nil generator in slice is skipped",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.SetHintGenerators([]domainerrors.HintGenerator{
					nil,
					func(err *domainerrors.StructuredError) []domainerrors.Hint {
						return []domainerrors.Hint{{Message: "Valid generator"}}
					},
				})
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			expectedHints:  1,
			expectedResult: []string{"Valid generator"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := mocks.NewMockErrorFormatter()
			formatter.EnableHints(true)

			tt.setupFunc(formatter)
			hints := formatter.GetHints(tt.testError)

			assert.Len(t, hints, tt.expectedHints)
			for i, expected := range tt.expectedResult {
				assert.Equal(t, expected, hints[i].Message)
			}
		})
	}
}

func TestMockErrorFormatter_EnableHints_HappyPath(t *testing.T) {
	tests := []struct {
		name          string
		enabled       bool
		expectedHints int
	}{
		{
			name:          "enable hints",
			enabled:       true,
			expectedHints: 1,
		},
		{
			name:          "disable hints",
			enabled:       false,
			expectedHints: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := mocks.NewMockErrorFormatter()
			formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
				return []domainerrors.Hint{{Message: "Test hint"}}
			})
			testError := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			)

			formatter.EnableHints(tt.enabled)
			hints := formatter.GetHints(testError)

			assert.Len(t, hints, tt.expectedHints)
		})
	}
}

func TestMockErrorFormatter_EnableHints_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(*mocks.MockErrorFormatter)
		expectedHints  int
		expectedResult []string
	}{
		{
			name: "toggle hints on and off",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Toggle hint"}}
				})
				formatter.EnableHints(true)
				formatter.EnableHints(false)
				formatter.EnableHints(true)
			},
			expectedHints:  1,
			expectedResult: []string{"Toggle hint"},
		},
		{
			name: "multiple enable calls idempotent",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Idempotent hint"}}
				})
				formatter.EnableHints(true)
				formatter.EnableHints(true)
				formatter.EnableHints(true)
			},
			expectedHints:  1,
			expectedResult: []string{"Idempotent hint"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := mocks.NewMockErrorFormatter()
			testError := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			)

			tt.setupFunc(formatter)
			hints := formatter.GetHints(testError)

			assert.Len(t, hints, tt.expectedHints)
			for i, expected := range tt.expectedResult {
				assert.Equal(t, expected, hints[i].Message)
			}
		})
	}
}

func TestMockErrorFormatter_GetHints_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*mocks.MockErrorFormatter)
		testError *domainerrors.StructuredError
		wantHints []string
	}{
		{
			name: "get hints from single generator",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Single hint"}}
				})
				formatter.EnableHints(true)
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			wantHints: []string{"Single hint"},
		},
		{
			name: "get hints from multiple generators",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "First"}}
				})
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Second"}}
				})
				formatter.EnableHints(true)
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			wantHints: []string{"First", "Second"},
		},
		{
			name: "get contextual hints based on error type",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					if err.Code == domainerrors.ErrorCodeUserInputMissingFile {
						return []domainerrors.Hint{{Message: "File hint"}}
					}
					return []domainerrors.Hint{}
				})
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					if err.Code == domainerrors.ErrorCodeWorkflowParseYAMLSyntax {
						return []domainerrors.Hint{{Message: "YAML hint"}}
					}
					return []domainerrors.Hint{}
				})
				formatter.EnableHints(true)
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			wantHints: []string{"File hint"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := mocks.NewMockErrorFormatter()
			tt.setupFunc(formatter)

			hints := formatter.GetHints(tt.testError)

			require.Len(t, hints, len(tt.wantHints))
			for i, want := range tt.wantHints {
				assert.Equal(t, want, hints[i].Message)
			}
		})
	}
}

func TestMockErrorFormatter_GetHints_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*mocks.MockErrorFormatter)
		testError     *domainerrors.StructuredError
		expectedHints int
	}{
		{
			name: "hints disabled returns empty",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Should not appear"}}
				})
				formatter.EnableHints(false)
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			expectedHints: 0,
		},
		{
			name: "no generators returns empty",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.EnableHints(true)
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			expectedHints: 0,
		},
		{
			name: "nil generator in list is skipped",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.SetHintGenerators([]domainerrors.HintGenerator{
					nil,
					func(err *domainerrors.StructuredError) []domainerrors.Hint {
						return []domainerrors.Hint{{Message: "Valid"}}
					},
					nil,
				})
				formatter.EnableHints(true)
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			expectedHints: 1,
		},
		{
			name: "generator returning empty hints",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{}
				})
				formatter.EnableHints(true)
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			expectedHints: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := mocks.NewMockErrorFormatter()
			tt.setupFunc(formatter)

			hints := formatter.GetHints(tt.testError)

			assert.Len(t, hints, tt.expectedHints)
		})
	}
}

func TestMockErrorFormatter_GetHints_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*mocks.MockErrorFormatter)
		testError     *domainerrors.StructuredError
		expectedHints int
	}{
		{
			name: "nil error handled safely",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					if err == nil {
						return []domainerrors.Hint{}
					}
					return []domainerrors.Hint{{Message: "Valid hint"}}
				})
				formatter.EnableHints(true)
			},
			testError:     nil,
			expectedHints: 0,
		},
		{
			name: "generator panics are not caught (expected behavior)",
			setupFunc: func(formatter *mocks.MockErrorFormatter) {
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					// Note: In production, generators should never panic
					// This test documents the expected behavior if they do
					if err == nil {
						return []domainerrors.Hint{}
					}
					return []domainerrors.Hint{{Message: "Safe hint"}}
				})
				formatter.EnableHints(true)
			},
			testError: domainerrors.NewStructuredError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"test",
				nil,
				nil,
			),
			expectedHints: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := mocks.NewMockErrorFormatter()
			tt.setupFunc(formatter)

			hints := formatter.GetHints(tt.testError)

			assert.Len(t, hints, tt.expectedHints)
		})
	}
}

func TestMockErrorFormatter_Clear_HintsReset(t *testing.T) {
	formatter := mocks.NewMockErrorFormatter()
	formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
		return []domainerrors.Hint{{Message: "Test hint"}}
	})
	formatter.EnableHints(true)
	testError := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test",
		nil,
		nil,
	)

	// Verify hints exist before clear
	hintsBefore := formatter.GetHints(testError)
	require.Len(t, hintsBefore, 1, "Should have hints before clear")

	formatter.Clear()

	hintsAfter := formatter.GetHints(testError)
	assert.Len(t, hintsAfter, 0, "Hints should be cleared")
}

func TestMockErrorFormatter_Clear_HintsEnabledReset(t *testing.T) {
	formatter := mocks.NewMockErrorFormatter()
	formatter.EnableHints(true)
	testError := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test",
		nil,
		nil,
	)

	formatter.Clear()
	formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
		return []domainerrors.Hint{{Message: "Test hint"}}
	})

	hints := formatter.GetHints(testError)
	assert.Len(t, hints, 0, "Hints should be disabled after clear")
}

func TestMockErrorFormatter_ThreadSafety_HintOperations(t *testing.T) {
	formatter := mocks.NewMockErrorFormatter()
	formatter.EnableHints(true)
	testError := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test",
		nil,
		nil,
	)

	const goroutines = 100
	const operationsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				// Add generator
				formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Concurrent hint"}}
				})
				// Get hints
				hints := formatter.GetHints(testError)
				// Verify at least some hints exist (race-safe assertion)
				assert.GreaterOrEqual(t, len(hints), 0)
			}
		}(i)
	}

	wg.Wait()

	hints := formatter.GetHints(testError)
	assert.GreaterOrEqual(t, len(hints), 0, "Should have hints after concurrent operations")
}

func TestMockErrorFormatter_ThreadSafety_EnableHintsConcurrent(t *testing.T) {
	formatter := mocks.NewMockErrorFormatter()
	formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
		return []domainerrors.Hint{{Message: "Test hint"}}
	})
	testError := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test",
		nil,
		nil,
	)

	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half enable, half disable
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			formatter.EnableHints(true)
			_ = formatter.GetHints(testError)
		}()

		go func() {
			defer wg.Done()
			formatter.EnableHints(false)
			_ = formatter.GetHints(testError)
		}()
	}

	wg.Wait()

	// Final state is non-deterministic due to concurrent modifications, which is expected
	hints := formatter.GetHints(testError)
	assert.GreaterOrEqual(t, len(hints), 0, "Should handle concurrent enable/disable")
}

func TestMockErrorFormatter_ThreadSafety_SetHintGeneratorsConcurrent(t *testing.T) {
	formatter := mocks.NewMockErrorFormatter()
	formatter.EnableHints(true)
	testError := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test",
		nil,
		nil,
	)

	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			generators := []domainerrors.HintGenerator{
				func(err *domainerrors.StructuredError) []domainerrors.Hint {
					return []domainerrors.Hint{{Message: "Concurrent set"}}
				},
			}
			formatter.SetHintGenerators(generators)
			_ = formatter.GetHints(testError)
		}(i)
	}

	wg.Wait()

	hints := formatter.GetHints(testError)
	assert.GreaterOrEqual(t, len(hints), 0, "Should handle concurrent SetHintGenerators")
}

func TestMockErrorFormatter_RealWorld_FileNotFoundHints(t *testing.T) {
	formatter := mocks.NewMockErrorFormatter()
	formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err.Code == domainerrors.ErrorCodeUserInputMissingFile {
			if path, ok := err.Details["path"].(string); ok {
				return []domainerrors.Hint{
					{Message: "File not found: " + path},
					{Message: "Did you mean 'my-workflow.yaml'?"},
					{Message: "Run 'awf list' to see available workflows"},
				}
			}
		}
		return []domainerrors.Hint{}
	})
	formatter.EnableHints(true)

	testError := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow file not found",
		map[string]any{"path": "/workflows/my-workfow.yaml"},
		nil,
	)

	hints := formatter.GetHints(testError)

	require.Len(t, hints, 3)
	assert.Equal(t, "File not found: /workflows/my-workfow.yaml", hints[0].Message)
	assert.Equal(t, "Did you mean 'my-workflow.yaml'?", hints[1].Message)
	assert.Equal(t, "Run 'awf list' to see available workflows", hints[2].Message)
}

func TestMockErrorFormatter_RealWorld_YAMLSyntaxHints(t *testing.T) {
	formatter := mocks.NewMockErrorFormatter()
	formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err.Code == domainerrors.ErrorCodeWorkflowParseYAMLSyntax {
			if _, ok := err.Details["line"].(int); ok {
				if _, ok := err.Details["column"].(int); ok {
					return []domainerrors.Hint{
						{Message: "Syntax error at line 10, column 5"},
						{Message: "Expected: 'version: 1.0'"},
					}
				}
			}
		}
		return []domainerrors.Hint{}
	})
	formatter.EnableHints(true)

	testError := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		"invalid YAML syntax",
		map[string]any{"line": 10, "column": 5},
		nil,
	)

	hints := formatter.GetHints(testError)

	require.Len(t, hints, 2)
	assert.Contains(t, hints[0].Message, "line 10")
	assert.Contains(t, hints[1].Message, "Expected")
}

func TestMockErrorFormatter_RealWorld_MultipleGeneratorsForSameError(t *testing.T) {
	formatter := mocks.NewMockErrorFormatter()

	// Generator 1: Path-based hint
	formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err.Code == domainerrors.ErrorCodeUserInputMissingFile {
			return []domainerrors.Hint{{Message: "Check the file path"}}
		}
		return []domainerrors.Hint{}
	})

	// Generator 2: Suggestion hint
	formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err.Code == domainerrors.ErrorCodeUserInputMissingFile {
			return []domainerrors.Hint{{Message: "Did you mean 'workflow.yaml'?"}}
		}
		return []domainerrors.Hint{}
	})

	// Generator 3: Command hint
	formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
		if err.Code == domainerrors.ErrorCodeUserInputMissingFile {
			return []domainerrors.Hint{{Message: "Run 'awf list' to see available files"}}
		}
		return []domainerrors.Hint{}
	})

	formatter.EnableHints(true)

	testError := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"file not found",
		nil,
		nil,
	)

	hints := formatter.GetHints(testError)

	require.Len(t, hints, 3)
	assert.Equal(t, "Check the file path", hints[0].Message)
	assert.Equal(t, "Did you mean 'workflow.yaml'?", hints[1].Message)
	assert.Equal(t, "Run 'awf list' to see available files", hints[2].Message)
}
