package sarif

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/scale-sync/open-review/review/internal/model"
)

type report struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []run  `json:"runs"`
}
type run struct {
	Tool struct {
		Driver struct {
			Name           string `json:"name"`
			InformationURI string `json:"informationUri"`
			Rules          []rule `json:"rules"`
		} `json:"driver"`
	} `json:"tool"`
	Results     []result `json:"results"`
	Invocations []struct {
		ExecutionSuccessful bool           `json:"executionSuccessful"`
		Properties          map[string]any `json:"properties"`
	} `json:"invocations"`
}
type rule struct {
	ID               string  `json:"id"`
	ShortDescription message `json:"shortDescription"`
}
type message struct {
	Text string `json:"text"`
}
type result struct {
	RuleID              string            `json:"ruleId"`
	Level               string            `json:"level"`
	Message             message           `json:"message"`
	Locations           []location        `json:"locations"`
	PartialFingerprints map[string]string `json:"partialFingerprints"`
}
type location struct {
	PhysicalLocation struct {
		ArtifactLocation struct {
			URI string `json:"uri"`
		} `json:"artifactLocation"`
		Region struct {
			StartLine int `json:"startLine"`
			EndLine   int `json:"endLine,omitempty"`
		} `json:"region"`
	} `json:"physicalLocation"`
}

func Format(review model.Result) ([]byte, error) {
	document := report{Schema: "https://json.schemastore.org/sarif-2.1.0.json", Version: "2.1.0", Runs: []run{{Results: []result{}}}}
	current := &document.Runs[0]
	current.Tool.Driver.Name = "open-review"
	current.Tool.Driver.InformationURI = "https://github.com/scale-sync/open-review"
	current.Tool.Driver.Rules = []rule{}
	current.Invocations = []struct {
		ExecutionSuccessful bool           `json:"executionSuccessful"`
		Properties          map[string]any `json:"properties"`
	}{{ExecutionSuccessful: review.Status == "complete" || review.Status == "empty", Properties: map[string]any{"reviewStatus": review.Status, "coverage": review.Coverage}}}
	rules := map[string]bool{}
	for _, raw := range review.Findings {
		var finding model.Finding
		if err := json.Unmarshal(raw, &finding); err != nil {
			return nil, err
		}
		if !filepath.IsLocal(finding.File) || strings.Contains(finding.File, `\`) || finding.StartLine < 1 || finding.EndLine < finding.StartLine || finding.Category != "bug" && finding.Category != "flag" {
			return nil, fmt.Errorf("finding %q has invalid SARIF location", finding.ID)
		}
		id := "open-review/" + finding.Category
		if !rules[id] {
			current.Tool.Driver.Rules = append(current.Tool.Driver.Rules, rule{ID: id, ShortDescription: message{Text: finding.Category}})
			rules[id] = true
		}
		level := "warning"
		if finding.Severity == "severe" {
			level = "error"
		}
		if finding.Severity == "informational" {
			level = "note"
		}
		value := result{RuleID: id, Level: level, Message: message{Text: finding.Title + ". " + finding.Explanation}, Locations: []location{{}}, PartialFingerprints: map[string]string{}}
		value.Locations[0].PhysicalLocation.ArtifactLocation.URI = (&url.URL{Path: finding.File}).EscapedPath()
		value.Locations[0].PhysicalLocation.Region.StartLine = finding.StartLine
		value.Locations[0].PhysicalLocation.Region.EndLine = finding.EndLine
		// shortcut: same-title findings in one file share a key; add code context if duplicates become common.
		fingerprint := sha256.Sum256([]byte(finding.File + "\x00" + finding.Category + "\x00" + strings.ToLower(strings.Join(strings.Fields(finding.Title), " "))))
		value.PartialFingerprints["openReview/v1"] = hex.EncodeToString(fingerprint[:])
		current.Results = append(current.Results, value)
	}
	return json.MarshalIndent(document, "", "  ")
}
