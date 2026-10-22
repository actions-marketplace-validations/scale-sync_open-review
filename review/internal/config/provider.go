package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/scale-sync/open-review/review/internal/model"
)

type Provider struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	BaseURL  string `json:"baseUrl,omitempty"`
	APIKey   string `json:"apiKey,omitempty"`
}

func Path() (string, error) {
	home := os.Getenv("OPEN_REVIEW_HOME")
	if home == "" {
		home = os.Getenv("OPENREVIEW_HOME")
	}
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = filepath.Join(userHome, ".config", "open-review")
	}
	return filepath.Join(home, "config.json"), nil
}

func Load() (Provider, error) {
	path, err := Path()
	if err != nil {
		return Provider{}, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Provider{}, nil
	}
	if err != nil {
		return Provider{}, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return Provider{}, errors.New("provider config must be a private regular file (mode 0600)")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Provider{}, err
	}
	var saved Provider
	if err := json.Unmarshal(raw, &saved); err != nil {
		return Provider{}, errors.New("invalid provider config JSON")
	}
	if err := Validate(saved); err != nil {
		return Provider{}, err
	}
	return saved, nil
}

func Save(saved Provider) error {
	if err := Validate(saved); err != nil {
		return err
	}
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if err := json.NewEncoder(temp).Encode(saved); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func Validate(saved Provider) error {
	if saved.Provider != "" && saved.Provider != "openai" && saved.Provider != "anthropic" && saved.Provider != "gemini" && saved.Provider != "openrouter" && saved.Provider != "azure" && saved.Provider != "bedrock" {
		return fmt.Errorf("unsupported provider %q", saved.Provider)
	}
	if strings.TrimSpace(saved.Model) != saved.Model || strings.ContainsAny(saved.Model, "\r\n") {
		return errors.New("invalid model name")
	}
	if saved.BaseURL != "" {
		u, err := url.Parse(saved.BaseURL)
		if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
			return errors.New("base URL must use HTTPS or local HTTP without credentials")
		}
	}
	return nil
}

func Resolve(override model.ModelIdentity) (Provider, error) {
	saved, err := Load()
	if err != nil {
		return Provider{}, err
	}
	name := override.Name
	if name == "" && override.Provider == "" {
		name = os.Getenv("MAIN_MODEL")
	}
	provider := first(override.Provider, os.Getenv("OPEN_REVIEW_PROVIDER"))
	if provider == "" && override.Name != "" {
		provider = infer(override.Name)
	}
	provider = first(provider, saved.Provider)
	if name == "" && saved.Provider == provider {
		name = saved.Model
	}
	if provider == "" {
		provider = infer(name)
	}
	if name == "" {
		switch provider {
		case "anthropic":
			name = "claude-sonnet-4-5"
		case "gemini":
			name = "gemini-2.5-flash"
		case "openrouter":
			name = "openrouter/free"
		case "azure":
			name = "gpt-4o"
		case "bedrock":
			name = "anthropic.claude-3-5-sonnet-20241022-v2:0"
		default:
			name = "gpt-4o"
		}
	}
	baseEnv := map[string]string{
		"openai":     "OPENAI_BASE_URL",
		"anthropic":  "ANTHROPIC_BASE_URL",
		"gemini":     "GEMINI_BASE_URL",
		"openrouter": "OPENROUTER_BASE_URL",
		"azure":      "AZURE_OPENAI_BASE_URL",
		"bedrock":    "AWS_BEDROCK_BASE_URL",
	}[provider]
	keyEnv := map[string]string{
		"openai":     "OPENAI_API_KEY",
		"anthropic":  "ANTHROPIC_API_KEY",
		"gemini":     "GEMINI_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
		"azure":      "AZURE_OPENAI_API_KEY",
		"bedrock":    "AWS_BEDROCK_API_KEY",
	}[provider]

	resolvedBase := os.Getenv(baseEnv)
	if provider == "azure" && resolvedBase == "" {
		resolvedBase = os.Getenv("AZURE_OPENAI_ENDPOINT")
	}
	if provider == "bedrock" && resolvedBase == "" {
		resolvedBase = os.Getenv("BEDROCK_BASE_URL")
	}
	defaultBaseURL := ""
	if provider == "openrouter" {
		defaultBaseURL = "https://openrouter.ai/api/v1"
	}
	resolved := Provider{Provider: provider, Model: name, BaseURL: first(override.BaseURL, resolvedBase, defaultBaseURL)}

	// OPEN_REVIEW_API_KEY is the provider-agnostic BYOK escape hatch. Provider-
	// specific variables still win when explicitly set for existing workflows.
	resolvedKey := first(os.Getenv(keyEnv), os.Getenv("OPEN_REVIEW_API_KEY"))
	if provider == "azure" && resolvedKey == "" {
		resolvedKey = os.Getenv("AZURE_API_KEY")
	}
	if provider == "bedrock" && resolvedKey == "" {
		resolvedKey = os.Getenv("BEDROCK_API_KEY")
	}

	if saved.Provider == provider {
		resolved.BaseURL = first(resolved.BaseURL, saved.BaseURL)
		resolved.APIKey = first(resolvedKey, saved.APIKey)
	} else {
		resolved.APIKey = resolvedKey
	}
	if err := Validate(resolved); err != nil {
		return Provider{}, err
	}
	if resolved.APIKey == "" {
		return Provider{}, fmt.Errorf("%s API key is required", provider)
	}
	return resolved, nil
}

func infer(name string) string {
	switch {
	case strings.HasPrefix(name, "claude-"):
		return "anthropic"
	case strings.HasPrefix(name, "gemini-"):
		return "gemini"
	case strings.HasPrefix(name, "openrouter/"):
		return "openrouter"
	case strings.HasPrefix(name, "azure/") || strings.Contains(name, "azure"):
		return "azure"
	case strings.HasPrefix(name, "bedrock/") || strings.Contains(name, "anthropic.claude"):
		return "bedrock"
	default:
		return "openai"
	}
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
