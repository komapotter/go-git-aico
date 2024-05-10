package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSelectCommitMessage(t *testing.T) {
	suggestions := []string{
		"Update README with new installation instructions",
		"Fix off-by-one error in the pagination logic",
		"Refactor user authentication to use middleware",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"1\n", suggestions[0]},
		{"2\n", suggestions[1]},
		{"3\n", suggestions[2]},
	}

	for _, test := range tests {
		outBuf := new(bytes.Buffer)
		reader := strings.NewReader(test.input)
		writer := outBuf

		selectedMessage, err := selectCommitMessage(suggestions, reader, writer)
		if err != nil {
			t.Errorf("selectCommitMessage returned an unexpected error: %v", err)
		}
		if selectedMessage != test.expected {
			t.Errorf("selectCommitMessage = %q, want %q", selectedMessage, test.expected)
		}
		if !strings.Contains(outBuf.String(), "Enter the number of your choice: ") {
			t.Errorf("Expected user prompt not found in output")
		}
	}
}
