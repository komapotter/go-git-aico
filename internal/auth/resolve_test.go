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

func TestResolveEnvOverridesKeyringAndConfig(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Set(ProviderOpenAI, "keyring-openai"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(ProviderAnthropic, "keyring-anthropic"); err != nil {
		t.Fatal(err)
	}
	file := FileConfig{ModelProvider: ProviderAnthropic}
	env := getenvMap(map[string]string{
		"MODEL_PROVIDER":    ProviderOpenAI,
		"OPENAI_API_KEY":    "env-openai",
		"ANTHROPIC_API_KEY": "env-anthropic",
	})

	creds, err := Resolve(env, file, store)
	if err != nil {
		t.Fatal(err)
	}
	if creds.ModelProvider != ProviderOpenAI || creds.ProviderSource != SourceEnvironment {
		t.Fatalf("provider = %s (%s)", creds.ModelProvider, creds.ProviderSource)
	}
	if creds.OpenAIKey != "env-openai" || creds.OpenAISource != SourceEnvironment {
		t.Fatalf("openai key source = %s", creds.OpenAISource)
	}
	if creds.AnthropicKey != "env-anthropic" || creds.AnthropicSource != SourceEnvironment {
		t.Fatalf("anthropic key source = %s", creds.AnthropicSource)
	}
}

func TestResolveKeyringUsedWhenEnvUnset(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Set(ProviderAnthropic, "keyring-anthropic"); err != nil {
		t.Fatal(err)
	}
	file := FileConfig{ModelProvider: ProviderAnthropic}

	creds, err := Resolve(getenvMap(nil), file, store)
	if err != nil {
		t.Fatal(err)
	}
	if creds.ModelProvider != ProviderAnthropic || creds.ProviderSource != SourceConfig {
		t.Fatalf("provider = %s (%s)", creds.ModelProvider, creds.ProviderSource)
	}
	key, err := creds.APIKey()
	if err != nil {
		t.Fatal(err)
	}
	if key != "keyring-anthropic" {
		t.Fatalf("APIKey = %q", key)
	}
	if creds.AnthropicSource != SourceKeyring {
		t.Fatalf("source = %s", creds.AnthropicSource)
	}
}

func TestResolveDefaultProviderIsOpenAI(t *testing.T) {
	creds, err := Resolve(getenvMap(nil), FileConfig{}, NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	if creds.ModelProvider != ProviderOpenAI || creds.ProviderSource != SourceDefault {
		t.Fatalf("provider = %s (%s)", creds.ModelProvider, creds.ProviderSource)
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
	creds, err := Resolve(env, FileConfig{}, store)
	if err != nil {
		t.Fatal(err)
	}
	if creds.OpenAIKey != "keyring-openai" || creds.OpenAISource != SourceKeyring {
		t.Fatalf("got key %q source %s", creds.OpenAIKey, creds.OpenAISource)
	}
}

func TestResolveInactiveKeyringErrorIsIgnored(t *testing.T) {
	store := &errStore{errFor: ProviderAnthropic, err: errors.New("dbus unavailable")}
	env := getenvMap(map[string]string{"OPENAI_API_KEY": "env-openai"})
	creds, err := Resolve(env, FileConfig{}, store)
	if err != nil {
		t.Fatalf("openai env should work even if anthropic keyring fails: %v", err)
	}
	if creds.OpenAIKey != "env-openai" {
		t.Fatalf("OpenAIKey = %q", creds.OpenAIKey)
	}
}

func TestResolveActiveKeyringErrorSuggestsRegister(t *testing.T) {
	store := &errStore{errFor: ProviderOpenAI, err: errors.New("dbus unavailable")}
	creds, err := Resolve(getenvMap(nil), FileConfig{}, store)
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
	st := LookupStatus(getenvMap(nil), FileConfig{ModelProvider: ProviderOpenAI}, store)
	ks := st.Keys[ProviderOpenAI]
	if !ks.Registered || ks.Source != SourceKeyring {
		t.Fatalf("status = %+v", ks)
	}
	if strings.Contains(ks.Source, secret) {
		t.Fatal("status leaked secret")
	}
}

func TestFileConfigRoundTripDoesNotWriteKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "git-aico", "config.yml")
	if err := SetActiveProvider(path, ProviderAnthropic); err != nil {
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
	if !strings.Contains(body, "model_provider: anthropic") {
		t.Fatalf("missing provider:\n%s", body)
	}
	cfg, err := LoadFileConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelProvider != ProviderAnthropic {
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
