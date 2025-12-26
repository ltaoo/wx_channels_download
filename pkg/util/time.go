package util

import (
	"strconv"
	"time"
)

// NowSeconds returns the current time in seconds.
func NowSeconds() int {
	return int(time.Now().Unix())
}
func NowSecondsStr() string {
	return strconv.Itoa(int(time.Now().Unix()))
}
func NowMillisStr() string {
	return strconv.Itoa(int(time.Now().UnixMilli()))
}

func TimeToSeconds(t time.Time) string {
	return strconv.Itoa(int(t.Unix()))
}

// TimeToMillis returns the timestamp in milliseconds for the given time.
func TimeToMillis(t time.Time) string {
	return strconv.Itoa(int(t.UnixMilli()))
}
