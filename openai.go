package aico

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	OpenAIKey     string `envconfig:"OPENAI_API_KEY" required:"true"`
	NumCandidates int    `envconfig:"NUM_CANDIDATES" default:"3"`
}

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponse struct {
	Choices []struct {
		Index        int           `json:"index"`
		Message      OpenAIMessage `json:"message"`
		LogProbs     interface{}   `json:"logprobs"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
}

// GenerateCommitMessages takes the output of `git diff` and generates three commit message suggestions.
func GenerateCommitMessages(diffOutput, openAIURL string, verbose bool) ([]string, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, err
	}

	// Create a question for the OpenAI API based on the diff output
	question := CreateOpenAIQuestion(diffOutput, cfg.NumCandidates)
	response, err := askOpenAI(openAIURL, cfg.OpenAIKey, question, verbose) // Now passing the verbose argument
	if err != nil {
		return nil, err
	}

	// Split the response into separate lines
	suggestions := strings.Split(response, "\n")
	for i, suggestion := range suggestions {
		suggestions[i] = strings.TrimPrefix(suggestion, "- ")
	}

	return suggestions, nil
}

func askOpenAI(openAIURL, openAIKey, question string, verbose bool) (string, error) {
	data := OpenAIRequest{
		Messages:    []OpenAIMessage{{Role: "user", Content: question}},
		Model:       "gpt-4-turbo", // Use an appropriate model
		Temperature: 0.1,           // Optional: control randomness
		MaxTokens:   450,           // Limit the length of the response
	}
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", openAIURL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openAIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-OK HTTP status from OpenAI: %s", resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if verbose {
		fmt.Println("Raw response from OpenAI:", string(respBody)) // Debugging line to print raw response
	}

	var apiResp OpenAIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", err
	}

	if len(apiResp.Choices) > 0 && apiResp.Choices[0].Message.Role == "assistant" {
		// Extract the content from the assistant's message
		//fmt.Println(apiResp.Choices[0].Message.Content)
		return strings.TrimSpace(apiResp.Choices[0].Message.Content), nil
	}

	return "", fmt.Errorf("no response from OpenAI")
}
