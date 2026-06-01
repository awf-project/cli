package application

import (
	"testing"

	"github.com/awf-project/cli/pkg/display"
)

func TestExecutionService_SetDisplayRendererFactory_StoresFactory(t *testing.T) {
	s := &ExecutionService{}
	called := false
	s.SetDisplayRendererFactory(func(stepID string) display.EventRenderer {
		called = true
		return nil
	})
	if s.displayRendererFactory == nil {
		t.Fatal("factory not stored")
	}
	_ = s.displayRendererFactory("step")
	if !called {
		t.Fatal("factory not invoked")
	}
}
