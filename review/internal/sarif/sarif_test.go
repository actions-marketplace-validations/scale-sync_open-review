package sarif

import (
	"encoding/json"
	"testing"

	"github.com/scale-sync/open-review/review/internal/model"
)

func TestFormatHasStableFingerprintAndValidLocation(t *testing.T) {
	finding := model.Finding{ID: "F1", Category: "bug", Severity: "severe", File: "src/a b.go", StartLine: 8, EndLine: 9, Title: "Missing check", Explanation: "Input can be nil"}
	raw, _ := json.Marshal(finding)
	review := model.Result{Status: "complete", Findings: []json.RawMessage{raw}}
	data, err := Format(review)
	if err != nil {
		t.Fatal(err)
	}
	var first report
	if err := json.Unmarshal(data, &first); err != nil {
		t.Fatal(err)
	}
	if first.Schema == "" || first.Version != "2.1.0" || len(first.Runs) != 1 || len(first.Runs[0].Tool.Driver.Rules) != 1 || len(first.Runs[0].Results) != 1 {
		t.Fatalf("incomplete SARIF: %s", data)
	}
	entry := first.Runs[0].Results[0]
	if entry.Level != "error" || entry.Locations[0].PhysicalLocation.ArtifactLocation.URI != "src/a%20b.go" || entry.Locations[0].PhysicalLocation.Region.StartLine != 8 {
		t.Fatalf("unexpected SARIF finding: %+v", entry)
	}
	finding.StartLine, finding.EndLine = 18, 19
	raw, _ = json.Marshal(finding)
	review.Findings = []json.RawMessage{raw}
	secondData, err := Format(review)
	if err != nil {
		t.Fatal(err)
	}
	var second report
	if err := json.Unmarshal(secondData, &second); err != nil {
		t.Fatal(err)
	}
	if entry.PartialFingerprints["openReview/v1"] != second.Runs[0].Results[0].PartialFingerprints["openReview/v1"] {
		t.Fatal("line drift changed fingerprint")
	}
	review.Status = "partial"
	data, err = Format(review)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &second); err != nil {
		t.Fatal(err)
	}
	if second.Runs[0].Invocations[0].ExecutionSuccessful {
		t.Fatal("partial review reported successful analysis")
	}
}
