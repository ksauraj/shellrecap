// internal/config/keys.go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Keys are the AI API keys saved from the Advanced panel. They're kept
// apart from the rest of the config, in a file only the user can read.
type Keys struct {
	Gemini string `json:"gemini,omitempty"`
	Groq   string `json:"groq,omitempty"`
}

// KeysPath is where the keys are saved, next to config.json
func KeysPath() (string, error) {
	p, err := path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "keys.json"), nil
}

// LoadKeys reads the saved keys, or none
func LoadKeys() Keys {
	p, err := KeysPath()
	if err != nil {
		return Keys{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Keys{}
	}
	var k Keys
	if json.Unmarshal(data, &k) != nil {
		return Keys{}
	}
	return k
}

// SaveKeys writes the keys readable and writable by the user only, or
// removes the file when there are none left
func SaveKeys(k Keys) error {
	p, err := KeysPath()
	if err != nil {
		return err
	}
	if k == (Keys{}) {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	// Created with 0600; Chmod as well in case an old file was left behind
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, p)
}
