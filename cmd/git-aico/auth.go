package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/komapotter/go-git-aico/internal/auth"
	"golang.org/x/term"
)

var errAuthUsage = errors.New("auth usage")

type authCLI struct {
	store      auth.Store
	configPath string
	getenv     auth.EnvLookup
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	readSecret func(provider string) (string, error)
	isTerminal func() bool
}

func newAuthCLI() (*authCLI, error) {
	path, err := auth.DefaultConfigPath()
	if err != nil {
		return nil, err
	}
	return &authCLI{
		store:      auth.KeyringStore{},
		configPath: path,
		getenv:     os.LookupEnv,
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		isTerminal: func() bool { return term.IsTerminal(int(os.Stdin.Fd())) },
	}, nil
}

func runAuth(args []string) error {
	cli, err := newAuthCLI()
	if err != nil {
		return err
	}
	return cli.run(args)
}

func (c *authCLI) run(args []string) error {
	if len(args) == 0 {
		printAuthHelp(c.stdout)
		return errAuthUsage
	}
	switch args[0] {
	case "register":
		return c.register(args[1:])
	case "remove":
		return c.remove(args[1:])
	case "status":
		return c.status(args[1:])
	case "switch":
		return c.switchProvider(args[1:])
	case "-h", "-help", "--help":
		printAuthHelp(c.stdout)
		return nil
	default:
		fmt.Fprintf(c.stderr, "unknown auth command %q\n\n", args[0])
		printAuthHelp(c.stderr)
		return errAuthUsage
	}
}

func printAuthHelp(w io.Writer) {
	fmt.Fprint(w, `
Usage: git-aico auth <command>

Store OpenAI/Anthropic API keys in the OS keyring (macOS Keychain,
Linux Secret Service, or Windows Credential Manager) instead of a
plaintext env file.

Commands:
  register   Store an API key in the OS keyring
  remove     Remove a stored API key from the OS keyring
  status     Show registered providers and the active provider
  switch     Set the active provider (openai or anthropic)

Run "git-aico auth <command> -h" for command help.

API keys are never printed. On macOS, the first Keychain access may
show a permission dialog.
`)
}

func (c *authCLI) newFlagSet(name string, usage func(io.Writer)) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	fs.Usage = func() { usage(fs.Output()) }
	return fs
}

func parseProviderFlags(fs *flag.FlagSet, args []string) (flagProvider string, positionals []string, err error) {
	var provider string
	fs.StringVar(&provider, "p", "", "Provider: openai or anthropic")
	fs.StringVar(&provider, "provider", "", "Provider: openai or anthropic")
	if err := fs.Parse(args); err != nil {
		return "", nil, err
	}
	return provider, fs.Args(), nil
}

func (c *authCLI) register(args []string) error {
	fs := c.newFlagSet("git-aico auth register", func(w io.Writer) {
		fmt.Fprint(w, `Usage: git-aico auth register [options] [provider]

Store an API key in the OS keyring. The key is prompted with echoing
disabled and is never printed.

Options:
  -p, -provider string   Provider to register: openai or anthropic

If the provider is omitted, you will be prompted to choose.
`)
	})
	flagProvider, rest, err := parseProviderFlags(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	provider, err := c.resolveProviderInput(flagProvider, rest, true)
	if err != nil {
		return err
	}

	secret, err := c.promptSecret(provider)
	if err != nil {
		return err
	}
	if err := c.store.Set(provider, secret); err != nil {
		return err
	}

	fileCfg, err := auth.LoadFileConfig(c.configPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(fileCfg.ModelProvider) == "" {
		if err := auth.SetActiveProvider(c.configPath, provider); err != nil {
			return err
		}
		fmt.Fprintf(c.stdout, "Registered %s API key. Active provider set to %s.\n", provider, provider)
		return nil
	}
	fmt.Fprintf(c.stdout, "Registered %s API key.\n", provider)
	fmt.Fprintf(c.stdout, "Active provider: %s. Use git-aico auth switch to change.\n", fileCfg.ModelProvider)
	return nil
}

func (c *authCLI) remove(args []string) error {
	fs := c.newFlagSet("git-aico auth remove", func(w io.Writer) {
		fmt.Fprint(w, `Usage: git-aico auth remove [options] [provider]

Remove a stored API key from the OS keyring.

Options:
  -p, -provider string   Provider to remove: openai or anthropic

If the provider is omitted, you will be prompted to choose.
`)
	})
	flagProvider, rest, err := parseProviderFlags(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	provider, err := c.resolveProviderInput(flagProvider, rest, true)
	if err != nil {
		return err
	}
	if err := c.store.Delete(provider); err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return fmt.Errorf("no API key registered for %s", provider)
		}
		return err
	}
	fmt.Fprintf(c.stdout, "Removed %s API key from the keyring.\n", provider)
	return nil
}

func (c *authCLI) status(args []string) error {
	fs := c.newFlagSet("git-aico auth status", func(w io.Writer) {
		fmt.Fprint(w, `Usage: git-aico auth status

Show which providers have API keys registered and which provider is
active. The raw key is never printed.

Environment variables override the keyring when set.
`)
	})
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if rest := fs.Args(); len(rest) > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(rest, " "))
	}

	fileCfg, err := auth.LoadFileConfig(c.configPath)
	if err != nil {
		return err
	}
	st := auth.LookupStatus(c.getenv, fileCfg, c.store)
	if st.ProviderErr != nil {
		fmt.Fprintf(c.stdout, "Active provider: %s (%s)\n", st.ActiveProvider, st.ProviderSource)
		fmt.Fprintf(c.stdout, "  error: %v\n\n", st.ProviderErr)
	} else {
		fmt.Fprintf(c.stdout, "Active provider: %s (%s)\n\n", st.ActiveProvider, st.ProviderSource)
	}
	for _, p := range auth.Providers {
		fmt.Fprintf(c.stdout, "  %-10s %s\n", p+":", formatKeyStatus(st.Keys[p]))
	}
	return nil
}

