package repository

// Source indicates where a workflow was discovered
type Source int

const (
	SourceEnv    Source = iota // AWF_WORKFLOWS_PATH environment variable
	SourceLocal                // ./.awf/workflows/
	SourceGlobal               // $XDG_CONFIG_HOME/awf/workflows/
)

func (s Source) String() string {
	switch s {
	case SourceEnv:
		return "env"
	case SourceLocal:
		return "local"
	case SourceGlobal:
		return "global"
	default:
		return "unknown"
	}
}

// WorkflowInfo contains workflow metadata including its source
type WorkflowInfo struct {
	Name   string
	Source Source
	Path   string
}
