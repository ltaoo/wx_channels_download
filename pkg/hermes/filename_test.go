package hermes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"wx_channel/pkg/testui/assert"
	"wx_channel/pkg/testui/require"
)

func TestResolveDuplicateFilenameAppendsSuffixBeforeExtension(t *testing.T) {
	basePath := t.TempDir()
	engine := &HermesEngine{cfg: HermesEngineConfig{BasePath: basePath}}

	for _, name := range []string{"clip.mp4", "clip(1).mp4"} {
		err := os.WriteFile(filepath.Join(basePath, name), nil, 0600)
		require.NoError(t, err)
	}

	name := engine.resolveDuplicateFilename("", "clip", ".mp4")
	assert.Equal(t, "clip(2).mp4", name)
}

func TestFilenameProcessorSanitizeFilename(t *testing.T) {
	processor := NewFilenameProcessor("", nil)

	name, err := processor.SanitizeFilename(`  report<>:"/\\|?*.mp4. `)
	require.NoError(t, err)
	assert.Equal(t, "report.mp4", name)

	name, err = processor.SanitizeFilename("con.txt")
	require.NoError(t, err)
	assert.Equal(t, "con.txt_", name)

	name, err = processor.SanitizeFilename(strings.Repeat("é", 100))
	require.NoError(t, err)
	assert.LessOrEqual(t, len(name), 235)
	assert.True(t, utf8.ValidString(name))
}

func TestFilenameProcessorProcessFilenameReservesRelativePaths(t *testing.T) {
	processor := NewFilenameProcessor("/downloads", nil)

	name, dir, err := processor.ProcessFilename("album/clip?.mp4")
	require.NoError(t, err)
	assert.Equal(t, "clip.mp4", name)
	assert.Equal(t, "album", dir)

	name, dir, err = processor.ProcessFilename("album/clip?.mp4")
	require.NoError(t, err)
	assert.Equal(t, "clip(1).mp4", name)
	assert.Equal(t, "album", dir)

	processor.RemoveFilename("clip.mp4", "album")
	name, _, err = processor.ProcessFilename("album/clip.mp4")
	require.NoError(t, err)
	assert.Equal(t, "clip.mp4", name)
}

func TestFilenameProcessorAppendExtensionRespectsFilenameLimit(t *testing.T) {
	processor := NewFilenameProcessor("", nil)
	for _, length := range []int{100, 200, 300} {
		t.Run(fmt.Sprintf("%d_characters", length), func(t *testing.T) {
			name, err := processor.AppendExtension(strings.Repeat("a", length), ".png")
			require.NoError(t, err)
			assert.True(t, strings.HasSuffix(name, ".png"))
			assert.LessOrEqual(t, len(name), 235)
			assert.True(t, utf8.ValidString(name))
			if length <= 200 {
				assert.Equal(t, length+len(".png"), len(name))
			} else {
				assert.Equal(t, 235, len(name))
			}
		})
	}
}

func TestFilenameProcessorAppendExtensionWithSubdirectory(t *testing.T) {
	processor := NewFilenameProcessor("", nil)
	name, err := processor.AppendExtension("AuthorName/VideoTitle_WT111", ".mp4")
	require.NoError(t, err)
	assert.Equal(t, "AuthorName/VideoTitle_WT111.mp4", name)
}

func TestFilenameProcessorAppendExtensionWithSubdirectoryLongName(t *testing.T) {
	processor := NewFilenameProcessor("", nil)
	longBase := strings.Repeat("a", 250)
	name, err := processor.AppendExtension("dir/"+longBase, ".mp4")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(name, "dir/"))
	assert.True(t, strings.HasSuffix(name, ".mp4"))
	assert.LessOrEqual(t, len(name), len("dir/")+235)
}

func TestProcessFilenamePreservesInputAndDeduplicates(t *testing.T) {
	items := []map[string]string{
		{"id": "1", "name": "same.mp4"},
		{"id": "2", "name": "same.mp4"},
	}

	got, err := ProcessFilename(nil, items, "/downloads")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "same.mp4", got[0]["name"])
	assert.Equal(t, "same(1).mp4", got[1]["name"])
	assert.Equal(t, "same.mp4", got[0]["full_path"])
	assert.Equal(t, "same(1).mp4", got[1]["full_path"])
	assert.Equal(t, "same.mp4", items[0]["name"])
	assert.Equal(t, "same.mp4", items[1]["name"])
}
