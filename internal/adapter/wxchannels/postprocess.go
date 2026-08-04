package wxchannels

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"wx_channel/internal/download/registry"
	"wx_channel/internal/pipeline"
	"wx_channel/pkg/hermes"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

// Postprocess performs wxchannels-specific decrypt and media conversion.
func (h *handler) Postprocess(ctx context.Context, info *hermes.PostprocessInfo, deps registry.PostprocessDeps) error {
	log := func(msg string, args ...interface{}) {
		deps.Logger.Info().Msg(fmt.Sprintf(msg, args...))
	}
	log("Postprocessor.wxchannels: task_id=%d processing %d resources", info.TaskID, len(info.Resources))

	for i := range info.Resources {
		r := &info.Resources[i]
		log("Postprocessor.wxchannels: task_id=%d resource[%d] id=%d name=%q resourceType=%q extra=%v",
			info.TaskID, i, r.ID, r.Name, r.Type, r.Extra)

		if r.Type == "STREAM" {
			if err := postprocessStream(ctx, r, deps.BasePath, log); err != nil {
				return err
			}
			continue
		}

		decodeKey := r.Extra["decode_key"]
		if decodeKey == "" {
			continue
		}
		pc := pipeline.NewContext()
		pc.Values["input_file"] = r.FilePath
		pc.Values["decode_key"] = decodeKey
		p := pipeline.NewBuilder("wxchannels_postprocess").Add("decrypt", DecryptNode).Build()
		if _, err := p.Run(ctx, pc); err != nil {
			return err
		}

		suffix, _ := info.Config["suffix"].(string)
		if suffix != ".mp3" && suffix != "mp3" {
			continue
		}
		pc2 := pipeline.NewContext()
		pc2.Values["decrypted_file"] = r.FilePath
		p2 := pipeline.NewBuilder("wxchannels_convert").Add("convert_mp3", ConvertMP3Node).Build()
		if _, err := p2.Run(ctx, pc2); err != nil {
			return err
		}
		mp3File, _ := pc2.Values["mp3_file"].(string)
		if mp3File == "" {
			continue
		}
		if r.FilePath != mp3File {
			_ = os.Remove(r.FilePath)
		}
		r.TargetExt = ".mp3"
		// Rename from unique_id-based filename to title-based filename
		if title := r.Extra["title"]; title != "" {
			newFile := filepath.Join(filepath.Dir(mp3File), sanitizeBGMName(title)+".mp3")
			if newFile != mp3File {
				if err := os.Rename(mp3File, newFile); err == nil {
					mp3File = newFile
				}
			}
		}
		r.Name = postprocessResourceName(deps.BasePath, mp3File)
		r.FilePath = mp3File
	}

	return nil
}

func postprocessStream(ctx context.Context, r *hermes.PostprocessResource, basePath string, log func(string, ...interface{})) error {
	pc := pipeline.NewContext()
	pc.Values["input_file"] = r.FilePath
	p := pipeline.NewBuilder("wxchannels_stream").Add("stream_convert", StreamConvertNode).Build()
	if _, err := p.Run(ctx, pc); err != nil {
		return err
	}
	mp4File, _ := pc.Values["mp4_file"].(string)
	if mp4File != "" && mp4File != r.FilePath {
		r.TargetExt = ".mp4"
		r.Name = postprocessResourceName(basePath, mp4File)
		r.FilePath = mp4File
		log("Postprocessor.wxchannels: stream output=%s", mp4File)
	}
	return nil
}

func postprocessResourceName(basePath, filePath string) string {
	relPath, _ := filepath.Rel(basePath, filePath)
	if relPath != "" && !filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Base(filePath)
}

// DecryptNode decrypts the downloaded file using the decode key.
// Context values:
//   - "input_file": path to the downloaded file
//   - "decode_key": decrypt key as uint64
//
// Output:
//   - "decrypted_file": path to the decrypted file
var DecryptNode = pipeline.NewFuncNode("decrypt", "decrypt", func(ctx context.Context, pc *pipeline.Context) error {
	inputFile, _ := pc.Values["input_file"].(string)
	if inputFile == "" {
		return fmt.Errorf("缺少 input_file")
	}

	decodeKeyStr, _ := pc.Values["decode_key"].(string)
	if decodeKeyStr == "" {
		if key, ok := pc.Values["decode_key"].(uint64); ok {
			decodeKeyStr = strconv.FormatUint(key, 10)
		}
	}
	key, err := strconv.ParseUint(decodeKeyStr, 10, 64)
	if err != nil {
		return fmt.Errorf("解析 decode_key 失败: %w", err)
	}

	tmpFile := inputFile + ".tmp"
	if err := scraper.DecryptFile(inputFile, tmpFile, key, 131072); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}
	if err := os.Rename(tmpFile, inputFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("原地替换解密文件失败: %w", err)
	}

	pc.Values["decrypted_file"] = inputFile
	return nil
})

// ConvertMP3Node converts the decrypted file to MP3 using ffmpeg.
// Context values:
//   - "decrypted_file": path to the decrypted file
//
// Output:
//   - "mp3_file": path to the MP3 file
var ConvertMP3Node = pipeline.NewFuncNode("convert_mp3", "convert_mp3", func(ctx context.Context, pc *pipeline.Context) error {
	decryptedFile, _ := pc.Values["decrypted_file"].(string)
	if decryptedFile == "" {
		return fmt.Errorf("缺少 decrypted_file")
	}

	baseName := filepath.Base(decryptedFile)
	ext := filepath.Ext(baseName)
	mp3File := filepath.Join(filepath.Dir(decryptedFile), strings.TrimSuffix(baseName, ext)+".mp3")

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", decryptedFile,
		"-vn",
		"-acodec", "libmp3lame",
		"-ab", "192k",
		"-f", "mp3",
		"-y",
		mp3File,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg 转换失败: %w\n%s", err, string(output))
	}

	pc.Values["mp3_file"] = mp3File
	return nil
})

// StreamConvertNode converts a stream file to a playable format.
// MKV and MP4 files pass through unchanged. FLV/TS files are remuxed to MP4.
//
// Context values:
//   - "input_file": original downloaded file path
//
// Output:
//   - "mp4_file": final playable file path (may be the original MKV or converted MP4)
var StreamConvertNode = pipeline.NewFuncNode("stream_convert", "stream_convert", func(ctx context.Context, pc *pipeline.Context) error {
	inputFile, _ := pc.Values["input_file"].(string)
	if inputFile == "" {
		return fmt.Errorf("缺少 input_file")
	}

	ext := strings.ToLower(filepath.Ext(inputFile))
	// MKV and MP4 can play directly, skip conversion
	if ext == ".mp4" || ext == ".mkv" {
		pc.Values["mp4_file"] = inputFile
		return nil
	}

	baseName := filepath.Base(inputFile)
	mp4File := filepath.Join(filepath.Dir(inputFile), strings.TrimSuffix(baseName, ext)+".mp4")

	// ffmpeg remux, not re-encode
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputFile,
		"-c", "copy",
		"-bsf:a", "aac_adtstoasc",
		"-movflags", "+faststart",
		"-y",
		mp4File,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(mp4File)
		return fmt.Errorf("ffmpeg stream remux 失败: %w\n%s", err, string(output))
	}

	// Delete the original file, replace with MP4
	if mp4File != inputFile {
		_ = os.Remove(inputFile)
	}

	pc.Values["mp4_file"] = mp4File
	return nil
})
