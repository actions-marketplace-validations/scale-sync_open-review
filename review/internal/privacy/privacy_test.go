package privacy

import (
	"strings"
	"testing"
)

func TestRedactSensitiveKeys(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Authorization: Bearer sk-proj-123456789012345678901234", "Authorization: Bearer [REDACTED]"},
		{"token is ghp_123456789012345678901234", "token is [REDACTED]"},
		{"aws: AKIAIOSFODNN7EXAMPLE", "aws: [REDACTED]"},
		{"slack: xoxb-1234567890-1234567890123-abcdef", "slack: [REDACTED]"},
		{"api_key='secret123456789012'", "api_key='[REDACTED]'"},
		{"safe clean text without secrets", "safe clean text without secrets"},
	}
	for _, tc := range cases {
		got := Redact(tc.input)
		if got != tc.want {
			t.Errorf("Redact(%q) = %q, want %q", tc.input, got, tc.want)
		}
		if strings.Contains(got, "sk-proj") || strings.Contains(got, "ghp_") || strings.Contains(got, "AKIA") {
			t.Errorf("Secret leaked in redacted string: %s", got)
		}
	}
}

func TestIsSecretPath(t *testing.T) {
	secrets := []string{
		".env",
		".env.local",
		".env.production",
		"config/server.key",
		"certs/cert.pem",
		"id_rsa",
		"id_ed25519",
		"aws_credentials.json",
	}
	for _, s := range secrets {
		if !IsSecretPath(s) {
			t.Errorf("IsSecretPath(%q) = false, want true", s)
		}
	}
	safe := []string{
		".env.example",
		"main.go",
		"README.md",
		"package.json",
	}
	for _, s := range safe {
		if IsSecretPath(s) {
			t.Errorf("IsSecretPath(%q) = true, want false", s)
		}
	}
}
