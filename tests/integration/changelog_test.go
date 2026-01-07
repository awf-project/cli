package integration

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Item: T012
// Feature: F047
// Tests comprehensive validation of CHANGELOG.md entry for F047 bug fix

// TestChangelog_F047Entry_HappyPath validates complete F047 documentation
func TestChangelog_F047Entry_HappyPath(t *testing.T) {
	// Arrange: Read CHANGELOG.md
	changelogPath := "../../CHANGELOG.md"
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("Failed to read CHANGELOG.md: %v", err)
	}

	changelog := string(content)

	// Act & Assert: Verify F047 entry exists
	if !strings.Contains(changelog, "**F047**") {
		t.Error("CHANGELOG.md missing F047 entry in Fixed section")
	}

	// Assert: Entry must be complete (no TODO markers)
	if strings.Contains(changelog, "[TODO:") {
		t.Error("CHANGELOG.md F047 entry is incomplete (contains TODO marker)")
	}

	// Assert: Entry must describe the JSON serialization fix
	requiredKeywords := []string{
		"loop",
		"JSON",
		"serialize",
	}

	for _, keyword := range requiredKeywords {
		if !strings.Contains(strings.ToLower(changelog), strings.ToLower(keyword)) {
			t.Errorf("CHANGELOG.md F047 entry missing required keyword: %q", keyword)
		}
	}
}

// TestChangelog_F047Entry_InFixedSection validates F047 is in correct section
func TestChangelog_F047Entry_InFixedSection(t *testing.T) {
	// Arrange: Read CHANGELOG.md
	changelogPath := "../../CHANGELOG.md"
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("Failed to read CHANGELOG.md: %v", err)
	}

	changelog := string(content)

	// Act: Find Fixed section and F047 entry
	fixedSectionRegex := regexp.MustCompile(`(?s)### Fixed\s+(.*?)\s+###`)
	matches := fixedSectionRegex.FindStringSubmatch(changelog)

	// Assert: Fixed section exists
	if len(matches) < 2 {
		t.Fatal("CHANGELOG.md missing '### Fixed' section")
	}

	fixedSection := matches[1]

	// Assert: F047 is in Fixed section
	if !strings.Contains(fixedSection, "**F047**") {
		t.Error("F047 entry not found in '### Fixed' section (should be under bug fixes)")
	}
}

// TestChangelog_F047Entry_FollowsFormat validates entry follows Keep a Changelog format
func TestChangelog_F047Entry_FollowsFormat(t *testing.T) {
	// Arrange: Read CHANGELOG.md
	changelogPath := "../../CHANGELOG.md"
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("Failed to read CHANGELOG.md: %v", err)
	}

	changelog := string(content)

	// Act: Extract F047 entry
	f047Regex := regexp.MustCompile(`- \*\*F047\*\*:[^\n]+`)
	matches := f047Regex.FindString(changelog)

	// Assert: Entry exists with proper format
	if matches == "" {
		t.Fatal("F047 entry not found or does not follow format: - **F047**: description")
	}

	// Assert: Entry has description (not just label)
	if strings.TrimSpace(matches) == "- **F047**:" {
		t.Error("F047 entry missing description after label")
	}
}

// TestChangelog_F047Entry_HasBulletPoints validates sub-bullets exist
func TestChangelog_F047Entry_HasBulletPoints(t *testing.T) {
	// Arrange: Read CHANGELOG.md
	changelogPath := "../../CHANGELOG.md"
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("Failed to read CHANGELOG.md: %v", err)
	}

	changelog := string(content)

	// Act: Find F047 entry and following lines
	lines := strings.Split(changelog, "\n")
	f047Index := -1
	for i, line := range lines {
		if strings.Contains(line, "**F047**") {
			f047Index = i
			break
		}
	}

	// Assert: F047 entry found
	if f047Index == -1 {
		t.Fatal("F047 entry not found in CHANGELOG.md")
	}

	// Assert: Next line(s) should have indented bullet points with details
	hasSubBullets := false
	for i := f047Index + 1; i < len(lines) && i < f047Index+5; i++ {
		line := lines[i]
		// Check for indented bullet (2 spaces + dash)
		if strings.HasPrefix(line, "  -") || strings.HasPrefix(line, "  *") {
			hasSubBullets = true
			break
		}
		// Stop at next main bullet or section
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "##") {
			break
		}
	}

	if !hasSubBullets {
		t.Error("F047 entry should have sub-bullets with implementation details (following pattern of Bug-48 and JSONStore entries)")
	}
}

// TestChangelog_F047Entry_MentionsTemplateInterpolation validates technical context
func TestChangelog_F047Entry_MentionsTemplateInterpolation(t *testing.T) {
	// Arrange: Read CHANGELOG.md
	changelogPath := "../../CHANGELOG.md"
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("Failed to read CHANGELOG.md: %v", err)
	}

	changelog := string(content)

	// Act & Assert: Verify technical terms related to the fix
	technicalTerms := []string{
		"template", // Template interpolation is the fix location
		"{{",       // Template syntax is relevant
	}

	// At least one technical term should be present
	foundTechnicalTerm := false
	for _, term := range technicalTerms {
		if strings.Contains(strings.ToLower(changelog), strings.ToLower(term)) {
			foundTechnicalTerm = true
			break
		}
	}

	if !foundTechnicalTerm {
		t.Error("F047 entry should mention template-related context (e.g., 'template', '{{.loop.Item}}')")
	}
}

