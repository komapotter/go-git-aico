package main

import (
	"bytes"
	"io"
	"os"
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
		inR, inW, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() failed with error: %v", err)
		}
		os.Stdin = inR
		defer inR.Close()
		defer inW.Close()

		outR, outW, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() failed with error: %v", err)
		}
		os.Stdout = outW
		defer outR.Close()
		defer outW.Close()

		go func() {
			defer inW.Close()
			_, _ = inW.Write([]byte(test.input))
		}()
		selectedMessage, err := selectCommitMessage(suggestions)
		if err != nil {
			t.Errorf("selectCommitMessage returned an unexpected error: %v", err)
		}
		if selectedMessage != test.expected {
			t.Errorf("selectCommitMessage = %q, want %q", selectedMessage, test.expected)
		}
		outW.Close()
		outBuf := new(bytes.Buffer)
		io.Copy(outBuf, outR)
		if !strings.Contains(outBuf.String(), "Enter the number of your choice: ") {
			t.Errorf("Expected user prompt not found in output")
		}
	}
}
