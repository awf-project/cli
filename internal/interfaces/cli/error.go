package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/awf-project/awf/internal/domain/errors"
	"github.com/awf-project/awf/internal/interfaces/cli/ui"
	"github.com/spf13/cobra"
)

// newErrorCommand creates the error code lookup command.
// Usage: awf error <code>
// Example: awf error USER.INPUT.MISSING_FILE
func newErrorCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "error [code]",
		Short: "Look up error code documentation",
		Long: `Look up error code documentation and display detailed information.

Without arguments, lists all available error codes.
With a code argument, displays description, resolution guidance, and related codes.

Examples:
  awf error                              # List all error codes
  awf error USER.INPUT.MISSING_FILE      # Lookup specific error code
  awf error WORKFLOW.VALIDATION          # Prefix match (shows all matching codes)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runError(cmd, cfg, args)
		},
	}
}

// runError executes the error code lookup command.
func runError(cmd *cobra.Command, cfg *Config, args []string) error {
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// List all error codes if no argument provided
	if len(args) == 0 {
		return listAllErrorCodes(writer)
	}

	// Lookup specific code or prefix
	query := args[0]

	// Try exact match first
	if entry, found := errors.GetCatalogEntry(errors.ErrorCode(query)); found {
		return displayErrorEntry(writer, entry)
	}

	// Try prefix match
	matches := findPrefixMatches(query)
	if len(matches) > 0 {
		return displayMultipleEntries(writer, matches)
	}

	// No matches found
	fmt.Fprintf(cmd.ErrOrStderr(), "Error: error code not found: %s\n", query)
	return fmt.Errorf("error code not found: %s", query)
}

// listAllErrorCodes lists all error codes from the catalog.
func listAllErrorCodes(writer *ui.OutputWriter) error {
	codes := errors.AllErrorCodes()

	// Sort codes for consistent output
	sort.Slice(codes, func(i, j int) bool {
		return string(codes[i]) < string(codes[j])
	})

	entries := make([]errors.CatalogEntry, 0, len(codes))
	for _, code := range codes {
		if entry, found := errors.GetCatalogEntry(code); found {
			entries = append(entries, entry)
		}
	}

	return displayMultipleEntries(writer, entries)
}

// findPrefixMatches finds all error codes that start with the given prefix.
func findPrefixMatches(prefix string) []errors.CatalogEntry {
	var matches []errors.CatalogEntry

	for code, entry := range errors.ErrorCatalog {
		if strings.HasPrefix(string(code), prefix) {
			matches = append(matches, entry)
		}
	}

	// Sort matches for consistent output
	sort.Slice(matches, func(i, j int) bool {
		return string(matches[i].Code) < string(matches[j].Code)
	})

	return matches
}

// displayErrorEntry displays a single error catalog entry.
func displayErrorEntry(writer *ui.OutputWriter, entry errors.CatalogEntry) error {
	if writer.IsJSONFormat() {
		// JSON format
		data := map[string]interface{}{
			"code":        string(entry.Code),
			"description": entry.Description,
			"resolution":  entry.Resolution,
		}
		if len(entry.RelatedCodes) > 0 {
			relatedStrs := make([]string, len(entry.RelatedCodes))
			for i, code := range entry.RelatedCodes {
				relatedStrs[i] = string(code)
			}
			data["related_codes"] = relatedStrs
		}
		return writer.WriteJSON(data)
	}

	// Text format
	out := writer.Out()
	fmt.Fprintf(out, "Error Code: %s\n", entry.Code)
	fmt.Fprintf(out, "\nDescription:\n  %s\n", entry.Description)
	fmt.Fprintf(out, "\nResolution:\n  %s\n", entry.Resolution)

	if len(entry.RelatedCodes) > 0 {
		fmt.Fprintf(out, "\nRelated Codes:\n")
		for _, code := range entry.RelatedCodes {
			fmt.Fprintf(out, "  - %s\n", code)
		}
	}

	return nil
}

// displayMultipleEntries displays multiple error catalog entries.
func displayMultipleEntries(writer *ui.OutputWriter, entries []errors.CatalogEntry) error {
	if writer.IsJSONFormat() {
		// JSON format: array of entries
		data := make([]map[string]interface{}, len(entries))
		for i, entry := range entries {
			entryData := map[string]interface{}{
				"code":        string(entry.Code),
				"description": entry.Description,
				"resolution":  entry.Resolution,
			}
			if len(entry.RelatedCodes) > 0 {
				relatedStrs := make([]string, len(entry.RelatedCodes))
				for j, code := range entry.RelatedCodes {
					relatedStrs[j] = string(code)
				}
				entryData["related_codes"] = relatedStrs
			}
			data[i] = entryData
		}
		return writer.WriteJSON(data)
	}

	// Text format: list all codes with descriptions
	out := writer.Out()
	for i, entry := range entries {
		if i > 0 {
			fmt.Fprintln(out) // Blank line between entries
		}
		fmt.Fprintf(out, "%s\n", entry.Code)
		fmt.Fprintf(out, "  %s\n", entry.Description)
		fmt.Fprintf(out, "  Resolution: %s\n", entry.Resolution)
		if len(entry.RelatedCodes) > 0 {
			relatedStrs := make([]string, len(entry.RelatedCodes))
			for j, code := range entry.RelatedCodes {
				relatedStrs[j] = string(code)
			}
			fmt.Fprintf(out, "  Related: %s\n", strings.Join(relatedStrs, ", "))
		}
	}

	return nil
}
