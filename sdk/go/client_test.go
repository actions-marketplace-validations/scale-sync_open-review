package openreview

import (
	"context"
	"testing"
)

func TestScanEmptyDirectory(t *testing.T) {
	result, err := Scan(context.Background(), t.TempDir(), ".", ModelIdentity{}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if result.ContractVersion != ContractVersion || result.Status != "empty" {
		t.Fatalf("unexpected result: version=%d status=%s", result.ContractVersion, result.Status)
	}
}
