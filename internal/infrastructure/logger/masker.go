package logger

import "strings"

// SecretMasker masks sensitive values in log fields.
type SecretMasker struct {
	patterns []string
}

// DefaultSecretPatterns contains the default patterns to detect secrets.
var DefaultSecretPatterns = []string{
	"SECRET_",
	"API_KEY",
	"PASSWORD",
}

// NewSecretMasker creates a masker with default patterns plus any additional ones.
func NewSecretMasker(additionalPatterns ...string) *SecretMasker {
	patterns := make([]string, 0, len(DefaultSecretPatterns)+len(additionalPatterns))
	patterns = append(patterns, DefaultSecretPatterns...)
	patterns = append(patterns, additionalPatterns...)
	return &SecretMasker{patterns: patterns}
}

// IsSecretKey checks if a key matches any secret pattern.
func (m *SecretMasker) IsSecretKey(key string) bool {
	upperKey := strings.ToUpper(key)
	for _, pattern := range m.patterns {
		if strings.HasPrefix(upperKey, pattern) {
			return true
		}
	}
	return false
}

// MaskFields replaces values of secret keys with "***".
// Fields are expected as key-value pairs: key1, val1, key2, val2...
func (m *SecretMasker) MaskFields(fields []any) []any {
	if len(fields) == 0 {
		return fields
	}

	result := make([]any, len(fields))
	copy(result, fields)

	for i := 0; i+1 < len(result); i += 2 {
		key, ok := result[i].(string)
		if !ok {
			continue
		}
		if m.IsSecretKey(key) {
			result[i+1] = "***"
		}
	}

	return result
}

// MaskText replaces secret values in text output with "***".
func (m *SecretMasker) MaskText(text string, env map[string]string) string {
	if len(env) == 0 || text == "" {
		return text
	}

	result := text

	// Collect secret values from env vars with secret keys
	// Sort by length descending to handle overlapping values correctly
	// (e.g., "abc" and "abcdef" - replace longer one first)
	type secretValue struct {
		value  string
		length int
	}
	var secrets []secretValue

	for key, value := range env {
		// Skip empty values
		if value == "" {
			continue
		}
		// Only process keys that match secret patterns
		if m.IsSecretKey(key) {
			secrets = append(secrets, secretValue{value: value, length: len(value)})
		}
	}

	// Sort by length descending (longer values first to handle overlaps)
	for i := 0; i < len(secrets); i++ {
		for j := i + 1; j < len(secrets); j++ {
			if secrets[j].length > secrets[i].length {
				secrets[i], secrets[j] = secrets[j], secrets[i]
			}
		}
	}

	// Replace each secret value with ***
	for _, secret := range secrets {
		// Use strings.ReplaceAll with the exact value
		// This handles special regex characters correctly
		result = strings.ReplaceAll(result, secret.value, "***")
	}

	return result
}
