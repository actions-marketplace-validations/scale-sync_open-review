package feedback

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	MaxLearnings      = 50
	MaxLearningChars  = 1000
	MaxPromptChars    = 8000 // ~2000 tokens
	defaultExpireDays = 90
)

var disallowedPatterns = []string{
	"<|im_start|>",
	"<|im_end|>",
	"<|endoftext|>",
	"system:",
	"assistant:",
	"human:",
	"```",
}

type Learning struct {
	ID         string     `json:"id"`
	Source     string     `json:"source"`
	Trigger    string     `json:"trigger"`
	Finding    string     `json:"finding"`
	Scope      string     `json:"scope"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	UsedCount  int        `json:"usedCount"`
	LastUsedAt time.Time  `json:"lastUsedAt"`
}

type Store struct {
	dir  string
	path string
	slug string
}

func RepoSlug(repoPath string) string {
	cleaned := strings.TrimSpace(repoPath)
	if cleaned == "" || cleaned == "." {
		if abs, err := filepath.Abs("."); err == nil {
			cleaned = filepath.Base(abs)
		} else {
			cleaned = "repo"
		}
	}
	cleaned = strings.TrimSuffix(cleaned, "/")
	cleaned = strings.ReplaceAll(cleaned, "/", "-")
	cleaned = strings.ReplaceAll(cleaned, "\\", "-")
	cleaned = strings.ReplaceAll(cleaned, ":", "-")
	return cleaned
}

func DefaultDir() (string, error) {
	if home := os.Getenv("OPEN_REVIEW_HOME"); home != "" {
		return filepath.Join(home, "learnings"), nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "open-review", "learnings"), nil
}

func Open(dir, repoSlug string) (*Store, error) {
	slug := RepoSlug(repoSlug)
	if slug == "" {
		return nil, errors.New("invalid repository slug")
	}
	if dir == "" {
		var err error
		dir, err = DefaultDir()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Store{
		dir:  dir,
		path: filepath.Join(dir, slug+".json"),
		slug: slug,
	}, nil
}

func (s *Store) List() ([]Learning, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Learning{}, nil
		}
		return nil, err
	}
	var items []Learning
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	now := time.Now()
	active := make([]Learning, 0, len(items))
	for _, item := range items {
		if item.ExpiresAt != nil && item.ExpiresAt.Before(now) {
			continue
		}
		active = append(active, item)
	}
	return active, nil
}

func (s *Store) Add(trigger, finding, source string, ttlDays int) (Learning, error) {
	trigger = strings.TrimSpace(trigger)
	finding = strings.TrimSpace(finding)
	if err := validateContent(trigger, finding); err != nil {
		return Learning{}, err
	}
	if source == "" {
		source = "manual"
	}
	if ttlDays <= 0 {
		ttlDays = defaultExpireDays
	}

	items, err := s.List()
	if err != nil {
		return Learning{}, err
	}

	// Prune oldest unused if capacity reached
	if len(items) >= MaxLearnings {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].UsedCount == items[j].UsedCount {
				return items[i].CreatedAt.Before(items[j].CreatedAt)
			}
			return items[i].UsedCount < items[j].UsedCount
		})
		items = items[1:]
	}

	idBuf := make([]byte, 4)
	if _, err := rand.Read(idBuf); err != nil {
		return Learning{}, err
	}
	id := hex.EncodeToString(idBuf)
	now := time.Now().UTC()
	expires := now.AddDate(0, 0, ttlDays)

	created := Learning{
		ID:         id,
		Source:     source,
		Trigger:    trigger,
		Finding:    finding,
		Scope:      s.slug,
		CreatedAt:  now,
		ExpiresAt:  &expires,
		UsedCount:  0,
		LastUsedAt: now,
	}

	items = append(items, created)
	if err := s.save(items); err != nil {
		return Learning{}, err
	}
	return created, nil
}

func validateContent(trigger, finding string) error {
	if trigger == "" || finding == "" {
		return errors.New("trigger and finding cannot be empty")
	}
	if len(trigger) > MaxLearningChars || len(finding) > MaxLearningChars {
		return fmt.Errorf("learning exceeds %d character limit", MaxLearningChars)
	}
	for _, p := range disallowedPatterns {
		if strings.Contains(strings.ToLower(trigger), p) || strings.Contains(strings.ToLower(finding), p) {
			return fmt.Errorf("learning content contains disallowed pattern %q", p)
		}
	}
	for _, r := range trigger + finding {
		if r < 32 {
			return errors.New("learning contains invalid control characters")
		}
	}
	return nil
}

func (s *Store) Delete(id string) error {
	items, err := s.List()
	if err != nil {
		return err
	}
	filtered := make([]Learning, 0, len(items))
	found := false
	for _, item := range items {
		if item.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !found {
		return errors.New("learning not found")
	}
	return s.save(filtered)
}

func (s *Store) RecordUsage(id string) error {
	items, err := s.List()
	if err != nil {
		return err
	}
	for i := range items {
		if items[i].ID == id {
			items[i].UsedCount++
			items[i].LastUsedAt = time.Now().UTC()
			return s.save(items)
		}
	}
	return errors.New("learning not found")
}

func (s *Store) Export() ([]byte, error) {
	items, err := s.List()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(items, "", "  ")
}

func (s *Store) Import(data []byte) (int, error) {
	var imported []Learning
	if err := json.Unmarshal(data, &imported); err != nil {
		return 0, err
	}
	current, err := s.List()
	if err != nil {
		return 0, err
	}
	byID := make(map[string]bool, len(current))
	for _, item := range current {
		byID[item.ID] = true
	}
	added := 0
	for _, item := range imported {
		if item.ID == "" || item.Scope != s.slug {
			return 0, errors.New("imported learning has invalid ID or repository scope")
		}
		if err := validateContent(item.Trigger, item.Finding); err != nil {
			return 0, fmt.Errorf("invalid imported learning %s: %w", item.ID, err)
		}
		if item.ExpiresAt != nil && item.ExpiresAt.Before(time.Now()) {
			continue
		}
		if byID[item.ID] {
			continue
		}
		current = append(current, item)
		byID[item.ID] = true
		added++
	}
	if len(current) > MaxLearnings {
		current = current[len(current)-MaxLearnings:]
	}
	if err := s.save(current); err != nil {
		return 0, err
	}
	return added, nil
}

func (s *Store) save(items []Learning) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func FormatForPrompt(learnings []Learning) string {
	if len(learnings) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Team Learnings\n")
	chars := sb.Len()
	for _, l := range learnings {
		line := fmt.Sprintf("- [%s] %s\n", l.Trigger, l.Finding)
		if chars+len(line) > MaxPromptChars {
			break
		}
		sb.WriteString(line)
		chars += len(line)
	}
	return strings.TrimRight(sb.String(), "\n")
}
