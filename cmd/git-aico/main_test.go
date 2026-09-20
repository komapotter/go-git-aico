package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/briandowns/spinner"
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

func TestParseModelResponse(t *testing.T) {
	tests := []struct {
		name         string
		response     string
		wantMessages []string
		wantErr      bool
	}{
		{
			name:         "valid response(new-line-code)",
			response:     "Suggest a commit message\nImprove code readability\nRefactor subsystem X for clarity",
			wantMessages: []string{"Suggest a commit message", "Improve code readability", "Refactor subsystem X for clarity"},
			wantErr:      false,
		},
		{
			name:         "valid response(hyphen-with-new-line-code)",
			response:     "- Suggest a commit message\n- Improve code readability\n- Refactor subsystem X for clarity",
			wantMessages: []string{"Suggest a commit message", "Improve code readability", "Refactor subsystem X for clarity"},
			wantErr:      false,
		},
		{
			name:         "valid response(double-new-line-code)",
			response:     "Suggest a commit message\n\nImprove code readability\n\nRefactor subsystem X for clarity",
			wantMessages: []string{"Suggest a commit message", "Improve code readability", "Refactor subsystem X for clarity"},
			wantErr:      false,
		},
		{
			name:         "empty response",
			response:     "",
			wantMessages: nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMessages, err := parseModelResponse(tt.response, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseModelResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !equalSlices(gotMessages, tt.wantMessages) {
				t.Errorf("parseModelResponse() = %v, want %v", gotMessages, tt.wantMessages)
			}
		})
	}
}

func TestSpinnerUsesBriandownsCharSet11(t *testing.T) {
	want := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
	if !equalSlices(spinner.CharSets[11], want) {
		t.Fatalf("spinner.CharSets[11] = %q, want %q", spinner.CharSets[11], want)
	}
	sp := newSpinner(io.Discard, true)
	if sp.inner.Delay != ghSpinnerInterval {
		t.Fatalf("Delay = %v, want %v", sp.inner.Delay, ghSpinnerInterval)
	}
	if !sp.inner.HideCursor {
		t.Fatal("HideCursor = false, want true")
	}
}

func TestSpinnerFixedWidthFrames(t *testing.T) {
	for i, frame := range spinner.CharSets[11] {
		if n := utf8.RuneCountInString(frame); n != 1 {
			t.Fatalf("CharSets[11][%d] = %q has width %d, want 1", i, frame, n)
		}
	}
}

func TestSpinnerLayoutIsFrameThenLabel(t *testing.T) {
	sp := newSpinner(io.Discard, true)
	if sp.inner.Prefix != "" {
		t.Fatalf("Prefix = %q, want empty so the braille glyph comes first", sp.inner.Prefix)
	}
	if sp.inner.Suffix != " "+spinnerLabel {
		t.Fatalf("Suffix = %q, want %q", sp.inner.Suffix, " "+spinnerLabel)
	}
}

func TestSpinnerStopIdempotent(t *testing.T) {
	sp := newSpinner(io.Discard, true)
	sp.start()
	time.Sleep(15 * time.Millisecond)
	sp.stop()
	sp.stop()

	idle := newSpinner(io.Discard, true)
	idle.stop()
}

func TestSpinnerNoAnimationWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	sp := newSpinner(&buf, false)
	sp.start()
	time.Sleep(20 * time.Millisecond)
	sp.stop()
	if buf.Len() != 0 {
		t.Fatalf("disabled spinner wrote %q", buf.String())
	}
}

func TestIsCharDevicePipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	if isCharDevice(w) || isCharDevice(r) {
		t.Fatal("pipe should not be treated as a TTY")
	}
}

// equalSlices checks if two slices of strings are equal
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
