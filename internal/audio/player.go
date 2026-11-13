package audio

import (
	"errors"
	"fmt"
	"io"
	"math"
	"sync/atomic"

	"github.com/disgoorg/disgo/voice"
	"layeh.com/gopus"
)

const (
	frameLength       = voice.OpusFrameSize * Channels
	maxOpusFrameBytes = 4000
)

// Player loops a Track as Opus frames. It implements voice.OpusFrameProvider,
// whose sender calls ProvideOpusFrame every 20ms from a single goroutine.
type Player struct {
	track       *Track
	encoder     *gopus.Encoder
	frame       []int16
	gain        float64
	loops       int
	loopsPlayed int
	position    int
	finished    bool
	onFinish    func()

	stopAfterLoop atomic.Bool
}

// NewPlayer creates a player. loops <= 0 means loop forever; volumePercent 100 keeps the original level.
// onFinish runs in its own goroutine once playback ends.
func NewPlayer(track *Track, loops, volumePercent int, onFinish func()) (*Player, error) {
	if track.Len() == 0 {
		return nil, errors.New("track is empty")
	}
	encoder, err := gopus.NewEncoder(SampleRate, Channels, gopus.Audio)
	if err != nil {
		return nil, fmt.Errorf("create opus encoder: %w", err)
	}
	return &Player{
		track:    track,
		encoder:  encoder,
		frame:    make([]int16, frameLength),
		gain:     float64(volumePercent) / 100,
		loops:    loops,
		onFinish: onFinish,
	}, nil
}

// StopAfterCurrentLoop ends playback when the current repetition finishes.
func (p *Player) StopAfterCurrentLoop() {
	p.stopAfterLoop.Store(true)
}

// ProvideOpusFrame returns the next encoded frame, or io.EOF once playback has ended.
func (p *Player) ProvideOpusFrame() ([]byte, error) {
	if p.finished {
		return nil, io.EOF
	}
	p.fillFrame()
	data, err := p.encoder.Encode(p.frame, voice.OpusFrameSize, maxOpusFrameBytes)
	if err != nil {
		return nil, fmt.Errorf("encode opus frame: %w", err)
	}
	p.advance()
	return data, nil
}

// Close is a no-op: the encoder holds no resources beyond memory.
func (p *Player) Close() {}

func (p *Player) fillFrame() {
	copied := copy(p.frame, p.track.samples[p.position:])
	clear(p.frame[copied:])
	applyGain(p.frame, p.gain)
}

func (p *Player) advance() {
	p.position += frameLength
	if p.position < p.track.Len() {
		return
	}
	p.position = 0
	p.loopsPlayed++
	if p.stopAfterLoop.Load() || (p.loops > 0 && p.loopsPlayed >= p.loops) {
		p.finished = true
		if p.onFinish != nil {
			go p.onFinish()
		}
	}
}

func applyGain(samples []int16, gain float64) {
	if gain == 1 {
		return
	}
	for i, sample := range samples {
		scaled := math.Round(float64(sample) * gain)
		samples[i] = int16(max(math.MinInt16, min(math.MaxInt16, scaled)))
	}
}
