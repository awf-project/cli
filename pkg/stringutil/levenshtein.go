// Package stringutil provides utility functions for string manipulation and comparison.
package stringutil

// LevenshteinDistance calculates the Levenshtein distance between two strings.
// The Levenshtein distance is the minimum number of single-character edits
// (insertions, deletions, or substitutions) required to change one string into another.
//
// Returns the edit distance as an integer. Returns 0 if strings are identical.
func LevenshteinDistance(s1, s2 string) int {
	// Convert strings to rune slices to handle unicode correctly
	r1 := []rune(s1)
	r2 := []rune(s2)
	len1 := len(r1)
	len2 := len(r2)

	// Handle edge cases
	if len1 == 0 {
		return len2
	}
	if len2 == 0 {
		return len1
	}

	// Create a 2D matrix for dynamic programming
	// We use len1+1 rows and len2+1 columns
	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}

	// Initialize first column (deletions from s1)
	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}

	// Initialize first row (insertions to s1)
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}

	// Fill the matrix
	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}

			// Take minimum of:
			// 1. Deletion: matrix[i-1][j] + 1
			// 2. Insertion: matrix[i][j-1] + 1
			// 3. Substitution: matrix[i-1][j-1] + cost
			deletion := matrix[i-1][j] + 1
			insertion := matrix[i][j-1] + 1
			substitution := matrix[i-1][j-1] + cost

			matrix[i][j] = min(deletion, min(insertion, substitution))
		}
	}

	return matrix[len1][len2]
}

// ClosestMatch finds the closest matching string from candidates based on Levenshtein distance.
// It returns the candidate with the smallest edit distance to target that is below the threshold.
//
// Parameters:
//   - target: The string to compare against
//   - candidates: A slice of candidate strings to compare
//   - threshold: Maximum allowed edit distance (inclusive). Use -1 for no threshold.
//
// Returns:
//   - match: The closest matching candidate string, or empty string if no match below threshold
//   - distance: The Levenshtein distance to the matched candidate, or -1 if no match
//
// If multiple candidates have the same minimal distance, returns the first one found.
func ClosestMatch(target string, candidates []string, threshold int) (match string, distance int) {
	// Handle empty candidates
	if len(candidates) == 0 {
		return "", -1
	}

	bestMatch := ""
	bestDistance := -1
	hasThreshold := threshold != -1

	for _, candidate := range candidates {
		dist := LevenshteinDistance(target, candidate)

		// If we have a threshold and distance exceeds it, skip this candidate
		if hasThreshold && dist > threshold {
			continue
		}

		// Update if this is first valid match or better than current best
		if bestDistance == -1 || dist < bestDistance {
			bestMatch = candidate
			bestDistance = dist
		}
	}

	// If we found a match, return it
	if bestDistance != -1 {
		return bestMatch, bestDistance
	}

	// No match found within threshold
	return "", -1
}
