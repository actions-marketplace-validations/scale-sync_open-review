package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSARIFConvertsSavedReviewWithoutModelCall(t *testing.T) {
	input := filepath.Join(t.TempDir(), "review.json")
	if err := os.WriteFile(input, []byte(`{"status":"complete","findings":[{"id":"F1","category":"bug","severity":"severe","file":"src/a.go","startLine":3,"endLine":3,"title":"Missing check","explanation":"nil input"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runSARIF([]string{"--input", input}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"version": "2.1.0"`) || !strings.Contains(output.String(), `"partialFingerprints"`) {
		t.Fatalf("incomplete SARIF: %s", output.String())
	}
}
