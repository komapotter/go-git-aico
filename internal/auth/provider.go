package auth

import (
	"fmt"
	"strings"
)

const (
	// ServiceName is the OS keyring service used for git-aico secrets.
	ServiceName = "git-aico"

	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
)

// Providers is the fixed set of providers that can be registered or switched.
var Providers = []string{ProviderOpenAI, ProviderAnthropic}

// NormalizeProvider lowercases and validates a provider name.
func NormalizeProvider(s string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(s))
	switch p {
	case ProviderOpenAI, ProviderAnthropic:
		return p, nil
	case "":
		return "", fmt.Errorf("provider is required (openai or anthropic)")
	default:
		return "", fmt.Errorf("unknown provider %q (supported: openai, anthropic)", s)
	}
}

// EnvKeyName returns the environment variable that overrides a provider's keyring secret.
func EnvKeyName(provider string) string {
	switch provider {
	case ProviderOpenAI:
		return "OPENAI_API_KEY"
	case ProviderAnthropic:
		return "ANTHROPIC_API_KEY"
	default:
		return ""
	}
}

// MissingKeyError explains how to supply an API key for the active provider.
func MissingKeyError(provider string) error {
	env := EnvKeyName(provider)
	if env == "" {
		return fmt.Errorf("unknown provider %q", provider)
	}
	return fmt.Errorf("%s is required when MODEL_PROVIDER=%s. Set %s, or run: git-aico auth register", env, provider, env)
}
