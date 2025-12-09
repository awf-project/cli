package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckMigration(t *testing.T) {
	// Reset the flag for testing
	migrationNoticeMu.Lock()
	migrationNoticeShown = false
	migrationNoticeMu.Unlock()

	var buf bytes.Buffer
	CheckMigration(&buf)

	// Output depends on whether ~/.awf exists
	// Just verify the function runs without error
	// If ~/.awf exists, should contain migration notice
	// If not, should be empty
	_ = buf.String()
}

func TestCheckMigration_OnlyShowsOnce(t *testing.T) {
	// Reset the flag for testing
	migrationNoticeMu.Lock()
	migrationNoticeShown = false
	migrationNoticeMu.Unlock()

	var buf1, buf2 bytes.Buffer

	CheckMigration(&buf1)
	CheckMigration(&buf2)

	// Second call should produce no output
	assert.Empty(t, buf2.String())
}
