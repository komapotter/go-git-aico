package main

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
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

func TestSpinnerUsesGhBrailleFrames(t *testing.T) {
	out := runSpinner(t, true, 5*time.Millisecond, 80*time.Millisecond)
	for _, frame := range ghSpinnerFrames {
		if !strings.Contains(out, frame) {
			t.Errorf("output missing gh frame %q: %q", frame, out)
		}
	}
	if strings.Contains(out, "|") || strings.Contains(out, "/") || strings.Contains(out, "-") {
		t.Errorf("output still looks like the ASCII spinner: %q", out)
	}
	if strings.Contains(out, "....") {
		t.Errorf("output has growing dots: %q", out)
	}
}

func TestSpinnerFixedWidthFrames(t *testing.T) {
	out := runSpinner(t, true, 8*time.Millisecond, 80*time.Millisecond)
	widths := visibleFrameWidths(out)
	if len(widths) < 2 {
		t.Fatalf("expected multiple frames, got %v from %q", widths, out)
	}
	want := widths[0]
	for i, w := range widths {
		if w != want {
			t.Fatalf("frame %d width %d != %d (output %q)", i, w, want, out)
		}
	}
}

func TestSpinnerLayoutIsLabelThenFrame(t *testing.T) {
	out := runSpinner(t, true, 8*time.Millisecond, 40*time.Millisecond)
	found := false
	for _, frame := range visibleFrames(out) {
		if !strings.HasPrefix(frame, spinnerLabel+" ") {
			t.Errorf("frame %q does not start with %q", frame, spinnerLabel+" ")
			continue
		}
		got := strings.TrimPrefix(frame, spinnerLabel+" ")
		if !containsFrame(got) {
			t.Errorf("frame %q does not end with a gh braille glyph", frame)
		}
		found = true
	}
	if !found {
		t.Fatalf("no visible frames in %q", out)
	}
}

func TestSpinnerStopClearsLineAndShowsCursor(t *testing.T) {
	var buf bytes.Buffer
	sp := testSpinner(&buf, true, 8*time.Millisecond)
	sp.start()
	time.Sleep(20 * time.Millisecond)
	sp.stop()

	out := buf.String()
	if !strings.Contains(out, hideCursorSeq) {
		t.Errorf("start did not hide cursor: %q", out)
	}
	if !strings.Contains(out, showCursorSeq) {
		t.Errorf("stop did not restore cursor: %q", out)
	}
	if !strings.Contains(out, clearLineSeq) {
		t.Errorf("stop did not clear the line: %q", out)
	}
	lastHide := strings.LastIndex(out, hideCursorSeq)
	lastShow := strings.LastIndex(out, showCursorSeq)
	lastClear := strings.LastIndex(out, clearLineSeq)
	if lastShow < lastHide || lastClear < lastShow {
		t.Errorf("expected hide, then frames, then show+clear; got %q", out)
	}
}

func TestSpinnerStopIdempotent(t *testing.T) {
	var buf bytes.Buffer
	sp := testSpinner(&buf, true, 8*time.Millisecond)
	sp.start()
	time.Sleep(15 * time.Millisecond)
	sp.stop()
	sp.stop()

	idle := newSpinner(io.Discard, true)
	idle.stop()
}

func TestSpinnerNoAnimationWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	sp := testSpinner(&buf, false, 8*time.Millisecond)
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

func testSpinner(w io.Writer, enabled bool, interval time.Duration) *spinner {
	sp := newSpinner(w, enabled)
	sp.interval = interval
	sp.color = false
	return sp
}

func runSpinner(t *testing.T, enabled bool, interval, wait time.Duration) string {
	t.Helper()
	var buf bytes.Buffer
	sp := testSpinner(&buf, enabled, interval)
	sp.start()
	time.Sleep(wait)
	sp.stop()
	return buf.String()
}

var ansiSeq = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

func stripANSI(s string) string {
	return ansiSeq.ReplaceAllString(s, "")
}

func visibleFrames(output string) []string {
	plain := stripANSI(output)
	plain = strings.ReplaceAll(plain, "\r", "\n")
	var frames []string
	for _, line := range strings.Split(plain, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		frames = append(frames, line)
	}
	return frames
}

func visibleFrameWidths(output string) []int {
	var widths []int
	for _, frame := range visibleFrames(output) {
		widths = append(widths, utf8.RuneCountInString(frame))
	}
	return widths
}

func containsFrame(s string) bool {
	for _, frame := range ghSpinnerFrames {
		if strings.Contains(s, frame) {
			return true
		}
	}
	return false
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
