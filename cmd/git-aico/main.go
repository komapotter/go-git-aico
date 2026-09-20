package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kelseyhightower/envconfig"

	aico "github.com/komapotter/go-git-aico"
	"github.com/komapotter/go-git-aico/internal/auth"
)

const (
	openAIURL    = "https://api.openai.com/v1/chat/completions"
	anthropicURL = "https://api.anthropic.com/v1/messages"
)

type Config struct {
	// API Keys
	OpenAIKey    string `envconfig:"OPENAI_API_KEY"`
	AnthropicKey string `envconfig:"ANTHROPIC_API_KEY"`

	// General config
	NumCandidates int    `envconfig:"NUM_CANDIDATES" default:"3"`
	ModelProvider string `envconfig:"MODEL_PROVIDER"` // "openai" or "anthropic"; resolved with config/keyring if unset

	// OpenAI config
	OpenAIModel       string  `envconfig:"OPENAI_MODEL" default:"gpt-4o"`
	OpenAITemperature float64 `envconfig:"OPENAI_TEMPERATURE" default:"0.1"`
	OpenAIMaxTokens   int     `envconfig:"OPENAI_MAX_TOKENS" default:"450"`

	// Anthropic config
	AnthropicModel       string  `envconfig:"ANTHROPIC_MODEL" default:"claude-3-haiku-20240307"`
	AnthropicTemperature float64 `envconfig:"ANTHROPIC_TEMPERATURE" default:"0.1"`
	AnthropicMaxTokens   int     `envconfig:"ANTHROPIC_MAX_TOKENS" default:"450"`
}

var (
	verbose        bool // Global flag to control verbose output
	japaneseOutput bool // Global flag to control Japanese output
)

// selectCommitMessage prompts the user to select a commit message from a list of suggestions.
func selectCommitMessage(suggestions []string) (string, error) {
	fmt.Println("? Choose a commit message")
	for i, suggestion := range suggestions {
		fmt.Printf(" %d. %s\n", i+1, strings.TrimSpace(suggestion))
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter the number of your choice: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		input = strings.TrimSpace(input)
		if input == "exit" {
			os.Exit(0)
		}
		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(suggestions) {
			fmt.Println("Invalid choice, please try again.")
			continue
		}
		return suggestions[choice-1], nil
	}
}

// parseModelResponse takes the response from the LLM and parses it into a list of commit message suggestions.
func parseModelResponse(response string, verbose bool) ([]string, error) {
	if response == "" {
		return nil, fmt.Errorf("response from model is empty")
	}

	var messages []string
	for _, line := range strings.Split(strings.TrimSpace(response), "\n") {
		trimmedLine := strings.TrimPrefix(line, "- ")
		if trimmedLine != "" {
			messages = append(messages, trimmedLine)
		}
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("no commit messages found in the response")
	}

	// Optionally print the candidate messages
	if verbose {
		fmt.Println("Candidate messages:")
		for _, message := range messages {
			fmt.Printf("msg: %v\n", message)
		}
	}

	return messages, nil
}

func registerAppFlags(fs *flag.FlagSet, verbose, japanese, showHelp, showVersion *bool) {
	fs.BoolVar(verbose, "v", false, "Enable verbose output")
	fs.BoolVar(japanese, "j", false, "Output commit message suggestions in Japanese")
	fs.BoolVar(showHelp, "h", false, "Show this help message")
	fs.BoolVar(showVersion, "V", false, "Print version and exit")
}

func printHelp() {
	helpText := `
Usage: git-aico [options]
       git-aico auth <command>

Options:
  -h        Show this help message
  -V        Print version and exit
  -v        Enable verbose output
  -j        Output commit message suggestions in Japanese

Auth commands:
  git-aico auth register   Store an API key in the OS keyring
  git-aico auth remove     Remove a stored API key from the OS keyring
  git-aico auth status     Show registered providers and the active provider
  git-aico auth switch     Switch active provider (openai or anthropic)

API key resolution order:
  1. Environment variables (OPENAI_API_KEY / ANTHROPIC_API_KEY / MODEL_PROVIDER)
  2. OS keyring + local config (~/.config/git-aico/config.yml)
  3. Error suggesting: git-aico auth register

Existing env-only workflows keep working without auth register.
On macOS, the first Keychain access may show a permission dialog.

Environment Variables:
  MODEL_PROVIDER       Model provider to use: "openai" or "anthropic" (default: openai, or local config)
  NUM_CANDIDATES       Number of commit message candidates to generate (default: 3)

  # OpenAI Configuration
  OPENAI_API_KEY       Your OpenAI API key (required when MODEL_PROVIDER=openai unless registered)
  OPENAI_MODEL         OpenAI model to use (default: gpt-4o)
  OPENAI_TEMPERATURE   Sampling temperature (default: 0.1)
  OPENAI_MAX_TOKENS    Maximum number of tokens in the response (default: 450)

  # Anthropic Configuration
  ANTHROPIC_API_KEY    Your Anthropic API key (required when MODEL_PROVIDER=anthropic unless registered)
  ANTHROPIC_MODEL      Anthropic model to use (default: claude-3-haiku-20240307)
  ANTHROPIC_TEMPERATURE Sampling temperature (default: 0.1)
  ANTHROPIC_MAX_TOKENS Maximum number of tokens in the response (default: 450)
`
	fmt.Println(helpText)
}

