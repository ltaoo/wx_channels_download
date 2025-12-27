package assets

import (
	_ "embed"
	"bytes"
	"io"
	"sync"
	"time"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

//go:embed done.wav
var doneWav []byte

var (
	lastPlayed  time.Time
	mu          sync.Mutex
	speakerOnce sync.Once
)

// PlayDoneAudio plays the done.mp3 sound effect.
// It throttles playback to at most once per second.
func PlayDoneAudio() error {
	mu.Lock()
	if time.Since(lastPlayed) < time.Second {
		mu.Unlock()
		return nil
	}
	lastPlayed = time.Now()
	mu.Unlock()

	reader := io.NopCloser(bytes.NewReader(doneWav))
	streamer, format, err := wav.Decode(reader)
	if err != nil {
		return err
	}
	var initErr error
	speakerOnce.Do(func() {
		initErr = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	})
	if initErr != nil {
		return initErr
	}
	done := make(chan struct{})
	speaker.Play(beep.Seq(streamer, beep.Callback(func() { close(done) })))
	<-done
	return nil
}
