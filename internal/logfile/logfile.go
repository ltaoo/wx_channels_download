// Package logfile owns the application log file: a size-rotated writer so a
// busy instance can never grow the log without bound.
package logfile

import (
	"errors"
	"os"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// MaxSizeMB is the size at which the live log is rotated.
	MaxSizeMB = 50
	// MaxBackups is the number of rotated logs kept on disk.
	MaxBackups = 3
)

// Writer is a size-rotated log file. It is an io.Writer, so it can be handed
// straight to zerolog.
type Writer struct {
	*lumberjack.Logger
	mu sync.Mutex
}

// Open returns the writer for path. The file itself is created on first write.
func Open(path string) *Writer {
	return &Writer{Logger: &lumberjack.Logger{
		Filename:   path,
		MaxSize:    MaxSizeMB,
		MaxBackups: MaxBackups,
		Compress:   true,
	}}
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Logger.Write(p)
}

// Truncate empties the live log file. The underlying writer reopens the file on
// the next write and re-reads its size, so rotation accounting stays correct
// afterwards. It is a no-op when the file does not exist yet.
func (w *Writer) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.Logger.Close(); err != nil {
		return err
	}
	if err := os.Truncate(w.Filename, 0); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
