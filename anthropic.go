package aico

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type AnthropicRequest struct {
	Model       string              `json:"model"`
	Messages    []AnthropicMessage  `json:"messages"`
	MaxTokens   int                 `json:"max_tokens"`
	Temperature float64             `json:"temperature"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicResponse struct {
	Content []struct {
		Type  string `json:"type"`
		Text  string `json:"text"`
	} `json:"content"`
}

func AskAnthropic(anthropicURL, anthropicKey, anthropicModel string, anthropicTemperature float64, anthropicMaxTokens int, question string, verbose bool) (string, error) {
	data := AnthropicRequest{
		Messages:    []AnthropicMessage{{Role: "user", Content: question}},
		Model:       anthropicModel,
		Temperature: anthropicTemperature,
		MaxTokens:   anthropicMaxTokens,
	}
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", anthropicURL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", anthropicKey)
	req.Header.Set("Anthropic-Version", "2023-06-01")

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
		return "", fmt.Errorf("received non-OK HTTP status from Anthropic: %s, response body: %s", resp.Status, string(respBody))
	}

	if verbose {
		fmt.Printf("\nRaw response from Anthropic: %v", string(respBody))
	}

	var apiResp AnthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", err
	}

	if len(apiResp.Content) > 0 && apiResp.Content[0].Type == "text" {
		return strings.TrimSpace(apiResp.Content[0].Text), nil
	}

	return "", fmt.Errorf("no response from Anthropic")
}
