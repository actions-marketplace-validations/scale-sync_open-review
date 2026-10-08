package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSandboxInvocationScrubsSecretsAndLimitsPermissions(t *testing.T) {
	directory := t.TempDir()
	deno := filepath.Join(directory, "deno")
	code := "#!/bin/sh\ncase \"$*\" in *--deny-net*--deny-env*--deny-run*--deny-write*--allow-read=*) ;; *) exit 3;; esac\nprintf '%s' \"${OPENAI_API_KEY-unset}\"\n"
	if err := os.WriteFile(deno, []byte(code), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("OPENAI_API_KEY", "private-test-key")
	result, err := Run(context.Background(), "console.log(GLOBALS.diff)", "+safe", "Test")
	if err != nil || result.Status != "complete" || result.Output != "unset" {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestSandboxTimeout(t *testing.T) {
	directory := t.TempDir()
	deno := filepath.Join(directory, "deno")
	if err := os.WriteFile(deno, []byte("#!/bin/sh\nexec /bin/sleep 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	result, err := Run(ctx, "while (true) {}", "patch", "Test")
	if err != nil || result.Status != "timeout" {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestDenoDeniesExternalIO(t *testing.T) {
	if !Available() {
		t.Skip("Deno is not installed")
	}
	directory := t.TempDir()
	secret := filepath.Join(directory, "secret")
	if err := os.WriteFile(secret, []byte("private-test-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	code := `for (const check of [
  { name: "read", run: () => Deno.readTextFile(` + "`" + secret + "`" + `) },
  { name: "env", run: () => Deno.env.get("OPENAI_API_KEY") },
  { name: "net", run: () => fetch("https://example.com") },
  { name: "run", run: () => new Deno.Command("/bin/echo").output() },
  { name: "write", run: () => Deno.writeTextFile("outside", "bad") },
]) { try { await check.run(); console.log(check.name + ":allowed"); } catch { console.log(check.name + ":denied"); } }`
	result, err := Run(context.Background(), code, "patch", "Test")
	if err != nil || result.Status != "complete" {
		t.Fatalf("result = %+v, %v", result, err)
	}
	for _, operation := range []string{"read", "env", "net", "run", "write"} {
		if !strings.Contains(result.Output, operation+":denied") {
			t.Fatalf("%s not denied: %s", operation, result.Output)
		}
	}
}
