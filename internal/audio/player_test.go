package audio

import (
	"errors"
	"io"
	"math"
	"sync/atomic"
	"testing"
	"time"
)

// framesTrack builds a track exactly n frames long.
func framesTrack(n int) *Track {
	samples := make([]int16, n*frameLength)
	for i := range samples {
		samples[i] = int16(i % 1000)
	}
	return NewTrack(samples)
}

func newTestPlayer(t *testing.T, track *Track, loops int, finished *atomic.Int32) *Player {
	t.Helper()
	player, err := NewPlayer(track, loops, 100, func() { finished.Add(1) })
	if err != nil {
		t.Fatalf("NewPlayer: %v", err)
	}
	return player
}

func countFramesUntilEOF(t *testing.T, player *Player, limit int) int {
	t.Helper()
	for frames := 0; frames < limit; frames++ {
		data, err := player.ProvideOpusFrame()
		if errors.Is(err, io.EOF) {
			return frames
		}
		if err != nil {
			t.Fatalf("ProvideOpusFrame: %v", err)
		}
		if len(data) == 0 {
			t.Fatalf("frame %d is empty", frames)
		}
	}
	t.Fatalf("no EOF after %d frames", limit)
	return 0
}

func waitForFinish(t *testing.T, finished *atomic.Int32) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for finished.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := finished.Load(); got != 1 {
		t.Fatalf("onFinish called %d times, want 1", got)
	}
}

func TestPlayerStopsAfterRequestedLoops(t *testing.T) {
	var finished atomic.Int32
	player := newTestPlayer(t, framesTrack(3), 2, &finished)

	if got := countFramesUntilEOF(t, player, 100); got != 6 {
		t.Fatalf("played %d frames, want 6", got)
	}
	waitForFinish(t, &finished)
}

func TestPlayerPadsPartialLastFrame(t *testing.T) {
	var finished atomic.Int32
	track := NewTrack(make([]int16, frameLength+10))
	player := newTestPlayer(t, track, 1, &finished)

	if got := countFramesUntilEOF(t, player, 100); got != 2 {
		t.Fatalf("played %d frames, want 2", got)
	}
}

func TestPlayerLoopsForeverWithoutLimit(t *testing.T) {
	var finished atomic.Int32
	player := newTestPlayer(t, framesTrack(2), 0, &finished)

	for range 50 {
		if _, err := player.ProvideOpusFrame(); err != nil {
			t.Fatalf("ProvideOpusFrame: %v", err)
		}
	}
	if finished.Load() != 0 {
		t.Fatal("infinite player finished")
	}
}

func TestPlayerStopAfterCurrentLoopFinishesTheRepetition(t *testing.T) {
	var finished atomic.Int32
	player := newTestPlayer(t, framesTrack(4), 0, &finished)

	for range 5 {
		if _, err := player.ProvideOpusFrame(); err != nil {
			t.Fatalf("ProvideOpusFrame: %v", err)
		}
	}
	player.StopAfterCurrentLoop()

	if got := countFramesUntilEOF(t, player, 100); got != 3 {
		t.Fatalf("played %d more frames, want 3 to finish the second loop", got)
	}
	waitForFinish(t, &finished)
}

func TestNewPlayerRejectsEmptyTrack(t *testing.T) {
	if _, err := NewPlayer(NewTrack(nil), 1, 100, nil); err == nil {
		t.Fatal("expected error for empty track")
	}
}

func TestApplyGainScalesAndClamps(t *testing.T) {
	samples := []int16{1000, -1000, 30000, -30000}
	applyGain(samples, 2)

	want := []int16{2000, -2000, math.MaxInt16, math.MinInt16}
	for i := range want {
		if samples[i] != want[i] {
			t.Fatalf("sample %d = %d, want %d", i, samples[i], want[i])
		}
	}
}

func TestApplyGainZeroMutes(t *testing.T) {
	samples := []int16{1234, -4321}
	applyGain(samples, 0)
	if samples[0] != 0 || samples[1] != 0 {
		t.Fatalf("samples = %v, want zeros", samples)
	}
}