func formatKeyStatus(ks auth.KeyStatus) string {
	if ks.Err != nil {
		return "not registered (keyring unavailable)"
	}
	if !ks.Registered {
		return "not registered"
	}
	switch ks.Source {
	case auth.SourceEnvironment:
		return "available (environment)"
	case auth.SourceKeyring:
		return "registered (keyring)"
	default:
		return "registered (" + ks.Source + ")"
	}
}

func (c *authCLI) switchProvider(args []string) error {
	fs := c.newFlagSet("git-aico auth switch", func(w io.Writer) {
		fmt.Fprint(w, `Usage: git-aico auth switch [options] [provider]

Set the active provider to openai or anthropic. This updates local
config (~/.config/git-aico/config.yml) and does not change API keys.

Options:
  -p, -provider string   Provider to activate: openai or anthropic

If the provider is omitted, you will be prompted to choose.
`)
	})
	flagProvider, rest, err := parseProviderFlags(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	provider, err := c.resolveProviderInput(flagProvider, rest, true)
	if err != nil {
		return err
	}
	if err := auth.SetActiveProvider(c.configPath, provider); err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "Active provider set to %s.\n", provider)
	fmt.Fprintf(c.stdout, "Note: MODEL_PROVIDER in the environment overrides this setting.\n")
	return nil
}

func (c *authCLI) resolveProviderInput(flagProvider string, rest []string, allowInteractive bool) (string, error) {
	if flagProvider != "" && len(rest) > 0 {
		return "", fmt.Errorf("provide a provider via -provider or as an argument, not both")
	}
	raw := flagProvider
	if raw == "" && len(rest) > 0 {
		raw = rest[0]
		rest = rest[1:]
	}
	if len(rest) > 0 {
		return "", fmt.Errorf("unexpected arguments: %s", strings.Join(rest, " "))
	}
	if raw != "" {
		return auth.NormalizeProvider(raw)
	}
	if !allowInteractive {
		return "", fmt.Errorf("provider is required (openai or anthropic)")
	}
	return c.chooseProviderInteractive()
}

func (c *authCLI) chooseProviderInteractive() (string, error) {
	if c.isTerminal != nil && !c.isTerminal() {
		return "", fmt.Errorf("provider is required (openai or anthropic); pass -provider")
	}
	fmt.Fprintln(c.stdout, "? Choose a provider")
	for i, p := range auth.Providers {
		fmt.Fprintf(c.stdout, " %d. %s\n", i+1, p)
	}
	reader := bufio.NewReader(c.stdin)
	for {
		fmt.Fprint(c.stdout, "Enter the number of your choice: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		input = strings.TrimSpace(input)
		switch input {
		case "1":
			return auth.ProviderOpenAI, nil
		case "2":
			return auth.ProviderAnthropic, nil
		case auth.ProviderOpenAI, auth.ProviderAnthropic:
			return input, nil
		default:
			fmt.Fprintln(c.stdout, "Invalid choice, please try again.")
		}
	}
}

func (c *authCLI) promptSecret(provider string) (string, error) {
	var secret string
	if c.readSecret != nil {
		s, err := c.readSecret(provider)
		if err != nil {
			return "", err
		}
		secret = s
	} else {
		prompt := fmt.Sprintf("Enter API key for %s (input hidden): ", provider)
		fmt.Fprint(c.stderr, prompt)
		fd := int(os.Stdin.Fd())
		if term.IsTerminal(fd) {
			b, err := term.ReadPassword(fd)
			fmt.Fprintln(c.stderr)
			if err != nil {
				return "", err
			}
			secret = string(b)
		} else {
			line, err := bufio.NewReader(c.stdin).ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return "", err
			}
			secret = line
		}
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}
	return secret, nil
}
