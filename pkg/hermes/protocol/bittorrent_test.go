package protocol

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTorrentProtocols(t *testing.T) {
	driver, err := NewTorrentDriver(TorrentConfig{DataDir: t.TempDir()})
	require.NoError(t, err)
	defer driver.client.Close()

	protocols := driver.Protocols()
	assert.ElementsMatch(t, []string{"bittorrent", "magnet", "bt", "torrent"}, protocols)
}

func TestParseInfoHashMagnet(t *testing.T) {
	magnet := "magnet:?xt=urn:btih:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, ok := parseInfoHash(magnet)
	assert.True(t, ok)
}

func TestParseInfoHashNonMagnet(t *testing.T) {
	_, ok := parseInfoHash("https://example.com/file.torrent")
	assert.False(t, ok)
}

func TestParseInfoHashInvalidMagnet(t *testing.T) {
	_, ok := parseInfoHash("magnet:?xt=invalid")
	assert.False(t, ok)
}

func TestTorrentDriverClose(t *testing.T) {
	driver, err := NewTorrentDriver(TorrentConfig{DataDir: t.TempDir()})
	require.NoError(t, err)
	errs := driver.client.Close()
	assert.Empty(t, errs)
}

func TestTorrentDriverDefaultDataDir(t *testing.T) {
	driver, err := NewTorrentDriver(TorrentConfig{})
	require.NoError(t, err)
	defer driver.client.Close()
	assert.NotNil(t, driver.client)
	assert.Equal(t, os.TempDir(), driver.config.DataDir)
}
