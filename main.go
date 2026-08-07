package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"wx_channel/cmd"
)

var AppVer = "26072315"
var Mode = "debug"

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	log_output := io.Writer(os.Stderr)
	log_filepath := filepath.Join(os.TempDir(), "wx_channels_download", "app.log")
	if err := os.MkdirAll(filepath.Dir(log_filepath), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
	} else if log_file, err := os.OpenFile(log_filepath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log file: %v\n", err)
	} else {
		defer log_file.Close()
		log_output = zerolog.MultiLevelWriter(os.Stderr, log_file)
	}

	logger := zerolog.New(log_output).With().Timestamp().Logger()
	log.Logger = logger
	log.Info().Str("version", AppVer).Str("mode", Mode).Str("log_path", log_filepath).Msg("application entry")

	if Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	if err := cmd.Execute(AppVer, Mode, os.Args[1:], &logger); err != nil {
		log.Error().Err(err).Msg("command failed")
		fmt.Printf("Failed to run: %v\n", err.Error())
	}
}
