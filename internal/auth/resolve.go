package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	SourceEnvironment = "environment"
	SourceKeyring     = "keyring"
	SourceConfig      = "config"
	SourceDefault     = "default"
)

// EnvLookup looks up an environment variable. The bool is false when unset.
type EnvLookup func(key string) (string, bool)

// Credentials is the merged view of env vars, the config file, and the keyring.
type Credentials struct {
	OpenAIKey       string
	AnthropicKey    string
	ModelProvider   string
	ProviderSource  string
	OpenAISource    string
	AnthropicSource string
	OpenAIErr       error
	AnthropicErr    error
}

// Resolve merges environment variables, the local config file, and the keyring.
// Priority: env (if set) > keyring/config > empty/default.
// Keyring lookup errors are stored on Credentials; Validate attaches them
// to MissingKeyError so callers still get the auth register hint.
func Resolve(getenv EnvLookup, file FileConfig, store Store) (Credentials, error) {
	if getenv == nil {
		getenv = os.LookupEnv
	}
	var c Credentials
	var err error
	c.ModelProvider, c.ProviderSource, err = resolveProvider(getenv, file)
	if err != nil {
		return c, err
	}

	c.OpenAIKey, c.OpenAISource, c.OpenAIErr = resolveKey(getenv, store, ProviderOpenAI)
	c.AnthropicKey, c.AnthropicSource, c.AnthropicErr = resolveKey(getenv, store, ProviderAnthropic)
	return c, nil
}

func resolveProvider(getenv EnvLookup, file FileConfig) (string, string, error) {
	if v, ok := getenv("MODEL_PROVIDER"); ok && strings.TrimSpace(v) != "" {
		p, err := NormalizeProvider(v)
		if err != nil {
			return "", "", err
		}
		return p, SourceEnvironment, nil
	}
	if strings.TrimSpace(file.ModelProvider) != "" {
		p, err := NormalizeProvider(file.ModelProvider)
		if err != nil {
			return "", "", err
		}
		return p, SourceConfig, nil
	}
	return ProviderOpenAI, SourceDefault, nil
}

func resolveKey(getenv EnvLookup, store Store, provider string) (string, string, error) {
	envName := EnvKeyName(provider)
	if v, ok := getenv(envName); ok && strings.TrimSpace(v) != "" {
		return v, SourceEnvironment, nil
	}
	if store == nil {
		return "", "", nil
	}
	secret, err := store.Get(provider)
	if err == nil {
		return secret, SourceKeyring, nil
	}
	if errors.Is(err, ErrNotFound) {
		return "", "", nil
	}
	return "", "", err
}

// APIKey returns the secret for the active provider, or MissingKeyError.
func (c Credentials) APIKey() (string, error) {
	switch c.ModelProvider {
	case ProviderOpenAI:
		if c.OpenAIKey == "" {
			return "", MissingKeyError(ProviderOpenAI)
		}
		return c.OpenAIKey, nil
	case ProviderAnthropic:
		if c.AnthropicKey == "" {
			return "", MissingKeyError(ProviderAnthropic)
		}
		return c.AnthropicKey, nil
	default:
		return "", MissingKeyError(c.ModelProvider)
	}
}

// Validate reports a missing API key for the active provider.
// Keyring failures are attached as context so callers still see
// `git-aico auth register` while retaining the underlying error.
func (c Credentials) Validate() error {
	if _, err := c.APIKey(); err == nil {
		return nil
	}
	var kerr error
	switch c.ModelProvider {
	case ProviderOpenAI:
		kerr = c.OpenAIErr
	case ProviderAnthropic:
		kerr = c.AnthropicErr
	}
	err := MissingKeyError(c.ModelProvider)
	if kerr != nil {
		return fmt.Errorf("%w (%v)", err, kerr)
	}
	return err
}

// KeyStatus is a non-secret view of one provider's credential.
type KeyStatus struct {
	Registered bool
	Source     string
	Err        error
}

// Status is a non-secret snapshot for `git-aico auth status`.
type Status struct {
	ActiveProvider string
	ProviderSource string
	ProviderErr    error
	Keys           map[string]KeyStatus
}

// LookupStatus reports which providers have keys without returning the secrets.
func LookupStatus(getenv EnvLookup, file FileConfig, store Store) Status {
	if getenv == nil {
		getenv = os.LookupEnv
	}
	st := Status{Keys: make(map[string]KeyStatus, len(Providers))}
	provider, source, err := resolveProvider(getenv, file)
	if err != nil {
		st.ProviderErr = err
		if v, ok := getenv("MODEL_PROVIDER"); ok && strings.TrimSpace(v) != "" {
			st.ActiveProvider = strings.TrimSpace(v)
			st.ProviderSource = SourceEnvironment
		} else {
			st.ActiveProvider = strings.TrimSpace(file.ModelProvider)
			st.ProviderSource = SourceConfig
		}
	} else {
		st.ActiveProvider = provider
		st.ProviderSource = source
	}
	for _, p := range Providers {
		secret, src, kerr := resolveKey(getenv, store, p)
		st.Keys[p] = KeyStatus{Source: src, Err: kerr, Registered: secret != "" && kerr == nil}
	}
	return st
}
