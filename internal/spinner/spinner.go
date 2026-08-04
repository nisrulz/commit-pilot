// Package spinner animates a working indicator while a long-running operation
// runs. It renders nothing when the output is not a terminal, so captured or
// piped output stays clean.
package spinner

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// spinnerFrames is a smooth braille spinner sequence.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Handle is a running spinner that can be paused while other output is written
// to the terminal and resumed afterwards, keeping streamed lines free of
// animation frames.
type Handle struct {
	mu       sync.Mutex
	stop     chan struct{}
	running  bool
	stopped  bool
	terminal bool
}

// Start animates a working indicator on stdout and returns a function that
// stops and clears it. It is a no-op when stdout is not a terminal.
func Start() func() {
	return newHandle(true).Stop
}

// StartPaused returns a spinner that stays hidden until Resume is called, so a
// caller can stream output first and only show the working indicator once the
// busy phase begins. It is a no-op when stdout is not a terminal.
func StartPaused() *Handle {
	return newHandle(false)
}

// newHandle builds a spinner in the given starting state and starts animating
// it when stdout is a terminal.
func newHandle(running bool) *Handle {
	h := &Handle{
		stop:     make(chan struct{}),
		running:  running,
		terminal: stdoutIsTerminal(),
	}
	if h.terminal {
		go h.animate()
	}
	return h
}

// animate redraws the spinner every 100ms while it is running. Frames are
// written under the mutex so they never interleave with a Pause or Stop clear.
func (h *Handle) animate() {
	i := 0
	for {
		select {
		case <-h.stop:
			return
		default:
		}
		h.mu.Lock()
		if h.running {
			fmt.Printf("\r  %s working ", spinnerFrames[i%len(spinnerFrames)])
			i++
		}
		h.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
}

// Pause hides the spinner so other output can be written without being
// overwritten by the animation. Resume brings it back.
func (h *Handle) Pause() {
	h.mu.Lock()
	h.running = false
	if h.terminal {
		fmt.Print("\r\033[K")
	}
	h.mu.Unlock()
}

// Resume shows the spinner again after a Pause.
func (h *Handle) Resume() {
	h.mu.Lock()
	h.running = true
	h.mu.Unlock()
}

// Stop stops the spinner and clears its line.
func (h *Handle) Stop() {
	h.mu.Lock()
	if h.stopped {
		h.mu.Unlock()
		return
	}
	h.stopped = true
	close(h.stop)
	if h.terminal {
		fmt.Print("\r\033[K")
	}
	h.mu.Unlock()
}

// stdoutIsTerminal reports whether stdout is a character device, i.e. a real
// terminal rather than a pipe or file.
func stdoutIsTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
