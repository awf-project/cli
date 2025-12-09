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
