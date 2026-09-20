package auth

import (
	"errors"
	"fmt"
	"sync"

	"github.com/zalando/go-keyring"
)

// ErrNotFound is returned when a provider has no secret in the store.
var ErrNotFound = errors.New("secret not found in keyring")

// Store is a secret backend keyed by provider (openai or anthropic).
type Store interface {
	Get(provider string) (string, error)
	Set(provider, secret string) error
	Delete(provider string) error
}

// KeyringStore persists API keys in the OS keyring via zalando/go-keyring.
type KeyringStore struct{}

func (KeyringStore) Get(provider string) (string, error) {
	if _, err := NormalizeProvider(provider); err != nil {
		return "", err
	}
	secret, err := keyring.Get(ServiceName, provider)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("reading %s API key from keyring: %w", provider, err)
	}
	return secret, nil
}

func (KeyringStore) Set(provider, secret string) error {
	if _, err := NormalizeProvider(provider); err != nil {
		return err
	}
	if secret == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	if err := keyring.Set(ServiceName, provider, secret); err != nil {
		return fmt.Errorf("storing %s API key in keyring: %w", provider, err)
	}
	return nil
}

func (KeyringStore) Delete(provider string) error {
	if _, err := NormalizeProvider(provider); err != nil {
		return err
	}
	err := keyring.Delete(ServiceName, provider)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("removing %s API key from keyring: %w", provider, err)
	}
	return nil
}

// MemoryStore is an in-memory Store for tests. It is safe for concurrent use.
type MemoryStore struct {
	mu sync.Mutex
	m  map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{m: make(map[string]string)}
}

func (s *MemoryStore) Get(provider string) (string, error) {
	if _, err := NormalizeProvider(provider); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	secret, ok := s.m[provider]
	if !ok {
		return "", ErrNotFound
	}
	return secret, nil
}

func (s *MemoryStore) Set(provider, secret string) error {
	if _, err := NormalizeProvider(provider); err != nil {
		return err
	}
	if secret == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = make(map[string]string)
	}
	s.m[provider] = secret
	return nil
}

func (s *MemoryStore) Delete(provider string) error {
	if _, err := NormalizeProvider(provider); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[provider]; !ok {
		return ErrNotFound
	}
	delete(s.m, provider)
	return nil
}
