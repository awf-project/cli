package cli

import (
	"fmt"
	"io"
	"sync"

	"github.com/awf-project/awf/internal/infrastructure/xdg"
)

var (
	migrationNoticeShown bool
	migrationNoticeMu    sync.Mutex
)

// CheckMigration shows a one-time migration notice if ~/.awf exists
func CheckMigration(w io.Writer) {
	migrationNoticeMu.Lock()
	defer migrationNoticeMu.Unlock()

	if migrationNoticeShown {
		return
	}

	if !xdg.LegacyDirExists() {
		return
	}

	migrationNoticeShown = true

	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "NOTICE: Legacy ~/.awf directory detected")
	_, _ = fmt.Fprintln(w, "AWF now uses XDG Base Directory Specification:")
	_, _ = fmt.Fprintf(w, "  Config:    %s\n", xdg.AWFConfigDir())
	_, _ = fmt.Fprintf(w, "  Data:      %s\n", xdg.AWFDataDir())
	_, _ = fmt.Fprintf(w, "  Workflows: %s\n", xdg.AWFWorkflowsDir())
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "To migrate, move your files:")
	_, _ = fmt.Fprintf(w, "  mv ~/.awf/workflows/* %s/\n", xdg.AWFWorkflowsDir())
	_, _ = fmt.Fprintf(w, "  mv ~/.awf/storage/states/* %s/\n", xdg.AWFStatesDir())
	_, _ = fmt.Fprintln(w, "")
}
