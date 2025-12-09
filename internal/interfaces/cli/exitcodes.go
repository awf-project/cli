package cli

// Exit codes following the error taxonomy
const (
	ExitSuccess   = 0 // Success
	ExitUser      = 1 // User error (bad input, missing file)
	ExitWorkflow  = 2 // Workflow error (invalid state reference, cycle)
	ExitExecution = 3 // Execution error (command failed, timeout)
	ExitSystem    = 4 // System error (IO, permissions)
)
