package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// gh / briandowns spinner CharSets[11]
var ghSpinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

const (
	ghSpinnerInterval = 120 * time.Millisecond
	spinnerLabel      = "Generating commit messages"
	hideCursorSeq     = "\033[?25l"
	showCursorSeq     = "\033[?25h"
	clearLineSeq      = "\r\033[K"
	cyanSeq           = "\033[36m"
	resetSeq          = "\033[0m"
)

type spinner struct {
	w          io.Writer
	frames     []string
	interval   time.Duration
	label      string
	color      bool
	enabled    bool
	hideCursor bool
	started    bool
	stopCh     chan struct{}
	doneCh     chan struct{}
	once       sync.Once
}

func newSpinner(w io.Writer, enabled bool) *spinner {
	return &spinner{
		w:          w,
		frames:     ghSpinnerFrames,
		interval:   ghSpinnerInterval,
		label:      spinnerLabel,
		color:      enabled && os.Getenv("NO_COLOR") == "",
		enabled:    enabled,
		hideCursor: enabled,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

func stdoutAndStderrAreTTY() bool {
	return isCharDevice(os.Stdout) && isCharDevice(os.Stderr)
}

func isCharDevice(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func (s *spinner) start() {
	if !s.enabled || s.started {
		return
	}
	s.started = true
	if s.hideCursor {
		fmt.Fprint(s.w, hideCursorSeq)
	}
	go s.loop()
}

func (s *spinner) loop() {
	defer close(s.doneCh)
	i := 0
	for {
		frame := s.frames[i%len(s.frames)]
		if s.color {
			frame = cyanSeq + frame + resetSeq
		}
		fmt.Fprintf(s.w, "\r%s %s", s.label, frame)
		i++
		select {
		case <-s.stopCh:
			return
		case <-time.After(s.interval):
		}
	}
}

func (s *spinner) stop() {
	s.once.Do(func() {
		if !s.started {
			return
		}
		close(s.stopCh)
		<-s.doneCh
		if s.hideCursor {
			fmt.Fprint(s.w, showCursorSeq)
		}
		fmt.Fprint(s.w, clearLineSeq)
	})
}
