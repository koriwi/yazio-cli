package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	Token        string `json:"token"`
	Email        string `json:"email"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "yazio-cli", "config.json"), nil
}

func SaveToken(email, accessToken, refreshToken string) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(Config{Token: accessToken, Email: email, RefreshToken: refreshToken})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func LoadToken() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return cfg.Token, nil
}

// LoadConfig returns the full saved config (email, access token, refresh token).
func LoadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// ConfigFilePath returns the path to the config file for display purposes.
func ConfigFilePath() string {
	path, _ := configPath()
	return path
}

func ClearToken() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}
