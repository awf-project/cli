package cli

import (
	"fmt"
	"io"
	"sync"

	"github.com/vanoix/awf/internal/infrastructure/xdg"
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

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "NOTICE: Legacy ~/.awf directory detected")
	fmt.Fprintln(w, "AWF now uses XDG Base Directory Specification:")
	fmt.Fprintf(w, "  Config:    %s\n", xdg.AWFConfigDir())
	fmt.Fprintf(w, "  Data:      %s\n", xdg.AWFDataDir())
	fmt.Fprintf(w, "  Workflows: %s\n", xdg.AWFWorkflowsDir())
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "To migrate, move your files:")
	fmt.Fprintf(w, "  mv ~/.awf/workflows/* %s/\n", xdg.AWFWorkflowsDir())
	fmt.Fprintf(w, "  mv ~/.awf/storage/states/* %s/\n", xdg.AWFStatesDir())
	fmt.Fprintln(w, "")
}
