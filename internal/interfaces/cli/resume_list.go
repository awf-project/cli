package cli

import (
	"github.com/spf13/cobra"
)

func newResumeListCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "resume-list",
		Short: "List resumable workflows",
		Long: `List all workflows that can be resumed.

Shows workflows that are not yet completed, displaying their current
status, progress, and when they were last updated.

Examples:
  awf resume-list
  awf resume-list --output=json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runResumeList(cmd, cfg)
		},
	}
}
