// Media fixtures adapted from dramadl; see ../../../../pkg/scraper/NOTICE and LICENSE.
package vod

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"wx_channel/pkg/scraper"

	"wx_channel/pkg/hermes"
)

func fixture_address(address string) error {
	link, err := url.Parse(address)
	if err != nil || link.User != nil || link.Fragment != "" || link.Scheme != "https" || (link.Hostname() != "youku.com" && !strings.HasSuffix(link.Hostname(), ".youku.com")) {
		return fmt.Errorf("invalid fixture media address")
	}
	return nil
}
func media_fixture(name string) string {
	return filepath.Join("testdata", name)
}
func require_media_tools(t *testing.T) {
	t.Helper()
	for _, program := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(program); err != nil {
			t.Skip("requires " + program)
		}
	}
}

type fixture_transport map[string]string

func (f fixture_transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	body, ok := f[req.URL.Path]
	status := http.StatusOK
	if !ok {
		status = http.StatusNotFound
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Header: make(http.Header), Request: req}, nil
}
func fixture_client(files map[string]string) *http.Client {
	return &http.Client{Transport: fixture_transport(files)}
}

func make_track_fixture(t *testing.T, track string) map[string]string {
	t.Helper()
	directory := t.TempDir()
	args := []string{"-nostdin", "-v", "error", "-i", media_fixture("tiny.mp4"), "-map", "0:" + track + ":0", "-c", "copy", "-hls_time", "2", "-hls_playlist_type", "vod", "-hls_segment_type", "fmp4", "-hls_fmp4_init_filename", "init.mp4", "-hls_segment_filename", filepath.Join(directory, "part-%d.m4s"), filepath.Join(directory, "index.m3u8")}
	if output, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("track fixture: %v %s", err, output)
	}
	entries, _ := os.ReadDir(directory)
	files := map[string]string{}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = string(data)
	}
	return files
}

