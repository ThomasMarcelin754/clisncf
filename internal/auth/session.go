package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Session holds the tokens from a headless login or Chrome extraction.
// Persisted to ~/.config/clisncf/session.json (0600, atomic write).
type Session struct {
	AccessToken  string    `json:"accessToken"`
	IDToken      string    `json:"idToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Email        string    `json:"email,omitempty"`
}

func sessionPath() string {
	dir := os.Getenv("CLISNCF_CONFIG_DIR")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config", "clisncf")
	}
	return filepath.Join(dir, "session.json")
}

// LoadSession reads the session from disk. Returns nil if no session exists.
func LoadSession() (*Session, error) {
	data, err := os.ReadFile(sessionPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session: %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("corrupt session file: %w", err)
	}
	return &s, nil
}

// Save writes the session to disk atomically (tmp + rename).
func (s *Session) Save() error {
	p := sessionPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Expired returns true if the access token has expired or will expire within 30s.
func (s *Session) Expired() bool {
	return time.Now().After(s.ExpiresAt.Add(-30 * time.Second))
}

// Clear removes the session file.
func ClearSession() error {
	err := os.Remove(sessionPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
