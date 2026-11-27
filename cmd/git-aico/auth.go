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
	getenv     auth.EnvLookup
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	readSecret func() (string, error)
}

func newAuthCLI() (*authCLI, error) {
	return &authCLI{
		store:  auth.KeyringStore{},
		getenv: os.LookupEnv,
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
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

Store an OpenAI API key in the OS keyring (macOS Keychain,
Linux Secret Service, or Windows Credential Manager) instead of a
plaintext env file.

Commands:
  register   Store an OpenAI API key in the OS keyring
  remove     Remove the stored OpenAI API key from the OS keyring
  status     Show whether an OpenAI API key is registered

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

func (c *authCLI) register(args []string) error {
	fs := c.newFlagSet("git-aico auth register", func(w io.Writer) {
		fmt.Fprint(w, `Usage: git-aico auth register

Store an OpenAI API key in the OS keyring. The key is prompted with echoing
disabled and is never printed.
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

	secret, err := c.promptSecret()
	if err != nil {
		return err
	}
	if err := c.store.Set(auth.ProviderOpenAI, secret); err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "Registered OpenAI API key.\n")
	return nil
}

func (c *authCLI) remove(args []string) error {
	fs := c.newFlagSet("git-aico auth remove", func(w io.Writer) {
		fmt.Fprint(w, `Usage: git-aico auth remove

Remove the stored OpenAI API key from the OS keyring.
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
	if err := c.store.Delete(auth.ProviderOpenAI); err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return fmt.Errorf("no API key registered for openai")
		}
		return err
	}
	fmt.Fprintf(c.stdout, "Removed OpenAI API key from the keyring.\n")
	return nil
}

func (c *authCLI) status(args []string) error {
	fs := c.newFlagSet("git-aico auth status", func(w io.Writer) {
		fmt.Fprint(w, `Usage: git-aico auth status

Show whether an OpenAI API key is registered. The raw key is never printed.

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

	st := auth.LookupStatus(c.getenv, c.store)
	fmt.Fprintf(c.stdout, "openai: %s\n", formatKeyStatus(st.OpenAI))
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

func (c *authCLI) promptSecret() (string, error) {
	var secret string
	if c.readSecret != nil {
		s, err := c.readSecret()
		if err != nil {
			return "", err
		}
		secret = s
	} else {
		fmt.Fprint(c.stderr, "Enter OpenAI API key (input hidden): ")
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
