package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/briandowns/spinner"
)

func TestPrintHelpDocumentsVersionFlag(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed with error: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	printHelp()
	w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading help output: %v", err)
	}
	r.Close()
	out := buf.String()
	if !strings.Contains(out, "-V") || !strings.Contains(out, "Print version and exit") {
		t.Fatalf("help missing -V documentation:\n%s", out)
	}
	if !strings.Contains(out, "-v") || !strings.Contains(out, "Enable verbose output") {
		t.Fatalf("help missing -v documentation:\n%s", out)
	}
	for _, cmd := range []string{"auth register", "auth remove", "auth status", "auth switch"} {
		if !strings.Contains(out, cmd) {
			t.Fatalf("help missing %s:\n%s", cmd, out)
		}
	}
	if !strings.Contains(out, "Environment variables") && !strings.Contains(out, "OPENAI_API_KEY") {
		t.Fatalf("help missing env documentation:\n%s", out)
	}
	if !strings.Contains(out, "git-aico auth register") {
		t.Fatalf("help missing resolution fallback:\n%s", out)
	}
}

func TestVersionFlagDoesNotConflictWithVerbose(t *testing.T) {
	parse := func(args ...string) (verbose, japanese, help, version bool) {
		fs := flag.NewFlagSet("git-aico", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		registerAppFlags(fs, &verbose, &japanese, &help, &version)
		if err := fs.Parse(args); err != nil {
			t.Fatalf("Parse(%q) error: %v", args, err)
		}
		return
	}

	verbose, _, _, version := parse("-V")
	if !version {
		t.Fatal("-V should set the version flag")
	}
	if verbose {
		t.Fatal("-V must not set verbose (-v)")
	}

	verbose, _, _, version = parse("-v")
	if !verbose {
		t.Fatal("-v should set verbose")
	}
	if version {
		t.Fatal("-v must not set the version flag (-V)")
	}

	verbose, japanese, help, version := parse("-v", "-j", "-V")
	if !verbose || !japanese || help || !version {
		t.Fatalf("-v -j -V = verbose=%v japanese=%v help=%v version=%v", verbose, japanese, help, version)
	}
}

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
