package ci

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPipelineExamplesArchiveRunnerOutput(t *testing.T) {
	for _, entry := range []struct{ file, scriptKey string }{{"gitlab/example.gitlab-ci.yml", "script"}, {"gitflic/gitflic-ci.yaml", "scripts"}} {
		data, err := os.ReadFile(entry.file)
		if err != nil {
			t.Fatal(err)
		}
		var config map[string]any
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Fatalf("%s: %v", entry.file, err)
		}
		job, ok := config["open-review"].(map[string]any)
		if !ok {
			t.Fatalf("%s has no review job", entry.file)
		}
		scripts, ok := job[entry.scriptKey].([]any)
		if !ok || len(scripts) == 0 {
			t.Fatalf("%s has no scripts", entry.file)
		}
		artifacts, ok := job["artifacts"].(map[string]any)
		if !ok || artifacts["when"] != "always" {
			t.Fatalf("%s does not always archive", entry.file)
		}
		if !strings.Contains(string(data), "integrations/ci/run.sh") || !strings.Contains(string(data), "open-review-artifacts/") || !strings.Contains(string(data), "@v1.6.0") {
			t.Fatalf("%s is not wired to runner output", entry.file)
		}
	}
	data, err := os.ReadFile("gerrit/Jenkinsfile")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"GERRIT_REFSPEC", "GERRIT_PATCHSET_REVISION", "integrations/ci/run.sh", "archiveArtifacts", "@v1.6.0"} {
		if !strings.Contains(string(data), required) {
			t.Fatalf("Gerrit example misses %s", required)
		}
	}
}
