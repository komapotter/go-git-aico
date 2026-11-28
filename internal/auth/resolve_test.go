package auth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func getenvMap(m map[string]string) EnvLookup {
	return func(key string) (string, bool) {
		v, ok := m[key]
		return v, ok
	}
}

func TestResolveEnvOverridesKeyring(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Set(ProviderOpenAI, "keyring-openai"); err != nil {
		t.Fatal(err)
	}
	env := getenvMap(map[string]string{
		"OPENAI_API_KEY": "env-openai",
	})

	creds, err := Resolve(env, store)
	if err != nil {
		t.Fatal(err)
	}
	if creds.OpenAIKey != "env-openai" || creds.OpenAISource != SourceEnvironment {
		t.Fatalf("openai key source = %s", creds.OpenAISource)
	}
}

func TestResolveKeyringUsedWhenEnvUnset(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Set(ProviderOpenAI, "keyring-openai"); err != nil {
		t.Fatal(err)
	}

	creds, err := Resolve(getenvMap(nil), store)
	if err != nil {
		t.Fatal(err)
	}
	key, err := creds.APIKey()
	if err != nil {
		t.Fatal(err)
	}
	if key != "keyring-openai" {
		t.Fatalf("APIKey = %q", key)
	}
	if creds.OpenAISource != SourceKeyring {
		t.Fatalf("source = %s", creds.OpenAISource)
	}
}

func TestResolveMissingKeySuggestsRegister(t *testing.T) {
	creds, err := Resolve(getenvMap(nil), NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := creds.APIKey(); err == nil {
		t.Fatal("expected missing key error")
	} else if !strings.Contains(err.Error(), "git-aico auth register") {
		t.Fatalf("error should suggest auth register: %v", err)
	}
}

func TestResolveEmptyEnvFallsBackToKeyring(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Set(ProviderOpenAI, "keyring-openai"); err != nil {
		t.Fatal(err)
	}
	env := getenvMap(map[string]string{"OPENAI_API_KEY": ""})
	creds, err := Resolve(env, store)
	if err != nil {
		t.Fatal(err)
	}
	if creds.OpenAIKey != "keyring-openai" || creds.OpenAISource != SourceKeyring {
		t.Fatalf("got key %q source %s", creds.OpenAIKey, creds.OpenAISource)
	}
}

func TestResolveActiveKeyringErrorSuggestsRegister(t *testing.T) {
	store := &errStore{errFor: ProviderOpenAI, err: errors.New("dbus unavailable")}
	creds, err := Resolve(getenvMap(nil), store)
	if err != nil {
		t.Fatal(err)
	}
	err = creds.Validate()
	if err == nil {
		t.Fatal("expected missing key error")
	}
	if !strings.Contains(err.Error(), "git-aico auth register") {
		t.Fatalf("error should suggest auth register: %v", err)
	}
	if !strings.Contains(err.Error(), "dbus unavailable") {
		t.Fatalf("error should include keyring failure: %v", err)
	}
}

func TestLookupStatusNeverIncludesSecrets(t *testing.T) {
	store := NewMemoryStore()
	secret := "sk-never-print-this-value"
	if err := store.Set(ProviderOpenAI, secret); err != nil {
		t.Fatal(err)
	}
	st := LookupStatus(getenvMap(nil), store)
	if !st.OpenAI.Registered || st.OpenAI.Source != SourceKeyring {
		t.Fatalf("status = %+v", st.OpenAI)
	}
	if strings.Contains(st.OpenAI.Source, secret) {
		t.Fatal("status leaked secret")
	}
}

func TestFileConfigRoundTripDoesNotWriteKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "git-aico", "config.yml")
	if err := SaveFileConfig(path, FileConfig{ModelProvider: ProviderOpenAI}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if strings.Contains(strings.ToLower(body), "api_key") || strings.Contains(body, "sk-") {
		t.Fatalf("config file must not contain secrets:\n%s", body)
	}
	cfg, err := LoadFileConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelProvider != ProviderOpenAI {
		t.Fatalf("ModelProvider = %q", cfg.ModelProvider)
	}
}

func TestLoadFileConfigMissingIsEmpty(t *testing.T) {
	cfg, err := LoadFileConfig(filepath.Join(t.TempDir(), "missing.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelProvider != "" {
		t.Fatalf("got %+v", cfg)
	}
}

func TestLoadFileConfigLegacyAnthropicDoesNotFail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("model_provider: anthropic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFileConfig(path); err != nil {
		t.Fatal(err)
	}
}

func TestKeyringStoreRoundTripWithMock(t *testing.T) {
	keyring.MockInit()
	var s KeyringStore
	if err := s.Set(ProviderOpenAI, "mock-secret"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ProviderOpenAI)
	if err != nil {
		t.Fatal(err)
	}
	if got != "mock-secret" {
		t.Fatalf("Get = %q", got)
	}
	if err := s.Delete(ProviderOpenAI); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ProviderOpenAI); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestNormalizeProvider(t *testing.T) {
	p, err := NormalizeProvider(" OpenAI ")
	if err != nil || p != ProviderOpenAI {
		t.Fatalf("got %q %v", p, err)
	}
	if _, err := NormalizeProvider("gemini"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := NormalizeProvider("anthropic"); err == nil {
		t.Fatal("expected anthropic to be rejected")
	}
}

func TestDefaultConfigPathUsesXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	got, err := DefaultConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "git-aico", "config.yml")
	if got != want {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

type errStore struct {
	errFor string
	err    error
}

func (s *errStore) Get(provider string) (string, error) {
	if provider == s.errFor {
		return "", s.err
	}
	return "", ErrNotFound
}

func (s *errStore) Set(string, string) error { return nil }

func (s *errStore) Delete(string) error { return nil }
