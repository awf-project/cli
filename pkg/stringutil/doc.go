// Package stringutil provides string manipulation and comparison utilities.
//
// This package implements string utility functions used across AWF for
// fuzzy matching, typo detection, and suggestion generation. Currently
// provides Levenshtein distance calculation for finding similar strings.
//
// Key features:
//   - Levenshtein distance calculation for edit distance measurement
//   - Closest match finding with configurable threshold
//   - Unicode-aware string comparison
//
// # Core Functions
//
// ## LevenshteinDistance (levenshtein.go)
//
// Calculate the minimum edit distance between two strings:
//   - Insertions, deletions, and substitutions each count as 1 edit
//   - Returns 0 for identical strings
//   - Unicode-safe (operates on runes, not bytes)
//   - Dynamic programming algorithm with O(m*n) time complexity
//
// ## ClosestMatch (levenshtein.go)
//
// Find the closest matching string from candidates:
//   - Returns candidate with smallest edit distance to target
//   - Configurable threshold for maximum allowed distance
//   - Returns first match if multiple candidates have same distance
//   - Returns empty string if no match below threshold
//
// # Usage Examples
//
// ## Basic Distance Calculation
//
//	distance := stringutil.LevenshteinDistance("kitten", "sitting")
//	// distance: 3 (substitute k->s, substitute e->i, insert g)
//
//	distance = stringutil.LevenshteinDistance("hello", "hello")
//	// distance: 0 (identical)
//
//	distance = stringutil.LevenshteinDistance("abc", "xyz")
//	// distance: 3 (substitute all three characters)
//
// ## Unicode Support
//
//	distance := stringutil.LevenshteinDistance("café", "cafe")
//	// distance: 1 (é -> e substitution)
//
//	distance = stringutil.LevenshteinDistance("日本", "日本語")
//	// distance: 1 (insert 語)
//
// ## Typo Detection
//
//	target := "parallel"
//	candidates := []string{"parallel", "serial", "sequential"}
//
//	match, distance := stringutil.ClosestMatch(target, candidates, 3)
//	// match: "parallel", distance: 1 (single typo)
//
// ## Command Suggestion
//
// Suggest correct command when user misspells:
//
//	userInput := "stauts"
//	commands := []string{"status", "start", "stop", "stats"}
//
//	suggestion, dist := stringutil.ClosestMatch(userInput, commands, 2)
//	if suggestion != "" {
//	    fmt.Printf("Unknown command '%s'. Did you mean '%s'?\n", userInput, suggestion)
//	}
//	// Output: Unknown command 'stauts'. Did you mean 'status'?
//
// ## Step Name Suggestion
//
// Suggest valid step names when workflow references unknown step:
//
//	invalidStep := "process_data"
//	validSteps := []string{"process_data", "fetch_data", "transform_data"}
//
//	suggestion, _ := stringutil.ClosestMatch(invalidStep, validSteps, 3)
//	if suggestion != "" {
//	    return fmt.Errorf("step '%s' not found, did you mean '%s'?", invalidStep, suggestion)
//	}
//
// ## Threshold Examples
//
//	candidates := []string{"apple", "banana", "cherry"}
//
//	// Strict threshold (max 2 edits)
//	match, dist := stringutil.ClosestMatch("aple", candidates, 2)
//	// match: "apple", dist: 1
//
//	// Lenient threshold (max 5 edits)
//	match, dist = stringutil.ClosestMatch("aple", candidates, 5)
//	// match: "apple", dist: 1 (same result, threshold not exceeded)
//
//	// No threshold (always return closest)
//	match, dist = stringutil.ClosestMatch("xyz", candidates, -1)
//	// match: "apple", dist: 4 (closest even though distance is large)
//
//	// Threshold too strict (no matches)
//	match, dist = stringutil.ClosestMatch("xyz", candidates, 2)
//	// match: "", dist: -1 (no candidate within threshold)
//
// ## Empty Input Handling
//
//	// Empty target
//	distance := stringutil.LevenshteinDistance("", "hello")
//	// distance: 5 (insert all 5 characters)
//
//	// Empty candidate
//	distance = stringutil.LevenshteinDistance("hello", "")
//	// distance: 5 (delete all 5 characters)
//
//	// Both empty
//	distance = stringutil.LevenshteinDistance("", "")
//	// distance: 0 (identical)
//
//	// Empty candidates list
//	match, dist := stringutil.ClosestMatch("target", []string{}, 3)
//	// match: "", dist: -1 (no candidates)
//
// # AWF Integration
//
// ## Unknown Step Name Errors
//
// When workflow validation detects unknown step reference:
//
//	// In workflow validation
//	if !stepExists(stepName) {
//	    validSteps := getAllStepNames()
//	    suggestion, dist := stringutil.ClosestMatch(stepName, validSteps, 3)
//	    if suggestion != "" && dist <= 2 {
//	        return fmt.Errorf("step '%s' not found. Did you mean '%s'?", stepName, suggestion)
//	    }
//	    return fmt.Errorf("step '%s' not found", stepName)
//	}
//
// ## Unknown Input Parameter Errors
//
// When user provides invalid input parameter:
//
//	invalidInput := "cofig_file"
//	validInputs := []string{"config_file", "output_dir", "log_level"}
//
//	suggestion, _ := stringutil.ClosestMatch(invalidInput, validInputs, 2)
//	if suggestion != "" {
//	    fmt.Fprintf(os.Stderr, "Unknown input '%s'. Did you mean '%s'?\n", invalidInput, suggestion)
//	}
//
// ## Unknown Operation Errors
//
// When workflow references non-existent plugin operation:
//
//	invalidOp := "slak.send"
//	availableOps := []string{"slack.send", "slack.notify", "email.send"}
//
//	suggestion, _ := stringutil.ClosestMatch(invalidOp, availableOps, 3)
//	if suggestion != "" {
//	    return fmt.Errorf("operation '%s' not found. Did you mean '%s'?", invalidOp, suggestion)
//	}
//
// # Performance Considerations
//
// ## Time Complexity
//
// LevenshteinDistance: O(m * n) where m and n are string lengths
//   - Uses dynamic programming with 2D matrix
//   - Memory: O(m * n) for matrix storage
//
// ## Optimization Opportunities
//
// For very long strings or many candidates:
//   - Early termination if distance exceeds threshold
//   - Use approximation algorithms (e.g., trigram matching)
//   - Cache distance calculations for repeated comparisons
//
// # Algorithm Details
//
// ## Levenshtein Distance Algorithm
//
// Dynamic programming approach:
//
//  1. Create (m+1) × (n+1) matrix
//  2. Initialize first row: [0, 1, 2, ..., n] (insertions)
//  3. Initialize first column: [0, 1, 2, ..., m] (deletions)
//  4. For each cell (i, j):
//     - If characters match: matrix[i][j] = matrix[i-1][j-1]
//     - Otherwise: matrix[i][j] = 1 + min(
//     matrix[i-1][j],   # deletion
//     matrix[i][j-1],   # insertion
//     matrix[i-1][j-1]  # substitution
//     )
//  5. Return matrix[m][n]
//
// ## Example Matrix
//
// Distance between "kitten" and "sitting":
//
//	      ""  s  i  t  t  i  n  g
//	""     0  1  2  3  4  5  6  7
//	k      1  1  2  3  4  5  6  7
//	i      2  2  1  2  3  4  5  6
//	t      3  3  2  1  2  3  4  5
//	t      4  4  3  2  1  2  3  4
//	e      5  5  4  3  2  2  3  4
//	n      6  6  5  4  3  3  2  3
//
// Result: 3 (bottom-right cell)
//
// # Design Principles
//
// ## Public API Surface
//
// This is a public package (pkg/) for external consumers:
//   - Stable API with semantic versioning
//   - No internal/ package dependencies
//   - Generic string utilities, not AWF-specific
//
// ## Unicode Correctness
//
// Operates on runes, not bytes:
//   - Correctly handles multi-byte UTF-8 characters
//   - Single edit for emoji insertion/deletion
//   - Works with non-ASCII languages (Chinese, Arabic, etc.)
//
// ## Simple API
//
// Two core functions with clear contracts:
//   - LevenshteinDistance: Pure distance calculation
//   - ClosestMatch: Convenience wrapper with threshold
//
// # Future Extensions
//
// Potential additions for enhanced string utilities:
//
//   - Damerau-Levenshtein distance (includes transpositions)
//   - Jaro-Winkler distance (optimized for short strings)
//   - Soundex/Metaphone (phonetic matching)
//   - Trigram similarity (faster approximation)
//   - Case-insensitive variants
//   - Configurable edit costs (e.g., insertion costs more than substitution)
//
// # Related Documentation
//
// See also:
//   - internal/domain/workflow: Workflow validation using suggestions
//   - internal/interfaces/cli: CLI command suggestions
//   - Wikipedia: Levenshtein distance algorithm
package stringutil
