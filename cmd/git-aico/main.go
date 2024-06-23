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

const openAIURL = "https://api.openai.com/v1/chat/completions"

type Config struct {
	OpenAIKey         string  `envconfig:"OPENAI_API_KEY" required:"true"`
	NumCandidates     int     `envconfig:"NUM_CANDIDATES" default:"3"`
	OpenAIModel       string  `envconfig:"OPENAI_MODEL" default:"gpt-4o"`
	OpenAITemperature float64 `envconfig:"OPENAI_TEMPERATURE" default:"0.1"`
	OpenAIMaxTokens   int     `envconfig:"OPENAI_MAX_TOKENS" default:"450"`
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
			fmt.Printf("\r  %c %s%s", spinnerChars[i%len(spinnerChars)], "Reading git diff staged ", dots)
			if time.Since(lastDotTime) >= time.Second {
				dots += "."
				lastDotTime = time.Now()
			}
			i++
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// parseOpenAIResponse takes the response from OpenAI and parses it into a list of commit message suggestions.
func parseOpenAIResponse(response string, verbose bool) ([]string, error) {
	if response == "" {
		return nil, fmt.Errorf("response from OpenAI is empty")
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

	if verbose {
		fmt.Printf("Using OpenAI model: %s\n", cfg.OpenAIModel)
	}

	// Create a question for the OpenAI API based on the diff output
	question := aico.CreateOpenAIQuestion(diffOutput, cfg.NumCandidates, japaneseOutput)
	// Ask OpenAI for commit message suggestions
	response, err := aico.AskOpenAI(openAIURL, cfg.OpenAIKey, cfg.OpenAIModel, cfg.OpenAITemperature, cfg.OpenAIMaxTokens, question, verbose)
	if err != nil {
		done <- true // Stop the spinner
		fmt.Println("Error asking OpenAI:", err)
		return
	}

	// Stop the spinner
	done <- true

	// Split the response into separate lines
	messages, err := parseOpenAIResponse(response, verbose)
	if err != nil {
		fmt.Println("Error split the response:", err)
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
