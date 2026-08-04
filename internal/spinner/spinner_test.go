package spinner

import (
	"testing"
	"unicode/utf8"
)

func TestStart(t *testing.T) {
	stop := Start()
	stop()
}

func TestStartPausedStartsHidden(t *testing.T) {
	h := StartPaused()
	h.Resume()
	h.Pause()
	h.Stop()
}

func TestHandleStopIsIdempotent(t *testing.T) {
	h := StartPaused()
	h.Stop()
	h.Stop()
}

func TestSpinnerFrames_singleWidth(t *testing.T) {
	if len(spinnerFrames) < 2 {
		t.Fatalf("expected several frames, got %d", len(spinnerFrames))
	}
	for _, f := range spinnerFrames {
		if utf8.RuneCountInString(f) != 1 {
			t.Errorf("spinner frame %q is not single-width", f)
		}
	}
}
