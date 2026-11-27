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
)

// EnvLookup looks up an environment variable. The bool is false when unset.
type EnvLookup func(key string) (string, bool)

// Credentials is the merged view of env vars and the keyring.
type Credentials struct {
	OpenAIKey    string
	OpenAISource string
	OpenAIErr    error
}

// Resolve merges environment variables and the keyring.
// Priority: env (if set) > keyring > empty.
// Keyring lookup errors are stored on Credentials; Validate attaches them
// to MissingKeyError so callers still get the auth register hint.
func Resolve(getenv EnvLookup, store Store) (Credentials, error) {
	if getenv == nil {
		getenv = os.LookupEnv
	}
	var c Credentials
	c.OpenAIKey, c.OpenAISource, c.OpenAIErr = resolveKey(getenv, store, ProviderOpenAI)
	return c, nil
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

// APIKey returns the OpenAI secret, or MissingKeyError.
func (c Credentials) APIKey() (string, error) {
	if c.OpenAIKey == "" {
		return "", MissingKeyError()
	}
	return c.OpenAIKey, nil
}

// Validate reports a missing OpenAI API key.
// Keyring failures are attached as context so callers still see
// `git-aico auth register` while retaining the underlying error.
func (c Credentials) Validate() error {
	if _, err := c.APIKey(); err == nil {
		return nil
	}
	err := MissingKeyError()
	if c.OpenAIErr != nil {
		return fmt.Errorf("%w (%v)", err, c.OpenAIErr)
	}
	return err
}

// KeyStatus is a non-secret view of the OpenAI credential.
type KeyStatus struct {
	Registered bool
	Source     string
	Err        error
}

// Status is a non-secret snapshot for `git-aico auth status`.
type Status struct {
	OpenAI KeyStatus
}

// LookupStatus reports whether an OpenAI key is present without returning the secret.
func LookupStatus(getenv EnvLookup, store Store) Status {
	if getenv == nil {
		getenv = os.LookupEnv
	}
	secret, src, kerr := resolveKey(getenv, store, ProviderOpenAI)
	return Status{
		OpenAI: KeyStatus{Source: src, Err: kerr, Registered: secret != "" && kerr == nil},
	}
}
