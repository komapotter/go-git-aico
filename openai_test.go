package aico

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAskOpenAI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type header not set correctly")
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("Authorization header not set correctly")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal("Failed to read request body:", err)
		}

		var keys map[string]json.RawMessage
		if err := json.Unmarshal(body, &keys); err != nil {
			t.Fatal("Failed to parse request body:", err)
		}
		if _, ok := keys["max_tokens"]; ok {
			t.Error("request must not send max_tokens; gpt-6-luna expects max_completion_tokens")
		}

		var reqBody OpenAIRequest
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Fatal("Failed to parse request body:", err)
		}

		if reqBody.Model != "gpt-6-luna" {
			t.Errorf("Expected model to be gpt-6-luna, got %s", reqBody.Model)
		}
		if reqBody.Temperature != 0.2 {
			t.Errorf("Expected temperature to be 0.2, got %f", reqBody.Temperature)
		}
		if reqBody.MaxCompletionTokens != 300 {
			t.Errorf("Expected max_completion_tokens to be 300, got %d", reqBody.MaxCompletionTokens)
		}
		if reqBody.ReasoningEffort != "none" {
			t.Errorf("Expected reasoning_effort to be none, got %s", reqBody.ReasoningEffort)
		}
		if len(reqBody.Messages) != 1 || reqBody.Messages[0].Role != "user" || reqBody.Messages[0].Content != "test question" {
			t.Errorf("Unexpected messages field: %v", reqBody.Messages)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "test response"
					},
					"finish_reason": "stop"
				}
			]
		}`))
	}))
	defer server.Close()

	response, err := AskOpenAI(
		server.URL,
		"test-key",
		"gpt-6-luna",
		0.2,
		300,
		"test question",
		false,
	)
	if err != nil {
		t.Error("Expected no error, got:", err)
	}
	if response != "test response" {
		t.Errorf("Expected response to be 'test response', got: %s", response)
	}
}

func TestAskOpenAIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "test error"}`))
	}))
	defer server.Close()

	_, err := AskOpenAI(
		server.URL,
		"test-key",
		"gpt-6-luna",
		0.2,
		300,
		"test question",
		false,
	)
	if err == nil {
		t.Error("Expected an error, got nil")
	}
}

func TestOpenAIRequestJSON(t *testing.T) {
	payload, err := json.Marshal(OpenAIRequest{
		Model:               "gpt-6-luna",
		Messages:            []OpenAIMessage{{Role: "user", Content: "hello"}},
		MaxCompletionTokens: 450,
		ReasoningEffort:     openAIChatCompletionsReasoningEffort,
		Temperature:         0.1,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(payload)
	for _, want := range []string{`"model":"gpt-6-luna"`, `"max_completion_tokens":450`, `"reasoning_effort":"none"`, `"temperature":0.1`} {
		if !strings.Contains(s, want) {
			t.Errorf("payload missing %s: %s", want, s)
		}
	}
	if strings.Contains(s, `"max_tokens"`) {
		t.Errorf("payload must not include max_tokens: %s", s)
	}
}
