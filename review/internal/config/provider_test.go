package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scale-sync/open-review/review/internal/model"
)

func TestProviderConfigPrivateAndOverride(t *testing.T) {
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-env-key")
	t.Setenv("MAIN_MODEL", "")
	if err := Save(Provider{Provider: "openai", Model: "gpt-4o", BaseURL: "http://localhost:1234/v1", APIKey: "private-test-key"}); err != nil {
		t.Fatal(err)
	}
	path, _ := Path()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %v, %v", info.Mode().Perm(), err)
	}
	saved, err := Load()
	if err != nil || saved.APIKey != "private-test-key" {
		t.Fatalf("saved config invalid: %v", err)
	}
	openai, err := Resolve(model.ModelIdentity{})
	if err != nil || openai.Model != "gpt-4o" || openai.APIKey != "private-test-key" {
		t.Fatalf("default = %+v, %v", openai, err)
	}
	other, err := Resolve(model.ModelIdentity{Name: "claude-test"})
	if err != nil || other.Provider != "anthropic" || other.Model != "claude-test" || other.APIKey != "anthropic-env-key" {
		t.Fatalf("override = %+v, %v", other, err)
	}
	t.Setenv("MAIN_MODEL", "gpt-4o")
	providerOnly, err := Resolve(model.ModelIdentity{Provider: "anthropic"})
	if err != nil || providerOnly.Provider != "anthropic" || !strings.HasPrefix(providerOnly.Model, "claude-") {
		t.Fatalf("provider-only override = %+v, %v", providerOnly, err)
	}
}

func TestProviderConfigRejectsMalformedAndUnsafeFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OPEN_REVIEW_HOME", home)
	path := filepath.Join(home, "config.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "invalid provider config") {
		t.Fatalf("error = %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"provider":"openai"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("error = %v", err)
	}
	if err := Validate(Provider{Provider: "openai", BaseURL: "http://external.example/v1"}); err == nil {
		t.Fatal("remote HTTP endpoint accepted")
	}
}

func TestOpenRouterUsesDedicatedEnvironmentAndSafeDefault(t *testing.T) {
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "gateway-test-key")
	resolved, err := Resolve(model.ModelIdentity{Provider: "openrouter"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Model != "openrouter/free" || resolved.BaseURL != "https://openrouter.ai/api/v1" || resolved.APIKey != "gateway-test-key" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestAzureAndBedrockProviders(t *testing.T) {
	t.Setenv("OPEN_REVIEW_HOME", t.TempDir())
	t.Setenv("AZURE_OPENAI_API_KEY", "azure-key-123")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://test-resource.openai.azure.com")
	azure, err := Resolve(model.ModelIdentity{Provider: "azure"})
	if err != nil {
		t.Fatal(err)
	}
	if azure.Provider != "azure" || azure.Model != "gpt-4o" || azure.BaseURL != "https://test-resource.openai.azure.com" || azure.APIKey != "azure-key-123" {
		t.Fatalf("azure = %+v", azure)
	}

	t.Setenv("AWS_BEDROCK_API_KEY", "bedrock-key-456")
	t.Setenv("BEDROCK_BASE_URL", "https://bedrock-runtime.us-east-1.amazonaws.com")
	bedrock, err := Resolve(model.ModelIdentity{Provider: "bedrock"})
	if err != nil {
		t.Fatal(err)
	}
	if bedrock.Provider != "bedrock" || bedrock.BaseURL != "https://bedrock-runtime.us-east-1.amazonaws.com" || bedrock.APIKey != "bedrock-key-456" {
		t.Fatalf("bedrock = %+v", bedrock)
	}
}
