package viewer

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"github.com/scale-sync/open-review/review/internal/session"
)

//go:embed templates/page.html static/style.css
var assets embed.FS

var page = template.Must(template.ParseFS(assets, "templates/page.html"))

type viewData struct {
	Title    string
	Repo     string
	Sessions []session.Summary
	Record   *session.Record
	Result   string
	Triage   []triageEntry
	Compare  *comparison
}

type triageEntry struct{ ID, State string }
type comparison struct {
	Left, Right    string
	Added, Removed []string
}

func Handler(dir, repo string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := assets.ReadFile("static/style.css")
		if err != nil {
			http.Error(w, "asset unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/" {
			items, err := session.List(dir, repo)
			if err != nil {
				http.Error(w, "sessions unavailable", http.StatusInternalServerError)
				return
			}
			render(w, viewData{Title: "Review sessions", Repo: repo, Sessions: items})
			return
		}
		if r.URL.Path == "/compare" {
			left, err := session.Read(dir, r.URL.Query().Get("left"), repo)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			right, err := session.Read(dir, r.URL.Query().Get("right"), repo)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			if len(left.Result) == 0 || len(right.Result) == 0 {
				http.Error(w, "both sessions need results", http.StatusBadRequest)
				return
			}
			before, err := findingIDs(left.Result)
			if err != nil {
				http.Error(w, "invalid session result", http.StatusInternalServerError)
				return
			}
			after, err := findingIDs(right.Result)
			if err != nil {
				http.Error(w, "invalid session result", http.StatusInternalServerError)
				return
			}
			added, removed := []string{}, []string{}
			for id := range after {
				if !before[id] {
					added = append(added, id)
				}
			}
			for id := range before {
				if !after[id] {
					removed = append(removed, id)
				}
			}
			sort.Strings(added)
			sort.Strings(removed)
			render(w, viewData{Title: "Compare sessions", Repo: repo, Compare: &comparison{left.Summary.ID, right.Summary.ID, added, removed}})
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 2 || len(parts) > 3 || parts[0] != "sessions" {
			http.NotFound(w, r)
			return
		}
		record, err := session.Read(dir, parts[1], repo)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if len(parts) == 3 {
			if parts[2] != "export" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Content-Disposition", "attachment; filename=review-session.json")
			_ = json.NewEncoder(w).Encode(record)
			return
		}
		var pretty []byte
		if len(record.Result) != 0 {
			var output bytes.Buffer
			if err := json.Indent(&output, record.Result, "", "  "); err == nil {
				pretty = output.Bytes()
			}
		}
		triage := make([]triageEntry, 0, len(record.Triage))
		for id, state := range record.Triage {
			triage = append(triage, triageEntry{id, state})
		}
		sort.Slice(triage, func(i, j int) bool { return triage[i].ID < triage[j].ID })
		render(w, viewData{Title: "Session " + record.Summary.ID, Repo: repo, Record: &record, Result: string(pretty), Triage: triage})
	})
	return mux
}

func findingIDs(raw json.RawMessage) (map[string]bool, error) {
	var result struct {
		Findings []struct {
			ID string `json:"id"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, finding := range result.Findings {
		if finding.ID != "" {
			ids[finding.ID] = true
		}
	}
	return ids, nil
}

func render(w http.ResponseWriter, data viewData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Execute(w, data); err != nil {
		http.Error(w, "page unavailable", http.StatusInternalServerError)
	}
}
