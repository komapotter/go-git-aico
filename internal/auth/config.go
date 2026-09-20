package auth

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

const configFileName = "config.yml"

const configFileHeader = "# git-aico local configuration. Do not store API keys in this file.\n"

// FileConfig holds non-secret settings. API keys must never be written here.
type FileConfig struct {
	ModelProvider string `yaml:"model_provider,omitempty"`
}

// DefaultConfigPath returns the local config file path.
// It uses $XDG_CONFIG_HOME/git-aico/config.yml when set, otherwise
// ~/.config/git-aico/config.yml (Windows: %AppData%/git-aico/config.yml).
func DefaultConfigPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "git-aico", configFileName), nil
	}
	if runtime.GOOS == "windows" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "git-aico", configFileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "git-aico", configFileName), nil
}

// LoadFileConfig reads non-secret settings. A missing file is not an error.
func LoadFileConfig(path string) (FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, nil
		}
		return FileConfig{}, err
	}
	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return FileConfig{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg, nil
}

// SaveFileConfig writes non-secret settings with restrictive permissions.
func SaveFileConfig(path string, cfg FileConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString(configFileHeader)
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		_ = enc.Close()
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

// SetActiveProvider updates model_provider in the config file.
func SetActiveProvider(path, provider string) error {
	p, err := NormalizeProvider(provider)
	if err != nil {
		return err
	}
	cfg, err := LoadFileConfig(path)
	if err != nil {
		return err
	}
	cfg.ModelProvider = p
	return SaveFileConfig(path, cfg)
}
