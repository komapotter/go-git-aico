package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"

	aico "github.com/komapotter/go-git-aico"
)

const (
	openAIURL    = "https://api.openai.com/v1/chat/completions"
	anthropicURL = "https://api.anthropic.com/v1/messages"
)

type Config struct {
	// API Keys
	OpenAIKey     string `envconfig:"OPENAI_API_KEY"`
	AnthropicKey  string `envconfig:"ANTHROPIC_API_KEY"`
	
	// General config
	NumCandidates int     `envconfig:"NUM_CANDIDATES" default:"3"`
	ModelProvider string  `envconfig:"MODEL_PROVIDER" default:"openai"` // "openai" or "anthropic"
	
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

// startSpinner starts a simple console spinner
func startSpinner(done chan bool) {
	spinnerChars := `|/-\`
	i := 0
	dots := ""
	lastDotTime := time.Now()
	for {
		select {
		case <-done:
			fmt.Printf("\r\033[K") // Clear the entire line when done
			return
		default:
			fmt.Printf("\r  %c %s%s", spinnerChars[i%len(spinnerChars)], "Generating commit messages ", dots)
			if time.Since(lastDotTime) >= time.Second {
				dots += "."
				lastDotTime = time.Now()
			}
			i++
			time.Sleep(100 * time.Millisecond)
		}
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

func printHelp() {
	helpText := `
Usage: git-aico [options]

Options:
  -h        Show this help message
  -v        Enable verbose output
  -j        Output commit message suggestions in Japanese

Environment Variables:
  MODEL_PROVIDER       Model provider to use: "openai" or "anthropic" (default: openai)
  NUM_CANDIDATES       Number of commit message candidates to generate (default: 3)

  # OpenAI Configuration
  OPENAI_API_KEY       Your OpenAI API key (required when MODEL_PROVIDER=openai)
  OPENAI_MODEL         OpenAI model to use (default: gpt-4o)
  OPENAI_TEMPERATURE   Sampling temperature (default: 0.1)
  OPENAI_MAX_TOKENS    Maximum number of tokens in the response (default: 450)

  # Anthropic Configuration
  ANTHROPIC_API_KEY    Your Anthropic API key (required when MODEL_PROVIDER=anthropic)
  ANTHROPIC_MODEL      Anthropic model to use (default: claude-3-haiku-20240307)
  ANTHROPIC_TEMPERATURE Sampling temperature (default: 0.1)
  ANTHROPIC_MAX_TOKENS Maximum number of tokens in the response (default: 450)
`
	fmt.Println(helpText)
}

func main() {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		fmt.Println("Error reading envvars", err)
		return
	}

	flag.BoolVar(&verbose, "v", false, "Enable verbose output")
	flag.BoolVar(&japaneseOutput, "j", false, "Output commit message suggestions in Japanese")
	showHelp := flag.Bool("h", false, "Show this help message")

	flag.Parse()

	if *showHelp {
		printHelp()
		return
	}

	// Validate required API keys based on selected provider
	switch cfg.ModelProvider {
	case "openai":
		if cfg.OpenAIKey == "" {
			fmt.Println("Error: OPENAI_API_KEY is required when MODEL_PROVIDER=openai")
			return
		}
	case "anthropic":
		if cfg.AnthropicKey == "" {
			fmt.Println("Error: ANTHROPIC_API_KEY is required when MODEL_PROVIDER=anthropic")
			return
		}
	default:
		fmt.Printf("Error: Unknown model provider: %s. Supported providers are 'openai' and 'anthropic'\n", cfg.ModelProvider)
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

	// Start the spinner
	done := make(chan bool)
	go startSpinner(done)

	// Create a question based on the diff output
	question := aico.CreateAIQuestion(diffOutput, cfg.NumCandidates, japaneseOutput)

	var response string
	// Call the appropriate API based on the selected provider
	if cfg.ModelProvider == "openai" {
		if verbose {
			fmt.Printf("Using OpenAI model: %s\n", cfg.OpenAIModel)
		}
		response, err = aico.AskOpenAI(openAIURL, cfg.OpenAIKey, cfg.OpenAIModel, cfg.OpenAITemperature, cfg.OpenAIMaxTokens, question, verbose)
	} else { // anthropic
		if verbose {
			fmt.Printf("Using Anthropic model: %s\n", cfg.AnthropicModel)
		}
		response, err = aico.AskAnthropic(anthropicURL, cfg.AnthropicKey, cfg.AnthropicModel, cfg.AnthropicTemperature, cfg.AnthropicMaxTokens, question, verbose)
	}

	if err != nil {
		done <- true // Stop the spinner
		fmt.Printf("Error asking %s: %v\n", strings.Title(cfg.ModelProvider), err)
		return
	}

	// Stop the spinner
	done <- true

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
