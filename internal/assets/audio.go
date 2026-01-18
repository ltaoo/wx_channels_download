//go:build !linux
// +build !linux

package assets

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

//go:embed done.wav
var done_wav []byte

var (
	last_played time.Time
	mu          sync.Mutex
	oto_once    sync.Once
	otoCtx      *oto.Context
	player      *oto.Player
	init_err    error
)

var doneVolume = 0.4

type WavMeta struct {
	sample_rate      int
	channels         int
	bites_per_sample int
	data_offset      int
	data_size        int
}

func parse_wav(b []byte) (*WavMeta, error) {
	if len(b) < 44 {
		return nil, errors.New("wav too short")
	}
	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, errors.New("not RIFF/WAVE")
	}
	var meta WavMeta
	off := 12
	for off+8 <= len(b) {
		chunkID := string(b[off : off+4])
		chunkSize := int(binary.LittleEndian.Uint32(b[off+4 : off+8]))
		off += 8
		if off+chunkSize > len(b) {
			return nil, errors.New("invalid chunk size")
		}
		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return nil, errors.New("invalid fmt chunk")
			}
			audioFormat := binary.LittleEndian.Uint16(b[off : off+2])
			if audioFormat != 1 {
				return nil, errors.New("unsupported WAV format (need PCM)")
			}
			meta.channels = int(binary.LittleEndian.Uint16(b[off+2 : off+4]))
			meta.sample_rate = int(binary.LittleEndian.Uint32(b[off+4 : off+8]))
			meta.bites_per_sample = int(binary.LittleEndian.Uint16(b[off+14 : off+16]))
		case "data":
			meta.data_offset = off
			meta.data_size = chunkSize
		}
		off += chunkSize
		if meta.data_offset != 0 && meta.sample_rate != 0 {
			break
		}
	}
	if meta.data_offset == 0 || meta.sample_rate == 0 || meta.channels == 0 || meta.bites_per_sample == 0 {
		return nil, errors.New("missing WAV metadata")
	}
	return &meta, nil
}

func applyVolumeInt16(p []byte, factor float64) {
	if factor <= 0 {
		for i := 0; i+1 < len(p); i += 2 {
			p[i] = 0
			p[i+1] = 0
		}
		return
	}
	if factor == 1 {
		return
	}
	for i := 0; i+1 < len(p); i += 2 {
		sample := int16(binary.LittleEndian.Uint16(p[i:]))
		scaled := int(float64(sample) * factor)
		if scaled > math.MaxInt16 {
			sampled := math.MaxInt16
			binary.LittleEndian.PutUint16(p[i:], uint16(int16(sampled)))
			continue
		}
		if scaled < math.MinInt16 {
			sampled := math.MinInt16
			binary.LittleEndian.PutUint16(p[i:], uint16(int16(sampled)))
			continue
		}
		binary.LittleEndian.PutUint16(p[i:], uint16(int16(scaled)))
	}
}

func PlayDoneAudio() error {
	mu.Lock()
	if time.Since(last_played) < time.Second {
		mu.Unlock()
		return nil
	}
	last_played = time.Now()
	mu.Unlock()

	oto_once.Do(func() {
		meta, err := parse_wav(done_wav)
		if err != nil {
			init_err = err
			return
		}

		op := &oto.NewContextOptions{}
		op.SampleRate = meta.sample_rate
		op.ChannelCount = meta.channels
		if meta.bites_per_sample == 16 {
			op.Format = oto.FormatSignedInt16LE
		} else {
			init_err = errors.New("unsupported bitsPerSample")
			return
		}
		var readyCh <-chan struct{}
		otoCtx, readyCh, init_err = oto.NewContext(op)
		if init_err != nil {
			return
		}
		if readyCh != nil {
			<-readyCh
		}

		start := meta.data_offset
		end := start + meta.data_size
		if end > len(done_wav) {
			end = len(done_wav)
		}
		data := make([]byte, end-start)
		copy(data, done_wav[start:end])
		applyVolumeInt16(data, doneVolume)
		reader := bytes.NewReader(data)
		player = otoCtx.NewPlayer(reader)
	})
	if init_err != nil {
		return init_err
	}

	mu.Lock()
	_, err := player.Seek(0, 0)
	if err != nil {
		mu.Unlock()
		return err
	}
	player.Play()
	mu.Unlock()

	for player.IsPlaying() {
		time.Sleep(time.Millisecond * 10)
	}
	return nil
}
