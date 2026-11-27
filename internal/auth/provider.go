package auth

import (
	"fmt"
	"strings"
)

const (
	// ServiceName is the OS keyring service used for git-aico secrets.
	ServiceName = "git-aico"

	ProviderOpenAI = "openai"
)

// NormalizeProvider lowercases and validates a provider name.
func NormalizeProvider(s string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(s))
	switch p {
	case ProviderOpenAI:
		return p, nil
	case "":
		return "", fmt.Errorf("provider is required (openai)")
	default:
		return "", fmt.Errorf("unknown provider %q (supported: openai)", s)
	}
}

// EnvKeyName returns the environment variable that overrides a provider's keyring secret.
func EnvKeyName(provider string) string {
	if provider == ProviderOpenAI {
		return "OPENAI_API_KEY"
	}
	return ""
}

// MissingKeyError explains how to supply an OpenAI API key.
func MissingKeyError() error {
	return fmt.Errorf("OPENAI_API_KEY is required. Set OPENAI_API_KEY, or run: git-aico auth register")
}
