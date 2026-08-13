package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultTTL for cached online data.
const DefaultTTL = 24 * time.Hour

// Store is a simple JSON file cache under dir.
type Store struct {
	Dir string
	TTL time.Duration
}

// New returns a Store rooted at dir.
func New(dir string) *Store {
	return &Store{Dir: dir, TTL: DefaultTTL}
}

type envelope struct {
	SavedAt time.Time       `json:"saved_at"`
	Data    json.RawMessage `json:"data"`
}

// Get loads key into dest if present and fresh.
// refresh forces miss. Returns false on miss/expiry/error.
func (s *Store) Get(key string, dest any, refresh bool) bool {
	if refresh || s == nil || s.Dir == "" {
		return false
	}
	path := s.path(key)
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		return false
	}
	ttl := s.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if time.Since(env.SavedAt) > ttl {
		return false
	}
	if err := json.Unmarshal(env.Data, dest); err != nil {
		return false
	}
	return true
}

// Put writes key data.
func (s *Store) Put(key string, data any) error {
	if s == nil || s.Dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("cache mkdir: %w", err)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	env := envelope{SavedAt: time.Now().UTC(), Data: raw}
	b, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(key), b, 0o644)
}

// SavedAt returns when key was saved, if known.
func (s *Store) SavedAt(key string) (time.Time, bool) {
	if s == nil {
		return time.Time{}, false
	}
	b, err := os.ReadFile(s.path(key))
	if err != nil {
		return time.Time{}, false
	}
	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		return time.Time{}, false
	}
	return env.SavedAt, true
}

func (s *Store) path(key string) string {
	// keep key filesystem-safe
	safe := filepath.Base(key) + ".json"
	return filepath.Join(s.Dir, safe)
}