// TestChangelog_F047Entry_NoTypos validates common typos are absent
func TestChangelog_F047Entry_NoTypos(t *testing.T) {
	// Arrange: Read CHANGELOG.md
	changelogPath := "../../CHANGELOG.md"
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("Failed to read CHANGELOG.md: %v", err)
	}

	changelog := string(content)

	// Act: Find F047 section
	lines := strings.Split(changelog, "\n")
	f047Index := -1
	var f047Section strings.Builder

	for i, line := range lines {
		if strings.Contains(line, "**F047**") {
			f047Index = i
		}
		if f047Index >= 0 && i >= f047Index {
			f047Section.WriteString(line + "\n")
			// Stop at next main bullet or section
			if i > f047Index && (strings.HasPrefix(line, "- **") || strings.HasPrefix(line, "##")) {
				break
			}
		}
	}

	f047Text := f047Section.String()

	// Assert: Check for common typos
	commonTypos := map[string]string{
		"seralize":  "serialize",
		"seralized": "serialized",
		"templat ":  "template",
		"fo_each":   "for_each",
		"forech":    "for_each",
		"jsn":       "JSON",
		"go map":    "Go map", // lowercase 'go' when referring to language
	}

	for typo, correct := range commonTypos {
		if strings.Contains(strings.ToLower(f047Text), strings.ToLower(typo)) {
			t.Errorf("F047 entry contains potential typo %q (should be %q)", typo, correct)
		}
	}
}

// TestChangelog_F047Entry_EdgeCase_NotDuplicated validates no duplicate entries
func TestChangelog_F047Entry_EdgeCase_NotDuplicated(t *testing.T) {
	// Arrange: Read CHANGELOG.md
	changelogPath := "../../CHANGELOG.md"
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("Failed to read CHANGELOG.md: %v", err)
	}

	changelog := string(content)

	// Act: Count F047 occurrences
	count := strings.Count(changelog, "**F047**")

	// Assert: Should appear exactly once
	if count == 0 {
		t.Error("F047 entry not found in CHANGELOG.md")
	} else if count > 1 {
		t.Errorf("F047 entry appears %d times (should appear exactly once)", count)
	}
}

// TestChangelog_F047Entry_EdgeCase_NotInWrongSection validates F047 not in Added/Changed
func TestChangelog_F047Entry_EdgeCase_NotInWrongSection(t *testing.T) {
	// Arrange: Read CHANGELOG.md
	changelogPath := "../../CHANGELOG.md"
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("Failed to read CHANGELOG.md: %v", err)
	}

	changelog := string(content)

	// Act: Check Added and Changed sections don't contain F047
	addedSectionRegex := regexp.MustCompile(`(?s)### Added\s+(.*?)\s+###`)
	changedSectionRegex := regexp.MustCompile(`(?s)### Changed\s+(.*?)\s+###`)

	addedMatches := addedSectionRegex.FindStringSubmatch(changelog)
	changedMatches := changedSectionRegex.FindStringSubmatch(changelog)

	// Assert: F047 should NOT be in Added section
	if len(addedMatches) >= 2 && strings.Contains(addedMatches[1], "**F047**") {
		t.Error("F047 is a bug fix and should not be in '### Added' section")
	}

	// Assert: F047 should NOT be in Changed section
	if len(changedMatches) >= 2 && strings.Contains(changedMatches[1], "**F047**") {
		t.Error("F047 is a bug fix and should not be in '### Changed' section")
	}
}

// TestChangelog_F047Entry_ErrorHandling_MissingFile validates graceful handling
func TestChangelog_F047Entry_ErrorHandling_MissingFile(t *testing.T) {
	// Arrange: Use non-existent path
	changelogPath := "../../NONEXISTENT_CHANGELOG.md"

	// Act: Try to read
	_, err := os.ReadFile(changelogPath)

	// Assert: Should get error (this test validates our test setup)
	if err == nil {
		t.Error("Expected error when reading non-existent file, got nil")
	}
}

// TestChangelog_UnreleasedSection_Exists validates Unreleased section exists
func TestChangelog_UnreleasedSection_Exists(t *testing.T) {
	// Arrange: Read CHANGELOG.md
	changelogPath := "../../CHANGELOG.md"
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("Failed to read CHANGELOG.md: %v", err)
	}

	changelog := string(content)

	// Act & Assert: Unreleased section must exist
	if !strings.Contains(changelog, "## [Unreleased]") {
		t.Error("CHANGELOG.md missing '## [Unreleased]' section (required by Keep a Changelog format)")
	}

	// Assert: Fixed section must exist under Unreleased
	unreleasedRegex := regexp.MustCompile(`(?sm)## \[Unreleased\](.*?)(?:^## |\z)`)
	matches := unreleasedRegex.FindStringSubmatch(changelog)

	if len(matches) < 2 {
		t.Fatal("Could not extract Unreleased section content")
	}

	unreleasedContent := matches[1]

	if !strings.Contains(unreleasedContent, "### Fixed") {
		t.Error("Unreleased section missing '### Fixed' subsection")
	}
}
