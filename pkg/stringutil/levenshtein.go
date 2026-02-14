// Package stringutil provides utility functions for string manipulation and comparison.
package stringutil

// LevenshteinDistance calculates the Levenshtein distance between two strings:
// the minimum number of single-character edits (insertions, deletions, or substitutions)
// required to change one string into another. Returns 0 if strings are identical.
func LevenshteinDistance(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)
	len1 := len(r1)
	len2 := len(r2)

	if len1 == 0 {
		return len2
	}
	if len2 == 0 {
		return len1
	}

	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}

	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}

	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}

			deletion := matrix[i-1][j] + 1
			insertion := matrix[i][j-1] + 1
			substitution := matrix[i-1][j-1] + cost

			matrix[i][j] = min(deletion, min(insertion, substitution))
		}
	}

	return matrix[len1][len2]
}

// ClosestMatch finds the closest matching string from candidates based on Levenshtein distance.
// Returns the candidate with the smallest edit distance to target that is below the threshold.
// Use threshold -1 for no threshold. Returns first match if multiple candidates have same minimal distance.
func ClosestMatch(target string, candidates []string, threshold int) (match string, distance int) {
	if len(candidates) == 0 {
		return "", -1
	}

	bestMatch := ""
	bestDistance := -1
	hasThreshold := threshold != -1

	for _, candidate := range candidates {
		dist := LevenshteinDistance(target, candidate)

		if hasThreshold && dist > threshold {
			continue
		}

		if bestDistance == -1 || dist < bestDistance {
			bestMatch = candidate
			bestDistance = dist
		}
	}

	if bestDistance != -1 {
		return bestMatch, bestDistance
	}

	return "", -1
}
