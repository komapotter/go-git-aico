package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/komapotter/go-git-aico/internal/auth"
)

func newTestAuthCLI(t *testing.T, store auth.Store) (*authCLI, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	return &authCLI{
		store:      store,
		configPath: filepath.Join(dir, "config.yml"),
		getenv:     func(string) (string, bool) { return "", false },
		stdin:      strings.NewReader(""),
		stdout:     stdout,
		stderr:     stderr,
		isTerminal: func() bool { return false },
	}, stdout, stderr
}

func TestAuthRegisterAndStatusHideSecret(t *testing.T) {
	store := auth.NewMemoryStore()
	cli, stdout, _ := newTestAuthCLI(t, store)
	secret := "sk-test-secret-do-not-print"
	cli.readSecret = func(string) (string, error) { return secret, nil }

	if err := cli.run([]string{"register", "-p", "openai"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), secret) {
		t.Fatalf("register output leaked secret:\n%s", stdout.String())
	}
	got, err := store.Get(auth.ProviderOpenAI)
	if err != nil || got != secret {
		t.Fatalf("store Get = %q, %v", got, err)
	}
	data, err := os.ReadFile(cli.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatalf("config file leaked secret:\n%s", data)
	}

	stdout.Reset()
	if err := cli.run([]string{"status"}); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if strings.Contains(out, secret) {
		t.Fatalf("status leaked secret:\n%s", out)
	}
	if !strings.Contains(out, "openai") || !strings.Contains(out, "registered") {
		t.Fatalf("status missing openai registration:\n%s", out)
	}
	if !strings.Contains(out, "Active provider: openai") {
		t.Fatalf("status missing active provider:\n%s", out)
	}
}

