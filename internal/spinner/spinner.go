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

// Start animates a working indicator on stdout and returns a function that
// stops and clears it. It is a no-op when stdout is not a terminal.
func Start() func() {
	if !stdoutIsTerminal() {
		return func() {}
	}
	stop := make(chan struct{})
	var once sync.Once
	go func() {
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
			}
			fmt.Printf("\r  %s working ", spinnerFrames[i%len(spinnerFrames)])
			i++
			time.Sleep(100 * time.Millisecond)
		}
	}()
	return func() {
		once.Do(func() {
			close(stop)
			fmt.Print("\r\033[K")
		})
	}
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
