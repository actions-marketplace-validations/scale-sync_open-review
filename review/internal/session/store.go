package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/scale-sync/open-review/review/internal/config"
	"github.com/scale-sync/open-review/review/internal/model"
)

type Event struct {
	Version int       `json:"version"`
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Status  string    `json:"status,omitempty"`
	Path    string    `json:"path,omitempty"`
	Group   int       `json:"group,omitempty"`
	Count   int       `json:"count,omitempty"`
}

type Summary struct {
	ID          string    `json:"id"`
	Fingerprint string    `json:"fingerprint"`
	Repo        string    `json:"repo"`
	Started     time.Time `json:"started"`
	Status      string    `json:"status"`
	Events      int       `json:"events"`
}

type Store struct {
	mu       sync.Mutex
	dir      string
	id       string
	finger   string
	repo     string
	reusable bool
	prior    []string
}

type contextKey struct{}

func WithStore(ctx context.Context, store *Store) context.Context {
	return context.WithValue(ctx, contextKey{}, store)
}

func FromContext(ctx context.Context) *Store {
	store, _ := ctx.Value(contextKey{}).(*Store)
	return store
}

type cacheRecord struct {
	Fingerprint string          `json:"fingerprint"`
	Group       string          `json:"group"`
	Value       json.RawMessage `json:"value"`
}

func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "open-review", "sessions"), nil
}

func Fingerprint(req model.Request) (string, error) {
	selected, err := config.Resolve(req.Model)
	if err != nil {
		return "", err
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		return "", err
	}
	build := sha256.Sum256(binary)
	data, err := json.Marshal(struct {
		Request  model.Request `json:"request"`
		Provider string        `json:"provider"`
		Model    string        `json:"model"`
		BaseURL  string        `json:"baseUrl"`
		Runtime  string        `json:"runtime"`
		Build    string        `json:"build"`
	}{req, selected.Provider, selected.Model, selected.BaseURL, runtime.Version(), hex.EncodeToString(build[:])})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func Open(dir string, req model.Request) (*Store, error) {
	if dir == "" {
		return nil, errors.New("session directory is required")
	}
	repo, err := canonicalRepo(req.RepoRoot)
	if err != nil {
		return nil, err
	}
	req.RepoRoot = repo
	finger, err := Fingerprint(req)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	prior := []string{}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), finger[:16]+"-") {
			prior = append(prior, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(prior)))
	path, err := os.MkdirTemp(dir, finger[:16]+"-")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return nil, err
	}
	store := &Store{dir: path, id: filepath.Base(path), finger: finger, repo: req.RepoRoot, reusable: req.Mode == "scan" || req.Mode != "workspace" && (req.HeadSHA != "" || req.CommitSHA != ""), prior: prior}
	if err := store.write("meta.json", Summary{ID: store.id, Fingerprint: finger, Repo: req.RepoRoot, Started: time.Now().UTC(), Status: "running"}); err != nil {
		return nil, err
	}
	if err := store.Record(Event{Type: "review", Status: "started"}); err != nil {
		return nil, err
	}
	return store, nil
}

func canonicalRepo(repo string) (string, error) {
	if repo == "" {
		return "", errors.New("repository path is required")
	}
	absolute, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

func (s *Store) ID() string { return s.id }

func (s *Store) Record(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	event.Version = 1
	event.Time = time.Now().UTC()
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(s.dir, "events.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func (s *Store) LoadGroup(index int, group string, target any) (bool, error) {
	if !s.reusable {
		return false, nil
	}
	name := fmt.Sprintf("group-%d.json", index)
	for _, dir := range s.prior {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		var record cacheRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return false, fmt.Errorf("malformed saved group: %w", err)
		}
		if record.Fingerprint != s.finger || record.Group != group {
			continue
		}
		if err := json.Unmarshal(record.Value, target); err != nil {
			return false, fmt.Errorf("malformed saved findings: %w", err)
		}
		return true, s.Record(Event{Type: "group", Status: "reused", Group: index})
	}
	return false, nil
}

func (s *Store) LoadPlan() (string, bool, error) {
	if !s.reusable {
		return "", false, nil
	}
	for _, dir := range s.prior {
		data, err := os.ReadFile(filepath.Join(dir, "plan.json"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", false, err
		}
		var record cacheRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return "", false, fmt.Errorf("malformed saved plan: %w", err)
		}
		if record.Fingerprint != s.finger || record.Group != "plan" {
			continue
		}
		var focus string
		if err := json.Unmarshal(record.Value, &focus); err != nil {
			return "", false, fmt.Errorf("malformed saved focus: %w", err)
		}
		return focus, true, s.Record(Event{Type: "plan", Status: "reused"})
	}
	return "", false, nil
}

func (s *Store) SavePlan(focus string) error {
	data, err := json.Marshal(focus)
	if err != nil {
		return err
	}
	if err := s.write("plan.json", cacheRecord{Fingerprint: s.finger, Group: "plan", Value: data}); err != nil {
		return err
	}
	return s.Record(Event{Type: "plan", Status: "complete"})
}

func (s *Store) SaveGroup(index int, group string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := s.write(fmt.Sprintf("group-%d.json", index), cacheRecord{Fingerprint: s.finger, Group: group, Value: data}); err != nil {
		return err
	}
	return s.Record(Event{Type: "group", Status: "complete", Group: index})
}

func (s *Store) Finish(result model.Result) error {
	if err := s.write("result.json", struct {
		Status   string                `json:"status"`
		Coverage []model.CoverageEntry `json:"coverage"`
		Findings []json.RawMessage     `json:"findings"`
		Summary  json.RawMessage       `json:"summary,omitempty"`
		Trace    json.RawMessage       `json:"trace,omitempty"`
		Audit    []model.AuditEvent    `json:"audit,omitempty"`
		Usage    model.Usage           `json:"usage"`
	}{result.Status, result.Coverage, result.Findings, result.Summary, result.Trace, result.Audit, result.Usage}); err != nil {
		return err
	}
	if err := s.Record(Event{Type: "review", Status: result.Status, Count: len(result.Findings)}); err != nil {
		return err
	}
	return s.write("meta.json", Summary{ID: s.id, Fingerprint: s.finger, Repo: s.repo, Started: s.started(), Status: result.Status})
}

func (s *Store) started() time.Time {
	var summary Summary
	data, err := os.ReadFile(filepath.Join(s.dir, "meta.json"))
	if err == nil && json.Unmarshal(data, &summary) == nil {
		return summary.Started
	}
	return time.Now().UTC()
}

func (s *Store) write(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(s.dir, ".pending-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(s.dir, name))
}
