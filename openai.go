package aico

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
)

// DefaultOpenAIReasoningEffort is the Chat Completions default for go-git-aico
// (short commit-message candidates). gpt-6-luna accepts temperature and
// function calling only when reasoning_effort is "none".
// See https://developers.openai.com/api/docs/models/gpt-6-luna
const DefaultOpenAIReasoningEffort = "none"

type OpenAIRequest struct {
	Model               string          `json:"model"`
	Messages            []OpenAIMessage `json:"messages"`
	MaxCompletionTokens int             `json:"max_completion_tokens"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
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

// NormalizeOpenAIReasoningEffort lowercases and validates OPENAI_REASONING_EFFORT.
func NormalizeOpenAIReasoningEffort(s string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "" {
		v = DefaultOpenAIReasoningEffort
	}
	switch v {
	case "none", "low", "medium", "high", "xhigh", "max":
		return v, nil
	default:
		return "", fmt.Errorf("invalid OPENAI_REASONING_EFFORT %q (allowed: none, low, medium, high, xhigh, max)", s)
	}
}

// isReasoningModel reports whether model needs Chat Completions reasoning_effort
// (gpt-6*, gpt-5*, and o-series such as o1 / o3 / o4-mini).
func isReasoningModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(m, "gpt-6") || strings.HasPrefix(m, "gpt-5") {
		return true
	}
	if len(m) >= 2 && m[0] == 'o' && unicode.IsDigit(rune(m[1])) {
		return true
	}
	return false
}

func buildOpenAIRequest(model, question, reasoningEffort string, temperature float64, maxTokens int) OpenAIRequest {
	req := OpenAIRequest{
		Messages:            []OpenAIMessage{{Role: "user", Content: question}},
		Model:               model,
		MaxCompletionTokens: maxTokens,
	}
	if isReasoningModel(model) {
		req.ReasoningEffort = reasoningEffort
	}
	if !isReasoningModel(model) || reasoningEffort == "none" {
		t := temperature
		req.Temperature = &t
	}
	return req
}

func AskOpenAI(openAIURL, openAIKey, openAIModel, reasoningEffort string, openAITemperature float64, openAIMaxTokens int, question string, verbose bool) (string, error) {
	data := buildOpenAIRequest(openAIModel, question, reasoningEffort, openAITemperature, openAIMaxTokens)
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
		return strings.TrimSpace(apiResp.Choices[0].Message.Content), nil
	}

	return "", fmt.Errorf("no response from OpenAI")
}
