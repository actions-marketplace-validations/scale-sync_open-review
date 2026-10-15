package viewer

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scale-sync/open-review/review/internal/model"
	"github.com/scale-sync/open-review/review/internal/session"
)

func TestViewerEscapesResultsAndScopesSessions(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-token")
	repo := t.TempDir()
	dir := t.TempDir()
	req := model.Request{ContractVersion: model.ContractVersion, Mode: "branch", RepoRoot: repo, HeadSHA: strings.Repeat("a", 40), Model: model.ModelIdentity{Provider: "openai", Name: "test-model"}}
	first, err := session.Open(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Finish(model.Result{Status: "complete", Findings: []json.RawMessage{json.RawMessage(`{"id":"A","title":"<script>alert(1)</script>"}`)}}); err != nil {
		t.Fatal(err)
	}
	second, err := session.Open(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Finish(model.Result{Status: "partial", Findings: []json.RawMessage{json.RawMessage(`{"id":"B"}`)}}); err != nil {
		t.Fatal(err)
	}
	handler := Handler(dir, repo)
	for _, check := range []struct{ path, want string }{
		{"/", "partial"},
		{"/sessions/" + first.ID(), "\\u003cscript\\u003e"},
		{"/sessions/" + first.ID() + "/export", `"status":"complete"`},
		{"/compare?left=" + first.ID() + "&right=" + second.ID(), "New findings"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", check.path, nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), check.want) {
			t.Fatalf("%s: %d %s", check.path, response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "<script>") {
			t.Fatal("unescaped stored content")
		}
	}
	for _, path := range []string{"/sessions/../../etc/passwd", "/sessions/" + first.ID() + "/wrong"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 404 && response.Code/100 != 3 {
			t.Fatalf("%s: %d", path, response.Code)
		}
	}
	foreign := httptest.NewRecorder()
	Handler(dir, t.TempDir()).ServeHTTP(foreign, httptest.NewRequest("GET", "/sessions/"+first.ID(), nil))
	if foreign.Code != 404 {
		t.Fatalf("foreign repository read: %d", foreign.Code)
	}
}
