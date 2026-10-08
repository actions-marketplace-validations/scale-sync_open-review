package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const maxOutput = 8_000

type Observation struct {
	Status string
	Output string
}

type limitedOutput struct{ bytes.Buffer }

func (out *limitedOutput) Write(input []byte) (int, error) {
	n := len(input)
	if out.Len() < maxOutput {
		_, _ = out.Buffer.Write(input[:min(n, maxOutput-out.Len())])
	}
	return n, nil
}

func Available() bool {
	_, err := exec.LookPath("deno")
	return err == nil
}

func Run(parent context.Context, code, patch, title string) (Observation, error) {
	if len(code) > 12_000 || len(patch) > 800_000 {
		return Observation{}, errors.New("sandbox input exceeds budget")
	}
	deno, err := exec.LookPath("deno")
	if err != nil {
		return Observation{Status: "unavailable"}, nil
	}
	directory, err := os.MkdirTemp("", "open-review-sandbox-")
	if err != nil {
		return Observation{}, err
	}
	defer os.RemoveAll(directory)
	globals, err := json.Marshal(map[string]string{"diff": patch, "title": title})
	if err != nil {
		return Observation{}, err
	}
	script := filepath.Join(directory, "check.ts")
	if err := os.WriteFile(script, []byte("const GLOBALS = "+string(globals)+";\n"+code), 0o600); err != nil {
		return Observation{}, err
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, deno, "run", "--no-config", "--no-lock", "--no-remote", "--no-npm", "--no-prompt", "--deny-net", "--deny-env", "--deny-run", "--deny-write", "--deny-import", "--deny-ffi", "--deny-sys", "--allow-read="+script, script)
	cmd.Dir = directory
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + directory, "DENO_DIR=" + directory, "NO_COLOR=1"}
	var stdout, stderr limitedOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	if ctx.Err() != nil {
		return Observation{Status: "timeout", Output: "Verification timed out"}, nil
	}
	output := strings.TrimSpace(stdout.String())
	if err != nil {
		return Observation{Status: "failed", Output: strings.TrimSpace(stderr.String())}, nil
	}
	return Observation{Status: "complete", Output: output}, nil
}