func TestSeparateFMP4DiscontinuitiesKeepFullTimeline(t *testing.T) {
	require_media_tools(t)
	video_files, audio_files := make_track_fixture(t, "v"), make_track_fixture(t, "a")
	files := map[string]string{}
	manifests := map[string]string{}
	for _, track := range []struct {
		name string
		data map[string]string
	}{{"video", video_files}, {"audio", audio_files}} {
		init_file := filepath.Join(t.TempDir(), "init.mp4")
		if err := os.WriteFile(init_file, []byte(track.data["init.mp4"]), 0600); err != nil {
			t.Fatal(err)
		}
		output, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=time_base", "-of", "csv=p=0", init_file).Output()
		if err != nil {
			t.Fatal(err)
		}
		_, denominator, ok := strings.Cut(strings.TrimSpace(string(output)), "/")
		scale, err := strconv.ParseUint(denominator, 10, 64)
		if !ok || err != nil {
			t.Fatal("invalid fixture time base")
		}
		files["/"+track.name+"/init.mp4"] = track.data["init.mp4"]
		manifest := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:2\n"
		for period := 0; period < 3; period++ {
			fragment := append([]byte(nil), []byte(track.data["part-0.m4s"])...)
			index := bytes.Index(fragment, []byte("tfdt"))
			if index < 0 {
				t.Fatal("fixture missing tfdt")
			}
			offset := uint64(period) * 28 * scale / 10
			if fragment[index+4] == 1 {
				binary.BigEndian.PutUint64(fragment[index+8:], binary.BigEndian.Uint64(fragment[index+8:])+offset)
			} else {
				binary.BigEndian.PutUint32(fragment[index+8:], binary.BigEndian.Uint32(fragment[index+8:])+uint32(offset))
			}
			path := "/" + track.name + "/part-" + strconv.Itoa(period) + ".m4s"
			files[path] = string(fragment)
			if period > 0 {
				manifest += "#EXT-X-DISCONTINUITY\n"
			}
			manifest += "#EXT-X-MAP:URI=\"https://pl-ali.youku.com/" + track.name + "/init.mp4\"\n#EXTINF:2,\nhttps://pl-ali.youku.com" + path + "\n"
		}
		manifests[track.name] = manifest + "#EXT-X-ENDLIST\n"
	}
	filename := filepath.Join(t.TempDir(), "complete.mp4")
	source := scraper.Video{Playlist: manifests["video"], AudioPlaylist: manifests["audio"], Info: scraper.PlayInfo{Width: 32, Height: 32, Duration: 6}}
	job := raw_fixture_job(t, source, files, filename)
	if err := PostprocessMediaResources(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	resource := &job.Resources[0]
	if resource.Extra["audio_merged"] != "true" {
		t.Fatal("audio merge metadata missing")
	}
	if err := validate_media(context.Background(), filename, scraper.PlayInfo{Width: 32, Height: 32, Duration: 6, SeparateAudio: true}); err != nil {
		t.Fatal(err)
	}
}

func TestAdapterMergesRawHLSAndDecryptsAES128(t *testing.T) {
	require_media_tools(t)
	segment, err := os.ReadFile(media_fixture("tiny.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, encrypted := range []bool{false, true} {
		t.Run(strconv.FormatBool(encrypted), func(t *testing.T) {
			files := map[string]string{"/one.ts": string(segment)}
			playlist := "#EXTM3U\n#EXT-X-TARGETDURATION:2\n"
			if encrypted {
				key := []byte("0123456789abcdef")
				block, _ := aes.NewCipher(key)
				padding := aes.BlockSize - len(segment)%aes.BlockSize
				input := append(append([]byte(nil), segment...), bytes.Repeat([]byte{byte(padding)}, padding)...)
				output := make([]byte, len(input))
				cipher.NewCBCEncrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(output, input)
				files["/one.ts"], files["/key"] = string(output), string(key)
				playlist += "#EXT-X-KEY:METHOD=AES-128,URI=\"https://pl-ali.youku.com/key\",IV=0x00000000000000000000000000000000\n"
			}
			playlist += "#EXTINF:2,\nhttps://pl-ali.youku.com/one.ts\n#EXT-X-ENDLIST\n"
			filename := filepath.Join(t.TempDir(), "video.mp4")
			source := scraper.Video{Playlist: playlist, Info: scraper.PlayInfo{Width: 32, Height: 32, Duration: 2}}
			job := raw_fixture_job(t, source, files, filename)
			resource := &job.Resources[0]
			raw, _ := os.ReadFile(filename)
			if err := PostprocessMediaResources(context.Background(), job); err != nil {
				t.Fatal(err)
			}
			final, _ := os.ReadFile(filename)
			if string(final[4:8]) != "ftyp" || resource.Size != int64(len(final)) || resource.Downloaded != resource.Size || resource.Extra["postprocessed"] != "true" {
				t.Fatal("final MP4 or resource facts missing")
			}
			// Successful retries keep the finished file intact.
			if err := PostprocessMediaResources(context.Background(), job); err != nil {
				t.Fatal(err)
			}
			retried, _ := os.ReadFile(filename)
			if !bytes.Equal(final, retried) {
				t.Fatal("retry changed finalized output")
			}
			// The durable MP4 marker also recovers a crash before metadata commit.
			resource.Size = int64(len(raw))
			delete(resource.Extra, "postprocessed")
			if err := PostprocessMediaResources(context.Background(), job); err != nil || resource.Size != int64(len(final)) {
				t.Fatalf("retry did not refresh final size: %v", err)
			}

		})
	}
}

func TestHEVCPlaybackMetadataProducesCompatibleMP4Tag(t *testing.T) {
	require_media_tools(t)
	encoders, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoders, []byte("libx265")) {
		t.Skip("requires libx265 fixture encoder")
	}
	path := filepath.Join(t.TempDir(), "hevc.ts")
	if output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=64x64:rate=5:duration=2", "-f", "lavfi", "-i", "sine=duration=2", "-c:v", "libx265", "-x265-params", "pools=1:frame-threads=1", "-c:a", "aac", "-shortest", "-f", "mpegts", path).CombinedOutput(); err != nil {
		t.Fatalf("HEVC fixture generation: %v %s", err, output)
	}
	segment, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(t.TempDir(), "video.mp4")
	source := scraper.Video{Playlist: "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\nhttps://pl-ali.youku.com/hevc.ts\n#EXT-X-ENDLIST\n", Info: scraper.PlayInfo{Codec: "hevc", Width: 64, Height: 64, Duration: 2}}
	job := raw_fixture_job(t, source, map[string]string{"/hevc.ts": string(segment)}, filename)
	if err := PostprocessMediaResources(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_name,codec_tag_string", "-of", "json", filename).Output()
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Streams []struct {
			Codec string `json:"codec_name"`
			Tag   string `json:"codec_tag_string"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &metadata); err != nil || len(metadata.Streams) != 1 || metadata.Streams[0].Codec != "hevc" || metadata.Streams[0].Tag != "hvc1" {
		t.Fatalf("HEVC output lacks a compatible sample entry: %s %v", output, err)
	}
}

func TestAdapterDecryptsCENCAndRetainsRawFilesOnFailure(t *testing.T) {
	require_media_tools(t)
	key_hex := "0123456789abcdef0123456789abcdef"
	key, _ := hex.DecodeString(key_hex)
	encrypted_path := filepath.Join(t.TempDir(), "encrypted.mp4")
	if output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-i", media_fixture("tiny.mp4"), "-c", "copy", "-encryption_scheme", "cenc-aes-ctr", "-encryption_key", key_hex, "-encryption_kid", "00000000000000000000000000000001", encrypted_path).CombinedOutput(); err != nil {
		t.Fatalf("fixture encryption: %v %s", err, output)
	}
	encrypted, _ := os.ReadFile(encrypted_path)
	for _, valid_key := range []bool{true, false} {
		t.Run(strconv.FormatBool(valid_key), func(t *testing.T) {
			content_key := key
			if !valid_key {
				content_key = make([]byte, 16)
			}
			source := scraper.Video{Info: scraper.PlayInfo{Address: "https://pl-ali.youku.com/video.mp4", Key: content_key, Width: 32, Height: 32, Duration: 2}}
			filename := filepath.Join(t.TempDir(), "video.mp4")
			job := raw_fixture_job(t, source, map[string]string{"/video.mp4": string(encrypted)}, filename)
			original, _ := os.ReadFile(filename)
			err := PostprocessMediaResources(context.Background(), job)
			if valid_key {
				if err != nil {
					t.Fatal(err)
				}
				if err := validate_media(context.Background(), filename, source.Info); err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("wrong decryption key accepted")
				}
				remaining, _ := os.ReadFile(filename)
				if !bytes.Equal(original, remaining) {
					t.Fatal("failed postprocess replaced raw media")
				}
				entries, _ := os.ReadDir(filepath.Dir(filename))
				if len(entries) != 1 {
					t.Fatal("postprocess left temporary files")
				}
			}
		})
	}
}

func TestAdapterMissingToolsAndCancellationKeepRawResources(t *testing.T) {
	data, err := os.ReadFile(media_fixture("tiny.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(t.TempDir(), "input.mp4")
	source := scraper.Video{Info: scraper.PlayInfo{Address: "https://pl-ali.youku.com/video.mp4", Width: 32, Height: 32, Duration: 2}}
	t.Setenv("PATH", t.TempDir())
	job := raw_fixture_job(t, source, map[string]string{"/video.mp4": string(data)}, filename)
	original, _ := os.ReadFile(filename)
	if err := PostprocessMediaResources(context.Background(), job); err == nil || !strings.Contains(err.Error(), "ffmpeg") {
		t.Fatalf("missing tool should fail postprocess: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := PostprocessMediaResources(ctx, job); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	remaining, _ := os.ReadFile(filename)
	if !bytes.Equal(original, remaining) {
		t.Fatal("failure replaced original resource")
	}
}

func TestAdapterRejectsUnsafeResourcePaths(t *testing.T) {
	for _, name := range []string{"../outside", "/tmp/outside", "video/../../outside", "video/symlink.ts"} {
		plan := media_plan{Version: 1, Fingerprint: "fixture", Info: scraper.PlayInfo{Width: 32, Height: 32, Duration: 2}, Items: []plan_item{{UniqueID: "episode", Path: name}}}
		if _, err := parse_plan(string_marshal(plan)); err == nil {
			t.Fatalf("unsafe resource path accepted: %s", name)
		}
	}
	directory := t.TempDir()
	manifest := filepath.Join(directory, "playlist.m3u8")
	for _, uri := range []string{"https://evil.example/segment.ts", "../resource-00000.ts", "file:/etc/passwd"} {
		if err := os.WriteFile(manifest, []byte("#EXTM3U\n#EXTINF:2,\n"+uri+"\n#EXT-X-ENDLIST\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := validate_local_manifest(manifest); err == nil {
			t.Fatalf("external URI accepted: %s", uri)
		}
	}
}

func TestPlanDoesNotRetainSignedSourceAddress(t *testing.T) {
	source := scraper.Video{Info: scraper.PlayInfo{Address: "https://pl-ali.youku.com/video.mp4?token=private", Width: 32, Height: 32, Duration: 2}}
	source.CheckMediaAddress = fixture_address
	plan, err := plan_source("episode", source)
	if err != nil {
		t.Fatal(err)
	}
	metadata, _ := json.Marshal(plan)
	if bytes.Contains(metadata, []byte("private")) {
		t.Fatal("signed source address retained in postprocess plan")
	}
}

func TestPlanRequiresSourceAddressPolicy(t *testing.T) {
	source := scraper.Video{Info: scraper.PlayInfo{Address: "https://pl-ali.youku.com/video.mp4", Width: 32, Height: 32, Duration: 2}}
	if _, err := plan_source("episode", source); err == nil || !strings.Contains(err.Error(), "地址校验") {
		t.Fatalf("unvalidated source accepted: %v", err)
	}
}

func raw_fixture_job(t *testing.T, source scraper.Video, files map[string]string, filename string) *hermes.TaskJob {
	t.Helper()
	source.CheckMediaAddress = fixture_address
	plan, err := plan_source("episode", source)
	if err != nil {
		t.Fatal(err)
	}
	job := &hermes.TaskJob{}
	for index, item := range plan.Items {
		path := filename
		extra := map[string]string{"vod_plan": string_marshal(plan)}
		if index > 0 {
			path = filepath.Join(filepath.Dir(filename), item.UniqueID)
			extra = map[string]string{"vod_owner": "episode"}
		}
		link, _ := url.Parse(item.Address)
		data, ok := files[link.Path]
		if !ok {
			t.Fatalf("missing source fixture: %s", item.Address)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		job.Resources = append(job.Resources, hermes.ResourceJob{ID: index + 1, UniqueID: item.UniqueID, Kind: "video/mp4", FilePath: path, Extra: extra})
	}
	return job
}
