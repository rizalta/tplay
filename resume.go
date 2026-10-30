package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const maxEntries = 100

type resumeEntry struct {
	Position  float64 `json:"position"`
	Duration  float64 `json:"duration,omitempty"`
	UpdatedAt int64   `json:"updated_at"`
}

func resumePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "resume.json"), nil
}

func resumeKey(infoHash, filePath string) string {
	return strings.ToLower(infoHash) + ":" + filePath
}

type resumeStore struct {
	path    string
	mu      sync.RWMutex
	entries map[string]resumeEntry
}

func newResumeStore() *resumeStore {
	path, _ := resumePath()
	store := &resumeStore{
		path:    path,
		mu:      sync.RWMutex{},
		entries: make(map[string]resumeEntry),
	}
	store.load()

	return store
}

func (s *resumeStore) load() {
	if s.path == "" {
		return
	}

	resumeFile, err := os.Open(s.path)
	if err != nil {
		return
	}
	defer func() { _ = resumeFile.Close() }()

	decoder := json.NewDecoder(resumeFile)
	if err := decoder.Decode(&s.entries); err != nil {
		return
	}
}

func (s *resumeStore) save() error {
	if s.path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	resumeFile, err := os.Create(s.path)
	if err != nil {
		return fmt.Errorf("creating resume file: %w", err)
	}
	defer func() { _ = resumeFile.Close() }()

	encoder := json.NewEncoder(resumeFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(s.entries); err != nil {
		return fmt.Errorf("encoding resume file: %w", err)
	}

	return nil
}

func (s *resumeStore) get(infoHash, filePath string) (resumeEntry, bool) {
	key := resumeKey(infoHash, filePath)
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[key]
	return e, ok
}

func (s *resumeStore) set(infoHash, filePath string, pos, dur float64) error {
	key := resumeKey(infoHash, filePath)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = resumeEntry{
		Position:  pos,
		Duration:  dur,
		UpdatedAt: time.Now().Unix(),
	}

	return s.save()
}

func (s *resumeStore) remove(infoHash, filePath string) error {
	key := resumeKey(infoHash, filePath)
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, key)
	return s.save()
}

func (s *resumeStore) recordPlayback(infoHash, filePath string, pos, dur float64) error {
	if pos < 5 {
		return nil
	}

	if dur > 0 {
		threshold := min(60.0, 0.05*dur)
		if dur-pos <= threshold {
			return s.remove(infoHash, filePath)
		}
	}

	return s.set(infoHash, filePath, pos, dur)
}

func (s *resumeStore) removeTorrent(infoHash string) error {
	prefix := strings.ToLower(infoHash) + ":"

	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for key := range s.entries {
		if strings.HasPrefix(key, prefix) {
			delete(s.entries, key)
			changed = true
		}
	}

	if changed {
		return s.save()
	}
	return nil
}

func (s *resumeStore) cleanOldEntries(maxDays int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.entries) == 0 {
		return nil
	}

	changed := false

	keys := make([]string, 0, len(s.entries))
	for k := range s.entries {
		keys = append(keys, k)
	}

	slices.SortFunc(keys, func(a, b string) int {
		return cmp.Compare(s.entries[b].UpdatedAt, s.entries[a].UpdatedAt)
	})

	if len(keys) > maxEntries {
		for _, k := range keys[maxEntries:] {
			delete(s.entries, k)
		}
		changed = true
		keys = keys[:maxEntries]
	}

	if maxDays > 0 {
		ttl := time.Duration(maxDays) * 24 * time.Hour
		now := time.Now()

		for i := len(keys) - 1; i >= 0; i-- {
			entry := s.entries[keys[i]]
			if entry.UpdatedAt > 0 && now.Sub(time.Unix(entry.UpdatedAt, 0)) > ttl {
				delete(s.entries, keys[i])
				changed = true
			} else {
				break
			}
		}
	}

	if changed {
		return s.save()
	}
	return nil
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	h := int(d / time.Hour)
	m := int(d % time.Hour / time.Minute)
	s := int(d % time.Minute / time.Second)

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
