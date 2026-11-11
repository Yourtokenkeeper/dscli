// Package config loads the DeepSeek C-end credentials this client reuses.
//
// The client never asks for a password: it reads the token the official
// Android app already stored on the device (shared_prefs/mmkv → key_user_info)
// or a JSON file the user writes by hand.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Session is the credential blob. Only Token is required.
type Session struct {
	Token string `json:"token"`
	UID   string `json:"uid,omitempty"`
	// DeviceID is optional; the web client sends it when present.
	DeviceID string `json:"device_id,omitempty"`
}

// DefaultPaths returns the lookup order for the session file.
func DefaultPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".dsh", "deepseek", "session.json"),
		filepath.Join(home, ".deepseek", "session.json"),
		filepath.Join(home, ".config", "deepseek", "session.json"),
	}
}

// Load resolves the session from, in order: explicit path, $DS_TOKEN,
// $DS_SESSION_FILE, then the default paths.
func Load(explicit string) (Session, error) {
	if explicit != "" {
		return readFile(explicit)
	}
	if tok := os.Getenv("DS_TOKEN"); tok != "" {
		return Session{Token: tok, DeviceID: os.Getenv("DS_DEVICE_ID")}, nil
	}
	if p := os.Getenv("DS_SESSION_FILE"); p != "" {
		return readFile(p)
	}
	var lastErr error
	for _, p := range DefaultPaths() {
		s, err := readFile(p)
		if err == nil {
			return s, nil
		}
		lastErr = err
	}
	return Session{}, fmt.Errorf("no DeepSeek session found (set DS_TOKEN or write %s): %w",
		DefaultPaths()[0], lastErr)
}

func readFile(path string) (Session, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Session{}, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return Session{}, fmt.Errorf("%s: %w", path, err)
	}
	if s.Token == "" {
		return Session{}, fmt.Errorf("%s: missing token", path)
	}
	return s, nil
}
