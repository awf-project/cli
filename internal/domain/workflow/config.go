package workflow

// OutputLimits configures output capture limits and streaming behavior.
// C019: Prevents OOM from unbounded StepState.Output/Stderr growth.
type OutputLimits struct {
	MaxSize           int64  // Max bytes per output field (0 = unlimited)
	StreamLargeOutput bool   // If true, stream to file; if false, truncate
	TempDir           string // Directory for temp files
}

// LoopMemoryConfig configures loop memory retention behavior.
// C019: Prevents OOM from unbounded LoopResult.Iterations accumulation.
type LoopMemoryConfig struct {
	MaxRetainedIterations int // 0 = keep all (default for backward compatibility)
}

// DefaultOutputLimits returns the default output configuration.
// Maintains backward compatibility with no limits.
func DefaultOutputLimits() OutputLimits {
	return OutputLimits{
		MaxSize:           0,     // unlimited by default
		StreamLargeOutput: false, // truncate by default
		TempDir:           "",    // use system temp dir
	}
}

// DefaultLoopMemoryConfig returns the default loop memory configuration.
// Maintains backward compatibility with unlimited iteration retention.
func DefaultLoopMemoryConfig() LoopMemoryConfig {
	return LoopMemoryConfig{
		MaxRetainedIterations: 0, // unlimited by default
	}
}
