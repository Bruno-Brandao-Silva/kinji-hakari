package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
)

const (
	SampleRate = 48000
	Channels   = 2
)

// Track holds decoded interleaved 16-bit PCM at SampleRate with Channels channels.
type Track struct {
	samples []int16
}

// NewTrack wraps already decoded interleaved PCM samples.
func NewTrack(samples []int16) *Track {
	return &Track{samples: samples}
}

// DecodeFile decodes an audio file with ffmpeg once, so playback never spawns processes.
func DecodeFile(ctx context.Context, path string) (*Track, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-i", path,
		"-f", "s16le", "-ar", strconv.Itoa(SampleRate), "-ac", strconv.Itoa(Channels), "pipe:1")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("decode %s: %w: %s", path, err, stderr.String())
	}
	if stdout.Len() == 0 {
		return nil, errors.New("decoded audio is empty")
	}

	samples := make([]int16, stdout.Len()/2)
	if err := binary.Read(&stdout, binary.LittleEndian, samples); err != nil {
		return nil, fmt.Errorf("read decoded pcm: %w", err)
	}
	return NewTrack(samples), nil
}

// Len returns the number of interleaved samples in the track.
func (t *Track) Len() int {
	return len(t.samples)
}
