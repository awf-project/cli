package application

// markExecutionStarted records that the goroutine running execution id is alive.
func (a *Adapter) markExecutionStarted(id string) {
	a.execMu.Lock()
	defer a.execMu.Unlock()
	if a.runningExecutions == nil {
		a.runningExecutions = make(map[string]struct{})
	}
	a.runningExecutions[id] = struct{}{}
}

// markExecutionFinished records that the goroutine running execution id returned.
func (a *Adapter) markExecutionFinished(id string) {
	a.execMu.Lock()
	defer a.execMu.Unlock()
	delete(a.runningExecutions, id)
}

// PendingExecutions returns the IDs of the executions whose goroutine has not
// returned yet. A finished execution is absent whether it succeeded, failed or
// was cancelled. `awf serve` polls it at shutdown to find the runs a restart cut
// short, because a session leaves the registry only on explicit Close and a
// cancelled session closes before its execution finishes.
func (a *Adapter) PendingExecutions() []string {
	a.execMu.Lock()
	defer a.execMu.Unlock()
	ids := make([]string, 0, len(a.runningExecutions))
	for id := range a.runningExecutions {
		ids = append(ids, id)
	}
	return ids
}
