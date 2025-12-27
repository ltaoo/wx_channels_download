//go:build !cgo
// +build !cgo

package assets

// PlayDoneAudio plays the done.mp3 sound effect.
// It throttles playback to at most once per second.
func PlayDoneAudio() error {
	return nil
}