func isAuthCommand(args []string) bool {
	return len(args) > 0 && args[0] == "auth"
}

// credentialStore is the secret backend used when generating commits.
// Tests replace it with an in-memory store; production uses the OS keyring.
var credentialStore auth.Store = auth.KeyringStore{}

func loadAppConfig() (Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return cfg, fmt.Errorf("reading envvars: %w", err)
	}

	configPath, err := auth.DefaultConfigPath()
	if err != nil {
		return cfg, err
	}
	fileCfg, err := auth.LoadFileConfig(configPath)
	if err != nil {
		return cfg, err
	}
	creds, err := auth.Resolve(os.LookupEnv, fileCfg, credentialStore)
	if err != nil {
		return cfg, err
	}
	cfg.OpenAIKey = creds.OpenAIKey
	cfg.AnthropicKey = creds.AnthropicKey
	cfg.ModelProvider = creds.ModelProvider
	if err := creds.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func main() {
	if isAuthCommand(os.Args[1:]) {
		if err := runAuth(os.Args[2:]); err != nil {
			if !errors.Is(err, errAuthUsage) {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			os.Exit(1)
		}
		return
	}

	var showHelp, showVersion bool
	registerAppFlags(flag.CommandLine, &verbose, &japaneseOutput, &showHelp, &showVersion)
	flag.Parse()

	if showHelp {
		printHelp()
		return
	}
	if showVersion {
		fmt.Println(versionString())
		return
	}

	cfg, err := loadAppConfig()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Execute git diff and get the output
	diffOutput, err := aico.ExecuteGitDiffStaged()
	if err != nil {
		fmt.Println("Error reading diff:", err)
		return
	}

	if diffOutput == "" {
		fmt.Println("No changes detected")
		return
	}

	if verbose {
		if cfg.ModelProvider == "openai" {
			fmt.Printf("Using OpenAI model: %s\n", cfg.OpenAIModel)
		} else {
			fmt.Printf("Using Anthropic model: %s\n", cfg.AnthropicModel)
		}
	}

	sp := newSpinner(os.Stderr, stdoutAndStderrAreTTY())
	sp.start()
	defer sp.stop()

	// Create a question based on the diff output
	question := aico.CreateAIQuestion(diffOutput, cfg.NumCandidates, japaneseOutput)

	var response string
	// Call the appropriate API based on the selected provider
	if cfg.ModelProvider == "openai" {
		response, err = aico.AskOpenAI(openAIURL, cfg.OpenAIKey, cfg.OpenAIModel, cfg.OpenAITemperature, cfg.OpenAIMaxTokens, question, verbose)
	} else { // anthropic
		response, err = aico.AskAnthropic(anthropicURL, cfg.AnthropicKey, cfg.AnthropicModel, cfg.AnthropicTemperature, cfg.AnthropicMaxTokens, question, verbose)
	}
	sp.stop()

	if err != nil {
		fmt.Printf("Error asking %s: %v\n", strings.Title(cfg.ModelProvider), err)
		return
	}

	// Split the response into separate lines
	messages, err := parseModelResponse(response, verbose)
	if err != nil {
		fmt.Println("Error parsing the response:", err)
		return
	}

	// Check if the number of messages matches the expected number of candidates
	if len(messages) != cfg.NumCandidates {
		fmt.Printf("Error: Expected %d commit message candidates, but got %d\n", cfg.NumCandidates, len(messages))
		return
	}

	// Prompt the user to select a commit message
	selectedMessage, err := selectCommitMessage(messages)
	if err != nil {
		fmt.Println("Error selecting commit message:", err)
		return
	}

	// Commit the changes with the selected commit message
	if err := aico.CommitChanges(selectedMessage); err != nil {
		fmt.Println("Error committing changes:", err)
		return
	}

	fmt.Println("Changes committed successfully with message:", selectedMessage)
}
