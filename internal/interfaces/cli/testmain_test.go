package cli_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain redirects default audit persistence before any CLI test can construct
// a command. It owns and removes only the temporary XDG data root it creates.
func TestMain(m *testing.M) {
	flag.Parse()
	verifyAudit := isUnfilteredTestRun()

	dataHome, err := os.MkdirTemp("", "awf-cli-test-xdg-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create test XDG data root: %v\n", err)
		os.Exit(1)
	}

	previousDataHome, wasSet := os.LookupEnv("XDG_DATA_HOME")
	if err := os.Setenv("XDG_DATA_HOME", dataHome); err != nil {
		fmt.Fprintf(os.Stderr, "set test XDG data root: %v\n", err)
		_ = os.RemoveAll(dataHome)
		os.Exit(1)
	}

	code := m.Run()
	verificationErr := error(nil)
	if verifyAudit {
		verificationErr = verifyRedirectedAudit(dataHome)
	}

	if wasSet {
		err = os.Setenv("XDG_DATA_HOME", previousDataHome)
	} else {
		err = os.Unsetenv("XDG_DATA_HOME")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "restore XDG_DATA_HOME: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	if err := os.RemoveAll(dataHome); err != nil {
		fmt.Fprintf(os.Stderr, "remove test XDG data root: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	if verificationErr != nil {
		fmt.Fprintf(os.Stderr, "redirected audit verification: %v\n", verificationErr)
		code = 1
	}

	os.Exit(code)
}

func isUnfilteredTestRun() bool {
	return flagValue("test.list") == "" && flagValue("test.run") == "" && flagValue("test.skip") == ""
}

func flagValue(name string) string {
	if testFlag := flag.Lookup(name); testFlag != nil {
		return testFlag.Value.String()
	}
	return ""
}

func verifyRedirectedAudit(dataHome string) error {
	auditPath := filepath.Join(dataHome, "awf", "audit.jsonl")
	info, err := os.Stat(auditPath) //nolint:gosec // G703: dataHome is the temporary root owned by TestMain, not user input.
	if err != nil {
		return fmt.Errorf("stat %s: %w", auditPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", auditPath)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%s is empty", auditPath)
	}

	contents, err := os.ReadFile(auditPath) //nolint:gosec // G703: only the audit file below TestMain's owned temporary root is read.
	if err != nil {
		return fmt.Errorf("read %s: %w", auditPath, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(contents)), "\n") {
		if !json.Valid([]byte(line)) {
			return fmt.Errorf("%s contains invalid JSONL", auditPath)
		}
	}

	fmt.Printf("B019 redirected audit: %s (%d bytes)\n", filepath.Join("awf", "audit.jsonl"), info.Size())
	return nil
}
