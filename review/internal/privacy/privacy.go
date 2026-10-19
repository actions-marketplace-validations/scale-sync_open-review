package privacy

import (
	"regexp"
	"strings"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b(sk-[a-zA-Z0-9_-]{20,})\b`),
	regexp.MustCompile(`\b(ghp_[a-zA-Z0-9]{20,})\b`),
	regexp.MustCompile(`\b(ghs_[a-zA-Z0-9]{20,})\b`),
	regexp.MustCompile(`\b(AKIA[0-9A-Z]{16})\b`),
	regexp.MustCompile(`\b(xox[baprs]-[0-9a-zA-Z-]{10,})\b`),
	regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*['"]?)([a-zA-Z0-9_\-]{16,})(['"]?)`),
}

// Redact replaces recognized API tokens and secrets with [REDACTED].
func Redact(text string) string {
	for _, p := range secretPatterns[:5] {
		text = p.ReplaceAllString(text, "[REDACTED]")
	}
	text = secretPatterns[5].ReplaceAllString(text, "${1}[REDACTED]${3}")
	return text
}

// IsSecretPath reports whether a file path or name is a known secret or credential file.
func IsSecretPath(path string) bool {
	parts := strings.Split(strings.ToLower(path), "/")
	base := parts[len(parts)-1]
	if base == ".env" || (strings.HasPrefix(base, ".env.") && base != ".env.example") {
		return true
	}
	for _, ext := range []string{".pem", ".key", ".pkcs12", ".p12", ".pfx"} {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}
	if base == "id_rsa" || base == "id_ed25519" || base == "id_ecdsa" || base == "id_dsa" {
		return true
	}
	if strings.Contains(base, "credential") || strings.Contains(base, "id_rsa") {
		return true
	}
	return false
}
