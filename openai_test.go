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
		if reqBody.Temperature == nil || *reqBody.Temperature != 0.2 {
			t.Errorf("Expected temperature to be 0.2, got %v", reqBody.Temperature)
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
		"none",
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
		"none",
		0.2,
		300,
		"test question",
		false,
	)
	if err == nil {
		t.Error("Expected an error, got nil")
	}
}

func TestOpenAIRequestPayloads(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		effort     string
		temp       float64
		wantEffort string
		wantTemp   *float64
	}{
		{
			name:       "default luna none+temp",
			model:      "gpt-6-luna",
			effort:     DefaultOpenAIReasoningEffort,
			temp:       0.1,
			wantEffort: "none",
			wantTemp:   floatPtr(0.1),
		},
		{
			name:       "luna low omits temperature",
			model:      "gpt-6-luna",
			effort:     "low",
			temp:       0.1,
			wantEffort: "low",
			wantTemp:   nil,
		},
		{
			name:       "gpt-4o omits reasoning_effort",
			model:      "gpt-4o",
			effort:     "none",
			temp:       0.1,
			wantEffort: "",
			wantTemp:   floatPtr(0.1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := json.Marshal(buildOpenAIRequest(tt.model, "hello", tt.effort, tt.temp, 450))
			if err != nil {
				t.Fatal(err)
			}
			var keys map[string]json.RawMessage
			if err := json.Unmarshal(payload, &keys); err != nil {
				t.Fatal(err)
			}
			if _, ok := keys["max_tokens"]; ok {
				t.Errorf("payload must not include max_tokens: %s", payload)
			}

			gotEffort, hasEffort := keys["reasoning_effort"]
			if tt.wantEffort == "" {
				if hasEffort {
					t.Errorf("reasoning_effort = %s, want omitted", gotEffort)
				}
			} else {
				if !hasEffort {
					t.Fatal("reasoning_effort missing")
				}
				var v string
				if err := json.Unmarshal(gotEffort, &v); err != nil {
					t.Fatal(err)
				}
				if v != tt.wantEffort {
					t.Errorf("reasoning_effort = %q, want %q", v, tt.wantEffort)
				}
			}

			gotTemp, hasTemp := keys["temperature"]
			if tt.wantTemp == nil {
				if hasTemp {
					t.Errorf("temperature = %s, want omitted", gotTemp)
				}
			} else {
				if !hasTemp {
					t.Fatal("temperature missing")
				}
				var v float64
				if err := json.Unmarshal(gotTemp, &v); err != nil {
					t.Fatal(err)
				}
				if v != *tt.wantTemp {
					t.Errorf("temperature = %v, want %v", v, *tt.wantTemp)
				}
			}
		})
	}
}

func TestIsReasoningModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"gpt-6-luna", true},
		{"gpt-6-astra", true},
		{"GPT-6.1-sol", true},
		{"gpt-5", true},
		{"gpt-5.6-sol", true},
		{"o1", true},
		{"o1-mini", true},
		{"o3-mini", true},
		{"o4-mini", true},
		{"gpt-4o", false},
		{"gpt-4o-mini", false},
		{"gpt-4.1", false},
		{"gpt-3.5-turbo", false},
	}
	for _, tt := range tests {
		if got := isReasoningModel(tt.model); got != tt.want {
			t.Errorf("isReasoningModel(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestNormalizeOpenAIReasoningEffort(t *testing.T) {
	got, err := NormalizeOpenAIReasoningEffort(" HIGH ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "high" {
		t.Fatalf("got %q, want high", got)
	}
	if _, err := NormalizeOpenAIReasoningEffort("minimal"); err == nil {
		t.Fatal("expected error for invalid effort")
	} else if !strings.Contains(err.Error(), "OPENAI_REASONING_EFFORT") {
		t.Fatalf("error %v should mention OPENAI_REASONING_EFFORT", err)
	}
}

func floatPtr(v float64) *float64 { return &v }