func TestAuthSwitchAndRemove(t *testing.T) {
	store := auth.NewMemoryStore()
	if err := store.Set(auth.ProviderAnthropic, "sk-anth"); err != nil {
		t.Fatal(err)
	}
	cli, stdout, _ := newTestAuthCLI(t, store)
	if err := cli.run([]string{"switch", "anthropic"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Active provider set to anthropic") {
		t.Fatalf("switch output:\n%s", stdout.String())
	}
	cfg, err := auth.LoadFileConfig(cli.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelProvider != auth.ProviderAnthropic {
		t.Fatalf("config provider = %q", cfg.ModelProvider)
	}

	stdout.Reset()
	if err := cli.run([]string{"remove", "--provider", "anthropic"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(auth.ProviderAnthropic); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("expected key removed, got %v", err)
	}
}

func TestAuthSwitchRejectsUnknownProvider(t *testing.T) {
	cli, _, _ := newTestAuthCLI(t, auth.NewMemoryStore())
	err := cli.run([]string{"switch", "gemini"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("got %v", err)
	}
}

func TestAuthRemoveMissingKey(t *testing.T) {
	cli, _, _ := newTestAuthCLI(t, auth.NewMemoryStore())
	err := cli.run([]string{"remove", "-p", "openai"})
	if err == nil || !strings.Contains(err.Error(), "no API key registered") {
		t.Fatalf("got %v", err)
	}
}

func TestAuthUnknownCommand(t *testing.T) {
	cli, _, stderr := newTestAuthCLI(t, auth.NewMemoryStore())
	err := cli.run([]string{"login"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), "unknown auth command") {
		t.Fatalf("stderr:\n%s", stderr.String())
	}
}

func TestAuthRegisterInteractive(t *testing.T) {
	store := auth.NewMemoryStore()
	cli, stdout, _ := newTestAuthCLI(t, store)
	cli.stdin = strings.NewReader("2\n")
	cli.isTerminal = func() bool { return true }
	cli.readSecret = func(provider string) (string, error) {
		if provider != auth.ProviderAnthropic {
			t.Fatalf("provider = %s", provider)
		}
		return "sk-anth-interactive", nil
	}
	if err := cli.run([]string{"register"}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(auth.ProviderAnthropic)
	if err != nil || got != "sk-anth-interactive" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if !strings.Contains(stdout.String(), "anthropic") {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestAuthHelp(t *testing.T) {
	cli, stdout, _ := newTestAuthCLI(t, auth.NewMemoryStore())
	if err := cli.run([]string{"-h"}); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	for _, cmd := range []string{"register", "remove", "status", "switch"} {
		if !strings.Contains(out, cmd) {
			t.Fatalf("auth help missing %s:\n%s", cmd, out)
		}
	}
	if strings.Contains(out, "login") || strings.Contains(out, "logout") {
		t.Fatalf("auth help should not mention login/logout:\n%s", out)
	}
}

func TestIsAuthCommand(t *testing.T) {
	if !isAuthCommand([]string{"auth", "status"}) {
		t.Fatal("expected auth command")
	}
	if isAuthCommand([]string{"-v"}) || isAuthCommand(nil) {
		t.Fatal("flags only is not auth")
	}
}

func TestLoadAppConfigEnvOverridesKeyring(t *testing.T) {
	orig := credentialStore
	t.Cleanup(func() { credentialStore = orig })

	store := auth.NewMemoryStore()
	if err := store.Set(auth.ProviderOpenAI, "keyring-openai"); err != nil {
		t.Fatal(err)
	}
	credentialStore = store

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "git-aico", "config.yml")
	if err := auth.SetActiveProvider(path, auth.ProviderAnthropic); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MODEL_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "env-openai")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("NUM_CANDIDATES", "3")

	cfg, err := loadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelProvider != auth.ProviderOpenAI {
		t.Fatalf("provider = %s", cfg.ModelProvider)
	}
	if cfg.OpenAIKey != "env-openai" {
		t.Fatalf("OpenAIKey = %q", cfg.OpenAIKey)
	}
}

func TestLoadAppConfigUsesKeyringWhenEnvUnset(t *testing.T) {
	orig := credentialStore
	t.Cleanup(func() { credentialStore = orig })

	store := auth.NewMemoryStore()
	if err := store.Set(auth.ProviderAnthropic, "keyring-anthropic"); err != nil {
		t.Fatal(err)
	}
	credentialStore = store

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "git-aico", "config.yml")
	if err := auth.SetActiveProvider(path, auth.ProviderAnthropic); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MODEL_PROVIDER", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	cfg, err := loadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelProvider != auth.ProviderAnthropic {
		t.Fatalf("provider = %s", cfg.ModelProvider)
	}
	if cfg.AnthropicKey != "keyring-anthropic" {
		t.Fatalf("AnthropicKey = %q", cfg.AnthropicKey)
	}
}

func TestLoadAppConfigMissingKeySuggestsRegister(t *testing.T) {
	orig := credentialStore
	t.Cleanup(func() { credentialStore = orig })
	credentialStore = auth.NewMemoryStore()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("MODEL_PROVIDER", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, err := loadAppConfig()
	if err == nil {
		t.Fatal("expected missing key error")
	}
	if !strings.Contains(err.Error(), "git-aico auth register") {
		t.Fatalf("got %v", err)
	}
}

func TestAuthStatusShowsEnvironmentWithoutSecret(t *testing.T) {
	store := auth.NewMemoryStore()
	cli, stdout, _ := newTestAuthCLI(t, store)
	secret := "sk-env-must-not-appear"
	cli.getenv = func(key string) (string, bool) {
		if key == "OPENAI_API_KEY" {
			return secret, true
		}
		if key == "MODEL_PROVIDER" {
			return "openai", true
		}
		return "", false
	}
	if err := cli.run([]string{"status"}); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if strings.Contains(out, secret) {
		t.Fatalf("status leaked secret:\n%s", out)
	}
	if !strings.Contains(out, "available (environment)") {
		t.Fatalf("status:\n%s", out)
	}
	if !strings.Contains(out, "Active provider: openai (environment)") {
		t.Fatalf("status:\n%s", out)
	}
}
