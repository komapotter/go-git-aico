package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	aico "github.com/komapotter/go-git-aico"
)

const openAIURL = "https://api.openai.com/v1/chat/completions"

var verbose bool // Global flag to control verbose output

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

func main() {
	verbose = false // Default verbose to false
	if len(os.Args) > 1 && os.Args[1] == "-v" {
		verbose = true
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

	// Generate commit messages based on the diff
	messages, err := aico.GenerateCommitMessages(diffOutput, openAIURL, verbose)
	if err != nil {
		done <- true // Stop the spinner
		fmt.Println("Error generating commit messages:", err)
		return
	}

	// Stop the spinner
	done <- true

	if len(messages) == 0 {
		fmt.Println("Error: No commit message candidates were generated")
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
