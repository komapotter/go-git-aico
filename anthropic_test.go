package aico

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAskAnthropic(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type header not set correctly")
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Error("X-API-Key header not set correctly")
		}
		if r.Header.Get("Anthropic-Version") != "2023-06-01" {
			t.Error("Anthropic-Version header not set correctly")
		}

		// Parse request body
		var reqBody AnthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Error("Failed to parse request body:", err)
		}

		// Check request body fields
		if reqBody.Model != "claude-test-model" {
			t.Errorf("Expected model to be claude-test-model, got %s", reqBody.Model)
		}
		if reqBody.Temperature != 0.2 {
			t.Errorf("Expected temperature to be 0.2, got %f", reqBody.Temperature)
		}
		if reqBody.MaxTokens != 300 {
			t.Errorf("Expected max_tokens to be 300, got %d", reqBody.MaxTokens)
		}
		if len(reqBody.Messages) != 1 || reqBody.Messages[0].Role != "user" || reqBody.Messages[0].Content != "test question" {
			t.Errorf("Unexpected messages field: %v", reqBody.Messages)
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := `{
			"content": [
				{
					"type": "text",
					"text": "test response"
				}
			]
		}`
		w.Write([]byte(resp))
	}))
	defer server.Close()

	// Call the function
	response, err := AskAnthropic(
		server.URL,
		"test-key",
		"claude-test-model",
		0.2,
		300,
		"test question",
		false,
	)

	// Check results
	if err != nil {
		t.Error("Expected no error, got:", err)
	}
	if response != "test response" {
		t.Errorf("Expected response to be 'test response', got: %s", response)
	}
}

func TestAskAnthropicError(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "test error"}`))
	}))
	defer server.Close()

	// Call the function
	_, err := AskAnthropic(
		server.URL,
		"test-key",
		"claude-test-model",
		0.2,
		300,
		"test question",
		false,
	)

	// Check results
	if err == nil {
		t.Error("Expected an error, got nil")
	}
}
