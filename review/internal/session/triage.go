package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var triageStates = map[string]bool{"open": true, "fixed": true, "ignored": true, "false_positive": true}

func ReadTriage(dir, id, repo string) (map[string]string, error) {
	path, _, err := ownedPath(dir, id, repo)
	if err != nil {
		return nil, err
	}
	file := filepath.Join(path, "triage.json")
	info, err := os.Lstat(file)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("triage file is not regular")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	states := map[string]string{}
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, err
	}
	for finding, state := range states {
		if finding == "" || !triageStates[state] {
			return nil, errors.New("malformed triage state")
		}
	}
	return states, nil
}

func SetTriage(dir, id, repo, findingID, state string) (map[string]string, error) {
	if !triageStates[state] || findingID == "" {
		return nil, errors.New("invalid finding ID or triage state")
	}
	path, _, err := ownedPath(dir, id, repo)
	if err != nil {
		return nil, err
	}
	result, err := os.ReadFile(filepath.Join(path, "result.json"))
	if err != nil {
		return nil, err
	}
	var review struct {
		Findings []struct {
			ID string `json:"id"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(result, &review); err != nil {
		return nil, err
	}
	found := false
	for _, finding := range review.Findings {
		if finding.ID == findingID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("finding %q is not in this session", findingID)
	}
	states, err := ReadTriage(dir, id, repo)
	if err != nil {
		return nil, err
	}
	if state == "open" {
		delete(states, findingID)
	} else {
		states[findingID] = state
	}
	data, err := json.Marshal(states)
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(path, ".triage-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(file.Name(), filepath.Join(path, "triage.json")); err != nil {
		return nil, err
	}
	return states, nil
}
