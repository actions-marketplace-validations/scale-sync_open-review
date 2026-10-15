package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

type Record struct {
	Summary Summary           `json:"summary"`
	Events  []Event           `json:"events"`
	Result  json.RawMessage   `json:"result,omitempty"`
	Triage  map[string]string `json:"triage,omitempty"`
}

var validID = regexp.MustCompile(`^[a-f0-9]{16}-[A-Za-z0-9]+$`)

func List(dir, repo string) ([]Summary, error) {
	if repo != "" {
		var err error
		repo, err = canonicalRepo(repo)
		if err != nil {
			return nil, err
		}
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Summary{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := []Summary{}
	for _, entry := range entries {
		if !entry.IsDir() || !validID.MatchString(entry.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name(), "meta.json"))
		if err != nil {
			return nil, err
		}
		var item Summary
		if err := json.Unmarshal(data, &item); err != nil || item.ID != entry.Name() {
			return nil, fmt.Errorf("malformed session %s", entry.Name())
		}
		if repo == "" || item.Repo == repo {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Started.After(items[j].Started) })
	return items, nil
}

func Read(dir, id, repo string) (Record, error) {
	path, summary, err := ownedPath(dir, id, repo)
	if err != nil {
		return Record{}, err
	}
	data, err := os.ReadFile(filepath.Join(path, "events.jsonl"))
	if err != nil {
		return Record{}, err
	}
	result := Record{Summary: summary, Events: []Event{}}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil || event.Version != 1 || event.Type == "" {
			return Record{}, fmt.Errorf("malformed session event in %s", id)
		}
		result.Events = append(result.Events, event)
	}
	if err := scanner.Err(); err != nil {
		return Record{}, err
	}
	result.Summary.Events = len(result.Events)
	result.Result, err = os.ReadFile(filepath.Join(path, "result.json"))
	if errors.Is(err, os.ErrNotExist) {
		result.Result = nil
		result.Triage, err = ReadTriage(dir, id, repo)
		return result, err
	}
	if err != nil {
		return Record{}, err
	}
	if !json.Valid(result.Result) {
		return Record{}, fmt.Errorf("malformed session result in %s", id)
	}
	result.Triage, err = ReadTriage(dir, id, repo)
	return result, err
}

func Delete(dir, id, repo string) error {
	path, _, err := ownedPath(dir, id, repo)
	if err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func ownedPath(dir, id, repo string) (string, Summary, error) {
	if !validID.MatchString(id) || repo == "" {
		return "", Summary{}, errors.New("invalid session ID or repository")
	}
	var err error
	repo, err = canonicalRepo(repo)
	if err != nil {
		return "", Summary{}, err
	}
	path := filepath.Join(dir, id)
	info, err := os.Lstat(path)
	if err != nil {
		return "", Summary{}, err
	}
	if !info.IsDir() {
		return "", Summary{}, errors.New("session path is not a directory")
	}
	data, err := os.ReadFile(filepath.Join(path, "meta.json"))
	if err != nil {
		return "", Summary{}, err
	}
	var summary Summary
	if err := json.Unmarshal(data, &summary); err != nil || summary.ID != id || summary.Repo != repo {
		return "", Summary{}, errors.New("session does not belong to this repository")
	}
	return path, summary, nil
}
