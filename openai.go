package aico

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

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

func AskOpenAI(openAIURL, openAIKey, openAIModel string, openAITemperature float64, openAIMaxTokens int, question string, verbose bool) (string, error) {
	data := OpenAIRequest{
		Messages:    []OpenAIMessage{{Role: "user", Content: question}},
		Model:       openAIModel,       // Use the model from the configuration
		Temperature: openAITemperature, // Use the temperature from the configuration
		MaxTokens:   openAIMaxTokens,   // Use the max tokens from the configuration
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-OK HTTP status from OpenAI: %s, response body: %s", resp.Status, string(respBody))
	}

	if verbose {
		fmt.Printf("\nRaw response from OpenAI: %v", string(respBody)) // Debugging line to print raw response
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
